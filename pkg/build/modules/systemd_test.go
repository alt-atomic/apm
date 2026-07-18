package modules

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
)

func TestSystemdAction(t *testing.T) {
	cases := []struct {
		masked, enabled bool
		wantAction      string
	}{
		{false, false, "disable"},
		{true, false, "mask"},
		{false, true, "enable"},
		{true, true, "enable"}, // enable wins over mask
	}
	for _, c := range cases {
		if got, _ := systemdAction(c.masked, c.enabled); got != c.wantAction {
			t.Errorf("systemdAction(%v,%v) = %q, want %q", c.masked, c.enabled, got, c.wantAction)
		}
	}
}

func TestSystemctlArgs(t *testing.T) {
	cases := []struct {
		name   string
		action string
		global bool
		want   []string
	}{
		{"plain", "enable", false, []string{"systemctl", "enable", "svc"}},
		{"global", "mask", true, []string{"systemctl", "--global", "mask", "svc"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := systemctlArgs(c.action, c.global, "svc"); !reflect.DeepEqual(got, c.want) {
				t.Errorf("args = %v, want %v", got, c.want)
			}
		})
	}
}

func TestSystemdBody_DispatchesAndCollects(t *testing.T) {
	runner := &fakeRunner{stdout: "Created symlink"}
	rt := &fakeRuntime{runner: runner}

	b := &SystemdBody{Targets: []string{"nginx.service"}, Enabled: true, Global: true}
	if _, err := b.Execute(context.Background(), rt); err != nil {
		t.Fatalf("Execute() = %v", err)
	}
	want := []string{"systemctl", "--global", "enable", "nginx.service"}
	if !reflect.DeepEqual(runner.gotArgs, want) {
		t.Errorf("gotArgs = %v, want %v", runner.gotArgs, want)
	}
	// runner output is accumulated
	if !reflect.DeepEqual(rt.collected, []string{"Created symlink"}) {
		t.Errorf("collected = %v", rt.collected)
	}
}

func TestSystemdBody_EmptyTargets(t *testing.T) {
	runner := &fakeRunner{}
	rt := &fakeRuntime{runner: runner}

	if _, err := (&SystemdBody{}).Execute(context.Background(), rt); err != nil {
		t.Fatalf("Execute() = %v", err)
	}
	// no targets means no systemctl call and nothing collected
	if runner.calls != 0 {
		t.Errorf("runner called %d times, want 0", runner.calls)
	}
	if len(rt.collected) != 0 {
		t.Errorf("collected = %v, want none", rt.collected)
	}
}

func TestSystemdBody_RunnerError(t *testing.T) {
	rt := &fakeRuntime{runner: &fakeRunner{err: errors.New("exit status 1"), stderr: "Unit svc does not exist"}}

	b := &SystemdBody{Targets: []string{"svc"}, Enabled: true}
	_, err := b.Execute(context.Background(), rt)
	if err == nil {
		t.Fatal("expected runner error to propagate")
	}
	// error carries both the exit status and the systemctl stderr reason
	msg := err.Error()
	if !strings.Contains(msg, "exit status 1") || !strings.Contains(msg, "Unit svc does not exist") {
		t.Errorf("error = %q, want exit status and stderr reason", msg)
	}
}
