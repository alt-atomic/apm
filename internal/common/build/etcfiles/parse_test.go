package etcfiles

import (
	"os"
	"path/filepath"
	"testing"
)

func TestTokenizeQuoted(t *testing.T) {
	tests := []struct {
		input    string
		expected []string
	}{
		{"d /var/lib 0755 root root - -", []string{"d", "/var/lib", "0755", "root", "root", "-", "-"}},
		{`L "/path with spaces" - - - - target`, []string{"L", "/path with spaces", "-", "-", "-", "-", "target"}},
		{`u bin 1:1 "bin" / -`, []string{"u", "bin", "1:1", "bin", "/", "-"}},
		{"", nil},
		{"  \t  ", nil},
		{`g root 0`, []string{"g", "root", "0"}},
	}

	for _, tt := range tests {
		got := TokenizeQuoted(tt.input)
		if len(got) != len(tt.expected) {
			t.Errorf("TokenizeQuoted(%q) = %v, want %v", tt.input, got, tt.expected)
			continue
		}
		for i := range got {
			if got[i] != tt.expected[i] {
				t.Errorf("TokenizeQuoted(%q)[%d] = %q, want %q", tt.input, i, got[i], tt.expected[i])
			}
		}
	}
}

func TestParseColonFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test")
	content := "root:x:0:0:root:/root:/bin/bash\n# comment\n+nis\nbin:x:1:1:bin:/bin:/dev/null\n"
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	entries, err := ParseColonFile(path, ParsePasswdLine)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}
	if entries[0].Name != "root" || entries[0].UID != 0 {
		t.Errorf("first entry = %+v, want root:0", entries[0])
	}
	if entries[1].Name != "bin" || entries[1].UID != 1 {
		t.Errorf("second entry = %+v, want bin:1", entries[1])
	}
}

func TestParseColonFileMissing(t *testing.T) {
	entries, err := ParseColonFile("/nonexistent/path", ParsePasswdLine)
	if err != nil {
		t.Fatal(err)
	}
	if entries != nil {
		t.Errorf("expected nil for missing file, got %v", entries)
	}
}
