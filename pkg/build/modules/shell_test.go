package modules

import (
	"context"
	"errors"
	"reflect"
	"testing"
)

func TestShellBody_RunsAndCollects(t *testing.T) {
	runner := &fakeRunner{stdout: "some output"}
	rt := &fakeRuntime{runner: runner}

	b := &ShellBody{Command: "echo hi"}
	out, err := b.Execute(context.Background(), rt)
	if err != nil {
		t.Fatalf("Execute() = %v", err)
	}
	// command is passed to the runner verbatim
	if !reflect.DeepEqual(runner.gotArgs, []string{"echo hi"}) {
		t.Errorf("gotArgs = %v, want [echo hi]", runner.gotArgs)
	}
	// stdout is collected when not quiet
	if !reflect.DeepEqual(rt.collected, []string{"some output"}) {
		t.Errorf("collected = %v", rt.collected)
	}
	// env is empty (no output command executed by the fake)
	if env := out.(ShellOutput).Env; len(env) != 0 {
		t.Errorf("Env = %v, want empty", env)
	}
}

func TestShellBody_QuietSkipsCollect(t *testing.T) {
	runner := &fakeRunner{stdout: "hidden"}
	rt := &fakeRuntime{runner: runner}

	b := &ShellBody{Command: "echo hi", Quiet: true}
	if _, err := b.Execute(context.Background(), rt); err != nil {
		t.Fatalf("Execute() = %v", err)
	}
	if len(rt.collected) != 0 {
		t.Errorf("collected = %v, want none in quiet mode", rt.collected)
	}
}

func TestShellBody_RunnerError(t *testing.T) {
	runner := &fakeRunner{err: errors.New("boom")}
	rt := &fakeRuntime{runner: runner}

	b := &ShellBody{Command: "false"}
	if _, err := b.Execute(context.Background(), rt); err == nil {
		t.Fatal("expected runner error to propagate")
	}
}
