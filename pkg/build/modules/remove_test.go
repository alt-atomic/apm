package modules

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestRemoveBody_RelativeTarget(t *testing.T) {
	b := &RemoveBody{Targets: []string{"rel/path"}}
	if _, err := b.Execute(context.Background(), &fakeRuntime{}); err == nil {
		t.Fatal("expected error for relative target")
	}
}

func TestRemoveBody_RemovesFileAndDir(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "file")
	sub := filepath.Join(dir, "sub")
	writeFile(t, file, "x")
	if err := os.Mkdir(sub, 0755); err != nil {
		t.Fatal(err)
	}

	b := &RemoveBody{Targets: []string{file, sub}}
	if _, err := b.Execute(context.Background(), &fakeRuntime{}); err != nil {
		t.Fatalf("Execute() = %v", err)
	}
	for _, p := range []string{file, sub} {
		if _, err := os.Stat(p); !os.IsNotExist(err) {
			t.Errorf("%s should be removed", p)
		}
	}
}

func TestRemoveBody_InsideClearsFile(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "file")
	writeFile(t, file, "content")

	b := &RemoveBody{Targets: []string{file}, Inside: true}
	if _, err := b.Execute(context.Background(), &fakeRuntime{}); err != nil {
		t.Fatalf("Execute() = %v", err)
	}
	// file stays but is emptied
	if got := mustReadFile(t, file); got != "" {
		t.Errorf("file = %q, want empty", got)
	}
}

func TestRemoveBody_InsideCleansDir(t *testing.T) {
	dir := t.TempDir()
	sub := filepath.Join(dir, "sub")
	if err := os.Mkdir(sub, 0755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(sub, "inner"), "x")

	b := &RemoveBody{Targets: []string{sub}, Inside: true}
	if _, err := b.Execute(context.Background(), &fakeRuntime{}); err != nil {
		t.Fatalf("Execute() = %v", err)
	}
	// dir stays but its content is gone
	if _, err := os.Stat(sub); err != nil {
		t.Errorf("dir should remain: %v", err)
	}
	entries, err := os.ReadDir(sub)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Errorf("dir should be empty, has %d entries", len(entries))
	}
}
