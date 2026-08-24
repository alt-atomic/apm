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
			protocol.In("options", "a{sv}"),
			protocol.Out("job", "u"),
		}},
		{Name: "Remove", Args: []introspect.Arg{
			protocol.In("packages", "as"),
			protocol.In("options", "a{sv}"),
			protocol.Out("job", "u"),
		}},
		{Name: "Reinstall", Args: []introspect.Arg{
			protocol.In("packages", "as"),
			protocol.Out("job", "u"),
		}},
		{Name: "Upgrade", Args: []introspect.Arg{
			protocol.In("options", "a{sv}"),
			protocol.Out("job", "u"),
		}},
		{Name: "Update", Args: []introspect.Arg{
			protocol.In("options", "a{sv}"),
			protocol.Out("job", "u"),
		}},
		{Name: "CheckInstall", Args: []introspect.Arg{
			protocol.In("packages", "as"),
			protocol.Out("changes", "a{sv}"),
		}},
		{Name: "CheckRemove", Args: []introspect.Arg{
			protocol.In("packages", "as"),
			protocol.In("options", "a{sv}"),
			protocol.Out("changes", "a{sv}"),
		}},
		{Name: "CheckUpgrade", Args: []introspect.Arg{
			protocol.Out("changes", "a{sv}"),
		}},
		{Name: "List", Args: []introspect.Arg{
			protocol.In("query", "a{sv}"),
			protocol.In("filters", "aa(sss)"),
			protocol.Out("total", "u"),
			protocol.Out("packages", "aa{sv}"),
		}},
		{Name: "Info", Args: []introspect.Arg{
			protocol.In("name", "s"),
			protocol.Out("package", "a{sv}"),
		}},
		{Name: "MultiInfo", Args: []introspect.Arg{
			protocol.In("names", "as"),
			protocol.Out("packages", "aa{sv}"),
			protocol.Out("not_found", "as"),
		}},
		{Name: "Search", Args: []introspect.Arg{
			protocol.In("text", "s"),
			protocol.In("installed", "b"),
			protocol.Out("packages", "aa{sv}"),
		}},
		{Name: "Sections", Args: []introspect.Arg{
			protocol.Out("sections", "as"),
		}},
		{Name: "FilterFields", Args: []introspect.Arg{
			protocol.Out("fields", "aa{sv}"),
		}},
		{Name: "AptConfig", Args: []introspect.Arg{
			protocol.Out("options", "a{ss}"),
		}},
		{Name: "SetAptConfig", Args: []introspect.Arg{
			protocol.In("options", "a{ss}"),
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
			protocol.Out("status", "a{sv}"),
		}},
		{Name: "Update", Args: []introspect.Arg{
			protocol.In("options", "a{sv}"),
			protocol.Out("job", "u"),
		}},
		{Name: "Apply", Args: []introspect.Arg{
			protocol.In("options", "a{sv}"),
			protocol.Out("job", "u"),
		}},
		{Name: "Switch", Args: []introspect.Arg{
			protocol.In("image", "s"),
			protocol.In("options", "a{sv}"),
			protocol.Out("job", "u"),
		}},
		{Name: "History", Args: []introspect.Arg{
			protocol.In("image", "s"),
			protocol.In("limit", "u"),
			protocol.In("offset", "u"),
			protocol.Out("total", "u"),
			protocol.Out("history", "aa{sv}"),
		}},
		{Name: "GetConfig", Args: []introspect.Arg{
			protocol.Out("yaml", "s"),
		}},
		{Name: "SaveConfig", Args: []introspect.Arg{
			protocol.In("yaml", "s"),
		}},
		{Name: "SyncGroups", Args: []introspect.Arg{
			protocol.Out("result", "a{sv}"),
		}},
		{Name: "FixNss", Args: []introspect.Arg{
			protocol.Out("result", "a{sv}"),
		}},
	},
}

// applicationsIntrospectionV2 описание интерфейса org.altlinux.APM2.Applications.
var applicationsIntrospectionV2 = introspect.Interface{
	Name: protocol.ApplicationsIface,
	Methods: []introspect.Method{
		{Name: "Update", Args: []introspect.Arg{
			protocol.Out("job", "u"),
		}},
		{Name: "Info", Args: []introspect.Arg{
			protocol.In("name", "s"),
			protocol.Out("info", "a{sv}"),
		}},
		{Name: "List", Args: []introspect.Arg{
			protocol.In("query", "a{sv}"),
			protocol.In("filters", "aa(sss)"),
			protocol.Out("total", "u"),
			protocol.Out("applications", "aa{sv}"),
		}},
		{Name: "Categories", Args: []introspect.Arg{
			protocol.Out("categories", "as"),
		}},
		{Name: "FilterFields", Args: []introspect.Arg{
			protocol.Out("fields", "aa{sv}"),
		}},
	},
}
