package reply

import (
	"fmt"
	"strings"
)

func (r *ResponseRenderer) renderPlain(dataMap map[string]interface{}, isError ...bool) string {
	var lines []string

	keys := sortedKeys(dataMap)
	if msgVal, ok := dataMap["message"]; ok {
		msg := fmt.Sprintf("%v", msgVal)
		if len(isError) > 0 && isError[0] {
			msg = r.errorMsgStyle.Render(msg)
		}
		lines = append(lines, msg)
		if len(keys) > 0 {
			lines = append(lines, "")
		}
	}

	inner := dataMap
	if len(keys) == 1 {
		if mm, ok := dataMap[keys[0]].(map[string]interface{}); ok {
			inner = normalizeDataMap(mm)
			keys = sortedKeys(inner)
		}
	}

	for _, k := range keys {
		lines = append(lines, r.plainField("", k, inner[k])...)
	}

	return strings.Join(lines, "\n")
}

func (r *ResponseRenderer) plainField(prefix, key string, value interface{}) []string {
	fullLabel := TranslateKey(key)
	if prefix != "" {
		fullLabel = prefix + "." + fullLabel
	}

	switch vv := value.(type) {
	case nil, string, bool, int, float64:
		return []string{r.formatScalarFieldWithLabel(fullLabel, key, value)}

	case map[string]interface{}:
		normalized := normalizeDataMap(vv)
		var lines []string
		for _, k := range sortedKeys(normalized) {
			lines = append(lines, r.plainField(fullLabel, k, normalized[k])...)
		}
		return lines

	case []interface{}:
		if len(vv) == 0 {
			return nil
		}
		if isScalarSlice(vv) {
			parts := make([]string, len(vv))
			for i, elem := range vv {
				parts[i] = fmt.Sprintf("%v", elem)
			}
			return []string{fmt.Sprintf("%s: %s", fullLabel, strings.Join(parts, ", "))}
		}
		var lines []string
		for i, elem := range vv {
			itemPrefix := fmt.Sprintf("%s.%d", fullLabel, i+1)
			if mm, ok := elem.(map[string]interface{}); ok {
				normalized := normalizeDataMap(mm)
				for _, k := range sortedKeys(normalized) {
					lines = append(lines, r.plainField(itemPrefix, k, normalized[k])...)
				}
			} else {
				lines = append(lines, fmt.Sprintf("%s: %v", itemPrefix, elem))
			}
		}
		return lines

	default:
		return []string{fmt.Sprintf("%s: %v", fullLabel, value)}
	}
}

func isScalarSlice(vv []interface{}) bool {
	for _, elem := range vv {
		switch elem.(type) {
		case map[string]interface{}, []interface{}:
			return false
		}
	}
	return true
}
