package modules

import (
	"context"
	"os"
	"path/filepath"
	"syscall"
	"testing"
)

func TestMkdirBody_RelativeTarget(t *testing.T) {
	b := &MkdirBody{Targets: []string{"rel/dir"}, Perm: "rwxr-xr-x"}
	if _, err := b.Execute(context.Background(), &fakeRuntime{}); err == nil {
		t.Fatal("expected error for relative target")
	}
}

func TestMkdirBody_CreatesDirs(t *testing.T) {
	// zero umask for a deterministic mode assertion
	old := syscall.Umask(0)
	defer syscall.Umask(old)

	dir := t.TempDir()
	a := filepath.Join(dir, "a", "b")
	c := filepath.Join(dir, "c")

	b := &MkdirBody{Targets: []string{a, c}, Perm: "rwxr-xr-x"}
	if _, err := b.Execute(context.Background(), &fakeRuntime{}); err != nil {
		t.Fatalf("Execute() = %v", err)
	}
	for _, p := range []string{a, c} {
		info, err := os.Stat(p)
		if err != nil {
			t.Fatalf("Stat(%s): %v", p, err)
		}
		if !info.IsDir() {
			t.Errorf("%s is not a dir", p)
		}
		if info.Mode().Perm() != 0755 {
			t.Errorf("%s perm = %o, want 0755", p, info.Mode().Perm())
		}
	}
}

func TestMkdirBody_InvalidPerm(t *testing.T) {
	b := &MkdirBody{Targets: []string{t.TempDir() + "/x"}, Perm: "nope"}
	if _, err := b.Execute(context.Background(), &fakeRuntime{}); err == nil {
		t.Fatal("expected error for invalid perm")
	}
}
