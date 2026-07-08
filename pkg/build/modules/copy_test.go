package modules

import (
	"context"
	"path/filepath"
	"testing"
)

func TestCopyBody_Check(t *testing.T) {
	if err := (&CopyBody{}).Check(); err != nil {
		t.Errorf("Check() = %v, want nil", err)
	}
}

func TestCopyBody_RelativeDestination(t *testing.T) {
	b := &CopyBody{Source: "/tmp/src", Destination: "relative/dst"}
	if _, err := b.Execute(context.Background(), &fakeRuntime{}); err == nil {
		t.Fatal("expected error for relative destination")
	}
}

func TestCopyBody_CopiesFile(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src.txt")
	dst := filepath.Join(dir, "dst.txt")
	writeFile(t, src, "hello")

	b := &CopyBody{Source: src, Destination: dst}
	if _, err := b.Execute(context.Background(), &fakeRuntime{}); err != nil {
		t.Fatalf("Execute() = %v", err)
	}
	if got := mustReadFile(t, dst); got != "hello" {
		t.Errorf("dst content = %q, want %q", got, "hello")
	}
}

func TestCopyBody_ReplaceExisting(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src.txt")
	dst := filepath.Join(dir, "dst.txt")
	writeFile(t, src, "new")
	writeFile(t, dst, "old")

	b := &CopyBody{Source: src, Destination: dst, Replace: true}
	if _, err := b.Execute(context.Background(), &fakeRuntime{}); err != nil {
		t.Fatalf("Execute() = %v", err)
	}
	if got := mustReadFile(t, dst); got != "new" {
		t.Errorf("dst content = %q, want %q", got, "new")
	}
}
