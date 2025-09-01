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

package apt

import (
	"apm/internal/common/appstream"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

type Package struct {
	Name             string               `json:"name"`
	Section          string               `json:"section"`
	InstalledSize    int                  `json:"installedSize"`
	Maintainer       string               `json:"maintainer"`
	Version          string               `json:"version"`
	VersionInstalled string               `json:"versionInstalled"`
	Depends          []string             `json:"depends"`
	Provides         []string             `json:"provides"`
	Size             int                  `json:"size"`
	Filename         string               `json:"filename"`
	Description      string               `json:"description"`
	AppStream        *appstream.Component `json:"appStream"`
	Changelog        string               `json:"lastChangelog"`
	Installed        bool                 `json:"installed"`
	TypePackage      int                  `json:"typePackage"`
}

var soNameRe = regexp.MustCompile(`^(.+?\.so(?:\.[0-9]+)*).*`)

func CleanDependency(s string) string {
	s = strings.TrimSpace(s)

	if m := soNameRe.FindStringSubmatch(s); len(m) > 1 && strings.HasPrefix(s, "lib") {
		return m[1]
	}

	if idx := strings.IndexByte(s, '('); idx != -1 {
		inner := s[idx+1:]
		if j := strings.IndexByte(inner, ')'); j != -1 {
			inner = inner[:j]
		}
		// Если скобка не закрыта, inner уже содержит всё до конца строки
		inner = strings.TrimSpace(inner)

		if inner != "" && (strings.Contains(inner, ".so") || strings.Contains(inner, "/")) {
			return s
		}
	}

	// Убираем версионные ограничения только для обычных зависимостей пакетов
	s = regexp.MustCompile(`\s*\((?i:64bit)\)$`).ReplaceAllString(s, "")
	s = regexp.MustCompile(`\s*\([<>!=][^)]*\)$`).ReplaceAllString(s, "")

	if idx := strings.IndexByte(s, '('); idx != -1 {
		inner := s[idx+1:]
		if j := strings.IndexByte(inner, ')'); j != -1 {
			inner = inner[:j]
		}
		inner = strings.TrimSpace(inner)
		if inner == "" {
			s = strings.TrimSpace(s[:idx])
		}
	}

	s = strings.TrimSpace(s)
	if idx := strings.IndexByte(s, ':'); idx > 0 && s[0] >= '0' && s[0] <= '9' {
		s = s[idx+1:]
	}

	return s
}

func IsRegularFileAndIsPackage(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return false
		}
		return false
	}
	if !info.Mode().IsRegular() {
		return false
	}
	ext := strings.ToLower(filepath.Ext(path))
	if ext != ".rpm" {
		return false
	}
	return true
}
