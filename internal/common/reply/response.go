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

package reply

import (
	"apm/internal/common/app"
	"apm/internal/common/helper"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"reflect"
	"sort"
	"unicode"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/lipgloss/tree"
	"golang.org/x/crypto/ssh/terminal"
)

// APIResponse описывает итоговую структуру ответа.
type APIResponse struct {
	Data        interface{} `json:"data"`
	Error       bool        `json:"error"`
	Transaction string      `json:"transaction,omitempty"`
}

// getEnumeratorStyle возвращает стиль нумерации (веток).
func getEnumeratorStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Foreground(lipgloss.Color(lib.Env.Colors.Enumerator)).
		MarginRight(1)
}

// getAdaptiveItemColor возвращает адаптивный цвет для пунктов.
func getAdaptiveItemColor() lipgloss.AdaptiveColor {
	return lipgloss.AdaptiveColor{
		Light: lib.Env.Colors.ItemLight, // для светлой темы
		Dark:  lib.Env.Colors.ItemDark,  // для тёмной темы
	}
}

// getAccentStyle возвращает стиль акцента.
func getAccentStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color(lib.Env.Colors.Accent))
}

// getMessageStyle возвращает стиль для message
func getMessageStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Bold(true).
		MarginBottom(1)
}

// getErrorMessageStyle возвращает стиль для message в случае ошибки
func getErrorMessageStyle() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(lipgloss.Color(lib.Env.Colors.Error))
}

// getItemStyle возвращает стиль для узлов дерева.
func getItemStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Foreground(getAdaptiveItemColor())
}

// IsTTY пользователь запустил приложение в интерактивной консоли
func IsTTY() bool {
	return terminal.IsTerminal(int(os.Stdout.Fd()))
}

func formatField(key string, value interface{}) string {
	valStr := fmt.Sprintf("%v", value)
	if key == "name" {
		return getAccentStyle().Render(valStr)
	}

	if key == "packageName" {
		return getAccentStyle().Render(valStr)
	}
	return valStr
}

// buildTreeFromMap рекурсивно строит дерево (tree.Tree) из map[string]interface{}.
func buildTreeFromMap(prefix string, data map[string]interface{}, isError bool) *tree.Tree {
	// Создаем корень дерева
	t := tree.New().Root(prefix)

	// 1) Если у нас есть "message", обрабатываем его первым
	if msgVal, haveMsg := data["message"]; haveMsg {
		switch vv := msgVal.(type) {
		case string:
			if isError {
				t.Child(getErrorMessageStyle().Render(vv))
			} else {
				t.Child(getMessageStyle().Render(vv))
			}
		case int, float64, bool:
			if isError {
				t.Child(getErrorMessageStyle().Render(fmt.Sprintf("%v", vv)))
			} else {
				t.Child(getMessageStyle().Render(fmt.Sprintf("%v", vv)))
			}
		case map[string]interface{}:
			subTree := buildTreeFromMap("message", vv, isError)
			t.Child(subTree)
		case []interface{}:
			listNode := tree.New().Root("message")
			for i, elem := range vv {
				listNode.Child(fmt.Sprintf("%d) %v", i+1, elem))
			}
			t.Child(listNode)
		default:
			rv := reflect.ValueOf(msgVal)
			switch rv.Kind() {
			case reflect.Struct:
				b, err := json.Marshal(vv)
				if err == nil {
					var mm map[string]interface{}
					if err2 := json.Unmarshal(b, &mm); err2 == nil {
						subTree := buildTreeFromMap("message", mm, isError)
						t.Child(subTree)
					} else {
						t.Child(fmt.Sprintf("message: %s", fmt.Sprintf(app.T_("%T (unknown type)"), vv)))
					}
				}
			case reflect.Slice:
				b, err := json.Marshal(vv)
				if err == nil {
					var arr []interface{}
					if err2 := json.Unmarshal(b, &arr); err2 == nil {
						listNode := tree.New().Root("message")
						for i, elem := range arr {
							if mm, ok := elem.(map[string]interface{}); ok {
								subTree := buildTreeFromMap(fmt.Sprintf("%d)", i+1), mm, isError)
								listNode.Child(subTree)
							} else {
								listNode.Child(fmt.Sprintf("%d) %v", i+1, elem))
							}
						}
						t.Child(listNode)
					} else {
						t.Child(fmt.Sprintf("message: %s", fmt.Sprintf(app.T_("%T (slice of unknown type)"), vv)))
					}
				}
			default:
				t.Child(fmt.Sprintf("message: %s", fmt.Sprintf(app.T_("%T (unknown type)"), vv)))
			}
		}
	}

	// 2) Собираем и сортируем остальные ключи, пропуская "message"
	keys := make([]string, 0, len(data))
	for k := range data {
		if k == "message" {
			continue
		}
		keys = append(keys, k)
	}
	sort.Strings(keys)

	// 3) Обрабатываем остальные ключи
	for _, k := range keys {
		v := data[k]
		switch vv := v.(type) {

		//----------------------------------------------------------------------
		// СЛУЧАЙ: значение == nil
		case nil:
			t.Child(fmt.Sprintf(app.T_("%s: no"), TranslateKey(k)))
			//t.Child(fmt.Sprintf("%s: []", translateKey(k)))

		//----------------------------------------------------------------------
		// СЛУЧАЙ: строка
		case string:
			if vv == "" {
				t.Child(fmt.Sprintf(app.T_("%s: no"), TranslateKey(k)))
			} else {
				t.Child(fmt.Sprintf("%s: %s", TranslateKey(k), formatField(k, vv)))
			}

		//----------------------------------------------------------------------
		// СЛУЧАЙ: булевский (true/false) → "да"/"нет"
		case bool:
			var boolStr string
			if vv {
				boolStr = app.T_("yes")
			} else {
				boolStr = app.T_("no")
			}
			t.Child(fmt.Sprintf("%s: %s", TranslateKey(k), boolStr))

		//----------------------------------------------------------------------
		// СЛУЧАЙ: числа (int, float64)
		case int, float64:
			if k == "size" || k == "installedSize" || k == "downloadSize" || k == "installSize" {
				sizeVal := 0
				switch valueTyped := vv.(type) {
				case int:
					sizeVal = valueTyped
				case float64:
					sizeVal = int(valueTyped)
				}

				sizeHuman := helper.AutoSize(sizeVal)
				t.Child(fmt.Sprintf("%s: %s", TranslateKey(k), sizeHuman))
			} else {
				// Стандартный путь для всех остальных чисел
				t.Child(fmt.Sprintf("%s: %v", TranslateKey(k), vv))
			}

		//----------------------------------------------------------------------
		// СЛУЧАЙ: вложенная map
		case map[string]interface{}:
			subTree := buildTreeFromMap(TranslateKey(k), vv, isError)
			t.Child(subTree)

		//----------------------------------------------------------------------
		// СЛУЧАЙ: срез (slice) из interface{}
		case []interface{}:
			if len(vv) == 0 {
				t.Child(fmt.Sprintf("%s: []", TranslateKey(k))) // пустой срез
				continue
			}
			listNode := tree.New().Root(TranslateKey(k))
			for i, elem := range vv {
				if mm, ok := elem.(map[string]interface{}); ok {
					subTree := buildTreeFromMap(fmt.Sprintf("%d)", i+1), mm, isError)
					listNode.Child(subTree)
				} else {
					listNode.Child(fmt.Sprintf("%d) %v", i+1, elem))
				}
			}
			t.Child(listNode)

		// СЛУЧАЙ: срез из map[string]interface{}
		case []map[string]interface{}:
			if len(vv) == 0 {
				// Показываем пустой массив как []
				t.Child(fmt.Sprintf("%s: []", TranslateKey(k)))
				continue
			}
			listNode := tree.New().Root(TranslateKey(k))
			for i, elem := range vv {
				subTree := buildTreeFromMap(fmt.Sprintf("%d)", i+1), elem, isError)
				listNode.Child(subTree)
			}
			t.Child(listNode)

			//----------------------------------------------------------------------
			// ДРУГИЕ СЛУЧАИ: структуры, срезы непонятных типов и т.д.
		default:
			rv := reflect.ValueOf(v)
			switch rv.Kind() {

			//------------------------------------------------------------------
			// СЛУЧАЙ: структура
			case reflect.Struct:
				b, err := json.Marshal(vv)
				if err == nil {
					var mm map[string]interface{}
					if err2 := json.Unmarshal(b, &mm); err2 == nil {
						subTree := buildTreeFromMap(TranslateKey(k), mm, isError)
						t.Child(subTree)
						continue
					}
				}
				t.Child(fmt.Sprintf("%s: %s", TranslateKey(k), fmt.Sprintf(app.T_("%T (unknown type)"), vv)))

			// СЛУЧАЙ: указатель (попробуем развернуть через JSON как структуру/срез)
			case reflect.Ptr:
				b, err := json.Marshal(vv)
				if err == nil {
					var mm map[string]interface{}
					if err2 := json.Unmarshal(b, &mm); err2 == nil {
						subTree := buildTreeFromMap(TranslateKey(k), mm, isError)
						t.Child(subTree)
						continue
					}
					var arr []interface{}
					if err2 := json.Unmarshal(b, &arr); err2 == nil {
						listNode := tree.New().Root(TranslateKey(k))
						for i, elem := range arr {
							if mm, ok := elem.(map[string]interface{}); ok {
								subTree := buildTreeFromMap(fmt.Sprintf("%d)", i+1), mm, isError)
								listNode.Child(subTree)
							} else {
								listNode.Child(fmt.Sprintf("%d) %v", i+1, elem))
							}
						}
						t.Child(listNode)
						continue
					}
				}
				t.Child(fmt.Sprintf("%s: %s", TranslateKey(k), fmt.Sprintf(app.T_("%T (unknown type)"), vv)))

			//------------------------------------------------------------------
			// СЛУЧАЙ: срез (slice) непонятного типа
			case reflect.Slice:
				b, err := json.Marshal(vv)
				if err == nil {
					var arr []interface{}
					if err2 := json.Unmarshal(b, &arr); err2 == nil {
						listNode := tree.New().Root(TranslateKey(k))
						for i, elem := range arr {
							if mm, ok := elem.(map[string]interface{}); ok {
								subTree := buildTreeFromMap(fmt.Sprintf("%d)", i+1), mm, isError)
								listNode.Child(subTree)
							} else {
								listNode.Child(fmt.Sprintf("%d) %v", i+1, elem))
							}
						}
						t.Child(listNode)
						continue
					}
				}
				t.Child(fmt.Sprintf("%s: %s", TranslateKey(k), fmt.Sprintf(app.T_("%T (slice of unknown type)"), vv)))

			//------------------------------------------------------------------
			default:
				t.Child(fmt.Sprintf("%s: %s", TranslateKey(k), fmt.Sprintf(app.T_("%T (unknown type)"), vv)))
			}
		}
	}

	return t
}

// CliResponse рендерит ответ в зависимости от формата (dbus/json/text).
func CliResponse(ctx context.Context, resp APIResponse) error {
	StopSpinner()
	format := lib.Env.Format
	txVal := ctx.Value(helper.TransactionKey)
	txStr, ok := txVal.(string)
	if ok {
		resp.Transaction = txStr
	}

	switch format {
	// ---------------------------------- JSON ----------------------------------
	case "json":
		// Если нет ошибки, убираем "message"
		if !resp.Error {
			if dataMap, ok := resp.Data.(map[string]interface{}); ok {
				delete(dataMap, "message")
			}
		}
		b, err := json.MarshalIndent(resp, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(b))

	// ---------------------------------- TEXT (по умолчанию) ------------------
	default:
		switch data := resp.Data.(type) {

		case map[string]interface{}:
			// если ошибка и message с маленькой буквы - делаем заглавную.
			if resp.Error {
				if rawMsg, haveMsg := data["message"]; haveMsg {
					if msgStr, ok := rawMsg.(string); ok && len(msgStr) > 0 {
						runes := []rune(msgStr)
						if unicode.IsLower(runes[0]) {
							runes[0] = unicode.ToUpper(runes[0])
							data["message"] = string(runes)
						}
					}
				}
			}

			var t *tree.Tree
			if resp.Error {
				t = buildTreeFromMap("", data, resp.Error)
			} else {
				t = buildTreeFromMap("", data, resp.Error)
			}

			var rootColor lipgloss.Style
			if resp.Error {
				rootColor = lipgloss.NewStyle().
					Bold(true).
					Foreground(lipgloss.Color(lib.Env.Colors.Error)) // красный
			} else {
				rootColor = lipgloss.NewStyle().
					Bold(true).
					Foreground(lipgloss.Color(lib.Env.Colors.Success)) // зелёный
			}

			t.Enumerator(tree.RoundedEnumerator).
				EnumeratorStyle(getEnumeratorStyle()).
				RootStyle(rootColor).
				ItemStyle(getItemStyle())

			fmt.Println(t.String())

		default:
			var message string
			switch dd := resp.Data.(type) {
			case map[string]string:
				message = dd["message"]
			case string:
				message = dd
			default:
				message = fmt.Sprintf("%v", dd)
			}
			fmt.Println(message)
		}
	}

	// возвращаем пустую ошибку, что бы вызвать статус exit code 1
	if resp.Error {
		return errors.New("")
	}

	return nil
}
