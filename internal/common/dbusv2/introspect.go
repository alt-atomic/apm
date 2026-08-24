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
	"altlinux.space/alt-atomic/apm/internal/common/dbusv2/protocol"

	"github.com/godbus/dbus/v5/introspect"
)

// jobsIntrospection описание интерфейса Jobs.
var jobsIntrospection = introspect.Interface{
	Name: protocol.JobsIface,
	Methods: []introspect.Method{
		{Name: "List", Args: []introspect.Arg{
			protocol.Out("jobs", "aa{sv}"),
		}},
		{Name: "Get", Args: []introspect.Arg{
			protocol.In("job", "u"),
			protocol.Out("state", "a{sv}"),
		}},
		{Name: "Cancel", Args: []introspect.Arg{
			protocol.In("job", "u"),
		}},
	},
	Signals: []introspect.Signal{
		{Name: "JobStarted", Args: []introspect.Arg{
			protocol.SignalArg("job", "u"),
			protocol.SignalArg("domain", "s"),
			protocol.SignalArg("kind", "s"),
		}},
		{Name: "JobProgress", Args: []introspect.Arg{
			protocol.SignalArg("job", "u"),
			protocol.SignalArg("event", "s"),
			protocol.SignalArg("type", "s"),
			protocol.SignalArg("state", "s"),
			protocol.SignalArg("progress", "d"),
			protocol.SignalArg("progress_done", "s"),
		}},
		{Name: "JobFinished", Args: []introspect.Arg{
			protocol.SignalArg("job", "u"),
			protocol.SignalArg("status", "s"),
			protocol.SignalArg("message", "s"),
			protocol.SignalArg("result", "a{sv}"),
		}},
	},
}
