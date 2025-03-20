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
	"golang.org/x/text/language"
	"os"
	"strings"
)

// GetSystemLocale возвращает базовый язык системы в виде language.Tag.
func GetSystemLocale() language.Tag {
	var locale string
	if v := os.Getenv("LC_ALL"); v != "" {
		locale = stripAfterDot(v)
	} else if v := os.Getenv("LC_MESSAGES"); v != "" {
		locale = stripAfterDot(v)
	} else {
		locale = stripAfterDot(os.Getenv("LANG"))
	}

	// Приводим строку к формату BCP 47 (заменяем "_" на "-").
	locale = strings.Replace(locale, "_", "-", 1)
	tag, err := language.Parse(locale)
	if err != nil {
		return language.English
	}

	base, _ := tag.Base()
	return language.Make(base.String())
}

func stripAfterDot(locale string) string {
	if idx := strings.Index(locale, "."); idx != -1 {
		return locale[:idx]
	}
	return locale
}
