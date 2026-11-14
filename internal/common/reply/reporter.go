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
	"sync"

	"altlinux.space/alt-atomic/apm/internal/common/app"
	"altlinux.space/alt-atomic/apm/internal/common/helper"
)

// Reporter инкапсулирует доставку ответов и событий приложения.
type Reporter struct {
	appConfig *app.Config
	renderer  *responseRenderer
	sinksMu   sync.RWMutex
	sinks     []Sink
}

// NewReporter создаёт Reporter поверх appConfig.
func NewReporter(appConfig *app.Config) *Reporter {
	return &Reporter{
		appConfig: appConfig,
		renderer:  newResponseRenderer(appConfig),
	}
}

// CliResponse рендерит APIResponse в выбранном формате вывода.
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

// dispatchEvent отображает уведомление и передаёт его зарегистрированным приёмникам.
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

	r.notifySinks(ctx, eventData)
}

// SendTaskResult передаёт результат фоновой задачи зарегистрированным приёмникам.
func (r *Reporter) SendTaskResult(ctx context.Context, taskName string, data any, taskErr error) {
	r.taskResultSinks(ctx, taskName, data, taskErr)
}
