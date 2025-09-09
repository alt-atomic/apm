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

package helper

import (
	"apm/internal/common/app"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// ClearALRPackageName очищаем названия alr пакетов от постфиксов
func ClearALRPackageName(name string) string {
	if idx := strings.Index(name, "+alr"); idx != -1 {
		return name[:idx]
	}
	return name
}

// CleanPackageName удаляет служебные суффиксы (#EVR, :epoch, .32bit).
func CleanPackageName(pkg string) string {
	if idx := strings.Index(pkg, "#"); idx != -1 {
		pkg = pkg[:idx]
	}
	if idx := strings.Index(pkg, ":"); idx != -1 {
		pkg = pkg[idx+1:]
	}
	if strings.HasSuffix(pkg, ".32bit") {
		pkg = strings.TrimSuffix(pkg, ".32bit")
	}
	return pkg
}

// GetVersionFromAptCache преобразует полную версию пакетов из apt ALT в коротких вид
func GetVersionFromAptCache(s string) (string, error) {
	parts := strings.Split(s, ":")
	var candidate string
	if len(parts) > 1 && regexp.MustCompile(`^\d+$`).MatchString(parts[0]) {
		candidate = parts[1]
	} else {
		candidate = parts[0]
	}

	if idx := strings.Index(candidate, "-alt"); idx != -1 {
		numericPart := candidate[:idx]
		if strings.Contains(numericPart, ".") {
			candidate = numericPart
		}
	}

	if candidate == "" {
		return "", fmt.Errorf(app.T_("version not found"))
	}
	return candidate, nil
}

// AutoSize возвращает размер данных для int
func AutoSize(value int) string {
	mb := float64(value) / (1024 * 1024)
	return fmt.Sprintf(app.T_("%.2f MB"), mb)
}

// ParseBool пытается преобразовать значение к bool.
func ParseBool(val interface{}) (bool, bool) {
	switch x := val.(type) {
	case bool:
		return x, true
	case int:
		return x != 0, true
	case string:
		lower := strings.ToLower(x)
		if lower == "true" {
			return true, true
		} else if lower == "false" {
			return false, true
		}
		if iv, err := strconv.Atoi(x); err == nil {
			return iv != 0, true
		}
	}
	return false, false
}
