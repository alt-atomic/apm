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

package schema

import (
	"apm/internal/common/build/models"
)

// GetEmbeddedComments возвращает карту комментариев из встроенных исходников моделей
func GetEmbeddedComments() map[string]map[string]string {
	sources := models.GetAllSources()

	comments := make(map[string]map[string]string)

	for _, src := range sources {
		parsed := parseCommentsFromSource(src)
		for structName, fields := range parsed {
			if comments[structName] == nil {
				comments[structName] = make(map[string]string)
			}
			for fieldName, comment := range fields {
				comments[structName][fieldName] = comment
			}
		}
	}

	return comments
}

// parseCommentsFromSource парсит комментарии из строки исходного кода
func parseCommentsFromSource(source string) map[string]map[string]string {
	p := NewCommentParser()
	_ = p.ParseSource(source)
	return p.GetAllComments()
}
