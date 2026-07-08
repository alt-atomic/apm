package modules

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestMoveBody_RelativePaths(t *testing.T) {
	cases := []struct {
		name string
		body *MoveBody
	}{
		{"relative source", &MoveBody{Source: "rel/src", Destination: "/abs/dst"}},
		{"relative destination", &MoveBody{Source: "/abs/src", Destination: "rel/dst"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if _, err := c.body.Execute(context.Background(), &fakeRuntime{}); err == nil {
				t.Fatal("expected error")
			}
		})
	}
}

func TestMoveBody_MovesFile(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src")
	dst := filepath.Join(dir, "dst")
	writeFile(t, src, "payload")

	b := &MoveBody{Source: src, Destination: dst}
	if _, err := b.Execute(context.Background(), &fakeRuntime{}); err != nil {
		t.Fatalf("Execute() = %v", err)
	}
	if _, err := os.Stat(src); !os.IsNotExist(err) {
		t.Error("source should be gone after move")
	}
	if got := mustReadFile(t, dst); got != "payload" {
		t.Errorf("dst = %q, want %q", got, "payload")
	}
}

func TestMoveBody_ReplaceExisting(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src")
	dst := filepath.Join(dir, "dst")
	writeFile(t, src, "new")
	writeFile(t, dst, "old")

	b := &MoveBody{Source: src, Destination: dst, Replace: true}
	if _, err := b.Execute(context.Background(), &fakeRuntime{}); err != nil {
		t.Fatalf("Execute() = %v", err)
	}
	if got := mustReadFile(t, dst); got != "new" {
		t.Errorf("dst = %q, want %q", got, "new")
	}
}

func TestMoveBody_CreateLink(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src")
	dst := filepath.Join(dir, "dst")
	writeFile(t, src, "payload")

	b := &MoveBody{Source: src, Destination: dst, CreateLink: true}
	if _, err := b.Execute(context.Background(), &fakeRuntime{}); err != nil {
		t.Fatalf("Execute() = %v", err)
	}
	// source position now holds a symlink to destination
	info, err := os.Lstat(src)
	if err != nil {
		t.Fatalf("Lstat: %v", err)
	}
	if info.Mode()&os.ModeSymlink == 0 {
		t.Fatal("source should be a symlink")
	}
	if got := mustReadFile(t, src); got != "payload" {
		t.Errorf("via link = %q, want %q", got, "payload")
	}
}
