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

// JSONSchema представляет JSON Schema согласно draft 2020-12
type JSONSchema struct {
	Schema      string                 `json:"$schema,omitempty"`
	ID          string                 `json:"$id,omitempty"`
	Ref         string                 `json:"$ref,omitempty"`
	Title       string                 `json:"title,omitempty"`
	Description string                 `json:"description,omitempty"`
	Type        string                 `json:"type,omitempty"`
	Properties  map[string]*JSONSchema `json:"properties,omitempty"`
	Items       *JSONSchema            `json:"items,omitempty"`
	Required    []string               `json:"required,omitempty"`
	Enum        []string               `json:"enum,omitempty"`
	Examples    []any                  `json:"examples,omitempty"`
	Defs        map[string]*JSONSchema `json:"$defs,omitempty"`
	OneOf       []*JSONSchema          `json:"oneOf,omitempty"`

	// Валидация
	Format   string `json:"format,omitempty"`
	Pattern  string `json:"pattern,omitempty"`
	MinItems *int   `json:"minItems,omitempty"`

	// additionalProperties может быть bool или схемой
	AdditionalProperties any `json:"additionalProperties,omitempty"`

	// Расширения для дополнительной метаинформации (x-* prefixed)
	XNeeds      string `json:"x-needs,omitempty"`
	XConflicts  string `json:"x-conflicts,omitempty"`
	XModuleType string `json:"x-module-type,omitempty"`

	DependentRequired MapSlice `json:"dependentRequired,omitempty"`
}

// MapSlice для сериализации dependentRequired
type MapSlice map[string][]string

// FieldMeta содержит метаданные поля из тегов
type FieldMeta struct {
	Name      string   // Имя поля в JSON/YAML
	Required  bool     // Обязательное поле
	Needs     string   // Зависимость от другого поля
	Conflicts string   // Конфликт с другим полем
	Enum      []string // Возможные значения
}
