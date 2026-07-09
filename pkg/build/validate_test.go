// Atomic Package Manager
// Copyright (C) 2025 Дмитрий Удалов dmitry@udalov.online
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package build_test

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	build "altlinux.space/alt-atomic/apm/pkg/build"
	_ "altlinux.space/alt-atomic/apm/pkg/build/modules"
)

func writeInclude(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}

// TestValidateNestedIncludes checks that a valid a->b->c include chain passes.
func TestValidateNestedIncludes(t *testing.T) {
	dir := t.TempDir()
	writeInclude(t, dir, "a.yml", "modules:\n  - type: include\n    body:\n      targets: [b.yml]\n")
	writeInclude(t, dir, "b.yml", "modules:\n  - type: include\n    body:\n      targets: [c.yml]\n")
	writeInclude(t, dir, "c.yml", "modules:\n  - name: leaf\n    type: shell\n    body:\n      command: \"true\"\n")

	cfg, err := build.ParseYamlConfigData([]byte("modules:\n  - type: include\n    body:\n      targets: [a.yml]\n"))
	if err != nil {
		t.Fatalf("parse top: %v", err)
	}
	if err := build.ValidateConfigRecursive(&cfg, dir, build.Version{}); err != nil {
		t.Errorf("nested includes should validate: %v", err)
	}
}

// TestValidateCircularInclude checks that a cyclic include is detected.
func TestValidateCircularInclude(t *testing.T) {
	dir := t.TempDir()
	writeInclude(t, dir, "a.yml", "modules:\n  - type: include\n    body:\n      targets: [b.yml]\n")
	writeInclude(t, dir, "b.yml", "modules:\n  - type: include\n    body:\n      targets: [a.yml]\n")

	cfg, err := build.ParseYamlConfigData([]byte("modules:\n  - type: include\n    body:\n      targets: [a.yml]\n"))
	if err != nil {
		t.Fatalf("parse top: %v", err)
	}
	err = build.ValidateConfigRecursive(&cfg, dir, build.Version{})
	if err == nil {
		t.Fatal("circular include not detected")
	}
	if !strings.Contains(err.Error(), "circular") {
		t.Errorf("want circular error, got: %v", err)
	}
}

// TestValidateMissingTarget checks that a missing include target errors.
func TestValidateMissingTarget(t *testing.T) {
	dir := t.TempDir()
	cfg, err := build.ParseYamlConfigData([]byte("modules:\n  - type: include\n    body:\n      targets: [nope.yml]\n"))
	if err != nil {
		t.Fatalf("parse top: %v", err)
	}
	if err := build.ValidateConfigRecursive(&cfg, dir, build.Version{}); err == nil {
		t.Error("missing include target should error")
	}
}

// TestValidateResolvesEnvInTarget checks that ${{ Env.* }} in an include target resolves.
func TestValidateResolvesEnvInTarget(t *testing.T) {
	t.Setenv("FLAVOUR", "qcm")
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "flavour"), 0755); err != nil {
		t.Fatal(err)
	}
	writeInclude(t, filepath.Join(dir, "flavour"), "qcm.yml", "modules: []\n")

	cfg, err := build.ParseYamlConfigData([]byte("modules:\n  - type: include\n    body:\n      targets:\n        - flavour/${{ Env.FLAVOUR }}.yml\n"))
	if err != nil {
		t.Fatalf("parse top: %v", err)
	}
	if err := build.ValidateConfigRecursive(&cfg, dir, build.Version{}); err != nil {
		t.Errorf("env-resolved include target should validate: %v", err)
	}
}

// TestValidateResolvedMissingTargetErrors checks that a resolved-but-missing target errors.
func TestValidateResolvedMissingTargetErrors(t *testing.T) {
	t.Setenv("FLAVOUR", "absent")
	dir := t.TempDir()
	cfg, err := build.ParseYamlConfigData([]byte("modules:\n  - type: include\n    body:\n      targets:\n        - flavour/${{ Env.FLAVOUR }}.yml\n"))
	if err != nil {
		t.Fatalf("parse top: %v", err)
	}
	if err := build.ValidateConfigRecursive(&cfg, dir, build.Version{}); err == nil {
		t.Error("resolved-but-missing include target should error")
	}
}

// TestValidateResolvesGoArchInTarget checks that ${{ GoArch }} in an include target resolves.
func TestValidateResolvesGoArchInTarget(t *testing.T) {
	dir := t.TempDir()
	sub := filepath.Join(dir, "arch")
	if err := os.MkdirAll(sub, 0755); err != nil {
		t.Fatal(err)
	}
	writeInclude(t, sub, runtime.GOARCH+".yml", "modules: []\n")

	cfg, err := build.ParseYamlConfigData([]byte("modules:\n  - type: include\n    body:\n      targets:\n        - arch/${{ GoArch }}.yml\n"))
	if err != nil {
		t.Fatalf("parse top: %v", err)
	}
	if err := build.ValidateConfigRecursive(&cfg, dir, build.Version{}); err != nil {
		t.Errorf("GoArch-resolved include target should validate: %v", err)
	}
}

// TestValidateSkipsRuntimeTarget checks that a runtime-only target is deferred, not errored.
func TestValidateSkipsRuntimeTarget(t *testing.T) {
	dir := t.TempDir()
	cfg, err := build.ParseYamlConfigData([]byte("modules:\n  - type: include\n    body:\n      targets:\n        - out/${{ Modules.foo.Id }}.yml\n"))
	if err != nil {
		t.Fatalf("parse top: %v", err)
	}
	if err := build.ValidateConfigRecursive(&cfg, dir, build.Version{}); err != nil {
		t.Errorf("runtime-dependent include target should be skipped, got: %v", err)
	}
}
