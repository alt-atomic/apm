package lint

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRunTmpAnalyzeEmpty(t *testing.T) {
	root := t.TempDir()
	os.MkdirAll(filepath.Join(root, "run"), 0755)
	os.MkdirAll(filepath.Join(root, "tmp"), 0755)

	var a RunTmpAnalysis
	if err := a.Analyze(testContext(), root); err != nil {
		t.Fatal(err)
	}
	if len(a.Unexpected) != 0 {
		t.Errorf("expected empty, got %v", a.Unexpected)
	}
}

func TestRunTmpAnalyzeFindsContent(t *testing.T) {
	root := t.TempDir()
	os.MkdirAll(filepath.Join(root, "run", "something"), 0755)
	os.MkdirAll(filepath.Join(root, "tmp", "leftover"), 0755)

	var a RunTmpAnalysis
	if err := a.Analyze(testContext(), root); err != nil {
		t.Fatal(err)
	}
	if len(a.Unexpected) != 2 {
		t.Errorf("expected 2 entries, got %v", a.Unexpected)
	}
}

func TestRunTmpAnalyzeIgnoredPrefixes(t *testing.T) {
	root := t.TempDir()
	os.MkdirAll(filepath.Join(root, "tmp", "apm-src", "deep", "dir"), 0755)
	os.MkdirAll(filepath.Join(root, "tmp", "go-cache", "ab"), 0755)

	var a RunTmpAnalysis
	if err := a.Analyze(testContext(), root); err != nil {
		t.Fatal(err)
	}
	if len(a.Unexpected) != 0 {
		t.Errorf("expected all ignored, got %v", a.Unexpected)
	}
}

func TestRunTmpAnalyzePrunesKnownFiles(t *testing.T) {
	root := t.TempDir()
	// Создаём путь до known runtime file
	resolveDir := filepath.Join(root, "run", "systemd", "resolve")
	os.MkdirAll(resolveDir, 0755)
	os.WriteFile(filepath.Join(resolveDir, "stub-resolv.conf"), []byte(""), 0644)

	var a RunTmpAnalysis
	if err := a.Analyze(testContext(), root); err != nil {
		t.Fatal(err)
	}
	// stub-resolv.conf и его пустые родители должны быть удалены
	for _, p := range a.Unexpected {
		if p == "/run/systemd/resolve/stub-resolv.conf" ||
			p == "/run/systemd/resolve" ||
			p == "/run/systemd" {
			t.Errorf("expected %q to be pruned, but found in unexpected", p)
		}
	}
}

func TestRunTmpAnalyzeMissingDirs(t *testing.T) {
	root := t.TempDir()
	// Без /run и /tmp — не должно падать

	var a RunTmpAnalysis
	if err := a.Analyze(testContext(), root); err != nil {
		t.Fatal(err)
	}
	if len(a.Unexpected) != 0 {
		t.Errorf("expected empty, got %v", a.Unexpected)
	}
}

func TestPruneKnownPaths(t *testing.T) {
	paths := []string{
		"/run/systemd",
		"/run/systemd/resolve",
		"/run/systemd/resolve/stub-resolv.conf",
		"/run/other",
	}
	known := []string{"/run/systemd/resolve/stub-resolv.conf"}

	result := pruneKnownPaths(paths, known)
	if len(result) != 1 || result[0] != "/run/other" {
		t.Errorf("expected [/run/other], got %v", result)
	}
}

func TestPruneKnownPathsKeepsParentWithOtherChildren(t *testing.T) {
	paths := []string{
		"/run/systemd",
		"/run/systemd/resolve",
		"/run/systemd/resolve/stub-resolv.conf",
		"/run/systemd/journal",
	}
	known := []string{"/run/systemd/resolve/stub-resolv.conf"}

	result := pruneKnownPaths(paths, known)
	// /run/systemd должен остаться, т.к. у него есть /run/systemd/journal
	found := false
	for _, p := range result {
		if p == "/run/systemd" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected /run/systemd to remain, got %v", result)
	}
}
