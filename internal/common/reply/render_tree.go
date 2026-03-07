package reply

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/lipgloss/tree"
)

func (r *ResponseRenderer) renderTree(dataMap map[string]interface{}, isError bool) string {
	t := r.buildTreeFromMap("", dataMap, isError)

	var rootColor lipgloss.Style
	if isError {
		rootColor = lipgloss.NewStyle().Bold(true).
			Foreground(lipgloss.Color(r.GetColors().ResultError))
	} else {
		rootColor = lipgloss.NewStyle().Bold(true).
			Foreground(lipgloss.Color(r.GetColors().ResultSuccess))
	}

	t.Enumerator(tree.RoundedEnumerator).
		EnumeratorStyle(r.enumeratorStyle).
		RootStyle(rootColor).
		ItemStyle(r.itemStyle)

	return t.String()
}

func (r *ResponseRenderer) buildTreeFromMap(prefix string, data map[string]interface{}, isError bool) *tree.Tree {
	t := tree.New().Root(prefix)

	if msgVal, ok := data["message"]; ok {
		switch vv := msgVal.(type) {
		case string:
			if isError {
				t.Child(r.errorMsgStyle.Render(vv))
			} else {
				t.Child(r.messageStyle.Render(vv))
			}
		case map[string]interface{}:
			subTree := r.buildTreeFromMap("message", normalizeDataMap(vv), isError)
			t.Child(subTree)
		case []interface{}:
			listNode := tree.New().Root("message")
			for i, elem := range vv {
				if mm, ok := elem.(map[string]interface{}); ok {
					subTree := r.buildTreeFromMap(fmt.Sprintf("%d)", i+1), normalizeDataMap(mm), isError)
					listNode.Child(subTree)
				} else {
					listNode.Child(fmt.Sprintf("%d) %v", i+1, elem))
				}
			}
			t.Child(listNode)
		default:
			rendered := fmt.Sprintf("%v", vv)
			if isError {
				t.Child(r.errorMsgStyle.Render(rendered))
			} else {
				t.Child(r.messageStyle.Render(rendered))
			}
		}
	}

	for _, k := range sortedKeys(data) {
		v := data[k]

		switch vv := v.(type) {
		case nil, string, bool, int, float64:
			t.Child(r.formatScalarField(k, v))

		case map[string]interface{}:
			subTree := r.buildTreeFromMap(TranslateKey(k), normalizeDataMap(vv), isError)
			t.Child(subTree)

		case []interface{}:
			if len(vv) == 0 {
				t.Child(fmt.Sprintf("%s: []", TranslateKey(k)))
				continue
			}
			listNode := tree.New().Root(TranslateKey(k))
			for i, elem := range vv {
				if mm, ok := elem.(map[string]interface{}); ok {
					subTree := r.buildTreeFromMap(fmt.Sprintf("%d)", i+1), normalizeDataMap(mm), isError)
					listNode.Child(subTree)
				} else {
					listNode.Child(fmt.Sprintf("%d) %v", i+1, elem))
				}
			}
			t.Child(listNode)

		default:
			t.Child(fmt.Sprintf("%s: %v", TranslateKey(k), v))
		}
	}

	return t
}
