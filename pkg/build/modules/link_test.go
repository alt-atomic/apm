package modules

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestLinkBody_RelativeTarget(t *testing.T) {
	b := &LinkBody{Target: "relative/link", To: "/etc/x"}
	if _, err := b.Execute(context.Background(), &fakeRuntime{}); err == nil {
		t.Fatal("expected error for relative target")
	}
}

func TestLinkBody_MissingParentDir(t *testing.T) {
	b := &LinkBody{Target: "/nonexistent-dir-xyz/link", To: "/etc/x"}
	if _, err := b.Execute(context.Background(), &fakeRuntime{}); err == nil {
		t.Fatal("expected error for missing parent dir")
	}
}

func TestLinkBody_RelativeTo(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "link")

	b := &LinkBody{Target: target, To: "some/rel/path"}
	if _, err := b.Execute(context.Background(), &fakeRuntime{}); err != nil {
		t.Fatalf("Execute() = %v", err)
	}
	got, err := os.Readlink(target)
	if err != nil {
		t.Fatalf("Readlink: %v", err)
	}
	if got != "some/rel/path" {
		t.Errorf("link = %q, want %q", got, "some/rel/path")
	}
}

func TestLinkBody_AbsoluteToBecomesRelative(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "link")
	to := filepath.Join(dir, "file")

	b := &LinkBody{Target: target, To: to}
	if _, err := b.Execute(context.Background(), &fakeRuntime{}); err != nil {
		t.Fatalf("Execute() = %v", err)
	}
	// absolute To is stored as a path relative to target's dir
	got, err := os.Readlink(target)
	if err != nil {
		t.Fatalf("Readlink: %v", err)
	}
	if got != "file" {
		t.Errorf("link = %q, want %q", got, "file")
	}
}

func TestLinkBody_ReplaceExisting(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "link")
	writeFile(t, target, "old")

	b := &LinkBody{Target: target, To: "new/target", Replace: true}
	if _, err := b.Execute(context.Background(), &fakeRuntime{}); err != nil {
		t.Fatalf("Execute() = %v", err)
	}
	got, err := os.Readlink(target)
	if err != nil {
		t.Fatalf("Readlink: %v", err)
	}
	if got != "new/target" {
		t.Errorf("link = %q, want %q", got, "new/target")
	}
}
