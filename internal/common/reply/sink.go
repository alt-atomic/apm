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

import "context"

// Sink получает события Reporter дополнительно к штатным транспортам.
type Sink interface {
	Notify(ctx context.Context, ev *EventData)
	TaskResult(ctx context.Context, name string, data interface{}, taskErr error)
}

// AddSink регистрирует дополнительный приёмник событий.
func (r *Reporter) AddSink(s Sink) {
	r.sinksMu.Lock()
	defer r.sinksMu.Unlock()
	r.sinks = append(r.sinks, s)
}

// notifySinks рассылает событие всем зарегистрированным приёмникам.
func (r *Reporter) notifySinks(ctx context.Context, ev *EventData) {
	r.sinksMu.RLock()
	defer r.sinksMu.RUnlock()
	for _, s := range r.sinks {
		s.Notify(ctx, ev)
	}
}

// taskResultSinks рассылает результат задачи всем приёмникам.
func (r *Reporter) taskResultSinks(ctx context.Context, name string, data interface{}, taskErr error) {
	r.sinksMu.RLock()
	defer r.sinksMu.RUnlock()
	for _, s := range r.sinks {
		s.TaskResult(ctx, name, data, taskErr)
	}
}
