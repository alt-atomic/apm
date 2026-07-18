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
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	build "altlinux.space/alt-atomic/apm/pkg/build"
)

const (
	modsYaml = "image: base\nmodules:\n  - type: fake\n    body:\n      foo: hi\n"
	modsJSON = `{"image":"base","modules":[{"type":"fake","body":{"foo":"hi"}}]}`
)

// TestReadAndParseModulesFileCodec checks that a local .yaml and .json source
// are both parsed and yield identical modules (codec picked by extension).
func TestReadAndParseModulesFileCodec(t *testing.T) {
	dir := t.TempDir()
	yPath := filepath.Join(dir, "m.yaml")
	jPath := filepath.Join(dir, "m.json")
	if err := os.WriteFile(yPath, []byte(modsYaml), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(jPath, []byte(modsJSON), 0644); err != nil {
		t.Fatal(err)
	}

	yMods, err := build.ReadAndParseModules(yPath)
	if err != nil {
		t.Fatalf("yaml file: %v", err)
	}
	jMods, err := build.ReadAndParseModules(jPath)
	if err != nil {
		t.Fatalf("json file: %v", err)
	}

	if len(*jMods) != 1 {
		t.Fatalf("json produced %d modules, want 1", len(*jMods))
	}
	if !reflect.DeepEqual(*yMods, *jMods) {
		t.Errorf("file yaml/json modules differ:\n yaml=%+v\n json=%+v", *yMods, *jMods)
	}
}

// TestReadAndParseModulesURLCodec checks that a remote .yaml and .json source
// are both parsed and yield identical modules (JSON over URL used to be broken).
func TestReadAndParseModulesURLCodec(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/m.yaml", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(modsYaml))
	})
	mux.HandleFunc("/m.json", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(modsJSON))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	yMods, err := build.ReadAndParseModules(srv.URL + "/m.yaml")
	if err != nil {
		t.Fatalf("yaml url: %v", err)
	}
	jMods, err := build.ReadAndParseModules(srv.URL + "/m.json")
	if err != nil {
		t.Fatalf("json url: %v", err)
	}

	if len(*jMods) != 1 {
		t.Fatalf("json url produced %d modules, want 1", len(*jMods))
	}
	if !reflect.DeepEqual(*yMods, *jMods) {
		t.Errorf("url yaml/json modules differ:\n yaml=%+v\n json=%+v", *yMods, *jMods)
	}
}
