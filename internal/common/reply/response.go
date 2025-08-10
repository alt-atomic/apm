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
	"apm/internal/common/helper"
	"apm/lib"
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

// Глобальные стили для дерева.
var (
	// Стиль нумерации (веток).
	enumeratorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#2aa1b3")).
			MarginRight(1)

	// Адаптивный цвет для пунктов (для светлой/тёмной темы).
	adaptiveItemColor = lipgloss.AdaptiveColor{
		Light: "#171717", // для светлой темы
		Dark:  "#c4c8c6", // для тёмной темы
	}

	accentStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#a2734c"))

	// Стиль для узлов дерева.
	itemStyle = lipgloss.NewStyle().
			Foreground(adaptiveItemColor)
)

// IsTTY пользователь запустил приложение в интерактивной консоли
func IsTTY() bool {
	return terminal.IsTerminal(int(os.Stdout.Fd()))
}

func formatField(key string, value interface{}) string {
	valStr := fmt.Sprintf("%v", value)
	if key == "name" {
		return accentStyle.Render(valStr)
	}

	return valStr
}

// buildTreeFromMap рекурсивно строит дерево (tree.Tree) из map[string]interface{}.
func buildTreeFromMap(prefix string, data map[string]interface{}) *tree.Tree {
	// Создаем корень дерева
	t := tree.New().Root(prefix)

	// 1) Если у нас есть "message", обрабатываем его первым
	if msgVal, haveMsg := data["message"]; haveMsg {
		switch vv := msgVal.(type) {
		case string:
			t.Child(vv)
		case int, float64, bool:
			t.Child(fmt.Sprintf("%v", vv))
		case map[string]interface{}:
			subTree := buildTreeFromMap("message", vv)
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
						subTree := buildTreeFromMap("message", mm)
						t.Child(subTree)
					} else {
						t.Child(fmt.Sprintf("message: %s", fmt.Sprintf(lib.T_("%T (unknown type)"), vv)))
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
								subTree := buildTreeFromMap(fmt.Sprintf("%d)", i+1), mm)
								listNode.Child(subTree)
							} else {
								listNode.Child(fmt.Sprintf("%d) %v", i+1, elem))
							}
						}
						t.Child(listNode)
					} else {
						t.Child(fmt.Sprintf("message: %s", fmt.Sprintf(lib.T_("%T (slice of unknown type)"), vv)))
					}
				}
			default:
				t.Child(fmt.Sprintf("message: %s", fmt.Sprintf(lib.T_("%T (unknown type)"), vv)))
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
			t.Child(fmt.Sprintf(lib.T_("%s: no"), TranslateKey(k)))
			//t.Child(fmt.Sprintf("%s: []", translateKey(k)))

		//----------------------------------------------------------------------
		// СЛУЧАЙ: строка
		case string:
			if vv == "" {
				t.Child(fmt.Sprintf(lib.T_("%s: no"), TranslateKey(k)))
			} else {
				t.Child(fmt.Sprintf("%s: %s", TranslateKey(k), formatField(k, vv)))
			}

		//----------------------------------------------------------------------
		// СЛУЧАЙ: булевский (true/false) → "да"/"нет"
		case bool:
			var boolStr string
			if vv {
				boolStr = lib.T_("yes")
			} else {
				boolStr = lib.T_("no")
			}
			t.Child(fmt.Sprintf("%s: %s", TranslateKey(k), boolStr))

		//----------------------------------------------------------------------
		// СЛУЧАЙ: числа (int, float64)
		case int, float64:
			if k == "size" || k == "installedSize" {
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
			subTree := buildTreeFromMap(TranslateKey(k), vv)
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
					subTree := buildTreeFromMap(fmt.Sprintf("%d)", i+1), mm)
					listNode.Child(subTree)
				} else {
					listNode.Child(fmt.Sprintf("%d) %v", i+1, elem))
				}
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
						subTree := buildTreeFromMap(TranslateKey(k), mm)
						t.Child(subTree)
						continue
					}
				}
				t.Child(fmt.Sprintf("%s: %s", TranslateKey(k), fmt.Sprintf(lib.T_("%T (unknown type)"), vv)))

			// СЛУЧАЙ: указатель (попробуем развернуть через JSON как структуру/срез)
			case reflect.Ptr:
				b, err := json.Marshal(vv)
				if err == nil {
					var mm map[string]interface{}
					if err2 := json.Unmarshal(b, &mm); err2 == nil {
						subTree := buildTreeFromMap(TranslateKey(k), mm)
						t.Child(subTree)
						continue
					}
					var arr []interface{}
					if err2 := json.Unmarshal(b, &arr); err2 == nil {
						listNode := tree.New().Root(TranslateKey(k))
						for i, elem := range arr {
							if mm, ok := elem.(map[string]interface{}); ok {
								subTree := buildTreeFromMap(fmt.Sprintf("%d)", i+1), mm)
								listNode.Child(subTree)
							} else {
								listNode.Child(fmt.Sprintf("%d) %v", i+1, elem))
							}
						}
						t.Child(listNode)
						continue
					}
				}
				t.Child(fmt.Sprintf("%s: %s", TranslateKey(k), fmt.Sprintf(lib.T_("%T (unknown type)"), vv)))

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
								subTree := buildTreeFromMap(fmt.Sprintf("%d)", i+1), mm)
								listNode.Child(subTree)
							} else {
								listNode.Child(fmt.Sprintf("%d) %v", i+1, elem))
							}
						}
						t.Child(listNode)
						continue
					}
				}
				t.Child(fmt.Sprintf("%s: %s", TranslateKey(k), fmt.Sprintf(lib.T_("%T (slice of unknown type)"), vv)))

			//------------------------------------------------------------------
			default:
				t.Child(fmt.Sprintf("%s: %s", TranslateKey(k), fmt.Sprintf(lib.T_("%T (unknown type)"), vv)))
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
				t = buildTreeFromMap("⚛", data)
			} else {
				t = buildTreeFromMap("⚛", data)
			}

			var rootColor lipgloss.Style
			if resp.Error {
				rootColor = lipgloss.NewStyle().
					Bold(true).
					Foreground(lipgloss.Color("9")) // красный
			} else {
				rootColor = lipgloss.NewStyle().
					Bold(true).
					Foreground(lipgloss.Color("2")) // зелёный
			}

			t.Enumerator(tree.RoundedEnumerator).
				EnumeratorStyle(enumeratorStyle).
				RootStyle(rootColor).
				ItemStyle(itemStyle)

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
