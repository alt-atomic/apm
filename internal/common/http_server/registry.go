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
	"go/ast"
	"go/parser"
	"go/token"
	"reflect"
	"regexp"
	"strings"
)

// Endpoint описывает API endpoint
type Endpoint struct {
	// Имя метода в Actions
	Method string
	// HTTP метод (GET, POST, PUT, DELETE)
	HTTPMethod string
	// HTTP путь (/api/v1/packages/install)
	HTTPPath string
	// Имя D-Bus метода
	DBusMethod string
	// Тип запроса (для POST/PUT) - опционально, можно не использовать
	RequestType string
	// Тип ответа
	ResponseType string
	// Требуемое разрешение (manage, read)
	Permission string
	// Краткое описание
	Summary string
	// Полное описание
	Description string
	// Теги для группировки
	Tags []string
	// Query параметры (для GET) - для OpenAPI документации
	QueryParams []QueryParam
	// Path параметры
	PathParams []string
	// Маппинг параметров на аргументы метода
	ParamMappings []ParamMapping
}

// QueryParam описывает query параметр
type QueryParam struct {
	Name        string
	Type        string
	Required    bool
	Description string
}

// ParamMapping описывает маппинг HTTP параметра на аргумент метода
type ParamMapping struct {
	// Source откуда брать: path, query, body
	Source string
	// Name имя в HTTP запросе
	Name string
	// ArgIndex индекс аргумента в методе (0 = ctx, 1 = первый аргумент)
	ArgIndex int
	// Type тип параметра
	Type string
	// Default значение по умолчанию
	Default string
}

// Registry хранит все зарегистрированные endpoints
type Registry struct {
	endpoints     []Endpoint
	responseTypes map[string]reflect.Type
}

// NewRegistry создает новый registry
func NewRegistry() *Registry {
	return &Registry{
		endpoints:     make([]Endpoint, 0),
		responseTypes: make(map[string]reflect.Type),
	}
}

// RegisterResponseTypes регистрирует несколько типов ответов
func (r *Registry) RegisterResponseTypes(types map[string]reflect.Type) {
	for name, typ := range types {
		r.responseTypes[name] = typ
	}
}

// GetResponseTypes возвращает все зарегистрированные типы ответов
func (r *Registry) GetResponseTypes() map[string]reflect.Type {
	return r.responseTypes
}

// GetHTTPEndpoints возвращает HTTP endpoints
func (r *Registry) GetHTTPEndpoints() []Endpoint {
	var result []Endpoint
	for _, ep := range r.endpoints {
		if ep.HTTPPath != "" {
			result = append(result, ep)
		}
	}
	return result
}

// ParseAnnotations парсит исходный код и извлекает аннотации из комментариев
func (r *Registry) ParseAnnotations(sourceCode string) error {
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, "", sourceCode, parser.ParseComments)
	if err != nil {
		return err
	}

	// Собираем комментарии перед функциями
	for _, decl := range node.Decls {
		funcDecl, ok := decl.(*ast.FuncDecl)
		if !ok {
			continue
		}

		// Проверяем что это метод Actions
		if funcDecl.Recv == nil || len(funcDecl.Recv.List) == 0 {
			continue
		}

		recvType := funcDecl.Recv.List[0].Type
		if starExpr, ok := recvType.(*ast.StarExpr); ok {
			if ident, ok := starExpr.X.(*ast.Ident); ok {
				if ident.Name != "Actions" {
					continue
				}
			}
		}

		if funcDecl.Doc == nil {
			continue
		}
		comment := funcDecl.Doc.Text()

		// Парсим аннотации
		ep := r.parseEndpointAnnotations(funcDecl.Name.Name, comment)
		if ep != nil {
			r.endpoints = append(r.endpoints, *ep)
		}
	}

	return nil
}

// parseEndpointAnnotations парсит аннотации из комментария
func (r *Registry) parseEndpointAnnotations(methodName string, comment string) *Endpoint {
	ep := &Endpoint{
		Method: methodName,
	}

	hasAnnotations := false
	lines := strings.Split(comment, "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)

		// @http POST /api/v1/packages/install
		if match := regexp.MustCompile(`@http\s+(\w+)\s+(\S+)`).FindStringSubmatch(line); match != nil {
			ep.HTTPMethod = match[1]
			ep.HTTPPath = match[2]
			hasAnnotations = true

			// Извлекаем path параметры
			pathParamRe := regexp.MustCompile(`\{(\w+)\}`)
			matches := pathParamRe.FindAllStringSubmatch(ep.HTTPPath, -1)
			for _, m := range matches {
				ep.PathParams = append(ep.PathParams, m[1])
			}
		}

		// @dbus_doc MethodName
		if match := regexp.MustCompile(`@dbus_doc\s+(\w+)`).FindStringSubmatch(line); match != nil {
			ep.DBusMethod = match[1]
			hasAnnotations = true
		}

		// @request RequestType
		if match := regexp.MustCompile(`@request\s+(\w+)`).FindStringSubmatch(line); match != nil {
			ep.RequestType = match[1]
			hasAnnotations = true
		}

		// @response ResponseType
		if match := regexp.MustCompile(`@response\s+(\w+)`).FindStringSubmatch(line); match != nil {
			ep.ResponseType = match[1]
			hasAnnotations = true
		}

		// @permission manage|read
		if match := regexp.MustCompile(`@permission\s+(\w+)`).FindStringSubmatch(line); match != nil {
			ep.Permission = match[1]
			hasAnnotations = true
		}

		// @summary Description
		if match := regexp.MustCompile(`@summary\s+(.+)`).FindStringSubmatch(line); match != nil {
			ep.Summary = match[1]
			hasAnnotations = true
		}

		// @description Description
		if match := regexp.MustCompile(`@description\s+(.+)`).FindStringSubmatch(line); match != nil {
			ep.Description = match[1]
			hasAnnotations = true
		}

		// @tag TagName
		if match := regexp.MustCompile(`@tag\s+(\w+)`).FindStringSubmatch(line); match != nil {
			ep.Tags = append(ep.Tags, match[1])
			hasAnnotations = true
		}

		// @query name:type:required:description
		if match := regexp.MustCompile(`@query\s+(\w+):(\w+):(\w+):?(.+)?`).FindStringSubmatch(line); match != nil {
			qp := QueryParam{
				Name:     match[1],
				Type:     match[2],
				Required: match[3] == "true",
			}
			if len(match) > 4 {
				qp.Description = strings.TrimSpace(match[4])
			}
			ep.QueryParams = append(ep.QueryParams, qp)
			hasAnnotations = true
		}

		if match := regexp.MustCompile(`@param\s+(\w+):(\w+):([^\s:]+)(?::(\S+))?`).FindStringSubmatch(line); match != nil {
			pm := ParamMapping{
				Name:     match[1],
				Source:   match[2],
				Type:     match[3],
				ArgIndex: len(ep.ParamMappings) + 1,
			}
			if len(match) > 4 && match[4] != "" {
				pm.Default = match[4]
			}
			ep.ParamMappings = append(ep.ParamMappings, pm)
			hasAnnotations = true

			if pm.Source == "query" {
				ep.QueryParams = append(ep.QueryParams, QueryParam{
					Name:     pm.Name,
					Type:     pm.Type,
					Required: pm.Default == "",
				})
			}
		}
	}

	if !hasAnnotations {
		return nil
	}

	// Теги должны быть указаны через @tag в аннотациях
	// Если не указаны — используем имя модуля из пути как fallback
	if len(ep.Tags) == 0 && ep.HTTPPath != "" {
		parts := strings.Split(ep.HTTPPath, "/")
		if len(parts) >= 4 {
			ep.Tags = []string{parts[3]}
		}
	}

	return ep
}
