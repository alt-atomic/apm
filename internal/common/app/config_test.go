package app

import (
	"os"
	"testing"
)

func TestHasComposefsArg(t *testing.T) {
	const digest = "395a2efb3b1ce4f7849becebe087df5ab50514b57c57b8a4ac9a38ecd8e5747533f5ffe73ac61e1f1930496c4a1c7800fc4c778af0852169acc426c394065c5e"
	const initrd = `initrd=\EFI\Linux\bootc_composefs-` + digest + `\initrd`

	for _, tt := range []struct {
		cmdline string
		want    bool
	}{
		{initrd + " rootflags=subvol=/@ rw composefs.digest=v1-sha512-12:" + digest, true},
		{"composefs=" + digest + " quiet", true},
		{initrd + " rw quiet", false},
		{"other=composefs.digest=" + digest, false},
	} {
		if got := hasComposefsArg(tt.cmdline); got != tt.want {
			t.Errorf("hasComposefsArg(%q) = %v, want %v", tt.cmdline, got, tt.want)
		}
	}
}

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
