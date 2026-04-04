package lint

import (
	"apm/internal/common/app"
	"apm/internal/common/testutil"
	"context"
	"os"
	"path/filepath"
	"testing"
)

func testContext() context.Context {
	cfg := testutil.DefaultAppConfig()
	return context.WithValue(context.Background(), app.AppConfigKey, cfg)
}

func TestExtractPath(t *testing.T) {
	tests := []struct {
		line     string
		expected string
	}{
		{"d /var/lib 0755 root root - -", "/var/lib"},
		{"L /var/mail - - - - spool/mail", "/var/mail"},
		{`d "/path with spaces" 0755 root root - -`, "/path with spaces"},
		{"d /var/run/test 0755 root root - -", "/run/test"},
		{"d /var/run 0755 root root - -", "/run"},
		{"", ""},
		{"d", ""},
	}

	for _, tt := range tests {
		got := extractPath(tt.line)
		if got != tt.expected {
			t.Errorf("extractPath(%q) = %q, want %q", tt.line, got, tt.expected)
		}
	}
}

func TestCanonicalizePath(t *testing.T) {
	tests := []struct {
		input, expected string
	}{
		{"/var/run/test", "/run/test"},
		{"/var/run", "/run"},
		{"/var/lib", "/var/lib"},
		{"/run/test", "/run/test"},
	}
	for _, tt := range tests {
		got := canonicalizePath(tt.input)
		if got != tt.expected {
			t.Errorf("canonicalizePath(%q) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}

func TestEscapeUnescapePath(t *testing.T) {
	tests := []string{
		"/var/lib/normal",
		"/var/lib/with spaces/dir",
		"/var/lib/with\ttab",
	}
	for _, path := range tests {
		escaped := escapePath(path)
		unescaped := unescapePath(escaped)
		// Для путей с пробелами нужно убрать кавычки
		if len(unescaped) > 1 && unescaped[0] == '"' {
			unescaped = unescaped[1 : len(unescaped)-1]
		}
		if unescaped != path {
			t.Errorf("roundtrip failed: %q -> %q -> %q", path, escaped, unescaped)
		}
	}
}

func TestUnescapePathHex(t *testing.T) {
	got := unescapePath(`/var/lib/test\x20dir`)
	if got != "/var/lib/test dir" {
		t.Errorf("unescapePath hex = %q, want %q", got, "/var/lib/test dir")
	}
}

func TestTmpfilesAnalyze(t *testing.T) {
	root := t.TempDir()

	// Создаём структуру /var с директориями и симлинком
	os.MkdirAll(filepath.Join(root, "var", "lib", "test"), 0755)
	os.Symlink("../target", filepath.Join(root, "var", "lib", "link"))

	// Создаём существующий tmpfiles.d
	tmpDir := filepath.Join(root, "usr", "lib", "tmpfiles.d")
	os.MkdirAll(tmpDir, 0755)
	os.WriteFile(filepath.Join(tmpDir, "base.conf"), []byte("d /var/lib 0755 root root - -\n"), 0644)

	var a tmpFilesAnalysis
	if err := a.Analyze(testContext(), root); err != nil {
		t.Fatal(err)
	}

	// /var/lib покрыт existing, но /var/lib/test и /var/lib/link — нет
	if len(a.Missing) == 0 {
		t.Error("expected missing entries")
	}

	foundDir := false
	foundLink := false
	for _, e := range a.Missing {
		if e.Path == "/var/lib/test" && e.Type == "d" {
			foundDir = true
		}
		if e.Path == "/var/lib/link" && e.Type == "L" {
			foundLink = true
		}
	}
	if !foundDir {
		t.Error("missing /var/lib/test directory entry")
	}
	if !foundLink {
		t.Error("missing /var/lib/link symlink entry")
	}

	if _, ok := a.Existing["/var/lib"]; !ok {
		t.Error("expected /var/lib in existing")
	}
}

func TestTmpfilesAnalyzeRegularFile(t *testing.T) {
	root := t.TempDir()
	os.MkdirAll(filepath.Join(root, "var", "lib"), 0755)
	os.WriteFile(filepath.Join(root, "var", "lib", "data.db"), []byte("data"), 0644)

	var a tmpFilesAnalysis
	if err := a.Analyze(testContext(), root); err != nil {
		t.Fatal(err)
	}

	found := false
	for _, e := range a.Missing {
		if e.Path == "/var/lib/data.db" && e.Type == "z" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected /var/lib/data.db as z entry in missing, got %v", a.Missing)
	}
}

func TestTmpfilesAnalyzeEtcSymlink(t *testing.T) {
	root := t.TempDir()

	etcDir := filepath.Join(root, "etc")
	os.MkdirAll(etcDir, 0755)

	os.WriteFile(filepath.Join(etcDir, "hostname"), []byte("test"), 0644)
	os.Symlink("/proc/mounts", filepath.Join(etcDir, "mtab"))

	var a tmpFilesAnalysis
	if err := a.Analyze(testContext(), root); err != nil {
		t.Fatal(err)
	}

	var foundFile, foundLink bool
	for _, e := range a.Missing {
		if e.Path == "/etc/hostname" && e.Type == "z" {
			foundFile = true
		}
		if e.Path == "/etc/mtab" && e.Type == "L" {
			foundLink = true
		}
	}
	if !foundFile {
		t.Error("expected /etc/hostname as z entry")
	}
	if !foundLink {
		t.Errorf("expected /etc/mtab as L entry, got entries: %v", a.Missing)
	}
}

func TestTmpfilesAnalyzeEmpty(t *testing.T) {
	root := t.TempDir()

	var a tmpFilesAnalysis
	if err := a.Analyze(testContext(), root); err != nil {
		t.Fatal(err)
	}
	if len(a.Missing) != 0 || len(a.Unsupported) != 0 {
		t.Error("expected empty results for empty rootfs")
	}
}
