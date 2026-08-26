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

package system

import (
	"altlinux.space/alt-atomic/apm/internal/common/dbusv2/protocol"

	"github.com/godbus/dbus/v5/introspect"
)

// packagesIntrospectionV2 описание интерфейса org.altlinux.APM2.Packages.
var packagesIntrospectionV2 = introspect.Interface{
	Name: protocol.PackagesIface,
	Methods: []introspect.Method{
		{Name: "Install", Args: []introspect.Arg{
			protocol.In("packages", "as"),
			protocol.In("options_json", "s"),
			protocol.Out("job", "s"),
		}},
		{Name: "Remove", Args: []introspect.Arg{
			protocol.In("packages", "as"),
			protocol.In("options_json", "s"),
			protocol.Out("job", "s"),
		}},
		{Name: "Reinstall", Args: []introspect.Arg{
			protocol.In("packages", "as"),
			protocol.Out("job", "s"),
		}},
		{Name: "Upgrade", Args: []introspect.Arg{
			protocol.In("options_json", "s"),
			protocol.Out("job", "s"),
		}},
		{Name: "Update", Args: []introspect.Arg{
			protocol.In("options_json", "s"),
			protocol.Out("job", "s"),
		}},
		{Name: "CheckInstall", Args: []introspect.Arg{
			protocol.In("packages", "as"),
			protocol.Out("job", "s"),
		}},
		{Name: "CheckRemove", Args: []introspect.Arg{
			protocol.In("packages", "as"),
			protocol.In("options_json", "s"),
			protocol.Out("job", "s"),
		}},
		{Name: "CheckUpgrade", Args: []introspect.Arg{
			protocol.Out("job", "s"),
		}},
		{Name: "List", Args: []introspect.Arg{
			protocol.In("request_json", "s"),
			protocol.Out("json", "s"),
		}},
		{Name: "Info", Args: []introspect.Arg{
			protocol.In("name", "s"),
			protocol.Out("json", "s"),
		}},
		{Name: "MultiInfo", Args: []introspect.Arg{
			protocol.In("names", "as"),
			protocol.Out("json", "s"),
		}},
		{Name: "Search", Args: []introspect.Arg{
			protocol.In("text", "s"),
			protocol.In("installed", "b"),
			protocol.Out("json", "s"),
		}},
		{Name: "Sections", Args: []introspect.Arg{
			protocol.Out("sections", "as"),
		}},
		{Name: "FilterFields", Args: []introspect.Arg{
			protocol.Out("json", "s"),
		}},
		{Name: "AptConfig", Args: []introspect.Arg{
			protocol.Out("json", "s"),
		}},
		{Name: "SetAptConfig", Args: []introspect.Arg{
			protocol.In("options_json", "s"),
		}},
	},
	Properties: []introspect.Property{
		{Name: "Version", Type: "s", Access: "read"},
		{Name: "IsAtomic", Type: "b", Access: "read"},
		{Name: "KernelSupported", Type: "b", Access: "read"},
	},
}

// imageIntrospectionV2 описание интерфейса org.altlinux.APM2.Image.
var imageIntrospectionV2 = introspect.Interface{
	Name: protocol.ImageIface,
	Methods: []introspect.Method{
		{Name: "Status", Args: []introspect.Arg{
			protocol.Out("json", "s"),
		}},
		{Name: "Update", Args: []introspect.Arg{
			protocol.In("options_json", "s"),
			protocol.Out("job", "s"),
		}},
		{Name: "Apply", Args: []introspect.Arg{
			protocol.In("options_json", "s"),
			protocol.Out("job", "s"),
		}},
		{Name: "Switch", Args: []introspect.Arg{
			protocol.In("image", "s"),
			protocol.In("options_json", "s"),
			protocol.Out("job", "s"),
		}},
		{Name: "History", Args: []introspect.Arg{
			protocol.In("image", "s"),
			protocol.In("request_json", "s"),
			protocol.Out("json", "s"),
		}},
		{Name: "GetConfig", Args: []introspect.Arg{
			protocol.Out("json", "s"),
		}},
		{Name: "SaveConfig", Args: []introspect.Arg{
			protocol.In("config_json", "s"),
		}},
		{Name: "SyncGroups", Args: []introspect.Arg{
			protocol.Out("json", "s"),
		}},
		{Name: "FixNss", Args: []introspect.Arg{
			protocol.Out("json", "s"),
		}},
	},
}

// applicationsIntrospectionV2 описание интерфейса org.altlinux.APM2.Applications.
var applicationsIntrospectionV2 = introspect.Interface{
	Name: protocol.ApplicationsIface,
	Methods: []introspect.Method{
		{Name: "Update", Args: []introspect.Arg{
			protocol.Out("job", "s"),
		}},
		{Name: "Info", Args: []introspect.Arg{
			protocol.In("name", "s"),
			protocol.Out("json", "s"),
		}},
		{Name: "List", Args: []introspect.Arg{
			protocol.In("request_json", "s"),
			protocol.Out("json", "s"),
		}},
		{Name: "Categories", Args: []introspect.Arg{
			protocol.Out("categories", "as"),
		}},
		{Name: "FilterFields", Args: []introspect.Arg{
			protocol.Out("json", "s"),
		}},
	},
}
