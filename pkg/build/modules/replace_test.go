package modules

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestReplaceBody_RelativeTarget(t *testing.T) {
	b := &ReplaceBody{Target: "rel/file", Pattern: "a", Repl: "b"}
	if _, err := b.Execute(context.Background(), &fakeRuntime{}); err == nil {
		t.Fatal("expected error for relative target")
	}
}

func TestReplaceBody_MissingFile(t *testing.T) {
	b := &ReplaceBody{Target: filepath.Join(t.TempDir(), "nope"), Pattern: "a", Repl: "b"}
	if _, err := b.Execute(context.Background(), &fakeRuntime{}); err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestReplaceBody_ReplacesPerLine(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "file")
	writeFile(t, file, "foo 1\nfoo 2\nbar 3")

	b := &ReplaceBody{Target: file, Pattern: `foo`, Repl: "baz"}
	if _, err := b.Execute(context.Background(), &fakeRuntime{}); err != nil {
		t.Fatalf("Execute() = %v", err)
	}
	if got := mustReadFile(t, file); got != "baz 1\nbaz 2\nbar 3" {
		t.Errorf("file = %q", got)
	}
}

func TestReplaceBody_PreservesPerm(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "file")
	writeFile(t, file, "x")
	if err := os.Chmod(file, 0640); err != nil {
		t.Fatal(err)
	}

	b := &ReplaceBody{Target: file, Pattern: "x", Repl: "y"}
	if _, err := b.Execute(context.Background(), &fakeRuntime{}); err != nil {
		t.Fatalf("Execute() = %v", err)
	}
	info, err := os.Stat(file)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0640 {
		t.Errorf("perm = %o, want 0640", info.Mode().Perm())
	}
}

func TestReplaceBody_InvalidPattern(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "file")
	writeFile(t, file, "x")

	b := &ReplaceBody{Target: file, Pattern: "[unterminated", Repl: "y"}
	if _, err := b.Execute(context.Background(), &fakeRuntime{}); err == nil {
		t.Fatal("expected error for invalid regex")
	}
}
