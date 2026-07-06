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
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"unicode"

	"altlinux.space/alt-atomic/apm/internal/common/apmerr"
	"altlinux.space/alt-atomic/apm/internal/common/app"
	"altlinux.space/alt-atomic/apm/internal/common/helper"
	"altlinux.space/alt-atomic/apm/pkg/render"

	"golang.org/x/term"
)

// Подключаем к рендереру переводы apm и словарь имён полей
func init() {
	render.SetTranslator(func(msgid string) string { return app.T_(msgid) })
	render.SetKeyTranslator(TranslateKey)
}

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

type responseRenderer struct {
	appConfig *app.Config
	*render.Renderer
}

func newResponseRenderer(appConfig *app.Config) *responseRenderer {
	return &responseRenderer{
		appConfig: appConfig,
		Renderer: render.New(appConfig.ConfigManager.GetColors(),
			render.WithAccentKeys("name", "packageName", "url"),
			render.WithValueFormatter(formatSizeValue),
		),
	}
}

// formatSizeValue выводит поля размеров в мегабайтах
func formatSizeValue(key string, value interface{}) (string, bool) {
	switch key {
	case "size", "installedSize", "downloadSize", "installSize":
		switch vv := value.(type) {
		case int:
			return helper.AutoSize(vv), true
		case float64:
			return helper.AutoSize(int(vv)), true
		}
	}
	return "", false
}

func IsTTY() bool {
	return term.IsTerminal(int(os.Stdout.Fd()))
}

// IsInteractive возвращает true если формат text и терминал интерактивный (для TUI и других штук)
func IsInteractive(appConfig *app.Config) bool {
	return appConfig.ConfigManager.GetConfig().Format == app.FormatText && IsTTY()
}

// MessageWithHint дополняет message подсказкой об использовании --full
func MessageWithHint(message string, full bool) string {
	if full {
		return message
	}
	return message + ". " + app.T_("Use --full for detailed output")
}

func (r *responseRenderer) CliResponse(ctx context.Context, resp APIResponse) error {
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
			if dataMap := render.ToDataMap(resp.Data); dataMap != nil {
				delete(dataMap, "message")
				if len(fields) > 0 {
					dataMap = render.FilterFields(render.NormalizeDataMap(dataMap), fields)
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
			dataMap := render.ToDataMap(resp.Data)
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

func (r *responseRenderer) renderText(dataMap map[string]interface{}, isError bool) string {
	dataMap = render.NormalizeDataMap(dataMap)
	if r.appConfig != nil {
		if fields := r.appConfig.ConfigManager.GetConfig().Fields; len(fields) > 0 {
			dataMap = render.FilterFields(dataMap, fields)
		}
	}
	formatType := app.FormatTypeTree
	if r.appConfig != nil {
		formatType = r.appConfig.ConfigManager.GetConfig().FormatType
	}
	return r.RenderText(dataMap, render.FormatType(formatType), isError)
}
