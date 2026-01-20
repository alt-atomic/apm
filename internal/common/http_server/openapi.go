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

package http_server

import (
	"encoding/json"
	"reflect"
	"strings"
)

// OpenAPISpec OpenAPI 3.0 спецификация
type OpenAPISpec struct {
	OpenAPI    string                `json:"openapi"`
	Info       OpenAPIInfo           `json:"info"`
	Servers    []OpenAPIServer       `json:"servers,omitempty"`
	Paths      map[string]PathItem   `json:"paths"`
	Components OpenAPIComponents     `json:"components,omitempty"`
	Tags       []OpenAPITag          `json:"tags,omitempty"`
	Security   []map[string][]string `json:"security,omitempty"`
}

// OpenAPIInfo информация об API
type OpenAPIInfo struct {
	Title       string `json:"title"`
	Description string `json:"description,omitempty"`
	Version     string `json:"version"`
}

// OpenAPIServer сервер API
type OpenAPIServer struct {
	URL         string `json:"url"`
	Description string `json:"description,omitempty"`
}

// OpenAPITag тег для группировки
type OpenAPITag struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}

// PathItem элемент пути
type PathItem struct {
	Get    *Operation `json:"get,omitempty"`
	Post   *Operation `json:"post,omitempty"`
	Put    *Operation `json:"put,omitempty"`
	Delete *Operation `json:"delete,omitempty"`
}

// Operation операция API
type Operation struct {
	Tags        []string            `json:"tags,omitempty"`
	Summary     string              `json:"summary,omitempty"`
	Description string              `json:"description,omitempty"`
	OperationID string              `json:"operationId,omitempty"`
	Parameters  []Parameter         `json:"parameters,omitempty"`
	RequestBody *RequestBody        `json:"requestBody,omitempty"`
	Responses   map[string]Response `json:"responses"`
}

// Parameter параметр запроса
type Parameter struct {
	Name        string  `json:"name"`
	In          string  `json:"in"`
	Description string  `json:"description,omitempty"`
	Required    bool    `json:"required,omitempty"`
	Schema      *Schema `json:"schema,omitempty"`
}

// RequestBody тело запроса
type RequestBody struct {
	Description string               `json:"description,omitempty"`
	Required    bool                 `json:"required,omitempty"`
	Content     map[string]MediaType `json:"content"`
}

// Response ответ API
type Response struct {
	Description string               `json:"description"`
	Content     map[string]MediaType `json:"content,omitempty"`
}

// MediaType тип медиа
type MediaType struct {
	Schema *Schema `json:"schema,omitempty"`
}

// Schema схема данных
type Schema struct {
	Type        string             `json:"type,omitempty"`
	Format      string             `json:"format,omitempty"`
	Description string             `json:"description,omitempty"`
	Properties  map[string]*Schema `json:"properties,omitempty"`
	Items       *Schema            `json:"items,omitempty"`
	Ref         string             `json:"$ref,omitempty"`
	Required    []string           `json:"required,omitempty"`
}

// OpenAPIComponents компоненты
type OpenAPIComponents struct {
	Schemas         map[string]*Schema        `json:"schemas,omitempty"`
	SecuritySchemes map[string]SecurityScheme `json:"securitySchemes,omitempty"`
}

// SecurityScheme схема безопасности
type SecurityScheme struct {
	Type         string `json:"type"`
	Scheme       string `json:"scheme,omitempty"`
	BearerFormat string `json:"bearerFormat,omitempty"`
	Description  string `json:"description,omitempty"`
}

// OpenAPIGenerator генератор OpenAPI из registry
type OpenAPIGenerator struct {
	registry *Registry
	version  string
	isAtomic bool
}

// NewOpenAPIGenerator создает новый генератор
func NewOpenAPIGenerator(registry *Registry, version string, isAtomic bool) *OpenAPIGenerator {
	return &OpenAPIGenerator{
		registry: registry,
		version:  version,
		isAtomic: isAtomic,
	}
}

// GenerateOpenAPI генерирует OpenAPI спецификацию как map для интерфейса http_server
func (g *OpenAPIGenerator) GenerateOpenAPI() map[string]interface{} {
	spec := g.Generate()
	// Конвертируем в map через JSON
	data, _ := json.Marshal(spec)
	var result map[string]interface{}
	_ = json.Unmarshal(data, &result)
	return result
}

// Generate генерирует OpenAPI спецификацию
func (g *OpenAPIGenerator) Generate() *OpenAPISpec {
	spec := &OpenAPISpec{
		OpenAPI: "3.0.3",
		Info: OpenAPIInfo{
			Title:       "APM HTTP API",
			Description: "Atomic Package Manager HTTP API for system package management. Use token format: 'read:password' or 'manage:password'",
			Version:     g.version,
		},
		Servers: []OpenAPIServer{},
		Paths:   make(map[string]PathItem),
		Tags:    []OpenAPITag{},
		Components: OpenAPIComponents{
			Schemas: g.generateSchemas(),
			SecuritySchemes: map[string]SecurityScheme{
				"bearerAuth": {
					Type:         "http",
					Scheme:       "bearer",
					BearerFormat: "permission:token",
					Description:  "API token authentication. Format: 'read:token' for read-only access or 'manage:token' for full access",
				},
			},
		},
		Security: []map[string][]string{
			{"bearerAuth": {}},
		},
	}

	// Собираем теги из всех endpoints
	tagsSet := make(map[string]bool)

	for _, ep := range g.registry.GetHTTPEndpoints() {
		// Пропускаем image endpoints для не-атомарных систем
		if !g.isAtomic && strings.Contains(ep.HTTPPath, "/image") {
			continue
		}

		// Собираем уникальные теги
		for _, tag := range ep.Tags {
			tagsSet[tag] = true
		}

		pathItem, exists := spec.Paths[ep.HTTPPath]
		if !exists {
			pathItem = PathItem{}
		}

		op := g.createOperation(ep)

		switch ep.HTTPMethod {
		case "GET":
			pathItem.Get = op
		case "POST":
			pathItem.Post = op
		case "PUT":
			pathItem.Put = op
		case "DELETE":
			pathItem.Delete = op
		}

		spec.Paths[ep.HTTPPath] = pathItem
	}

	// Добавляем собранные теги
	for tag := range tagsSet {
		spec.Tags = append(spec.Tags, OpenAPITag{Name: tag})
	}

	return spec
}

// createOperation создает операцию для endpoint
func (g *OpenAPIGenerator) createOperation(ep Endpoint) *Operation {
	op := &Operation{
		Tags:        ep.Tags,
		Summary:     ep.Summary,
		Description: ep.Description,
		OperationID: strings.ToLower(ep.HTTPMethod) + "_" + strings.ReplaceAll(strings.ReplaceAll(strings.ReplaceAll(ep.HTTPPath, "/", "_"), "{", ""), "}", ""),
		Responses: map[string]Response{
			"200": {
				Description: "Successful response",
				Content: map[string]MediaType{
					"application/json": {
						Schema: &Schema{Ref: "#/components/schemas/APIResponse"},
					},
				},
			},
			"400": {
				Description: "Bad request",
				Content: map[string]MediaType{
					"application/json": {
						Schema: &Schema{Ref: "#/components/schemas/APIResponse"},
					},
				},
			},
		},
	}

	// Path параметры
	for _, param := range ep.PathParams {
		op.Parameters = append(op.Parameters, Parameter{
			Name:     param,
			In:       "path",
			Required: true,
			Schema:   &Schema{Type: "string"},
		})
	}

	// Query параметры
	for _, qp := range ep.QueryParams {
		op.Parameters = append(op.Parameters, Parameter{
			Name:        qp.Name,
			In:          "query",
			Required:    qp.Required,
			Description: qp.Description,
			Schema:      &Schema{Type: g.mapType(qp.Type)},
		})
	}

	// Transaction header
	op.Parameters = append(op.Parameters, Parameter{
		Name:        "X-Transaction-ID",
		In:          "header",
		Description: "Transaction ID for tracking",
		Schema:      &Schema{Type: "string"},
	})

	// Request body для POST/PUT/DELETE
	if ep.HTTPMethod == "POST" || ep.HTTPMethod == "PUT" || ep.HTTPMethod == "DELETE" {
		// Если есть RequestType - используем его
		if ep.RequestType != "" {
			op.RequestBody = &RequestBody{
				Required: true,
				Content: map[string]MediaType{
					"application/json": {
						Schema: &Schema{Ref: "#/components/schemas/" + ep.RequestType},
					},
				},
			}
		} else if len(ep.ParamMappings) > 0 {
			// Генерируем body schema из ParamMappings
			bodySchema := g.generateBodySchemaFromMappings(ep.ParamMappings)
			if bodySchema != nil {
				op.RequestBody = &RequestBody{
					Required: true,
					Content: map[string]MediaType{
						"application/json": {
							Schema: bodySchema,
						},
					},
				}
			}
		}
	}

	return op
}

// generateBodySchemaFromMappings генерирует схему body из ParamMappings
func (g *OpenAPIGenerator) generateBodySchemaFromMappings(mappings []ParamMapping) *Schema {
	props := make(map[string]*Schema)
	var required []string

	for _, pm := range mappings {
		if pm.Source != "body" {
			continue
		}

		props[pm.Name] = &Schema{Type: g.mapType(pm.Type)}
		if pm.Type == "[]string" {
			props[pm.Name] = &Schema{
				Type:  "array",
				Items: &Schema{Type: "string"},
			}
		}

		if pm.Default == "" {
			required = append(required, pm.Name)
		}
	}

	if len(props) == 0 {
		return nil
	}

	return &Schema{
		Type:       "object",
		Properties: props,
		Required:   required,
	}
}

// mapType преобразует тип в OpenAPI тип
func (g *OpenAPIGenerator) mapType(typ string) string {
	// Если тип уже нормализован (boolean, integer), возвращаем как есть
	if typ == "boolean" || typ == "integer" {
		return typ
	}

	switch typ {
	case "int", "int32", "int64":
		return "integer"
	case "bool":
		return "boolean"
	case "[]string":
		return "array"
	default:
		return "string"
	}
}

// generateSchemas генерирует схемы из responseTypes
func (g *OpenAPIGenerator) generateSchemas() map[string]*Schema {
	schemas := map[string]*Schema{
		"APIResponse": {
			Type: "object",
			Properties: map[string]*Schema{
				"data":        {Type: "object", Description: "Response data"},
				"error":       {Type: "boolean", Description: "Error flag"},
				"transaction": {Type: "string", Description: "Transaction ID"},
			},
		},
	}

	// Добавляем схемы из registry
	for name, typ := range g.registry.GetResponseTypes() {
		if _, exists := schemas[name]; !exists {
			schemas[name] = g.typeToSchema(typ)
		}
	}

	return schemas
}

// typeToSchema преобразует reflect.Type в Schema
func (g *OpenAPIGenerator) typeToSchema(typ reflect.Type) *Schema {
	if typ.Kind() == reflect.Ptr {
		typ = typ.Elem()
	}

	switch typ.Kind() {
	case reflect.String:
		return &Schema{Type: "string"}
	case reflect.Int, reflect.Int32, reflect.Int64:
		return &Schema{Type: "integer"}
	case reflect.Float32, reflect.Float64:
		return &Schema{Type: "number"}
	case reflect.Bool:
		return &Schema{Type: "boolean"}
	case reflect.Slice:
		return &Schema{Type: "array", Items: g.typeToSchema(typ.Elem())}
	case reflect.Struct:
		props := make(map[string]*Schema)
		for i := 0; i < typ.NumField(); i++ {
			field := typ.Field(i)
			jsonTag := field.Tag.Get("json")
			if jsonTag == "-" {
				continue
			}
			name := strings.Split(jsonTag, ",")[0]
			if name == "" {
				name = field.Name
			}
			props[name] = g.typeToSchema(field.Type)
		}
		return &Schema{Type: "object", Properties: props}
	case reflect.Map:
		return &Schema{Type: "object"}
	case reflect.Interface:
		return &Schema{Type: "object"}
	}

	return &Schema{Type: "object"}
}
