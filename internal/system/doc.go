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

package system

import (
	_package "apm/internal/common/apt/package"
	aptlib "apm/internal/common/binding/apt/lib"
	"apm/internal/system/service"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"net/http"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"time"
)

// FilterField структура поля для фильтрации
type FilterField struct {
	Name string                          `json:"name"`
	Text string                          `json:"text"`
	Info map[_package.PackageType]string `json:"info"`
	Type string                          `json:"type"`
}

// InstallResponse структура ответа для Install/Remove методов
type InstallResponse struct {
	Message string                `json:"message"`
	Info    aptlib.PackageChanges `json:"info"`
}

// InfoResponse структура ответа для Info метода
type InfoResponse struct {
	Message     string           `json:"message"`
	PackageInfo _package.Package `json:"packageInfo"`
}

// ListResponse структура ответа для List/Search методов
type ListResponse struct {
	Message    string             `json:"message"`
	Packages   []_package.Package `json:"packages,omitempty"`
	TotalCount int                `json:"totalCount,omitempty"`
}

// UpdateResponse структура ответа для Update метода
type UpdateResponse struct {
	Message string `json:"message"`
	Count   int    `json:"count"`
}

// CheckResponse структура ответа для Check* методов
type CheckResponse struct {
	Message string                `json:"message"`
	Info    aptlib.PackageChanges `json:"info"`
}

// ImageStatusResponse структура ответа для ImageStatus метода
type ImageStatusResponse struct {
	Message     string      `json:"message"`
	BootedImage ImageStatus `json:"bootedImage"`
}

// ImageUpdateResponse структура ответа для ImageUpdate метода
type ImageUpdateResponse struct {
	Message     string      `json:"message"`
	BootedImage ImageStatus `json:"bootedImage"`
}

// ImageApplyResponse структура ответа для ImageApply метода
type ImageApplyResponse struct {
	Message     string      `json:"message"`
	BootedImage ImageStatus `json:"bootedImage"`
}

// ImageHistoryResponse структура ответа для ImageHistory метода
type ImageHistoryResponse struct {
	Message    string                 `json:"message"`
	History    []service.ImageHistory `json:"history"`
	TotalCount int64                  `json:"totalCount"`
}

// ImageConfigResponse структура ответа для ImageGetConfig/ImageSaveConfig методов
type ImageConfigResponse struct {
	Config service.Config `json:"config"`
}

// GetFilterFieldsResponse структура ответа для GetFilterFields метода
type GetFilterFieldsResponse []FilterField

// DBusMethodInfo содержит информацию о DBus методе
type DBusMethodInfo struct {
	Name         string
	Description  string
	ResponseType string
	Parameters   []DBusParameter
}

// DBusParameter описывает параметр метода
type DBusParameter struct {
	Name string
	Type string
}

// generateDBusDocHTML генерирует HTML документацию для DBus API
func generateDBusDocHTML() string {
	methods := parseDBusMethods()

	html := `<!DOCTYPE html>
<html>
<head>
    <title>APM DBus API Documentation</title>
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
    <h1>APM DBus API Documentation</h1>
    <p>Автоматически сгенерированная документация для DBus интерфейса APM.</p>
`

	for _, method := range methods {
		html += fmt.Sprintf(`
    <div class="method">
        <div class="method-name">%s</div>
        <div class="description">%s</div>
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
`, method.Name, method.Description,
			formatParameters(method.Parameters),
			method.ResponseType,
			generateDBusCommand(method),
			generateJSONExample(method.ResponseType))
	}

	html += `
</body>
</html>`

	return html
}

// getTypeString извлекает строковое представление типа из AST
func getTypeString(expr ast.Expr) string {
	switch t := expr.(type) {
	case *ast.Ident:
		return t.Name
	case *ast.ArrayType:
		return "[]" + getTypeString(t.Elt)
	case *ast.SelectorExpr:
		if x, ok := t.X.(*ast.Ident); ok {
			return x.Name + "." + t.Sel.Name
		}
		return "unknown"
	default:
		return "unknown"
	}
}

// formatParameters форматирует параметры для HTML
func formatParameters(params []DBusParameter) string {
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
func generateDBusCommand(method DBusMethodInfo) string {
	cmd := "dbus-send --system --print-reply --dest=org.altlinux.APM /org/altlinux/APM org.altlinux.APM.system." + method.Name

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
func generateJSONExample(responseType string) string {
	var example interface{}

	responseTypes := map[string]reflect.Type{
		"InstallResponse":         reflect.TypeOf(InstallResponse{}),
		"InfoResponse":            reflect.TypeOf(InfoResponse{}),
		"ListResponse":            reflect.TypeOf(ListResponse{}),
		"UpdateResponse":          reflect.TypeOf(UpdateResponse{}),
		"CheckResponse":           reflect.TypeOf(CheckResponse{}),
		"ImageStatusResponse":     reflect.TypeOf(ImageStatusResponse{}),
		"ImageUpdateResponse":     reflect.TypeOf(ImageUpdateResponse{}),
		"ImageApplyResponse":      reflect.TypeOf(ImageApplyResponse{}),
		"ImageHistoryResponse":    reflect.TypeOf(ImageHistoryResponse{}),
		"ImageConfigResponse":     reflect.TypeOf(ImageConfigResponse{}),
		"GetFilterFieldsResponse": reflect.TypeOf(GetFilterFieldsResponse{}),
	}

	if typ, exists := responseTypes[responseType]; exists {
		example = createExampleStruct(typ)
	} else {
		return `{"message": "Example response", "data": "See ` + responseType + ` structure above"}`
	}

	jsonBytes, err := json.MarshalIndent(example, "", "  ")
	if err != nil {
		return "Error generating JSON example"
	}

	return string(jsonBytes)
}

// createExampleStruct создаёт пример структуры с заполненными значениями
func createExampleStruct(typ reflect.Type) interface{} {
	if typ.Kind() == reflect.Slice {
		if typ.Elem().Name() == "FilterField" {
			slice := reflect.MakeSlice(typ, 1, 1)
			elem := slice.Index(0)
			elem.Set(reflect.ValueOf(FilterField{
				Name: "name",
				Text: "Package name",
				Type: "STRING",
			}))
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
			fieldValue.Set(reflect.ValueOf(createExampleStruct(field.Type)))
		default:
			continue
		}
	}

	return value.Interface()
}

// parseDBusMethods парсит dbus.go файл и извлекает информацию о методах
func parseDBusMethods() []DBusMethodInfo {
	var methods []DBusMethodInfo

	// Получаем путь к текущему файлу
	_, currentFile, _, _ := runtime.Caller(0)
	dbusFile := filepath.Join(filepath.Dir(currentFile), "dbus.go")

	// Парсим файл
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, dbusFile, nil, parser.ParseComments)
	if err != nil {
		return methods
	}

	// Проходим по функциям
	for _, decl := range node.Decls {
		if fn, ok := decl.(*ast.FuncDecl); ok {
			if fn.Recv != nil && strings.Contains(fn.Name.Name, "DBusWrapper") {
				continue
			}

			var responseType string
			var description string

			// Ищем комментарии
			if fn.Doc != nil {
				for _, comment := range fn.Doc.List {
					text := strings.TrimSpace(strings.TrimPrefix(comment.Text, "//"))
					if strings.HasPrefix(text, "doc_response:") {
						responseType = strings.TrimSpace(strings.TrimPrefix(text, "doc_response:"))
					} else if description == "" {
						description = text
					}
				}
			}

			if responseType != "" {
				var params []DBusParameter
				if fn.Type.Params != nil {
					for _, field := range fn.Type.Params.List {
						typeStr := getTypeString(field.Type)
						for _, name := range field.Names {
							if name.Name != "sender" && name.Name != "w" {
								params = append(params, DBusParameter{
									Name: name.Name,
									Type: typeStr,
								})
							}
						}
					}
				}

				methods = append(methods, DBusMethodInfo{
					Name:         fn.Name.Name,
					Description:  description,
					ResponseType: responseType,
					Parameters:   params,
				})
			}
		}
	}

	return methods
}

// startDocServer запускает веб-сервер с документацией
func startDocServer(ctx context.Context) error {
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		html := generateDBusDocHTML()
		fmt.Fprint(w, html)
	})

	server := &http.Server{
		Addr:         ":8081",
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	fmt.Println("Documentation server started at http://localhost:8081")
	fmt.Println("Press Ctrl+C to stop")

	go func() {
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			fmt.Printf("Server error: %v\n", err)
		}
	}()

	<-ctx.Done()

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	return server.Shutdown(shutdownCtx)
}
