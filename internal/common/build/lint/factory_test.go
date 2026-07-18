package lint

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWriteAndCleanFactory(t *testing.T) {
	root := t.TempDir()
	os.MkdirAll(filepath.Join(root, "var", "lib", "myapp"), 0755)
	os.WriteFile(filepath.Join(root, "var", "lib", "myapp", "state.db"), []byte("payload"), 0640)

	a := tmpFilesAnalysis{reporter: testReporter()}
	if err := a.Analyze(testContext(), root); err != nil {
		t.Fatal(err)
	}
	if _, err := a.WriteConf(root); err != nil {
		t.Fatal(err)
	}
	written, err := a.WriteFactory(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(written) != 1 || written[0] != "/var/lib/myapp/state.db" {
		t.Fatalf("expected one factory file, got %v", written)
	}

	factoryFile := filepath.Join(root, "usr", "share", "factory", "var", "lib", "myapp", "state.db")
	data, err := os.ReadFile(factoryFile)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "payload" {
		t.Errorf("factory content mismatch: %q", data)
	}
	info, _ := os.Stat(factoryFile)
	if info.Mode().Perm() != 0640 {
		t.Errorf("factory mode = %o, want 0640", info.Mode().Perm())
	}

	// чужой заводской файл — не из нашего конфига, чистка не должна его тронуть
	foreign := filepath.Join(root, "usr", "share", "factory", "etc", "nsswitch.conf")
	os.MkdirAll(filepath.Dir(foreign), 0755)
	os.WriteFile(foreign, []byte("systemd factory"), 0644)

	// повторный цикл: CleanFactory убирает только свои файлы и опустевшие каталоги
	b := tmpFilesAnalysis{reporter: testReporter()}
	if err = b.CleanFactory(root); err != nil {
		t.Fatal(err)
	}
	if _, err = os.Stat(factoryFile); !os.IsNotExist(err) {
		t.Error("expected own factory file removed")
	}
	if _, err = os.Stat(filepath.Join(root, "usr", "share", "factory", "var")); !os.IsNotExist(err) {
		t.Error("expected empty factory dirs pruned")
	}
	if _, err = os.Stat(foreign); err != nil {
		t.Error("expected foreign factory file kept")
	}
}

func TestFactoryKeepsSpecialBits(t *testing.T) {
	root := t.TempDir()
	os.MkdirAll(filepath.Join(root, "var", "bin"), 0755)
	tool := filepath.Join(root, "var", "bin", "tool")
	os.WriteFile(tool, []byte("#!/bin/sh"), 0644)
	if err := os.Chmod(tool, 0o711|os.ModeSetuid); err != nil {
		t.Fatal(err)
	}

	a := tmpFilesAnalysis{reporter: testReporter()}
	if err := a.Analyze(testContext(), root); err != nil {
		t.Fatal(err)
	}

	var line string
	for _, e := range a.Missing {
		if e.Path == "/var/bin/tool" {
			line = e.Line
		}
	}
	if !strings.Contains(line, " 4711 ") {
		t.Errorf("expected mode 4711 in conf line, got %q", line)
	}

	if _, err := a.WriteFactory(root); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(filepath.Join(root, "usr", "share", "factory", "var", "bin", "tool"))
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode()&os.ModeSetuid == 0 || info.Mode().Perm() != 0o711 {
		t.Errorf("factory copy mode = %v, want setuid+0711", info.Mode())
	}
}