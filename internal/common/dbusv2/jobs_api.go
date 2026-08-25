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

package dbusv2

import (
	"altlinux.space/alt-atomic/apm/internal/common/dbusv2/authz"
	"altlinux.space/alt-atomic/apm/internal/common/dbusv2/jobs"
	"altlinux.space/alt-atomic/apm/internal/common/dbusv2/wire"
	"altlinux.space/alt-atomic/apm/internal/common/polkit"

	"github.com/godbus/dbus/v5"
)

// JobsAPI DBus-объект интерфейса Jobs.
type JobsAPI struct {
	reg *jobs.Registry
	az  authz.Authorizer
}

// List возвращает снимки всех задач.
func (j *JobsAPI) List() (string, *dbus.Error) {
	return wire.JSONReply(j.reg.List(), nil)
}

// Get возвращает снимок задачи.
func (j *JobsAPI) Get(job uint32) (string, *dbus.Error) {
	state, err := j.reg.Get(job)
	return wire.JSONReply(state, err)
}

// Cancel отменяет задачу: владелец — свободно, остальные — через polkit.
func (j *JobsAPI) Cancel(msg dbus.Message, job uint32) *dbus.Error {
	return wire.Error(j.reg.Cancel(job, polkit.Sender(msg), func(action string) error {
		return j.az.Authorize(msg, action)
	}))
}
