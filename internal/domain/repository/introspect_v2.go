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

package repository

import (
	"altlinux.space/alt-atomic/apm/internal/common/dbusv2/protocol"

	"github.com/godbus/dbus/v5/introspect"
)

// repoIntrospectionV2 описание интерфейса org.altlinux.APM2.Repo.
var repoIntrospectionV2 = introspect.Interface{
	Name: protocol.RepoIface,
	Methods: []introspect.Method{
		{Name: "List", Args: []introspect.Arg{
			protocol.In("all", "b"),
			protocol.Out("json", "s"),
		}},
		{Name: "Branches", Args: []introspect.Arg{
			protocol.Out("branches", "as"),
		}},
		{Name: "TaskPackages", Args: []introspect.Arg{
			protocol.In("task", "s"),
			protocol.Out("job", "u"),
		}},
		{Name: "TestTask", Args: []introspect.Arg{
			protocol.In("task", "s"),
			protocol.Out("job", "u"),
		}},
		{Name: "Add", Args: []introspect.Arg{
			protocol.In("sources", "as"),
			protocol.In("date", "s"),
			protocol.Out("json", "s"),
		}},
		{Name: "Remove", Args: []introspect.Arg{
			protocol.In("sources", "as"),
			protocol.In("date", "s"),
			protocol.Out("json", "s"),
		}},
		{Name: "SetBranch", Args: []introspect.Arg{
			protocol.In("branch", "s"),
			protocol.In("date", "s"),
			protocol.Out("json", "s"),
		}},
		{Name: "Clean", Args: []introspect.Arg{
			protocol.Out("json", "s"),
		}},
		{Name: "CheckAdd", Args: []introspect.Arg{
			protocol.In("sources", "as"),
			protocol.In("date", "s"),
			protocol.Out("json", "s"),
		}},
		{Name: "CheckRemove", Args: []introspect.Arg{
			protocol.In("sources", "as"),
			protocol.In("date", "s"),
			protocol.Out("json", "s"),
		}},
		{Name: "CheckSetBranch", Args: []introspect.Arg{
			protocol.In("branch", "s"),
			protocol.In("date", "s"),
			protocol.Out("json", "s"),
		}},
		{Name: "CheckClean", Args: []introspect.Arg{
			protocol.Out("json", "s"),
		}},
	},
}
