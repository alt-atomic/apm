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

package jobs

import (
	"context"

	"altlinux.space/alt-atomic/apm/internal/common/reply"
)

// Sink транслирует события Reporter в сигналы JobProgress.
type Sink struct {
	reg *Registry
}

// NewSink создаёт приёмник событий для реестра.
func NewSink(reg *Registry) *Sink {
	return &Sink{reg: reg}
}

// Notify шлёт JobProgress, если событие принадлежит задаче.
func (s *Sink) Notify(ctx context.Context, ev *reply.EventData) {
	id, ok := FromContext(ctx)
	if !ok {
		return
	}
	s.reg.emit("JobProgress", id, ev.Name, ev.Type, ev.State, ev.ProgressPercent, ev.ProgressDone)
}

// TaskResult не используется: финал задачи шлёт сам Registry.
func (s *Sink) TaskResult(context.Context, string, interface{}, error) {}
