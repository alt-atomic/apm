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
	"reflect"
)

// Endpoint описывает API endpoint
type Endpoint struct {
	// Имя метода в Actions
	Method string
	// HTTP метод (GET, POST, PUT, DELETE)
	HTTPMethod string
	// HTTP путь (/api/v1/packages/install)
	HTTPPath string
	// Тип запроса (для POST/PUT) - опционально
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
	// Маппинг параметров - для OpenAPI документации body схемы
	ParamMappings []ParamMapping
}

// QueryParam описывает query параметр
type QueryParam struct {
	Name        string
	Type        string
	Required    bool
	Description string
}

// ParamMapping описывает маппинг HTTP параметра (для OpenAPI документации)
type ParamMapping struct {
	// Source откуда брать: path, query, body
	Source string
	// Name имя в HTTP запросе
	Name string
	// ArgIndex индекс аргумента в методе
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

// RegisterEndpoints регистрирует endpoints
func (r *Registry) RegisterEndpoints(endpoints []Endpoint) {
	r.endpoints = append(r.endpoints, endpoints...)
}

// RegisterResponseTypes регистрирует типы ответов для OpenAPI схем
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
