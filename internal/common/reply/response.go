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
	"apm/internal/common/apmerr"
	"apm/internal/common/app"
	"apm/internal/common/helper"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"reflect"
	"sort"
	"strings"
	"unicode"

	"github.com/charmbracelet/lipgloss"
	"golang.org/x/crypto/ssh/terminal"
)

type APIError struct {
	ErrorCode string `json:"errorCode"`
	Message   string `json:"message"`
}

type APIResponse struct {
	Data        interface{} `json:"data"`
	Error       *APIError   `json:"error"`
	Transaction string      `json:"transaction,omitempty"`
}

func OK(data interface{}) APIResponse {
	return APIResponse{Data: data}
}

func ErrorResponseFromError(err error) APIResponse {
	var apmErr apmerr.APMError
	if errors.As(err, &apmErr) {
		return APIResponse{Error: &APIError{ErrorCode: apmErr.Type, Message: err.Error()}}
	}
	return APIResponse{Error: &APIError{Message: err.Error()}}
}

type ResponseRenderer struct {
	appConfig       *app.Config
	colors          app.Colors
	enumeratorStyle lipgloss.Style
	accentStyle     lipgloss.Style
	messageStyle    lipgloss.Style
	errorMsgStyle   lipgloss.Style
}

func NewResponseRenderer(appConfig *app.Config) *ResponseRenderer {
	r := NewRendererFromColors(appConfig.ConfigManager.GetColors())
	r.appConfig = appConfig
	return r
}

func NewRendererFromColors(colors app.Colors) *ResponseRenderer {
	return &ResponseRenderer{
		colors:          colors,
		enumeratorStyle: lipgloss.NewStyle().Foreground(lipgloss.Color(colors.TreeBranch)).MarginRight(1),
		accentStyle:     lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(colors.Accent)),
		messageStyle:    lipgloss.NewStyle().Bold(true).MarginBottom(1),
		errorMsgStyle:   lipgloss.NewStyle().Foreground(lipgloss.Color(colors.ResultError)),
	}
}

func (r *ResponseRenderer) GetColors() app.Colors {
	return r.colors
}

func IsTTY() bool {
	return terminal.IsTerminal(int(os.Stdout.Fd()))
}

// IsInteractive возвращает true если формат text и терминал интерактивный (для TUI и других штук)
func IsInteractive(appConfig *app.Config) bool {
	return appConfig.ConfigManager.GetConfig().Format == app.FormatText && IsTTY()
}

func (r *ResponseRenderer) formatField(key string, value interface{}) string {
	valStr := fmt.Sprintf("%v", value)
	if key == "name" || key == "packageName" || key == "url" {
		return r.accentStyle.Render(valStr)
	}
	return valStr
}

func (r *ResponseRenderer) formatScalarValue(k string, v interface{}) string {
	switch vv := v.(type) {
	case nil:
		return app.T_("no")
	case string:
		if vv == "" {
			return app.T_("no")
		}
		return r.formatField(k, vv)
	case bool:
		if vv {
			return app.T_("yes")
		}
		return app.T_("no")
	case int, float64:
		if k == "size" || k == "installedSize" || k == "downloadSize" || k == "installSize" {
			var sizeVal int
			switch sv := vv.(type) {
			case int:
				sizeVal = sv
			case float64:
				sizeVal = int(sv)
			}
			return helper.AutoSize(sizeVal)
		}
		return fmt.Sprintf("%v", vv)
	default:
		return fmt.Sprintf("%v", vv)
	}
}

func (r *ResponseRenderer) formatScalarField(k string, v interface{}) string {
	return fmt.Sprintf("%s: %s", TranslateKey(k), r.formatScalarValue(k, v))
}

func (r *ResponseRenderer) formatScalarFieldWithLabel(label, k string, v interface{}) string {
	return fmt.Sprintf("%s: %s", label, r.formatScalarValue(k, v))
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

func normalizeValue(v interface{}) interface{} {
	switch v.(type) {
	case nil, string, bool, int, float64, map[string]interface{}, []interface{}:
		return v
	}
	rv := reflect.ValueOf(v)
	switch rv.Kind() {
	case reflect.Struct, reflect.Ptr:
		b, err := json.Marshal(v)
		if err != nil {
			return fmt.Sprintf("%v", v)
		}
		var mm map[string]interface{}
		if json.Unmarshal(b, &mm) == nil {
			return mm
		}
		var arr []interface{}
		if json.Unmarshal(b, &arr) == nil {
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
		if json.Unmarshal(b, &arr) == nil {
			return arr
		}
		return fmt.Sprintf("%v", v)
	default:
		return v
	}
}

func normalizeDataMap(data map[string]interface{}) map[string]interface{} {
	result := make(map[string]interface{}, len(data))
	for k, v := range data {
		result[k] = normalizeValue(v)
	}
	return result
}

func toDataMap(data interface{}) map[string]interface{} {
	if dm, ok := data.(map[string]interface{}); ok {
		return dm
	}
	b, err := json.Marshal(data)
	if err != nil {
		return nil
	}
	var mm map[string]interface{}
	if json.Unmarshal(b, &mm) == nil {
		return mm
	}
	return nil
}

// MessageWithHint дополняет message подсказкой об использовании --full
func MessageWithHint(message string, full bool) string {
	if full {
		return message
	}
	return message + ". " + app.T_("Use --full for detailed output")
}

func CliResponse(ctx context.Context, resp APIResponse) error {
	return NewResponseRenderer(app.GetAppConfig(ctx)).CliResponse(ctx, resp)
}

func (r *ResponseRenderer) CliResponse(ctx context.Context, resp APIResponse) error {
	StopSpinner(r.appConfig)
	format := r.appConfig.ConfigManager.GetConfig().Format
	txVal := ctx.Value(helper.TransactionKey)
	txStr, ok := txVal.(string)
	if ok {
		resp.Transaction = txStr
	}

	isError := resp.Error != nil

	fields := r.appConfig.ConfigManager.GetConfig().Fields

	switch format {
	case app.FormatJSON:
		if !isError {
			if dataMap := toDataMap(resp.Data); dataMap != nil {
				delete(dataMap, "message")
				if len(fields) > 0 {
					dataMap = filterFields(normalizeDataMap(dataMap), fields)
				}
				resp.Data = dataMap
			}
		}
		b, err := json.MarshalIndent(resp, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(b))

	default:
		var output string
		if isError {
			msg := resp.Error.Message
			if len(msg) > 0 {
				runes := []rune(msg)
				if unicode.IsLower(runes[0]) {
					runes[0] = unicode.ToUpper(runes[0])
					msg = string(runes)
				}
			}
			dataMap := map[string]interface{}{"message": msg}
			output = r.renderText(dataMap, true)
		} else {
			dataMap := toDataMap(resp.Data)
			if dataMap != nil {
				output = r.renderText(dataMap, false)
			} else {
				switch dd := resp.Data.(type) {
				case map[string]string:
					output = dd["message"]
				case string:
					output = dd
				default:
					output = fmt.Sprintf("%v", dd)
				}
			}
		}
		fmt.Println(output)
	}

	if isError {
		return errors.New("")
	}
	return nil
}

func (r *ResponseRenderer) renderText(dataMap map[string]interface{}, isError bool) string {
	dataMap = normalizeDataMap(dataMap)
	if r.appConfig != nil {
		if fields := r.appConfig.ConfigManager.GetConfig().Fields; len(fields) > 0 {
			dataMap = filterFields(dataMap, fields)
		}
	}
	formatType := app.FormatTypeTree
	if r.appConfig != nil {
		formatType = r.appConfig.ConfigManager.GetConfig().FormatType
	}
	return r.RenderText(dataMap, formatType, isError)
}

func (r *ResponseRenderer) RenderText(dataMap map[string]interface{}, formatType string, isError bool) string {
	dataMap = normalizeDataMap(dataMap)
	switch formatType {
	case app.FormatTypePlain:
		return r.renderPlain(dataMap, isError)
	default:
		return r.renderTree(dataMap, isError)
	}
}

func filterFields(data map[string]interface{}, fields []string) map[string]interface{} {
	target := data
	var wrapperKey string

	keys := sortedKeys(data)
	if len(keys) == 1 {
		if mm, ok := data[keys[0]].(map[string]interface{}); ok {
			wrapperKey = keys[0]
			target = mm
		}
	}

	// Разбираем поля: простые "name" и с точкой "appStream.id" для вложенных данных
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
				filtered[k] = filterFields(mm, children)
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
						fm := filterFields(mm, remainingFields)
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
