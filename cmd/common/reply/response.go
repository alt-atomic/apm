package reply

import (
	"apm/lib"
	"context"
	"encoding/json"
	"fmt"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/lipgloss/tree"
	"golang.org/x/crypto/ssh/terminal"
	"os"
	"reflect"
	"sort"
	"unicode"
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

// translateKey – вспомогательная функция для перевода ключа.
// Например, translateKey("name") → lib.T("response.name", "name")
func translateKey(k string) string {
	return lib.T("response."+k, k)
}

// IsTTY пользователь запустил приложение в интерактивной консоли
func IsTTY() bool {
	return terminal.IsTerminal(int(os.Stdout.Fd()))
}

func formatField(key string, value interface{}) string {
	valStr := fmt.Sprintf("%v", value)
	if key == "name" {
		return fmt.Sprintf("%s", accentStyle.Render(valStr))
	}

	return fmt.Sprintf("%s", valStr)
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
						t.Child(fmt.Sprintf("message: %T (неизвестный тип)", vv))
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
						t.Child(fmt.Sprintf("message: %T (срез неизвестного типа)", vv))
					}
				}
			default:
				t.Child(fmt.Sprintf("message: %T (неизвестный тип)", vv))
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
			t.Child(fmt.Sprintf("%s: нет", translateKey(k)))
			//t.Child(fmt.Sprintf("%s: []", translateKey(k)))

		//----------------------------------------------------------------------
		// СЛУЧАЙ: строка
		case string:
			if vv == "" {
				t.Child(fmt.Sprintf("%s: нет", translateKey(k)))
			} else {
				t.Child(fmt.Sprintf("%s: %s", translateKey(k), formatField(k, vv)))
			}

		//----------------------------------------------------------------------
		// СЛУЧАЙ: булевский (true/false) → "да"/"нет"
		case bool:
			var boolStr string
			if vv {
				boolStr = "да"
			} else {
				boolStr = "нет"
			}
			t.Child(fmt.Sprintf("%s: %s", translateKey(k), boolStr))

		//----------------------------------------------------------------------
		// СЛУЧАЙ: числа (int, float64)
		case int, float64:
			t.Child(fmt.Sprintf("%s: %v", translateKey(k), vv))

		//----------------------------------------------------------------------
		// СЛУЧАЙ: вложенная map
		case map[string]interface{}:
			subTree := buildTreeFromMap(translateKey(k), vv)
			t.Child(subTree)

		//----------------------------------------------------------------------
		// СЛУЧАЙ: срез (slice) из interface{}
		case []interface{}:
			if len(vv) == 0 {
				t.Child(fmt.Sprintf("%s: []", translateKey(k))) // пустой срез
				continue
			}
			listNode := tree.New().Root(translateKey(k))
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
						subTree := buildTreeFromMap(translateKey(k), mm)
						t.Child(subTree)
						continue
					}
				}
				t.Child(fmt.Sprintf("%s: %T (неизвестный тип)", translateKey(k), vv))

			//------------------------------------------------------------------
			// СЛУЧАЙ: срез (slice) непонятного типа
			case reflect.Slice:
				b, err := json.Marshal(vv)
				if err == nil {
					var arr []interface{}
					if err2 := json.Unmarshal(b, &arr); err2 == nil {
						listNode := tree.New().Root(translateKey(k))
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
				t.Child(fmt.Sprintf("%s: %T (срез неизвестного типа)", translateKey(k), vv))

			//------------------------------------------------------------------
			default:
				t.Child(fmt.Sprintf("%s: %T (неизвестный тип)", translateKey(k), vv))
			}
		}
	}

	return t
}

// CliResponse рендерит ответ в зависимости от формата (dbus/json/text).
func CliResponse(ctx context.Context, resp APIResponse) error {
	StopSpinner()
	format := lib.Env.Format
	txVal := ctx.Value("transaction")
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

	return nil
}
