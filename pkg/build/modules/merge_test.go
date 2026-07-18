package modules

import (
	"context"
	"os"
	"path/filepath"
	"syscall"
	"testing"
)

func TestMergeBody_RelativeDestination(t *testing.T) {
	b := &MergeBody{Source: "/tmp/src", Destination: "rel/dst"}
	if _, err := b.Execute(context.Background(), &fakeRuntime{}); err == nil {
		t.Fatal("expected error for relative destination")
	}
}

func TestMergeBody_Append(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src")
	dst := filepath.Join(dir, "dst")
	writeFile(t, src, "world")
	writeFile(t, dst, "hello ")

	b := &MergeBody{Source: src, Destination: dst}
	if _, err := b.Execute(context.Background(), &fakeRuntime{}); err != nil {
		t.Fatalf("Execute() = %v", err)
	}
	if got := mustReadFile(t, dst); got != "hello world" {
		t.Errorf("dst = %q, want %q", got, "hello world")
	}
}

func TestMergeBody_Prepend(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src")
	dst := filepath.Join(dir, "dst")
	writeFile(t, src, "head ")
	writeFile(t, dst, "tail")

	b := &MergeBody{Source: src, Destination: dst, Prepend: true}
	if _, err := b.Execute(context.Background(), &fakeRuntime{}); err != nil {
		t.Fatalf("Execute() = %v", err)
	}
	if got := mustReadFile(t, dst); got != "head tail" {
		t.Errorf("dst = %q, want %q", got, "head tail")
	}
}

func TestMergeBody_CreatesFileWithPerm(t *testing.T) {
	// zero umask for a deterministic mode assertion
	old := syscall.Umask(0)
	defer syscall.Umask(old)

	dir := t.TempDir()
	src := filepath.Join(dir, "src")
	dst := filepath.Join(dir, "new")
	writeFile(t, src, "data")

	b := &MergeBody{Source: src, Destination: dst, CreateFilePerm: "rw-r--r--"}
	if _, err := b.Execute(context.Background(), &fakeRuntime{}); err != nil {
		t.Fatalf("Execute() = %v", err)
	}
	info, err := os.Stat(dst)
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}
	if info.Mode().Perm() != 0644 {
		t.Errorf("perm = %o, want 0644", info.Mode().Perm())
	}
}

func TestMergeBody_InvalidPerm(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src")
	writeFile(t, src, "data")

	b := &MergeBody{Source: src, Destination: filepath.Join(dir, "dst"), CreateFilePerm: "bad"}
	if _, err := b.Execute(context.Background(), &fakeRuntime{}); err == nil {
		t.Fatal("expected error for invalid perm")
	}
}
