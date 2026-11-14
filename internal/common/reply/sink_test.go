package reply

import (
	"context"
	"errors"
	"testing"

	"altlinux.space/alt-atomic/apm/internal/common/testutil"
)

// recordingSink копит полученные события.
type recordingSink struct {
	events  []EventData
	results []string
}

func (s *recordingSink) Notify(_ context.Context, ev *EventData) {
	s.events = append(s.events, *ev)
}

func (s *recordingSink) TaskResult(_ context.Context, name string, _ interface{}, _ error) {
	s.results = append(s.results, name)
}

func TestSinkReceivesEvents(t *testing.T) {
	reporter := NewReporter(testutil.DefaultAppConfig())
	sink := &recordingSink{}
	reporter.AddSink(sink)

	ctx := context.Background()
	reporter.CreateEventNotification(ctx, StateBefore, WithEventName(EventSystemInstall))
	reporter.CreateEventNotification(ctx, StateAfter, WithEventName(EventSystemInstall))
	reporter.SendTaskResult(ctx, EventSystemInstall, nil, errors.New("boom"))

	if len(sink.events) != 2 {
		t.Fatalf("events = %d, want 2", len(sink.events))
	}
	if sink.events[0].Name != EventSystemInstall || sink.events[0].State != StateBefore {
		t.Errorf("first event = %+v", sink.events[0])
	}
	if len(sink.results) != 1 || sink.results[0] != EventSystemInstall {
		t.Errorf("results = %v", sink.results)
	}
}
