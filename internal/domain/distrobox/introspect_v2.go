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

package distrobox

import (
	"altlinux.space/alt-atomic/apm/internal/common/dbusv2/protocol"

	"github.com/godbus/dbus/v5/introspect"
)

// distroboxIntrospectionV2 описание интерфейса org.altlinux.APM2.Distrobox.
var distroboxIntrospectionV2 = introspect.Interface{
	Name: protocol.DistroboxIface,
	Methods: []introspect.Method{
		{Name: "ContainerList", Args: []introspect.Arg{
			protocol.Out("json", "s"),
		}},
		{Name: "ContainerAdd", Args: []introspect.Arg{
			protocol.In("image", "s"),
			protocol.In("name", "s"),
			protocol.In("options_json", "s"),
			protocol.Out("job", "u"),
		}},
		{Name: "ContainerRemove", Args: []introspect.Arg{
			protocol.In("name", "s"),
			protocol.Out("job", "u"),
		}},
		{Name: "Update", Args: []introspect.Arg{
			protocol.In("container", "s"),
			protocol.Out("job", "u"),
		}},
		{Name: "Install", Args: []introspect.Arg{
			protocol.In("container", "s"),
			protocol.In("name", "s"),
			protocol.In("options_json", "s"),
			protocol.Out("job", "u"),
		}},
		{Name: "Remove", Args: []introspect.Arg{
			protocol.In("container", "s"),
			protocol.In("name", "s"),
			protocol.In("options_json", "s"),
			protocol.Out("job", "u"),
		}},
		{Name: "Info", Args: []introspect.Arg{
			protocol.In("container", "s"),
			protocol.In("name", "s"),
			protocol.Out("json", "s"),
		}},
		{Name: "Search", Args: []introspect.Arg{
			protocol.In("container", "s"),
			protocol.In("text", "s"),
			protocol.Out("json", "s"),
		}},
		{Name: "List", Args: []introspect.Arg{
			protocol.In("container", "s"),
			protocol.In("request_json", "s"),
			protocol.Out("json", "s"),
		}},
		{Name: "FilterFields", Args: []introspect.Arg{
			protocol.Out("json", "s"),
		}},
	},
}

// iconsIntrospectionV2 описание интерфейса org.altlinux.APM2.Icons.
var iconsIntrospectionV2 = introspect.Interface{
	Name: protocol.IconsIface,
	Methods: []introspect.Method{
		{Name: "Icon", Args: []introspect.Arg{
			protocol.In("name", "s"),
			protocol.In("container", "s"),
			protocol.Out("icon", "ay"),
		}},
	},
}
