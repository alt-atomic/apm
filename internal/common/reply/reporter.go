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
	"errors"
)

// Reporter инкапсулирует доставку ответов и событий приложения.
type Reporter struct {
	appConfig *app.Config
	renderer  *responseRenderer
}

// NewReporter создаёт Reporter поверх appConfig.
func NewReporter(appConfig *app.Config) *Reporter {
	return &Reporter{
		appConfig: appConfig,
		renderer:  newResponseRenderer(appConfig),
	}
}

// CliResponse рендерит APIResponse в выбранном формате (text/json/dbus/http).
func (r *Reporter) CliResponse(ctx context.Context, resp APIResponse) error {
	return r.renderer.CliResponse(ctx, resp)
}

// CreateEventNotification создаёт EventData и отправляет через dispatchEvent.
func (r *Reporter) CreateEventNotification(ctx context.Context, state string, opts ...NotificationOption) {
	ed := EventData{
		Name:            "",
		State:           state,
		Type:            EventTypeNotification,
		ProgressPercent: 0,
	}
	for _, opt := range opts {
		opt(&ed)
	}
	if ed.Name == "" {
		ed.Name = "unknown"
	}
	if ed.View == "" {
		ed.View = getTaskText(ed.Name)
	}
	r.dispatchEvent(ctx, &ed)
}

// dispatchEvent отправляет уведомление выбранным транспортом (DBus, WebSocket, лог).
func (r *Reporter) dispatchEvent(ctx context.Context, eventData *EventData) {
	if txStr, ok := ctx.Value(helper.TransactionKey).(string); ok {
		eventData.Transaction = txStr
	}

	config := r.appConfig.ConfigManager.GetConfig()

	if config.Verbose {
		logVerboseEvent(eventData)
	} else {
		updateTask(r.appConfig, eventData.Type, eventData.Name, eventData.View, eventData.State, eventData.ProgressPercent, eventData.ProgressDone)
	}

	switch config.Format {
	case app.FormatDBus:
		sendNotificationResponse(eventData, r.appConfig.DBusManager.GetConnection())
	case app.FormatHTTP:
		sendWebSocketNotification(eventData)
	}
}

// SendTaskResult отправляет результат фоновой задачи через DBus или WebSocket.
func (r *Reporter) SendTaskResult(ctx context.Context, taskName string, data interface{}, taskErr error) {
	txStr, _ := ctx.Value(helper.TransactionKey).(string)

	event := TaskResultEvent{
		Type:        EventTypeTaskResult,
		Name:        taskName,
		Transaction: txStr,
		Data:        data,
	}

	if taskErr != nil {
		var apmErr apmerr.APMError
		if errors.As(taskErr, &apmErr) {
			event.Error = &APIError{ErrorCode: apmErr.Type, Message: taskErr.Error()}
		} else {
			event.Error = &APIError{Message: taskErr.Error()}
		}
		event.Data = nil
	}

	switch r.appConfig.ConfigManager.GetConfig().Format {
	case app.FormatDBus:
		sendTaskResultDBus(&event, r.appConfig.DBusManager.GetConnection())
	case app.FormatHTTP:
		sendTaskResultWebSocket(&event)
	}
}
