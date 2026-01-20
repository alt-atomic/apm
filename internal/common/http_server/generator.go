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
	"apm/internal/common/app"
	"apm/internal/common/helper"
	"apm/internal/common/reply"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"reflect"
	"strconv"
	"strings"
)

// HTTPGenerator генерирует HTTP handlers из registry
type HTTPGenerator struct {
	registry  *Registry
	appConfig *app.Config
	ctx       context.Context
}

// NewHTTPGenerator создает новый генератор
func NewHTTPGenerator(registry *Registry, appConfig *app.Config, ctx context.Context) *HTTPGenerator {
	return &HTTPGenerator{
		registry:  registry,
		appConfig: appConfig,
		ctx:       ctx,
	}
}

// RegisterRoutes регистрирует все HTTP маршруты из registry
func (g *HTTPGenerator) RegisterRoutes(mux *http.ServeMux, actions interface{}, isAtomic bool) map[string]bool {
	actionsValue := reflect.ValueOf(actions)
	registered := make(map[string]bool)

	for _, ep := range g.registry.GetHTTPEndpoints() {
		if !isAtomic && strings.Contains(ep.HTTPPath, "/image") {
			continue
		}

		handler := g.createHandler(ep, actionsValue)
		route := ep.HTTPMethod + " " + ep.HTTPPath
		mux.HandleFunc(route, handler)
		registered[route] = true
	}

	return registered
}

// createHandler создает HTTP handler для endpoint
func (g *HTTPGenerator) createHandler(ep Endpoint, actionsValue reflect.Value) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		method := actionsValue.MethodByName(ep.Method)
		if !method.IsValid() {
			g.writeError(w, fmt.Errorf("method %s not found", ep.Method), http.StatusInternalServerError)
			return
		}

		// Создаем контекст с transaction
		ctx := g.ctxWithTransaction(r)

		// Подготавливаем аргументы для вызова метода
		args, err := g.prepareArgs(ep, method, r, ctx)
		if err != nil {
			g.writeError(w, err, http.StatusBadRequest)
			return
		}

		// Вызываем метод
		results := method.Call(args)

		// Обрабатываем результаты (ожидаем (*reply.APIResponse, error))
		if len(results) != 2 {
			g.writeError(w, fmt.Errorf("unexpected return values"), http.StatusInternalServerError)
			return
		}

		// Проверяем ошибку
		if !results[1].IsNil() {
			err := results[1].Interface().(error)
			g.writeError(w, err, http.StatusInternalServerError)
			return
		}

		// Проверяем response
		if results[0].IsNil() {
			g.writeError(w, fmt.Errorf("nil response"), http.StatusInternalServerError)
			return
		}

		resp := results[0].Interface().(*reply.APIResponse)
		g.writeJSON(w, *resp)
	}
}

// prepareArgs подготавливает аргументы для вызова метода
func (g *HTTPGenerator) prepareArgs(ep Endpoint, method reflect.Value, r *http.Request, ctx context.Context) ([]reflect.Value, error) {
	methodType := method.Type()
	numParams := methodType.NumIn()

	args := make([]reflect.Value, numParams)

	// Парсим body один раз для всех body параметров
	var bodyData map[string]json.RawMessage
	if r.Method == http.MethodPost || r.Method == http.MethodPut || r.Method == http.MethodPatch {
		if err := json.NewDecoder(r.Body).Decode(&bodyData); err != nil && len(ep.ParamMappings) > 0 {
			// Игнорируем ошибку если нет body параметров
			hasBodyParams := false
			for _, pm := range ep.ParamMappings {
				if pm.Source == "body" {
					hasBodyParams = true
					break
				}
			}
			if hasBodyParams {
				return nil, fmt.Errorf("failed to parse request body: %w", err)
			}
		}
	}

	for i := 0; i < numParams; i++ {
		paramType := methodType.In(i)

		if paramType.Implements(reflect.TypeOf((*context.Context)(nil)).Elem()) {
			args[i] = reflect.ValueOf(ctx)
			continue
		}

		// Struct заполняется из body (POST) или query (GET) по json тегам
		if paramType.Kind() == reflect.Struct {
			if len(bodyData) > 0 {
				reqValue := reflect.New(paramType).Interface()
				fullBody, _ := json.Marshal(bodyData)
				if err := json.Unmarshal(fullBody, reqValue); err != nil {
					return nil, fmt.Errorf("failed to parse request body: %w", err)
				}
				args[i] = reflect.ValueOf(reqValue).Elem()
			} else {
				args[i] = g.fillStructFromQuery(r, paramType)
			}
			continue
		}

		// Для простых типов ищем маппинг по индексу или по имени в query
		var mapping *ParamMapping
		for _, pm := range ep.ParamMappings {
			if pm.ArgIndex == i {
				mapping = &pm
				break
			}
		}

		if mapping != nil {
			value, err := g.getParamFromMapping(*mapping, r, paramType, bodyData)
			if err != nil {
				return nil, err
			}
			args[i] = value
		} else {
			// Ищем любой @param с source=query подходящего типа
			found := false
			for _, pm := range ep.ParamMappings {
				if pm.Source == "query" {
					value := r.URL.Query().Get(pm.Name)
					if value == "" {
						value = pm.Default
					}
					if value != "" {
						if v, err := g.convertToType(value, paramType); err == nil {
							args[i] = v
							found = true
							break
						}
					}
				}
			}
			if !found {
				// Если не нашли параметр - используем zero value
				args[i] = reflect.Zero(paramType)
			}
		}
	}

	return args, nil
}

// getParamFromMapping получает значение параметра по маппингу
func (g *HTTPGenerator) getParamFromMapping(pm ParamMapping, r *http.Request, paramType reflect.Type, bodyData map[string]json.RawMessage) (reflect.Value, error) {
	switch pm.Source {
	case "path":
		value := r.PathValue(pm.Name)
		if value == "" && pm.Default != "" {
			value = pm.Default
		}
		if value == "" {
			return reflect.Zero(paramType), nil
		}
		return g.convertToType(value, paramType)

	case "query":
		value := r.URL.Query().Get(pm.Name)
		if value == "" && pm.Default != "" {
			value = pm.Default
		}
		if value == "" {
			return reflect.Zero(paramType), nil
		}
		return g.convertToType(value, paramType)

	case "body":
		raw, ok := bodyData[pm.Name]
		if !ok {
			// Используем default если есть
			if pm.Default != "" {
				return g.convertToType(pm.Default, paramType)
			}
			return reflect.Zero(paramType), nil
		}

		// Создаём значение нужного типа и декодируем
		val := reflect.New(paramType).Interface()
		if err := json.Unmarshal(raw, val); err != nil {
			return reflect.Value{}, fmt.Errorf("failed to parse body param %s: %w", pm.Name, err)
		}
		return reflect.ValueOf(val).Elem(), nil
	}

	return reflect.Zero(paramType), nil
}

// fillStructFromQuery заполняет структуру из query параметров по json тегам
func (g *HTTPGenerator) fillStructFromQuery(r *http.Request, structType reflect.Type) reflect.Value {
	structPtr := reflect.New(structType)
	structVal := structPtr.Elem()
	query := r.URL.Query()

	for i := 0; i < structType.NumField(); i++ {
		field := structType.Field(i)
		fieldVal := structVal.Field(i)

		if !fieldVal.CanSet() {
			continue
		}

		// Получаем имя из json тега
		jsonTag := field.Tag.Get("json")
		if jsonTag == "-" {
			continue
		}
		name := strings.Split(jsonTag, ",")[0]
		if name == "" {
			name = field.Name
		}

		// Получаем значение из query
		queryVal := query.Get(name)
		if queryVal == "" {
			// Проверяем множественные значения для слайсов
			if field.Type.Kind() == reflect.Slice {
				values := query[name]
				if len(values) > 0 {
					slice := reflect.MakeSlice(field.Type, len(values), len(values))
					for j, v := range values {
						if field.Type.Elem().Kind() == reflect.String {
							slice.Index(j).SetString(v)
						}
					}
					fieldVal.Set(slice)
				}
			}
			continue
		}

		// Конвертируем и устанавливаем значение
		switch field.Type.Kind() {
		case reflect.String:
			fieldVal.SetString(queryVal)
		case reflect.Int, reflect.Int64:
			if v, err := strconv.ParseInt(queryVal, 10, 64); err == nil {
				fieldVal.SetInt(v)
			}
		case reflect.Bool:
			fieldVal.SetBool(queryVal == "true" || queryVal == "1")
		case reflect.Slice:
			if field.Type.Elem().Kind() == reflect.String {
				// Поддержка формата ?filters=a&filters=b или ?filters=a,b
				values := query[name]
				if len(values) == 1 && strings.Contains(values[0], ",") {
					values = strings.Split(values[0], ",")
				}
				slice := reflect.MakeSlice(field.Type, len(values), len(values))
				for j, v := range values {
					slice.Index(j).SetString(strings.TrimSpace(v))
				}
				fieldVal.Set(slice)
			}
		}
	}

	return structVal
}

// convertToType конвертирует строку в нужный тип
func (g *HTTPGenerator) convertToType(value string, typ reflect.Type) (reflect.Value, error) {
	switch typ.Kind() {
	case reflect.String:
		return reflect.ValueOf(value), nil
	case reflect.Int, reflect.Int64:
		v, err := strconv.ParseInt(value, 10, 64)
		if err != nil {
			return reflect.Value{}, err
		}
		return reflect.ValueOf(int(v)).Convert(typ), nil
	case reflect.Bool:
		v := value == "true" || value == "1"
		return reflect.ValueOf(v), nil
	case reflect.Slice:
		if typ.Elem().Kind() == reflect.String {
			parts := strings.Split(value, ",")
			slice := reflect.MakeSlice(typ, len(parts), len(parts))
			for i, p := range parts {
				slice.Index(i).SetString(strings.TrimSpace(p))
			}
			return slice, nil
		}
	}

	return reflect.Zero(typ), nil
}

// ctxWithTransaction создает контекст с transaction
func (g *HTTPGenerator) ctxWithTransaction(r *http.Request) context.Context {
	tx := r.Header.Get("X-Transaction-ID")
	if tx == "" {
		tx = r.URL.Query().Get("transaction")
	}
	return context.WithValue(g.ctx, helper.TransactionKey, tx)
}

// writeJSON отправляет JSON ответ
func (g *HTTPGenerator) writeJSON(w http.ResponseWriter, resp reply.APIResponse) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	if resp.Error {
		w.WriteHeader(http.StatusBadRequest)
	}
	_ = json.NewEncoder(w).Encode(resp)
}

// writeError отправляет ошибку
func (g *HTTPGenerator) writeError(w http.ResponseWriter, err error, code int) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(reply.APIResponse{
		Data:  map[string]interface{}{"message": err.Error()},
		Error: true,
	})
}
