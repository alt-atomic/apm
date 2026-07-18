package modules

import (
	"context"
	"os"
	"testing"

	"altlinux.space/alt-atomic/apm/pkg/build"
	"altlinux.space/alt-atomic/apm/pkg/command"
)

// fakeRuntime implements build.RuntimeContext for module tests.
type fakeRuntime struct {
	runner    command.Runner
	collected []string
	includeFn func(ctx context.Context, target string) (map[string]*build.MapModule, error)
}

func (f *fakeRuntime) Runner() command.Runner { return f.runner }

func (f *fakeRuntime) CollectOutput(text string) { f.collected = append(f.collected, text) }

func (f *fakeRuntime) EmitEvent(_ context.Context, _, _, _ string) {}

func (f *fakeRuntime) ExecuteInclude(ctx context.Context, target string) (map[string]*build.MapModule, error) {
	if f.includeFn != nil {
		return f.includeFn(ctx, target)
	}
	return map[string]*build.MapModule{}, nil
}

// fakeRunner implements command.Runner: returns preset result and records args.
type fakeRunner struct {
	stdout  string
	stderr  string
	err     error
	gotArgs []string
	calls   int
}

func (r *fakeRunner) Run(_ context.Context, args []string, _ ...command.Option) (string, string, error) {
	r.calls++
	r.gotArgs = args
	return r.stdout, r.stderr, r.err
}

// writeFile writes content to path, fails the test on error.
func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("writeFile(%s): %v", path, err)
	}
}

// mustReadFile reads path, fails the test on error.
func mustReadFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("readFile(%s): %v", path, err)
	}
	return string(data)
}
