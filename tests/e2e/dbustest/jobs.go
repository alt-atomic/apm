//go:build e2e

package dbustest

import (
	"context"
	"testing"

	"github.com/godbus/dbus/v5"
)

type JobResult struct {
	ID      uint32
	Status  string
	Message string
	JSON    string
}

// DecodeJob decodes the JSON result carried by JobFinished
func DecodeJob[T any](t testing.TB, job JobResult) T {
	t.Helper()
	return DecodeJSON[T](t, job.JSON)
}

// WaitJob subscribes before making the request, starts a job-returning method,
// and waits for the matching JobFinished signal.
func (r *Request) WaitJob(t testing.TB, jobsInterface string) JobResult {
	t.Helper()

	match := []dbus.MatchOption{
		dbus.WithMatchObjectPath(r.client.path),
		dbus.WithMatchInterface(jobsInterface),
		dbus.WithMatchMember("JobFinished"),
	}
	signals := make(chan *dbus.Signal, 16)
	r.client.conn.Signal(signals)
	defer r.client.conn.RemoveSignal(signals)

	ctx, cancel := context.WithTimeout(context.Background(), r.client.timeout)
	defer cancel()
	if err := r.client.conn.AddMatchSignalContext(ctx, match...); err != nil {
		t.Fatalf("subscribe to %s.JobFinished: %v", jobsInterface, err)
	}
	defer func() {
		if err := r.client.conn.RemoveMatchSignal(match...); err != nil {
			t.Errorf("unsubscribe from %s.JobFinished: %v", jobsInterface, err)
		}
	}()

	var jobID uint32
	r.Store(t, &jobID)

	for {
		select {
		case signal := <-signals:
			finished := decodeJobFinished(t, signal)
			if finished.ID == jobID {
				return finished
			}
		case <-ctx.Done():
			t.Fatalf("wait for job %d: %v", jobID, ctx.Err())
			return JobResult{}
		}
	}
}

func decodeJobFinished(t testing.TB, signal *dbus.Signal) JobResult {
	t.Helper()

	if signal == nil {
		t.Fatal("JobFinished signal channel was closed")
	}
	if len(signal.Body) != 4 {
		t.Fatalf("JobFinished has %d body fields, want 4: %#v", len(signal.Body), signal.Body)
	}
	id, ok := signal.Body[0].(uint32)
	if !ok {
		t.Fatalf("JobFinished job has type %T, want uint32", signal.Body[0])
	}
	status, ok := signal.Body[1].(string)
	if !ok {
		t.Fatalf("JobFinished status has type %T, want string", signal.Body[1])
	}
	message, ok := signal.Body[2].(string)
	if !ok {
		t.Fatalf("JobFinished message has type %T, want string", signal.Body[2])
	}
	result, ok := signal.Body[3].(string)
	if !ok {
		t.Fatalf("JobFinished result has type %T, want string", signal.Body[3])
	}
	return JobResult{ID: id, Status: status, Message: message, JSON: result}
}
