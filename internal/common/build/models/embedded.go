// Atomic Package Manager
// Copyright (C) 2025 Дмитрий Удалов dmitry@udalов.online
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

package models

import (
	"embed"
	"strings"
)

// Встраиваем все .go файлы директории для парсинга комментариев при генерации схемы
//
//go:embed *.go
var sources embed.FS

// GetAllSources возвращает все исходники моделей
func GetAllSources() []string {
	entries, _ := sources.ReadDir(".")

	var result []string
	for _, entry := range entries {
		name := entry.Name()
		if name == "embedded.go" || strings.HasSuffix(name, "_test.go") {
			continue
		}

		data, err := sources.ReadFile(name)
		if err != nil {
			continue
		}
		result = append(result, string(data))
	}
	return result
}
