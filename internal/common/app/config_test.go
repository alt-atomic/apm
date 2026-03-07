package app

import (
	"os"
	"testing"
)

func TestExpandUser(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("cannot determine home dir")
	}

	tests := []struct {
		input string
		want  string
	}{
		{"~/test/path", home + "/test/path"},
		{"~/.cache/apm/apm.db", home + "/.cache/apm/apm.db"},
		{"/absolute/path", "/absolute/path"},
		{"relative/path", "relative/path"},
		{"~", "~"},
		{"", ""},
	}

	for _, tt := range tests {
		got := expandUser(tt.input)
		if got != tt.want {
			t.Errorf("expandUser(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestApplyBuildInfo_EmptyDoesNotOverwrite(t *testing.T) {
	cm := &configManagerImpl{config: &Configuration{
		CommandPrefix: "original",
		Environment:   "prod",
	}}

	cm.applyBuildInfo(BuildInfo{})

	if cm.config.CommandPrefix != "original" {
		t.Errorf("CommandPrefix should not be overwritten, got %q", cm.config.CommandPrefix)
	}
	if cm.config.Environment != "prod" {
		t.Errorf("Environment should not be overwritten, got %q", cm.config.Environment)
	}
}

func TestEnsurePath(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/subdir/test.db"

	if err := EnsurePath(path); err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Error("file should have been created")
	}

	if err := EnsurePath(path); err != nil {
		t.Errorf("second call should not fail: %v", err)
	}
}
