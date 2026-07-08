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
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	build "altlinux.space/alt-atomic/apm/pkg/build"
)

// fakeBody is a test-only module body covering typical field kinds.
type fakeBody struct {
	Foo string `yaml:"foo,omitempty" json:"foo,omitempty"`
	Num int    `yaml:"num,omitempty" json:"num,omitempty"`
}

func (f *fakeBody) Execute(context.Context, build.RuntimeContext) (any, error) { return nil, nil }

func init() {
	build.Register("fake", func() build.Body { return &fakeBody{} })
}

// TestModuleYamlJsonEquivalence checks that the same module in YAML and JSON
// parses into an identical Config, covering name/type/id/env/if/output/body.
func TestModuleYamlJsonEquivalence(t *testing.T) {
	yamlDoc := `
image: base
modules:
  - name: m1
    type: fake
    id: mod1
    if: "1 == 1"
    env:
      A: "1"
    output:
      result: ok
    body:
      foo: hello
      num: 7
`
	jsonDoc := `{
  "image": "base",
  "modules": [
    {
      "name": "m1",
      "type": "fake",
      "id": "mod1",
      "if": "1 == 1",
      "env": {"A": "1"},
      "output": {"result": "ok"},
      "body": {"foo": "hello", "num": 7}
    }
  ]
}`

	yamlCfg, err := build.ParseYamlConfigData([]byte(yamlDoc))
	if err != nil {
		t.Fatalf("yaml parse failed: %v", err)
	}
	jsonCfg, err := build.ParseJsonConfigData([]byte(jsonDoc))
	if err != nil {
		t.Fatalf("json parse failed: %v", err)
	}

	if !reflect.DeepEqual(yamlCfg, jsonCfg) {
		t.Errorf("YAML and JSON parse to different Config:\n yaml=%+v\n json=%+v", yamlCfg, jsonCfg)
	}
}

// TestModuleUnknownFieldParity checks that an unknown module-level field is
// rejected the same way by both formats.
func TestModuleUnknownFieldParity(t *testing.T) {
	yamlDoc := `
image: base
modules:
  - type: fake
    bogus: x
    body:
      foo: hi
`
	jsonDoc := `{"image":"base","modules":[{"type":"fake","bogus":"x","body":{"foo":"hi"}}]}`

	_, yErr := build.ParseYamlConfigData([]byte(yamlDoc))
	_, jErr := build.ParseJsonConfigData([]byte(jsonDoc))

	if (yErr == nil) != (jErr == nil) {
		t.Errorf("unknown module-level field handled differently: yamlErr=%v jsonErr=%v", yErr, jErr)
	}
}

// TestBodyUnknownFieldParity checks that an unknown field inside body is
// rejected the same way by both formats.
func TestBodyUnknownFieldParity(t *testing.T) {
	yamlDoc := `
image: base
modules:
  - type: fake
    body:
      foo: hi
      bogus: x
`
	jsonDoc := `{"image":"base","modules":[{"type":"fake","body":{"foo":"hi","bogus":"x"}}]}`

	_, yErr := build.ParseYamlConfigData([]byte(yamlDoc))
	_, jErr := build.ParseJsonConfigData([]byte(jsonDoc))

	if (yErr == nil) != (jErr == nil) {
		t.Errorf("unknown body field handled differently: yamlErr=%v jsonErr=%v", yErr, jErr)
	}
}

// TestConfigSaveMultilineUsesLiteral checks that Save writes a multiline field
// as a literal block, not an escaped double-quoted one-liner, and round-trips.
func TestConfigSaveMultilineUsesLiteral(t *testing.T) {
	const script = "install /dev/stdin /x <<'SH'\n#!/bin/bash\necho \"$d\"\nSH"
	cfg := build.Config{
		Image:   "base",
		Modules: []build.Module{{Type: "fake", Body: &fakeBody{Foo: script}}},
	}

	path := filepath.Join(t.TempDir(), "out.yml")
	if err := cfg.Save(path); err != nil {
		t.Fatalf("save: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), `\n`) {
		t.Errorf("Save emitted escaped \\n (double-quoted), want literal block:\n%s", data)
	}

	cfg2, err := build.ParseYamlConfigData(data)
	if err != nil {
		t.Fatalf("re-parse saved file: %v", err)
	}
	if got := cfg2.Modules[0].Body.(*fakeBody).Foo; got != script {
		t.Errorf("round-trip mismatch:\n got=%q\nwant=%q", got, script)
	}
}
