package jobs

import (
	"context"
	"errors"
	"reflect"
	"sync"
	"testing"
	"time"

	"altlinux.space/alt-atomic/apm/internal/common/apmerr"
	"altlinux.space/alt-atomic/apm/internal/common/reply"
)

// finishedSignal аргументы пойманного JobFinished.
type finishedSignal struct {
	id        string
	status    string
	errorType string
	message   string
	result    string
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
			id:        values[0].(string),
			status:    values[1].(string),
			errorType: values[2].(string),
			message:   values[3].(string),
			result:    values[4].(string),
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

func newTestRegistry(emit Emitter) *Registry {
	return NewRegistry(context.Background(), ":1.7", emit)
}

func assertNotFound(t *testing.T, err error) {
	t.Helper()
	if apmErr, ok := errors.AsType[apmerr.APMError](err); !ok || apmErr.Type != apmerr.ErrorTypeNotFound {
		t.Fatalf("err = %v, want NOT_FOUND", err)
	}
}

func assertValidation(t *testing.T, err error) {
	t.Helper()
	if apmErr, ok := errors.AsType[apmerr.APMError](err); !ok || apmErr.Type != apmerr.ErrorTypeValidation {
		t.Fatalf("err = %v, want VALIDATION", err)
	}
}

func TestJobLifecycleOK(t *testing.T) {
	col := newCollector()
	reg := newTestRegistry(col.emit)

	id := reg.Start(ResourceNone, "system", "Install", ":1.9", "action", func(ctx context.Context) (string, error) {
		if _, ok := FromContext(ctx); !ok {
			t.Error("job id missing from context")
		}
		return `{"message":"done"}`, nil
	})

	sig := col.waitFinished(t)
	if sig.id != id || sig.status != StateOK || sig.errorType != "" || sig.message != "" {
		t.Errorf("JobFinished = %+v", sig)
	}
	if sig.result != `{"message":"done"}` {
		t.Errorf("result = %v", sig.result)
	}

	// клиент мог узнать id уже после сигнала — результат обязан остаться в реестре
	state, err := reg.Get(id)
	if err != nil {
		t.Fatalf("Get after finish: %v", err)
	}
	if state.State != StateOK || string(state.Result) != `{"message":"done"}` {
		t.Errorf("state = %+v", state)
	}
	if state.Finished == 0 {
		t.Error("finished timestamp is not set")
	}
	if got := len(reg.List()); got != 1 {
		t.Errorf("List() len = %d, want 1", got)
	}
}

func TestJobLifecycleError(t *testing.T) {
	col := newCollector()
	reg := newTestRegistry(col.emit)

	id := reg.Start(ResourceNone, "system", "Install", ":1.9", "action", func(context.Context) (string, error) {
		return "", apmerr.New(apmerr.ErrorTypeApt, errors.New("boom"))
	})

	sig := col.waitFinished(t)
	if sig.status != StateError || sig.message != "boom" {
		t.Errorf("JobFinished = %+v", sig)
	}
	if sig.errorType != apmerr.ErrorTypeApt {
		t.Errorf("errorType = %q, want %q", sig.errorType, apmerr.ErrorTypeApt)
	}
	if sig.result != "{}" {
		t.Errorf("result = %q, want empty object", sig.result)
	}

	state, err := reg.Get(id)
	if err != nil {
		t.Fatalf("Get after finish: %v", err)
	}
	if state.ErrorType != apmerr.ErrorTypeApt || state.Message != "boom" || string(state.Result) != "{}" {
		t.Errorf("state = %+v", state)
	}
}

func TestJobPanicBecomesError(t *testing.T) {
	col := newCollector()
	reg := newTestRegistry(col.emit)

	reg.StartNoCancel(ResourceNone, "system", "Install", ":1.9", func(context.Context) (string, error) {
		panic("boom")
	})

	sig := col.waitFinished(t)
	if sig.status != StateError {
		t.Errorf("status = %s, want error", sig.status)
	}
	if sig.message == "" {
		t.Error("panic message is lost")
	}
}

func TestRunningJobVisible(t *testing.T) {
	col := newCollector()
	reg := newTestRegistry(col.emit)

	started := make(chan struct{})
	id := reg.Start(ResourceNone, "packages", "Install", ":1.9", "action", func(ctx context.Context) (string, error) {
		close(started)
		<-ctx.Done()
		return "", ctx.Err()
	})
	<-started

	d, err := reg.Get(id)
	if err != nil {
		t.Fatal(err)
	}
	if d.State != StateRunning || d.Domain != "packages" || d.Finished != 0 {
		t.Errorf("snapshot = %v", d)
	}
	if got := len(reg.List()); got != 1 {
		t.Errorf("List() len = %d, want 1", got)
	}

	if err = reg.Cancel(id, ":1.9", nil); err != nil {
		t.Fatalf("Cancel by owner: %v", err)
	}
	sig := col.waitFinished(t)
	if sig.status != StateCanceled || sig.errorType != apmerr.ErrorTypeCanceled {
		t.Errorf("JobFinished = %+v, want canceled", sig)
	}
}

func TestJobCancelForeignRequiresAuth(t *testing.T) {
	col := newCollector()
	reg := newTestRegistry(col.emit)

	started := make(chan struct{})
	id := reg.Start(ResourceNone, "system", "Install", ":1.9", "action", func(ctx context.Context) (string, error) {
		close(started)
		<-ctx.Done()
		return "", ctx.Err()
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

func TestCancelFinishedAndUnknownJobs(t *testing.T) {
	col := newCollector()
	reg := newTestRegistry(col.emit)

	id := reg.Start(ResourceNone, "system", "Install", ":1.9", "action", func(context.Context) (string, error) {
		return "{}", nil
	})
	col.waitFinished(t)

	// завершённая задача ещё в реестре, но отменять уже нечего
	assertValidation(t, reg.Cancel(id, ":1.9", nil))

	assertNotFound(t, reg.Cancel(":1.7-404", ":1.9", nil))
	_, err := reg.Get(":1.7-404")
	assertNotFound(t, err)
}

func TestNoCancelJobRejectsCancel(t *testing.T) {
	col := newCollector()
	reg := newTestRegistry(col.emit)

	started := make(chan struct{})
	release := make(chan struct{})
	id := reg.StartNoCancel(ResourceNone, "packages", "Install", ":1.9", func(context.Context) (string, error) {
		close(started)
		<-release
		return "{}", nil
	})
	<-started

	assertValidation(t, reg.Cancel(id, ":1.9", nil))

	d, _ := reg.Get(id)
	if d.Cancellable {
		t.Error("transaction must not be cancellable")
	}

	close(release)
	col.waitFinished(t)
}

func TestFinishedJobsExpire(t *testing.T) {
	col := newCollector()
	reg := newTestRegistry(col.emit)
	reg.retention = 0

	id := reg.Start(ResourceNone, "system", "Update", ":1.9", "action", func(context.Context) (string, error) {
		return "{}", nil
	})
	col.waitFinished(t)

	// следующая операция чистит просроченные записи
	time.Sleep(time.Millisecond)
	if got := len(reg.List()); got != 0 {
		t.Errorf("List() len = %d, want 0", got)
	}
	_, err := reg.Get(id)
	assertNotFound(t, err)
}

func TestJobIDsAreUniqueAndOrdered(t *testing.T) {
	reg := newTestRegistry(func(string, ...any) {})

	seen := make(map[string]bool, 16)
	var ids []string
	for range 16 {
		id := reg.StartNoCancel(ResourceNone, "packages", "Update", ":1.9", func(context.Context) (string, error) {
			return "{}", nil
		})
		if seen[id] {
			t.Fatalf("duplicate job id %q", id)
		}
		seen[id] = true
		ids = append(ids, id)
	}

	list := reg.List()
	if len(list) != len(ids) {
		t.Fatalf("List() len = %d, want %d", len(list), len(ids))
	}
	for i, state := range list {
		if state.ID != ids[i] {
			t.Fatalf("List()[%d] = %s, want %s", i, state.ID, ids[i])
		}
	}
}

func TestJobsWithSameResourceRunInFIFOOrder(t *testing.T) {
	col := newCollector()
	reg := newTestRegistry(col.emit)
	resource := ResourceKey("distrobox", "dev")
	started := make(chan int, 3)
	release := []chan struct{}{make(chan struct{}), make(chan struct{}), make(chan struct{})}

	ids := make([]string, 0, 3)
	for index := range 3 {
		ids = append(ids, reg.StartNoCancel(resource, "distrobox", "Install", ":1.9", func(context.Context) (string, error) {
			started <- index
			<-release[index]
			return "{}", nil
		}))
	}

	if got := <-started; got != 0 {
		t.Fatalf("first started job = %d, want 0", got)
	}
	for _, id := range ids[1:] {
		state, err := reg.Get(id)
		if err != nil {
			t.Fatal(err)
		}
		if state.State != StateQueued {
			t.Fatalf("job %s state = %s, want queued", id, state.State)
		}
	}
	select {
	case got := <-started:
		t.Fatalf("job %d started while the resource was occupied", got)
	default:
	}

	for index := range 3 {
		close(release[index])
		if sig := col.waitFinished(t); sig.id != ids[index] {
			t.Fatalf("finished job = %s, want %s", sig.id, ids[index])
		}
		if index+1 < len(ids) {
			select {
			case got := <-started:
				if got != index+1 {
					t.Fatalf("next started job = %d, want %d", got, index+1)
				}
			case <-time.After(5 * time.Second):
				t.Fatal("next queued job did not start")
			}
		}
	}
	reg.Shutdown()
	if n := reg.queue.Len(); n != 0 {
		t.Fatalf("resource queues left after completion: %d", n)
	}
}

func TestJobsWithDifferentResourcesRunConcurrently(t *testing.T) {
	col := newCollector()
	reg := newTestRegistry(col.emit)
	started := make(chan string, 2)
	release := make(chan struct{})

	for _, container := range []string{"dev", "build"} {
		reg.StartNoCancel(ResourceKey("distrobox", container), "distrobox", "Install", ":1.9", func(context.Context) (string, error) {
			started <- container
			<-release
			return "{}", nil
		})
	}

	seen := make(map[string]bool, 2)
	for range 2 {
		select {
		case container := <-started:
			seen[container] = true
		case <-time.After(5 * time.Second):
			t.Fatal("jobs on different resources did not run concurrently")
		}
	}
	if !seen["dev"] || !seen["build"] {
		t.Fatalf("started containers = %v", seen)
	}

	close(release)
	col.waitFinished(t)
	col.waitFinished(t)
}

func TestQueuedJobCanBeCanceledWithoutRunning(t *testing.T) {
	col := newCollector()
	reg := newTestRegistry(col.emit)
	resource := ResourceHost
	firstStarted := make(chan struct{})
	releaseFirst := make(chan struct{})

	reg.StartNoCancel(resource, "packages", "Install", ":1.9", func(context.Context) (string, error) {
		close(firstStarted)
		<-releaseFirst
		return "{}", nil
	})
	<-firstStarted

	invoked := make(chan struct{})
	id := reg.Start(resource, "image", "Apply", ":1.9", "action", func(context.Context) (string, error) {
		close(invoked)
		return "{}", nil
	})
	state, err := reg.Get(id)
	if err != nil {
		t.Fatal(err)
	}
	if state.State != StateQueued {
		t.Fatalf("state = %s, want queued", state.State)
	}

	if err := reg.Cancel(id, ":1.9", nil); err != nil {
		t.Fatalf("Cancel queued job: %v", err)
	}
	if sig := col.waitFinished(t); sig.id != id || sig.status != StateCanceled {
		t.Fatalf("JobFinished = %+v, want canceled job %s", sig, id)
	}
	select {
	case <-invoked:
		t.Fatal("canceled queued job was invoked")
	default:
	}

	close(releaseFirst)
	col.waitFinished(t)
}

func TestShutdownCancelsQueuedAndWaitsForRunningNoCancel(t *testing.T) {
	col := newCollector()
	reg := newTestRegistry(col.emit)
	runningContext := make(chan context.Context, 1)
	releaseRunning := make(chan struct{})

	firstID := reg.StartNoCancel(ResourceHost, "packages", "Install", ":1.9", func(ctx context.Context) (string, error) {
		runningContext <- ctx
		<-releaseRunning
		return "{}", nil
	})
	ctx := <-runningContext

	queuedInvoked := make(chan struct{})
	secondID := reg.StartNoCancel(ResourceHost, "packages", "Remove", ":1.9", func(context.Context) (string, error) {
		close(queuedInvoked)
		return "{}", nil
	})

	shutdownDone := make(chan struct{})
	go func() {
		reg.Shutdown()
		close(shutdownDone)
	}()

	if sig := col.waitFinished(t); sig.id != secondID || sig.status != StateCanceled {
		t.Fatalf("queued JobFinished = %+v, want canceled job %s", sig, secondID)
	}
	select {
	case <-queuedInvoked:
		t.Fatal("queued non-cancellable job ran during shutdown")
	default:
	}
	select {
	case <-ctx.Done():
		t.Fatal("running non-cancellable job context was canceled")
	default:
	}
	select {
	case <-shutdownDone:
		t.Fatal("Shutdown returned before running non-cancellable job finished")
	default:
	}

	close(releaseRunning)
	if sig := col.waitFinished(t); sig.id != firstID || sig.status != StateOK {
		t.Fatalf("running JobFinished = %+v, want successful job %s", sig, firstID)
	}
	select {
	case <-shutdownDone:
	case <-time.After(5 * time.Second):
		t.Fatal("Shutdown did not return")
	}
}

func TestSinkProgressCarriesMessage(t *testing.T) {
	var got []any
	reg := NewRegistry(context.Background(), ":1.7", func(member string, values ...any) {
		if member == "JobProgress" {
			got = values
		}
	})
	sink := NewSink(reg)

	event := &reply.EventData{
		Name:            "system.Install",
		Type:            reply.EventTypeProgress,
		State:           reply.StateBefore,
		View:            "Installing packages",
		ProgressPercent: 42,
		ProgressDone:    "1/2",
	}

	sink.Notify(context.Background(), event)
	if got != nil {
		t.Fatalf("event outside a job must be dropped, got %v", got)
	}

	sink.Notify(WithJob(context.Background(), ":1.7-1"), event)
	want := []any{":1.7-1", event.Name, event.Type, event.State, event.View, event.ProgressPercent, event.ProgressDone}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("JobProgress = %v, want %v", got, want)
	}
}
