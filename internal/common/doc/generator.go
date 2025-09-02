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

package doc

import (
	"apm/lib"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"reflect"
	"sort"
	"strings"
	"time"
)

// DBusMethodInfo содержит информацию о DBus методе
type DBusMethodInfo struct {
	Name         string
	ResponseType string
	Parameters   []DBusParameter
}

// DBusParameter описывает параметр метода
type DBusParameter struct {
	Name string
	Type string
}

// Config конфигурация для генерации документации
type Config struct {
	ModuleName    string
	DBusInterface string
	ServerPort    string
	DBusWrapper   interface{}
	DBusMethods   map[string]reflect.Type
}

// Generator генератор документации
type Generator struct {
	config Config
}

// NewGenerator создает новый генератор документации
func NewGenerator(config Config) *Generator {
	return &Generator{config: config}
}

// GenerateDBusDocHTML генерирует HTML документацию для DBus API
func (g *Generator) GenerateDBusDocHTML() string {
	methods := g.reflectDBusMethods()

	html := fmt.Sprintf(`<!DOCTYPE html>
<html>
<head>
    <title>APM %s DBus API Documentation</title>
    <meta charset="utf-8">
    <style>
        body { font-family: Arial, sans-serif; margin: 40px; background: #1d1d20; color: white; }
        .method { border: 1px solid #ddd; margin: 20px 0; padding: 20px; border-radius: 5px; }
        .method-name { color: #0066cc; font-size: 24px; font-weight: bold; }
        .description { margin: 10px 0; }
        .response-type { color: #cc6600; font-weight: bold; }
        .json-example { background: #f5f5f5; padding: 15px; border-radius: 3px; margin: 10px 0; background: #2b2b31; }
        .dbus-command { background: #2b2b31; padding: 15px; border-radius: 3px; margin: 10px 0; }
        pre { margin: 0; overflow-x: auto; }
        .parameters { margin: 10px 0; }
        .param { margin: 5px 0; }
    </style>
</head>
<body>
    <h1>APM %s DBus API Documentation</h1>
    <p>Автоматически сгенерированная документация для DBus интерфейса APM %s.</p>
`, g.config.ModuleName, g.config.ModuleName, g.config.ModuleName)

	for _, method := range methods {
		html += fmt.Sprintf(`
    <div class="method">
        <div class="method-name">%s</div>
        <div class="parameters">
            <strong>Parameters:</strong>
            %s
        </div>
		<div class="response-type">Response Type: %s</div>
        <div class="dbus-command">
            <strong>DBUS Command:</strong>
            <pre>%s</pre>
        </div>
        <div class="json-example">
            <strong>Response Structure:</strong>
            <pre>%s</pre>
        </div>
    </div>
`, method.Name,
			g.formatParameters(method.Parameters),
			method.ResponseType,
			g.generateDBusCommand(method),
			g.generateJSONExample(method.ResponseType))
	}

	html += `
</body>
</html>`

	return html
}

// reflectDBusMethods использует рефлексию для получения методов DBusWrapper
func (g *Generator) reflectDBusMethods() []DBusMethodInfo {
	var methods []DBusMethodInfo

	if g.config.DBusWrapper == nil {
		return methods
	}

	wrapperType := reflect.TypeOf(g.config.DBusWrapper)

	// Сортируем имена методов для стабильного порядка
	var methodNames []string
	for methodName := range g.config.DBusMethods {
		methodNames = append(methodNames, methodName)
	}
	sort.Strings(methodNames)

	for _, methodName := range methodNames {
		responseType := g.config.DBusMethods[methodName]
		method, exists := wrapperType.MethodByName(methodName)
		if !exists {
			continue
		}

		methodType := method.Type

		if methodType.NumOut() != 2 {
			continue
		}

		// Извлекаем параметры метода
		var params []DBusParameter
		paramIndex := 1
		for j := 1; j < methodType.NumIn(); j++ { // Пропускаем receiver
			paramType := methodType.In(j)

			// Пропускаем dbus.Sender и transaction
			if strings.Contains(paramType.String(), "dbus.Sender") {
				continue
			}

			params = append(params, DBusParameter{
				Name: fmt.Sprintf("param%d", paramIndex),
				Type: g.getTypeString(paramType),
			})
			paramIndex++
		}

		methods = append(methods, DBusMethodInfo{
			Name:         methodName,
			ResponseType: responseType.Name(),
			Parameters:   params,
		})
	}

	return methods
}

// getTypeString возвращает строковое представление типа
func (g *Generator) getTypeString(t reflect.Type) string {
	switch t.Kind() {
	case reflect.String:
		return "string"
	case reflect.Bool:
		return "bool"
	case reflect.Int, reflect.Int32, reflect.Int64:
		return "int"
	case reflect.Slice:
		return "[]" + g.getTypeString(t.Elem())
	default:
		return t.String()
	}
}

// formatParameters форматирует параметры для HTML
func (g *Generator) formatParameters(params []DBusParameter) string {
	if len(params) == 0 {
		return "None"
	}

	var parts []string
	for _, param := range params {
		parts = append(parts, fmt.Sprintf("<div class=\"param\">%s (%s)</div>", param.Name, param.Type))
	}
	return strings.Join(parts, "")
}

// generateDBusCommand генерирует пример dbus-send команды
func (g *Generator) generateDBusCommand(method DBusMethodInfo) string {
	cmd := fmt.Sprintf("dbus-send --system --print-reply --dest=org.altlinux.APM /org/altlinux/APM %s.%s", g.config.DBusInterface, method.Name)

	for _, param := range method.Parameters {
		switch param.Type {
		case "string":
			cmd += " string:\"example\""
		case "[]string":
			cmd += " array:string:\"package1\",\"package2\""
		case "bool":
			cmd += " boolean:true"
		case "int":
			cmd += " int32:10"
		default:
			cmd += " string:\"" + param.Name + "\""
		}
	}

	return cmd
}

// generateJSONExample создаёт JSON пример используя рефлексию
func (g *Generator) generateJSONExample(responseType string) string {
	var example interface{}

	// Ищем тип в карте методов
	for _, typ := range g.config.DBusMethods {
		if typ.Name() == responseType {
			example = g.createExampleStruct(typ)
			break
		}
	}

	if example == nil {
		return `{"message": "Example response", "data": "See ` + responseType + ` structure above"}`
	}

	jsonBytes, err := json.MarshalIndent(example, "", "  ")
	if err != nil {
		return "Error generating JSON example"
	}

	return string(jsonBytes)
}

// createExampleStruct создаёт пример структуры с заполненными значениями
func (g *Generator) createExampleStruct(typ reflect.Type) interface{} {
	if typ.Kind() == reflect.Slice {
		if typ.Elem().Name() == "FilterField" {
			slice := reflect.MakeSlice(typ, 1, 1)
			elem := slice.Index(0)
			exampleField := g.createExampleStruct(typ.Elem())
			elem.Set(reflect.ValueOf(exampleField))
			return slice.Interface()
		}
		return reflect.MakeSlice(typ, 0, 0).Interface()
	}

	if typ.Kind() != reflect.Struct {
		return reflect.Zero(typ).Interface()
	}

	value := reflect.New(typ).Elem()

	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)
		fieldValue := value.Field(i)

		if !fieldValue.CanSet() {
			continue
		}

		switch field.Type.Kind() {
		case reflect.String:
			if field.Name == "Message" {
				fieldValue.SetString("Example message")
			} else {
				fieldValue.SetString("example")
			}
		case reflect.Int, reflect.Int32, reflect.Int64:
			fieldValue.SetInt(42)
		case reflect.Bool:
			fieldValue.SetBool(true)
		case reflect.Slice:
			slice := reflect.MakeSlice(field.Type, 0, 0)
			fieldValue.Set(slice)
		case reflect.Struct:
			fieldValue.Set(reflect.ValueOf(g.createExampleStruct(field.Type)))
		default:
			continue
		}
	}

	return value.Interface()
}

// StartDocServer запускает веб-сервер с документацией
func (g *Generator) StartDocServer(ctx context.Context) error {
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		html := g.GenerateDBusDocHTML()
		_, err := fmt.Fprint(w, html)
		if err != nil {
			lib.Log.Fatal(err.Error())
			return
		}
	})

	server := &http.Server{
		Addr:         ":" + g.config.ServerPort,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	fmt.Printf("Documentation server started at http://localhost:%s\n", g.config.ServerPort)
	fmt.Println("Press Ctrl+C to stop")

	go func() {
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			lib.Log.Fatal(err.Error())
		}
	}()

	<-ctx.Done()

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	return server.Shutdown(shutdownCtx)
}
