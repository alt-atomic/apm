package reply

import (
	"apm/cmd/distrobox/dbus_event"
	"encoding/json"
	"fmt"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/lipgloss/tree"
	"github.com/urfave/cli/v3"
	"reflect"
	"sort"
	"time"
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

	// Стиль для узлов дерева.
	itemStyle = lipgloss.NewStyle().
			Foreground(adaptiveItemColor)
)

// buildTreeFromMap рекурсивно строит дерево (tree.Tree) из map[string]interface{}.
func buildTreeFromMap(prefix string, data map[string]interface{}) *tree.Tree {
	// Создаем корень дерева
	t := tree.New().Root(prefix)

	// 1) Если у нас есть "message", обрабатываем его первым
	if msgVal, haveMsg := data["message"]; haveMsg {
		// Хотим вывести его первым узлом
		switch vv := msgVal.(type) {
		case string:
			t.Child(fmt.Sprintf("message: %s", vv))
		case int, float64, bool:
			t.Child(fmt.Sprintf("message: %v", vv))
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
				// Round-trip, если это структура
				b, err := json.Marshal(vv)
				if err == nil {
					var mm map[string]interface{}
					if err2 := json.Unmarshal(b, &mm); err2 == nil {
						subTree := buildTreeFromMap("message", mm)
						t.Child(subTree)
						// не забываем continue не нужен, мы не в цикле
					} else {
						t.Child(fmt.Sprintf("message: %T (неизвестный тип)", vv))
					}
				}
			case reflect.Slice:
				// Это может быть срез (например, []Struct). Тоже round-trip
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
						// не забываем continue не нужен, мы не в цикле
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

	// 3) Обрабатываем остальные ключи, как прежде
	for _, k := range keys {
		v := data[k]
		switch vv := v.(type) {
		case string:
			t.Child(fmt.Sprintf("%s: %s", k, vv))

		case int, float64, bool:
			t.Child(fmt.Sprintf("%s: %v", k, vv))

		case map[string]interface{}:
			subTree := buildTreeFromMap(k, vv)
			t.Child(subTree)

		case []interface{}:
			listNode := tree.New().Root(k)
			for i, elem := range vv {
				if mm, ok := elem.(map[string]interface{}); ok {
					subTree := buildTreeFromMap(fmt.Sprintf("%d)", i+1), mm)
					listNode.Child(subTree)
				} else {
					listNode.Child(fmt.Sprintf("%d) %v", i+1, elem))
				}
			}
			t.Child(listNode)

		default:
			rv := reflect.ValueOf(v)
			switch rv.Kind() {
			case reflect.Struct:
				b, err := json.Marshal(vv)
				if err == nil {
					var mm map[string]interface{}
					if err2 := json.Unmarshal(b, &mm); err2 == nil {
						subTree := buildTreeFromMap(k, mm)
						t.Child(subTree)
						continue
					}
				}
				t.Child(fmt.Sprintf("%s: %T (неизвестный тип)", k, vv))

			case reflect.Slice:
				b, err := json.Marshal(vv)
				if err == nil {
					var arr []interface{}
					if err2 := json.Unmarshal(b, &arr); err2 == nil {
						listNode := tree.New().Root(k)
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
				t.Child(fmt.Sprintf("%s: %T (срез неизвестного типа)", k, vv))

			default:
				t.Child(fmt.Sprintf("%s: %T (неизвестный тип)", k, vv))
			}
		}
	}

	return t
}

// CliResponse рендерит ответ в зависимости от формата (dbus/json/text).
func CliResponse(cmd *cli.Command, resp APIResponse) error {
	format := cmd.String("format")
	resp.Transaction = cmd.String("transaction")

	switch format {
	// ---------------------------------- DBUS ----------------------------------
	case "dbus":
		if !resp.Error {
			if dataMap, ok := resp.Data.(map[string]interface{}); ok {
				delete(dataMap, "message")
			}
		}
		b, err := json.MarshalIndent(resp, "", "  ")
		if err != nil {
			return err
		}
		dbus_event.SendNotificationResponse(string(b))
		fmt.Println(string(b))
		time.Sleep(200 * time.Millisecond)

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
			var t *tree.Tree
			//if resp.Error {
			//	t = buildTreeFromMap("Ошибка:", data)
			//} else {
			//	t = buildTreeFromMap("Успешно:", data)
			//}
			t = buildTreeFromMap("Ответ:", data)

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
