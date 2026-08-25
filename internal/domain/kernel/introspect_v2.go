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

package kernel

import (
	"altlinux.space/alt-atomic/apm/internal/common/dbusv2/protocol"

	"github.com/godbus/dbus/v5/introspect"
)

// kernelIntrospectionV2 описание интерфейса org.altlinux.APM2.Kernel.
var kernelIntrospectionV2 = introspect.Interface{
	Name: protocol.KernelIface,
	Methods: []introspect.Method{
		{Name: "ListKernels", Args: []introspect.Arg{
			protocol.In("flavour", "s"),
			protocol.In("installed_only", "b"),
			protocol.Out("json", "s"),
		}},
		{Name: "Current", Args: []introspect.Arg{
			protocol.Out("json", "s"),
		}},
		{Name: "Install", Args: []introspect.Arg{
			protocol.In("flavour", "s"),
			protocol.In("modules", "as"),
			protocol.In("options_json", "s"),
			protocol.Out("job", "u"),
		}},
		{Name: "Update", Args: []introspect.Arg{
			protocol.In("flavour", "s"),
			protocol.In("modules", "as"),
			protocol.In("options_json", "s"),
			protocol.Out("job", "u"),
		}},
		{Name: "CheckInstall", Args: []introspect.Arg{
			protocol.In("flavour", "s"),
			protocol.In("modules", "as"),
			protocol.In("options_json", "s"),
			protocol.Out("json", "s"),
		}},
		{Name: "CheckUpdate", Args: []introspect.Arg{
			protocol.In("flavour", "s"),
			protocol.In("modules", "as"),
			protocol.In("options_json", "s"),
			protocol.Out("json", "s"),
		}},
		{Name: "CleanOld", Args: []introspect.Arg{
			protocol.In("options_json", "s"),
			protocol.Out("job", "u"),
		}},
		{Name: "CheckCleanOld", Args: []introspect.Arg{
			protocol.In("options_json", "s"),
			protocol.Out("json", "s"),
		}},
		{Name: "ListModules", Args: []introspect.Arg{
			protocol.In("flavour", "s"),
			protocol.Out("json", "s"),
		}},
		{Name: "InstallModules", Args: []introspect.Arg{
			protocol.In("flavour", "s"),
			protocol.In("modules", "as"),
			protocol.Out("job", "u"),
		}},
		{Name: "CheckInstallModules", Args: []introspect.Arg{
			protocol.In("flavour", "s"),
			protocol.In("modules", "as"),
			protocol.Out("json", "s"),
		}},
		{Name: "RemoveModules", Args: []introspect.Arg{
			protocol.In("flavour", "s"),
			protocol.In("modules", "as"),
			protocol.Out("job", "u"),
		}},
		{Name: "CheckRemoveModules", Args: []introspect.Arg{
			protocol.In("flavour", "s"),
			protocol.In("modules", "as"),
			protocol.Out("json", "s"),
		}},
	},
}
