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

package dbus_doc

import (
	"apm/internal/common/app"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"net/http"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"time"
)

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

// Config конфигурация для генерации
type Config struct {
	ModuleName    string
	DBusInterface string
	ServerPort    string
	DBusWrapper   interface{}
	ResponseTypes map[string]reflect.Type
	SourceCode    string
	DBusSession   string
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

	busType := "system"
	if g.config.DBusSession == "session" {
		busType = "session"
	}

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
        .events-section { border: 1px solid #4a9eff; margin: 20px 0; padding: 20px; border-radius: 5px; background: #1a2a3a; }
        .events-title { color: #4a9eff; font-size: 20px; font-weight: bold; margin-bottom: 10px; }
        pre { margin: 0; overflow-x: auto; }
        .parameters { margin: 10px 0; }
        .param { margin: 5px 0; }
    </style>
</head>
<body>
    <h1>APM %s DBus API Documentation</h1>
    <p>Автоматически сгенерированная документация для DBus интерфейса APM %s.</p>
    
    <div class="events-section">
        <div class="events-title">Прослушивание событий (Events)</div>
        <p>Для получения уведомлений о прогрессе операций и других событий, используйте команду:</p>
        <div class="dbus-command">
            <pre>gdbus monitor --%s --dest org.altlinux.APM</pre>
        </div>
        <p>События приходят в формате JSON через сигнал <code>org.altlinux.APM.Notification</code></p>
    </div>
`, g.config.ModuleName, g.config.ModuleName, g.config.ModuleName, busType)

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
`, method.Name,
			method.Description,
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

// reflectDBusMethods парсит исходный код и извлекает информацию о методах
func (g *Generator) reflectDBusMethods() []DBusMethodInfo {
	return g.parseSourceMethods()
}

// parseSourceMethods парсит исходный код для извлечения методов
func (g *Generator) parseSourceMethods() []DBusMethodInfo {
	var methods []DBusMethodInfo

	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, "", g.config.SourceCode, parser.ParseComments)
	if err != nil {
		return methods
	}

	// Карты для хранения информации из комментариев
	responseTypes := make(map[string]string)
	descriptions := make(map[string]string)

	// Извлекаем информацию из комментариев
	for _, comment := range node.Comments {
		text := comment.Text()

		var nextFunc *ast.FuncDecl
		for _, decl := range node.Decls {
			if funcDecl, ok := decl.(*ast.FuncDecl); ok {
				if int(funcDecl.Pos()) > int(comment.End()) {
					if funcDecl.Recv != nil && len(funcDecl.Recv.List) > 0 {
						nextFunc = funcDecl
						break
					}
				}
			}
		}

		if nextFunc == nil {
			continue
		}

		if match := regexp.MustCompile(`doc_response:\s*(\w+)`).FindStringSubmatch(text); match != nil {
			responseTypes[nextFunc.Name.Name] = match[1]
		}

		lines := strings.Split(text, "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line != "" && !strings.Contains(line, "doc_response:") {
				if strings.Contains(line, "–") {
					parts := strings.SplitN(line, "–", 2)
					if len(parts) == 2 {
						descriptions[nextFunc.Name.Name] = strings.TrimSpace(parts[1])
					}
				} else {
					descriptions[nextFunc.Name.Name] = line
				}
				break
			}
		}
	}

	// Извлекаем информацию о функциях
	for _, decl := range node.Decls {
		funcDecl, ok := decl.(*ast.FuncDecl)
		if !ok {
			continue
		}

		if funcDecl.Recv == nil || len(funcDecl.Recv.List) == 0 {
			continue
		}

		revType := funcDecl.Recv.List[0].Type
		if starExpr, ok := revType.(*ast.StarExpr); ok {
			if ident, ok := starExpr.X.(*ast.Ident); ok {
				if ident.Name != "DBusWrapper" {
					continue
				}
			}
		}

		methodName := funcDecl.Name.Name
		responseType, hasResponse := responseTypes[methodName]
		if !hasResponse {
			continue
		}

		// Извлекаем параметры
		var params []DBusParameter
		for _, param := range funcDecl.Type.Params.List {
			paramType := g.getASTTypeString(param.Type)

			if strings.Contains(paramType, "dbus.Sender") {
				continue
			}

			// Добавляем параметры с реальными именами
			for _, name := range param.Names {
				params = append(params, DBusParameter{
					Name: name.Name,
					Type: paramType,
				})
			}
		}

		description := descriptions[methodName]
		if description == "" {
			description = methodName
		}

		methods = append(methods, DBusMethodInfo{
			Name:         methodName,
			Description:  description,
			ResponseType: responseType,
			Parameters:   params,
		})
	}

	// Сортируем
	sort.Slice(methods, func(i, j int) bool {
		return methods[i].Name < methods[j].Name
	})

	return methods
}

// getASTTypeString преобразует AST тип в строку
func (g *Generator) getASTTypeString(expr ast.Expr) string {
	switch t := expr.(type) {
	case *ast.Ident:
		switch t.Name {
		case "string":
			return "string"
		case "bool":
			return "bool"
		case "int", "int32", "int64":
			return "int"
		default:
			return t.Name
		}
	case *ast.ArrayType:
		return "[]" + g.getASTTypeString(t.Elt)
	case *ast.SelectorExpr:
		return g.getASTTypeString(t.X) + "." + t.Sel.Name
	default:
		return "unknown"
	}
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
	sessionType := g.config.DBusSession
	if sessionType == "" {
		sessionType = "system"
	}
	cmd := fmt.Sprintf("dbus-send --%s --print-reply --dest=org.altlinux.APM /org/altlinux/APM %s.%s", sessionType, g.config.DBusInterface, method.Name)

	for _, param := range method.Parameters {
		switch param.Type {
		case "string":
			cmd += " string:\"example\""
		case "[]string":
			cmd += " array:string:\"example1\",\"example2\""
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

	if typ, exists := g.config.ResponseTypes[responseType]; exists {
		example = g.createExampleStruct(typ)
	}

	if example == nil {
		example = map[string]interface{}{
			"message": "Example response",
			"data":    "See " + responseType + " structure above",
		}
	}

	// Обёртываем в APIResponse структуру если она доступна
	if _, exists := g.config.ResponseTypes["APIResponse"]; exists {
		wrappedResponse := map[string]interface{}{
			"data":  example,
			"error": false,
		}
		example = wrappedResponse
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
			slice := reflect.MakeSlice(field.Type, 1, 1)
			elem := slice.Index(0)
			if elem.CanSet() {
				exampleElem := g.createExampleStruct(field.Type.Elem())
				elem.Set(reflect.ValueOf(exampleElem))
			}
			fieldValue.Set(slice)
		case reflect.Struct:
			fieldValue.Set(reflect.ValueOf(g.createExampleStruct(field.Type)))
		case reflect.Ptr:
			// Создаем новый экземпляр типа, на который указывает указатель
			elemType := field.Type.Elem()
			ptrValue := reflect.New(elemType)
			// Заполняем структуру, на которую указывает указатель
			filledValue := g.createExampleStruct(elemType)
			ptrValue.Elem().Set(reflect.ValueOf(filledValue))
			fieldValue.Set(ptrValue)
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
			if !strings.Contains(err.Error(), "broken pipe") {
				app.Log.Error("HTTP write error: " + err.Error())
			}
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
			app.Log.Fatal(err.Error())
		}
	}()

	<-ctx.Done()

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	return server.Shutdown(shutdownCtx)
}
