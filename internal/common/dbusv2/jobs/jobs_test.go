package jobs

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"altlinux.space/alt-atomic/apm/internal/common/apmerr"
	"altlinux.space/alt-atomic/apm/internal/common/dbusv2/wire"
)

// finishedSignal аргументы пойманного JobFinished.
type finishedSignal struct {
	id      uint32
	status  string
	message string
	result  wire.Dict
}

// signalCollector потокобезопасно копит эмитированные сигналы.
type signalCollector struct {
	mu       sync.Mutex
	members  []string
	finished chan finishedSignal
}

func newCollector() *signalCollector {
	return &signalCollector{finished: make(chan finishedSignal, 8)}
}

func (c *signalCollector) emit(member string, values ...any) {
	c.mu.Lock()
	c.members = append(c.members, member)
	c.mu.Unlock()
	if member == "JobFinished" {
		c.finished <- finishedSignal{
			id:      values[0].(uint32),
			status:  values[1].(string),
			message: values[2].(string),
			result:  values[3].(wire.Dict),
		}
	}
}

func (c *signalCollector) waitFinished(t *testing.T) finishedSignal {
	t.Helper()
	select {
	case sig := <-c.finished:
		return sig
	case <-time.After(5 * time.Second):
		t.Fatal("JobFinished signal not received")
		return finishedSignal{}
	}
}

func assertNotFound(t *testing.T, err error) {
	t.Helper()
	var apmErr apmerr.APMError
	if !errors.As(err, &apmErr) || apmErr.Type != apmerr.ErrorTypeNotFound {
		t.Fatalf("err = %v, want NOT_FOUND", err)
	}
}

func TestJobLifecycleOK(t *testing.T) {
	col := newCollector()
	reg := NewRegistry(context.Background(), col.emit)

	id := reg.Start("system", "Install", ":1.7", "action", func(ctx context.Context) (wire.Dict, error) {
		if _, ok := FromContext(ctx); !ok {
			t.Error("job id missing from context")
		}
		return wire.Dict{"message": wire.V("done")}, nil
	})

	sig := col.waitFinished(t)
	if sig.id != id || sig.status != StateOK {
		t.Errorf("JobFinished = %+v", sig)
	}
	if sig.result["message"].Value().(string) != "done" {
		t.Errorf("result = %v", sig.result)
	}

	// запись удалена в момент завершения
	if _, err := reg.Get(id); err == nil {
		t.Error("finished job must be removed from registry")
	}
	if got := len(reg.List()); got != 0 {
		t.Errorf("List() len = %d, want 0", got)
	}
}

func TestJobLifecycleError(t *testing.T) {
	col := newCollector()
	reg := NewRegistry(context.Background(), col.emit)

	reg.Start("system", "Install", ":1.7", "action", func(context.Context) (wire.Dict, error) {
		return nil, apmerr.New(apmerr.ErrorTypeApt, errors.New("boom"))
	})

	sig := col.waitFinished(t)
	if sig.status != StateError || sig.message != "boom" {
		t.Errorf("JobFinished = %+v", sig)
	}
	if et := sig.result["error_type"].Value().(string); et != apmerr.ErrorTypeApt {
		t.Errorf("error_type = %s, want %s", et, apmerr.ErrorTypeApt)
	}
}

func TestRunningJobVisible(t *testing.T) {
	col := newCollector()
	reg := NewRegistry(context.Background(), col.emit)

	started := make(chan struct{})
	id := reg.Start("packages", "Install", ":1.7", "action", func(ctx context.Context) (wire.Dict, error) {
		close(started)
		<-ctx.Done()
		return nil, ctx.Err()
	})
	<-started

	d, err := reg.Get(id)
	if err != nil {
		t.Fatal(err)
	}
	if d["state"].Value().(string) != StateRunning || d["domain"].Value().(string) != "packages" {
		t.Errorf("snapshot = %v", d)
	}
	if got := len(reg.List()); got != 1 {
		t.Errorf("List() len = %d, want 1", got)
	}

	if err = reg.Cancel(id, ":1.7", nil); err != nil {
		t.Fatalf("Cancel by owner: %v", err)
	}
	if sig := col.waitFinished(t); sig.status != StateCanceled {
		t.Errorf("status = %s, want canceled", sig.status)
	}
}

func TestJobCancelForeignRequiresAuth(t *testing.T) {
	col := newCollector()
	reg := NewRegistry(context.Background(), col.emit)

	started := make(chan struct{})
	id := reg.Start("system", "Install", ":1.7", "action", func(ctx context.Context) (wire.Dict, error) {
		close(started)
		<-ctx.Done()
		return nil, ctx.Err()
	})
	<-started

	denied := apmerr.New(apmerr.ErrorTypePermission, errors.New("denied"))
	err := reg.Cancel(id, ":1.99", func(action string) error {
		if action != "action" {
			t.Errorf("action = %s, want action", action)
		}
		return denied
	})
	if !errors.Is(err, denied) {
		t.Errorf("Cancel err = %v, want denied", err)
	}

	if err = reg.Cancel(id, ":1.99", func(string) error { return nil }); err != nil {
		t.Fatalf("Cancel authorized: %v", err)
	}
	col.waitFinished(t)
}

func TestCancelUnknownJobIsNotFound(t *testing.T) {
	col := newCollector()
	reg := NewRegistry(context.Background(), col.emit)

	reg.Start("system", "Install", ":1.7", "action", func(context.Context) (wire.Dict, error) {
		return wire.Dict{}, nil
	})
	col.waitFinished(t)

	assertNotFound(t, reg.Cancel(1, ":1.7", nil))
	_, err := reg.Get(42)
	assertNotFound(t, err)
}

func TestNoCancelJobRejectsCancel(t *testing.T) {
	col := newCollector()
	reg := NewRegistry(context.Background(), col.emit)

	started := make(chan struct{})
	release := make(chan struct{})
	id := reg.StartNoCancel("packages", "Install", ":1.7", func(context.Context) (wire.Dict, error) {
		close(started)
		<-release
		return wire.Dict{}, nil
	})
	<-started

	err := reg.Cancel(id, ":1.7", nil)
	var apmErr apmerr.APMError
	if !errors.As(err, &apmErr) || apmErr.Type != apmerr.ErrorTypeValidation {
		t.Errorf("Cancel = %v, want VALIDATION", err)
	}

	d, _ := reg.Get(id)
	if d["cancellable"].Value().(bool) {
		t.Error("transaction must not be cancellable")
	}

	close(release)
	col.waitFinished(t)
}
