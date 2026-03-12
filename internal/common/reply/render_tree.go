package reply

import (
	"fmt"
	"strconv"
	"strings"
)

type treeWriter struct {
	r       *ResponseRenderer
	sb      strings.Builder
	isError bool
	mid     string
	end     string
	pipe    string
	space   string
}

func (r *ResponseRenderer) renderTree(dataMap map[string]interface{}, isError bool) string {
	tw := &treeWriter{
		r:       r,
		isError: isError,
		mid:     r.enumeratorStyle.Render("├──"),
		end:     r.enumeratorStyle.Render("╰──"),
		pipe:    r.enumeratorStyle.Render("│") + "  ",
		space:   "    ",
	}

	tw.writeMapEntries(dataMap, "")

	s := tw.sb.String()
	if len(s) > 0 && s[len(s)-1] == '\n' {
		s = s[:len(s)-1]
	}
	return s
}

func (tw *treeWriter) branchFor(isLast bool) (branch, cont string) {
	if isLast {
		return tw.end, tw.space
	}
	return tw.mid, tw.pipe
}

func (tw *treeWriter) line(indent, branch, cont, text string) {
	if !strings.Contains(text, "\n") {
		tw.sb.WriteString(indent)
		tw.sb.WriteString(branch)
		tw.sb.WriteString(text)
		tw.sb.WriteByte('\n')
		return
	}
	lines := strings.Split(text, "\n")
	for i, l := range lines {
		if i == len(lines)-1 && l == "" {
			break
		}
		tw.sb.WriteString(indent)
		if i == 0 {
			tw.sb.WriteString(branch)
		} else {
			tw.sb.WriteString(cont)
		}
		tw.sb.WriteString(l)
		tw.sb.WriteByte('\n')
	}
}

func (tw *treeWriter) writeMapEntries(data map[string]interface{}, indent string) {
	keys := sortedKeys(data)
	_, hasMsg := data["message"]
	total := len(keys)
	if hasMsg {
		total++
	}
	if total == 0 {
		return
	}

	idx := 0
	if hasMsg {
		branch, cont := tw.branchFor(total == 1)
		tw.writeMessage(data["message"], indent, branch, cont)
		idx++
	}

	for _, k := range keys {
		branch, cont := tw.branchFor(idx == total-1)
		tw.writeEntry(k, data[k], indent, branch, cont)
		idx++
	}
}

func (tw *treeWriter) writeMessage(msg interface{}, indent, branch, cont string) {
	switch vv := msg.(type) {
	case string:
		if tw.isError {
			tw.line(indent, branch, cont, tw.r.errorMsgStyle.Render(vv))
		} else {
			styled := tw.r.messageStyle.Render(vv)
			tw.line(indent, branch, cont, strings.TrimRight(styled, " \n"))
			tw.sb.WriteString(indent)
			tw.sb.WriteString(cont)
			tw.sb.WriteByte('\n')
		}
	case map[string]interface{}:
		tw.line(indent, branch, cont, "message")
		tw.writeMapEntries(shallowNormalize(vv), indent+cont)
	case []interface{}:
		tw.line(indent, branch, cont, "message")
		tw.writeListEntries(vv, indent+cont)
	default:
		text := fmt.Sprintf("%v", vv)
		if tw.isError {
			tw.line(indent, branch, cont, tw.r.errorMsgStyle.Render(text))
		} else {
			styled := tw.r.messageStyle.Render(text)
			tw.line(indent, branch, cont, strings.TrimRight(styled, " \n"))
			tw.sb.WriteString(indent)
			tw.sb.WriteString(cont)
			tw.sb.WriteByte('\n')
		}
	}
}

func (tw *treeWriter) writeEntry(key string, value interface{}, indent, branch, cont string) {
	label := TranslateKey(key)
	switch vv := value.(type) {
	case nil, string, bool, int, float64:
		tw.line(indent, branch, cont, tw.r.formatScalarField(key, value))
	case map[string]interface{}:
		tw.line(indent, branch, cont, label)
		tw.writeMapEntries(shallowNormalize(vv), indent+cont)
	case []interface{}:
		if len(vv) == 0 {
			tw.line(indent, branch, cont, label+": []")
			return
		}
		tw.line(indent, branch, cont, label)
		tw.writeListEntries(vv, indent+cont)
	default:
		tw.line(indent, branch, cont, fmt.Sprintf("%s: %v", label, value))
	}
}

func (tw *treeWriter) writeListEntries(items []interface{}, indent string) {
	for i, elem := range items {
		branch, cont := tw.branchFor(i == len(items)-1)
		num := strconv.Itoa(i+1) + ")"

		switch vv := elem.(type) {
		case map[string]interface{}:
			tw.line(indent, branch, cont, num)
			tw.writeMapEntries(shallowNormalize(vv), indent+cont)
		default:
			tw.line(indent, branch, cont, num+" "+fmt.Sprintf("%v", elem))
		}
	}
}

// shallowNormalize возвращает map без копирования, если все значения уже базовых типов.
func shallowNormalize(data map[string]interface{}) map[string]interface{} {
	for _, v := range data {
		switch v.(type) {
		case nil, string, bool, int, float64, map[string]interface{}, []interface{}:
			continue
		default:
			return normalizeDataMap(data)
		}
	}
	return data
}
