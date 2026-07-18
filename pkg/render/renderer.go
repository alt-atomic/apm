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

package render

import (
	"bytes"
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
	"strconv"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// decodeJSON decodes with UseNumber so integers keep their exact text
func decodeJSON(b []byte, v interface{}) error {
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.UseNumber()
	return dec.Decode(v)
}

// FormatType selects the text layout
type FormatType string

// Text format types
const (
	FormatTypeTree  FormatType = "tree"
	FormatTypePlain FormatType = "plain"
)

// Renderer renders data maps as styled tree or plain text
type Renderer struct {
	colors          Colors
	enumeratorStyle lipgloss.Style
	accentStyle     lipgloss.Style
	messageStyle    lipgloss.Style
	errorMsgStyle   lipgloss.Style
	accentKeys      map[string]bool
	valueFormatter  func(key string, value interface{}) (string, bool)
}

// Option configures a Renderer
type Option func(*Renderer)

// WithAccentKeys sets keys whose values are highlighted with the accent style
func WithAccentKeys(keys ...string) Option {
	return func(r *Renderer) {
		r.accentKeys = make(map[string]bool, len(keys))
		for _, k := range keys {
			r.accentKeys[k] = true
		}
	}
}

// WithValueFormatter sets a custom scalar value formatter, handled=true overrides the default
func WithValueFormatter(fn func(key string, value interface{}) (string, bool)) Option {
	return func(r *Renderer) {
		r.valueFormatter = fn
	}
}

// New creates a Renderer with the given color scheme
func New(colors Colors, opts ...Option) *Renderer {
	r := &Renderer{
		colors:          colors,
		enumeratorStyle: lipgloss.NewStyle().Foreground(lipgloss.Color(colors.TreeBranch)).MarginRight(1),
		accentStyle:     lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(colors.Accent)),
		messageStyle:    lipgloss.NewStyle().Bold(true).MarginBottom(1),
		errorMsgStyle:   lipgloss.NewStyle().Foreground(lipgloss.Color(colors.ResultError)),
	}
	for _, opt := range opts {
		opt(r)
	}
	return r
}

// RenderText renders the data map in the given format type
func (r *Renderer) RenderText(dataMap map[string]interface{}, formatType FormatType, isError bool) string {
	dataMap = NormalizeDataMap(dataMap)
	switch formatType {
	case FormatTypePlain:
		return r.renderPlain(dataMap, isError)
	default:
		return r.renderTree(dataMap, isError)
	}
}

func (r *Renderer) formatField(key string, value interface{}) string {
	valStr := fmt.Sprintf("%v", value)
	if r.accentKeys[key] {
		return r.accentStyle.Render(valStr)
	}
	return valStr
}

func (r *Renderer) formatScalarValue(k string, v interface{}) string {
	switch v.(type) {
	case nil, string, bool, json.Number:
	default:
		if n, ok := canonicalNumber(v); ok {
			v = n
		}
	}
	if r.valueFormatter != nil {
		if s, ok := r.valueFormatter(k, v); ok {
			return s
		}
	}
	switch vv := v.(type) {
	case nil:
		return T_("no")
	case string:
		if vv == "" {
			return T_("no")
		}
		return r.formatField(k, vv)
	case bool:
		if vv {
			return T_("yes")
		}
		return T_("no")
	default:
		return fmt.Sprintf("%v", vv)
	}
}

func (r *Renderer) formatScalarField(k string, v interface{}) string {
	return fmt.Sprintf("%s: %s", translateKey(k), r.formatScalarValue(k, v))
}

func (r *Renderer) formatScalarFieldWithLabel(label, k string, v interface{}) string {
	return fmt.Sprintf("%s: %s", label, r.formatScalarValue(k, v))
}

// AsInt extracts an int from a canonical json.Number value
func AsInt(value interface{}) (int, bool) {
	vv, ok := value.(json.Number)
	if !ok {
		return 0, false
	}
	if n, err := vv.Int64(); err == nil {
		return int(n), true
	}
	if f, err := vv.Float64(); err == nil {
		return int(f), true
	}
	return 0, false
}

func sortedKeys(data map[string]interface{}) []string {
	keys := make([]string, 0, len(data))
	for k := range data {
		if k != "message" {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)
	return keys
}

// canonicalNumber converts any plain numeric value to json.Number.
// Types with their own String() keep their textual form.
func canonicalNumber(v interface{}) (json.Number, bool) {
	if _, ok := v.(fmt.Stringer); ok {
		return "", false
	}
	rv := reflect.ValueOf(v)
	switch rv.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return json.Number(strconv.FormatInt(rv.Int(), 10)), true
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return json.Number(strconv.FormatUint(rv.Uint(), 10)), true
	case reflect.Float32:
		return json.Number(strconv.FormatFloat(rv.Float(), 'g', -1, 32)), true
	case reflect.Float64:
		return json.Number(strconv.FormatFloat(rv.Float(), 'g', -1, 64)), true
	default:
		return "", false
	}
}

func normalizeValue(v interface{}) interface{} {
	switch v.(type) {
	case nil, string, bool, json.Number, map[string]interface{}, []interface{}:
		return v
	}
	if n, ok := canonicalNumber(v); ok {
		return n
	}
	rv := reflect.ValueOf(v)
	switch rv.Kind() {
	case reflect.Struct, reflect.Ptr:
		b, err := json.Marshal(v)
		if err != nil {
			return fmt.Sprintf("%v", v)
		}
		var mm map[string]interface{}
		if decodeJSON(b, &mm) == nil {
			return mm
		}
		var arr []interface{}
		if decodeJSON(b, &arr) == nil {
			return arr
		}
		return fmt.Sprintf("%v", v)
	case reflect.Slice:
		if _, ok := v.([]interface{}); ok {
			return v
		}
		if maps, ok := v.([]map[string]interface{}); ok {
			result := make([]interface{}, len(maps))
			for i, m := range maps {
				result[i] = m
			}
			return result
		}
		b, err := json.Marshal(v)
		if err != nil {
			return fmt.Sprintf("%v", v)
		}
		var arr []interface{}
		if decodeJSON(b, &arr) == nil {
			return arr
		}
		return fmt.Sprintf("%v", v)
	default:
		return v
	}
}

// NormalizeDataMap normalizes all map values to basic JSON types
func NormalizeDataMap(data map[string]interface{}) map[string]interface{} {
	result := make(map[string]interface{}, len(data))
	for k, v := range data {
		result[k] = normalizeValue(v)
	}
	return result
}

// ToDataMap converts any value to a data map via JSON round-trip
func ToDataMap(data interface{}) map[string]interface{} {
	if dm, ok := data.(map[string]interface{}); ok {
		return dm
	}
	b, err := json.Marshal(data)
	if err != nil {
		return nil
	}
	var mm map[string]interface{}
	if decodeJSON(b, &mm) == nil {
		return mm
	}
	return nil
}

// FilterFields keeps only the requested fields, dot notation selects nested ones
func FilterFields(data map[string]interface{}, fields []string) map[string]interface{} {
	target := data
	var wrapperKey string

	keys := sortedKeys(data)
	if len(keys) == 1 {
		if mm, ok := data[keys[0]].(map[string]interface{}); ok {
			wrapperKey = keys[0]
			target = mm
		}
	}

	// Split fields: plain "name" and dotted "appStream.id" for nested data
	simple := make(map[string]bool)
	nested := make(map[string][]string)
	for _, f := range fields {
		if dot := strings.IndexByte(f, '.'); dot >= 0 {
			parent := f[:dot]
			child := f[dot+1:]
			nested[parent] = append(nested[parent], child)
		} else {
			simple[f] = true
		}
	}

	filtered := make(map[string]interface{})
	remainingFields := make([]string, 0)
	for k, v := range target {
		if simple[k] {
			filtered[k] = v
			continue
		}
		if children, ok := nested[k]; ok {
			if mm, ok := v.(map[string]interface{}); ok {
				filtered[k] = FilterFields(mm, children)
			}
		}
	}

	for _, f := range fields {
		if _, ok := filtered[f]; !ok {
			remainingFields = append(remainingFields, f)
		}
	}

	if len(remainingFields) > 0 {
		for k, v := range target {
			if arr, ok := v.([]interface{}); ok {
				var items []interface{}
				for _, elem := range arr {
					if mm, ok := elem.(map[string]interface{}); ok {
						fm := FilterFields(mm, remainingFields)
						if len(fm) > 0 {
							items = append(items, fm)
						}
					}
				}
				if len(items) > 0 {
					filtered[k] = items
				}
			}
		}
	}

	result := make(map[string]interface{})
	if msg, ok := data["message"]; ok {
		result["message"] = msg
	}
	if wrapperKey != "" {
		result[wrapperKey] = filtered
	} else {
		for k, v := range filtered {
			result[k] = v
		}
	}
	return result
}
