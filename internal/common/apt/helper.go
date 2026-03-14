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
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var (
	re64bit             = regexp.MustCompile(`\s*\((?i:64bit)\)$`)
	reVersionConstraint = regexp.MustCompile(`\s*\([<>!=][^)]*\)$`)
)

func cleanSoName(s string) (string, bool) {
	idx := strings.Index(s, ".so")
	if idx == -1 {
		return "", false
	}
	end := idx + 3
	for end < len(s) && s[end] == '.' {
		j := end + 1
		if j >= len(s) || s[j] < '0' || s[j] > '9' {
			break
		}
		for j < len(s) && s[j] >= '0' && s[j] <= '9' {
			j++
		}
		end = j
	}
	return s[:end], true
}

func CleanDependency(s string) string {
	s = strings.TrimSpace(s)

	if strings.HasPrefix(s, "lib") {
		if cleaned, ok := cleanSoName(s); ok {
			return cleaned
		}
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
	s = re64bit.ReplaceAllString(s, "")
	s = reVersionConstraint.ReplaceAllString(s, "")

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
	if strings.ToLower(filepath.Ext(path)) != ".rpm" {
		return false
	}
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return info.Mode().IsRegular()
}
