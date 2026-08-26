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

package dbusv1

import (
	"context"
	"encoding/json"
	"errors"

	"altlinux.space/alt-atomic/apm/internal/common/apmerr"
	"altlinux.space/alt-atomic/apm/internal/common/app"
	"altlinux.space/alt-atomic/apm/internal/common/helper"
	"altlinux.space/alt-atomic/apm/internal/common/reply"

	"github.com/godbus/dbus/v5"
)

// signalName единственный сигнал API v1: JSON-строка с событием.
const signalName = "org.altlinux.APM.Notification"

// Sink рассылает события Reporter совместимыми сигналами API v1.
type Sink struct {
	conn *dbus.Conn
}

// NewSink создаёт приёмник событий поверх соединения демона.
func NewSink(conn *dbus.Conn) *Sink {
	return &Sink{conn: conn}
}

// Notify шлёт уведомление или прогресс.
func (s *Sink) Notify(_ context.Context, ev *reply.EventData) {
	if err := s.emit(ev); err != nil {
		app.Log.Error(app.T_("Error sending notification: %v"), err)
	}
}

// TaskResult шлёт результат фоновой задачи.
func (s *Sink) TaskResult(ctx context.Context, name string, data interface{}, taskErr error) {
	tx, _ := ctx.Value(helper.TransactionKey).(string)
	event := reply.TaskResultEvent{
		Type:        reply.EventTypeTaskResult,
		Name:        name,
		Transaction: tx,
		Data:        data,
	}
	if taskErr != nil {
		if apmErr, ok := errors.AsType[apmerr.APMError](taskErr); ok {
			event.Error = &reply.APIError{ErrorCode: apmErr.Type, Message: taskErr.Error()}
		} else {
			event.Error = &reply.APIError{Message: taskErr.Error()}
		}
		event.Data = nil
	}
	if err := s.emit(event); err != nil {
		app.Log.Error(app.T_("Error sending task result: %v"), err)
	}
}

// emit сериализует событие в JSON и шлёт сигналом Notification.
func (s *Sink) emit(event any) error {
	message, err := json.Marshal(event)
	if err != nil {
		return err
	}
	if s.conn == nil {
		return errors.New(app.T_("DBus connection is not initialized"))
	}
	return s.conn.Emit(Path, signalName, string(message))
}
