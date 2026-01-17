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
	"apm/internal/common/build/core"
	"encoding/json"
	"reflect"
	"strings"
)

// Generator генератор JSON Schema
type Generator struct {
	comments map[string]map[string]string // структура -> поле -> комментарий
}

// NewGenerator создаёт генератор с предзагруженными комментариями
func NewGenerator(comments map[string]map[string]string) *Generator {
	return &Generator{
		comments: comments,
	}
}

// getComment возвращает комментарий для поля структуры
func (g *Generator) getComment(structName, fieldName string) string {
	if fields, ok := g.comments[structName]; ok {
		if comment, ok := fields[fieldName]; ok {
			return comment
		}
	}
	return ""
}

// Generate генерирует полную JSON Schema для конфигов сборки
func (g *Generator) Generate() *JSONSchema {
	schema := &JSONSchema{
		Schema:      "https://json-schema.org/draft/2020-12/schema",
		ID:          "https://altlinux.space/alt-atomic/apm",
		Title:       "APM Build Config",
		Description: "Schema for APM image build configuration files",
		Type:        "object",
		Defs:        make(map[string]*JSONSchema),
	}

	// Генерируем схему для Config
	schema.Properties = map[string]*JSONSchema{
		"env": {
			Type:                 "object",
			Description:          "Environment variables for the build",
			AdditionalProperties: &JSONSchema{Type: "string"},
		},
		"image": {
			Type:        "string",
			Description: "Base image for building",
			Format:      "uri",
			Examples:    []any{"altlinux.space/alt-atomic/onyx/stable:latest", "altlinux.space/alt-atomic/kyanite/nightly:latest"},
		},
		"modules": {
			Type:        "array",
			Description: "List of build modules",
			Items:       &JSONSchema{Ref: "#/$defs/Module"},
			MinItems:    intPtr(1),
		},
	}
	schema.Required = []string{"modules"}

	// Генерируем схему для Module
	schema.Defs["Module"] = g.generateModuleSchema()

	// Генерируем схемы для всех типов Body
	g.generateBodySchemas(schema.Defs)

	return schema
}

// generateModuleSchema генерирует схему для Module
func (g *Generator) generateModuleSchema() *JSONSchema {
	moduleTypes := core.GetAllModuleTypes()

	return &JSONSchema{
		Title:       "Module",
		Type:        "object",
		Description: "Build module that performs a specific action",
		Properties: map[string]*JSONSchema{
			"name": {
				Type:        "string",
				Description: "Module name for logging",
				Examples:    []any{"Install base packages", "Configure repos"},
			},
			"type": {
				Type:        "string",
				Description: "Module type",
				Enum:        moduleTypes,
			},
			"id": {
				Type:        "string",
				Description: "Unique module identifier for referencing in expressions",
				Pattern:     "^[A-Za-z][A-Za-z0-9_]*$",
			},
			"env": {
				Type:                 "object",
				Description:          "Environment variables for this module",
				AdditionalProperties: &JSONSchema{Type: "string"},
			},
			"if": {
				Type:        "string",
				Description: "Condition in expr language format",
				Examples:    []any{"env.DEBUG == 'true'", "modules.setup.output.enabled"},
			},
			"body": {
				Description: "Module body (structure depends on type)",
				OneOf:       g.generateBodyOneOf(moduleTypes),
			},
			"output": {
				Type:                 "object",
				Description:          "Output data mapping",
				AdditionalProperties: &JSONSchema{Type: "string"},
			},
		},
		Required: []string{"type", "body"},
	}
}

// generateBodyOneOf генерирует oneOf для body в зависимости от type
func (g *Generator) generateBodyOneOf(types []string) []*JSONSchema {
	var oneOf []*JSONSchema
	for _, t := range types {
		defName := g.typeToDefName(t)
		oneOf = append(oneOf, &JSONSchema{
			Ref:         "#/$defs/" + defName,
			XModuleType: t,
		})
	}
	return oneOf
}

// typeToDefName конвертирует тип модуля в имя определения
func (g *Generator) typeToDefName(moduleType string) string {
	// packages -> PackagesBody
	parts := strings.Split(moduleType, "-")
	var result string
	for _, p := range parts {
		if len(p) > 0 {
			result += strings.ToUpper(p[:1]) + p[1:]
		}
	}
	return result + "Body"
}

// generateBodySchemas генерирует схемы для всех типов Body из ModelMap
func (g *Generator) generateBodySchemas(defs map[string]*JSONSchema) {
	// Генерируем схемы для всех Body из ModelMap
	for moduleType, factory := range core.ModelMap {
		body := factory()
		t := reflect.TypeOf(body)
		if t.Kind() == reflect.Ptr {
			t = t.Elem()
		}

		structName := t.Name() // например "PackagesBody"
		schema := g.generateStructSchema(body)

		// Title из имени типа модуля (packages -> Packages)
		schema.Title = capitalize(moduleType)

		// Description из doc comment структуры
		if docComment := g.getStructDoc(structName); docComment != "" {
			schema.Description = docComment
		}

		defs[structName] = schema

		// Обрабатываем вложенные структуры
		g.processNestedStructs(body, defs)
	}
}

// processNestedStructs находит и добавляет вложенные структуры в defs
func (g *Generator) processNestedStructs(v interface{}, defs map[string]*JSONSchema) {
	t := reflect.TypeOf(v)
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}

	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		fieldType := field.Type

		if fieldType.Kind() == reflect.Ptr {
			fieldType = fieldType.Elem()
		}

		if fieldType.Kind() == reflect.Struct {
			structName := fieldType.Name()
			// Пропускаем стандартные типы и уже добавленные
			if structName == "" || defs[structName] != nil {
				continue
			}

			// Создаём экземпляр вложенной структуры
			nestedValue := reflect.New(fieldType).Interface()
			schema := g.generateStructSchema(nestedValue)
			schema.Title = structName

			// Description из doc comment
			if docComment := g.getStructDoc(structName); docComment != "" {
				schema.Description = docComment
			}

			defs[structName] = schema
		}
	}
}

// getStructDoc возвращает doc comment для структуры (без имени структуры в начале)
func (g *Generator) getStructDoc(structName string) string {
	if fields, ok := g.comments[structName]; ok {
		if doc, ok := fields[StructDocKey]; ok {
			doc = strings.TrimSpace(doc)
			if strings.HasPrefix(doc, structName+" ") {
				doc = strings.TrimPrefix(doc, structName+" ")
			} else if strings.HasPrefix(doc, structName+"\n") {
				doc = strings.TrimPrefix(doc, structName+"\n")
			}
			return strings.TrimSpace(doc)
		}
	}
	return ""
}

// capitalize делает первую букву заглавной
func capitalize(s string) string {
	if len(s) == 0 {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

// generateStructSchema генерирует схему для структуры через рефлексию
func (g *Generator) generateStructSchema(v interface{}) *JSONSchema {
	t := reflect.TypeOf(v)
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}

	structName := t.Name()

	schema := &JSONSchema{
		Type:              "object",
		Properties:        make(map[string]*JSONSchema),
		DependentRequired: make(MapSlice),
	}

	var required []string

	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)

		if !field.IsExported() {
			continue
		}

		meta := g.parseFieldMeta(field)
		if meta.Name == "" || meta.Name == "-" {
			continue
		}

		fieldSchema := g.fieldToSchema(meta, field.Type)

		// Добавляем описание из комментария
		comment := g.getComment(structName, field.Name)
		if comment != "" {
			fieldSchema.Description = comment
		} else if meta.Description != "" {
			fieldSchema.Description = meta.Description
		}

		// Добавляем расширения
		if meta.Needs != "" {
			fieldSchema.XNeeds = toKebabCase(meta.Needs)
			schema.DependentRequired[meta.Name] = []string{toKebabCase(meta.Needs)}
		}
		if meta.Conflicts != "" {
			fieldSchema.XConflicts = toKebabCase(meta.Conflicts)
		}
		if len(meta.Enum) > 0 {
			fieldSchema.Enum = meta.Enum
		}

		schema.Properties[meta.Name] = fieldSchema

		if meta.Required {
			required = append(required, meta.Name)
		}
	}

	if len(required) > 0 {
		schema.Required = required
	}

	if len(schema.DependentRequired) == 0 {
		schema.DependentRequired = nil
	}

	return schema
}

// parseFieldMeta извлекает метаданные из тегов поля
func (g *Generator) parseFieldMeta(field reflect.StructField) FieldMeta {
	meta := FieldMeta{
		GoName: field.Name,
	}

	// Парсим yaml/json тег для имени
	if yamlTag := field.Tag.Get("yaml"); yamlTag != "" {
		parts := strings.Split(yamlTag, ",")
		meta.Name = parts[0]
	} else if jsonTag := field.Tag.Get("json"); jsonTag != "" {
		parts := strings.Split(jsonTag, ",")
		meta.Name = parts[0]
	}

	// Проверяем required тег
	if _, ok := field.Tag.Lookup("required"); ok {
		meta.Required = true
	}

	// Проверяем needs тег
	if needs := field.Tag.Get("needs"); needs != "" {
		meta.Needs = needs
	}

	// Проверяем conflicts тег
	if conflicts := field.Tag.Get("conflicts"); conflicts != "" {
		meta.Conflicts = conflicts
	}

	// Проверяем schema тег для enum и description
	if schemaTag := field.Tag.Get("schema"); schemaTag != "" {
		parts := strings.Split(schemaTag, ",")
		for _, part := range parts {
			if strings.HasPrefix(part, "enum=") {
				enumStr := strings.TrimPrefix(part, "enum=")
				meta.Enum = strings.Split(enumStr, "|")
			}
			if strings.HasPrefix(part, "desc=") {
				meta.Description = strings.TrimPrefix(part, "desc=")
			}
		}
	}

	return meta
}

// fieldToSchema конвертирует Go тип в JSON Schema
func (g *Generator) fieldToSchema(meta FieldMeta, t reflect.Type) *JSONSchema {
	schema := &JSONSchema{}

	switch t.Kind() {
	case reflect.String:
		schema.Type = "string"
	case reflect.Bool:
		schema.Type = "boolean"
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		schema.Type = "integer"
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		schema.Type = "integer"
	case reflect.Float32, reflect.Float64:
		schema.Type = "number"
	case reflect.Slice:
		schema.Type = "array"
		schema.Items = g.fieldToSchema(FieldMeta{}, t.Elem())
	case reflect.Map:
		schema.Type = "object"
		valueSchema := g.fieldToSchema(FieldMeta{}, t.Elem())
		schema.AdditionalProperties = valueSchema
	case reflect.Struct:
		// Для вложенных структур используем ссылку
		structName := t.Name()
		if structName != "" {
			schema.Ref = "#/$defs/" + structName
		} else {
			schema.Type = "object"
		}
	case reflect.Ptr:
		return g.fieldToSchema(meta, t.Elem())
	default:
		schema.Type = "string"
	}

	return schema
}

// GenerateJSON генерирует JSON строку схемы
func (g *Generator) GenerateJSON() (string, error) {
	schema := g.Generate()
	data, err := json.MarshalIndent(schema, "", "  ")
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// toKebabCase конвертирует PascalCase в kebab-case
func toKebabCase(s string) string {
	var result []rune
	for i, r := range s {
		if i > 0 && r >= 'A' && r <= 'Z' {
			result = append(result, '-')
		}
		result = append(result, r)
	}
	return strings.ToLower(string(result))
}

// intPtr возвращает указатель на int
func intPtr(i int) *int {
	return &i
}
