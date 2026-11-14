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
	"context"

	"altlinux.space/alt-atomic/apm/internal/common/app"
	"altlinux.space/alt-atomic/apm/internal/common/dbusv2"
	"altlinux.space/alt-atomic/apm/internal/common/dbusv2/authz"
	"altlinux.space/alt-atomic/apm/internal/common/dbusv2/jobs"
	"altlinux.space/alt-atomic/apm/internal/common/dbusv2/protocol"
	"altlinux.space/alt-atomic/apm/internal/common/dbusv2/wire"

	"github.com/godbus/dbus/v5"
)

// iconsModuleV2 модуль интерфейса org.altlinux.APM2.Icons (session bus):
// байтовые иконки пакетов — системных (container="") и контейнерных.
func iconsModuleV2(actions *Actions) dbusv2.Module {
	return dbusv2.Module{
		Iface:         protocol.IconsIface,
		Introspection: iconsIntrospectionV2,
		Build: func(ctx context.Context, _ *jobs.Registry, _ authz.Authorizer) any {
			return &IconsV2{ctx: ctx, actions: actions}
		},
		PostExport: func(ctx context.Context) {
			if err := actions.GetIconService().ReloadIcons(ctx); err != nil {
				app.Log.Error(err.Error())
			}
		},
	}
}

// IconsV2 DBus-объект интерфейса Icons.
type IconsV2 struct {
	ctx     context.Context
	actions *Actions
}

// Icon возвращает PNG-иконку пакета; container="" — системный пакет.
func (w *IconsV2) Icon(name string, container string) ([]byte, *dbus.Error) {
	data, err := w.actions.GetIconByPackage(w.ctx, name, container)
	if err != nil {
		return nil, wire.Error(err)
	}
	return data, nil
}
