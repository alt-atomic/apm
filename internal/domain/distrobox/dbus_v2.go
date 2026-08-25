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
	"altlinux.space/alt-atomic/apm/internal/common/polkit"
	"altlinux.space/alt-atomic/apm/internal/common/reply"
	"altlinux.space/alt-atomic/apm/internal/common/sandbox"

	"github.com/godbus/dbus/v5"
)

// DBusV2Modules интерфейсы v2 сессионной шины: Distrobox и Icons.
func DBusV2Modules(appConfig *app.Config, reporter *reply.Reporter) []dbusv2.Module {
	actions := NewActions(appConfig, reporter)
	return []dbusv2.Module{
		distroboxModuleV2(actions),
		iconsModuleV2(actions),
	}
}

// distroboxModuleV2 модуль интерфейса org.altlinux.APM2.Distrobox.
func distroboxModuleV2(actions *Actions) dbusv2.Module {
	return dbusv2.Module{
		Iface:         protocol.DistroboxIface,
		Introspection: distroboxIntrospectionV2,
		Build: func(ctx context.Context, reg *jobs.Registry, _ authz.Authorizer) any {
			return &DBusV2{
				ctx:     ctx,
				actions: actions,
				jobs:    reg,
			}
		},
	}
}

// DBusV2 DBus-объект интерфейса Distrobox; сессионная шина — авторизация не нужна.
type DBusV2 struct {
	ctx     context.Context
	actions *Actions
	jobs    *jobs.Registry
}

// startJob регистрирует фоновую задачу; отмена — только владельцем.
func (w *DBusV2) startJob(msg dbus.Message, kind string, fn func(ctx context.Context) (wire.Dict, error)) (uint32, *dbus.Error) {
	return w.jobs.Start("distrobox", kind, polkit.Sender(msg), "", fn), nil
}

// startTransaction регистрирует неотменяемую пакетную транзакцию.
func (w *DBusV2) startTransaction(msg dbus.Message, kind string, fn func(ctx context.Context) (wire.Dict, error)) (uint32, *dbus.Error) {
	return w.jobs.StartNoCancel("distrobox", kind, polkit.Sender(msg), fn), nil
}

// ContainerList возвращает список контейнеров.
func (w *DBusV2) ContainerList() ([]wire.Dict, *dbus.Error) {
	resp, err := w.actions.ContainerList(w.ctx)
	if err != nil {
		return nil, wire.Error(err)
	}
	return wire.Reply(wire.StructDicts(resp.Containers))
}

// ContainerAdd создаёт контейнер фоновой задачей.
func (w *DBusV2) ContainerAdd(msg dbus.Message, image string, name string, options wire.Dict) (uint32, *dbus.Error) {
	var packages, initHooks string
	if err := wire.ParseOptions(options, map[string]any{
		"packages":   &packages,
		"init_hooks": &initHooks,
	}); err != nil {
		return 0, wire.Error(err)
	}
	return w.startJob(msg, "ContainerAdd", func(ctx context.Context) (wire.Dict, error) {
		resp, err := w.actions.ContainerAdd(ctx, image, name, packages, initHooks)
		if err != nil {
			return nil, err
		}
		return wire.StructDict(resp)
	})
}

// ContainerRemove удаляет контейнер фоновой задачей.
func (w *DBusV2) ContainerRemove(msg dbus.Message, name string) (uint32, *dbus.Error) {
	return w.startJob(msg, "ContainerRemove", func(ctx context.Context) (wire.Dict, error) {
		resp, err := w.actions.ContainerRemove(ctx, name)
		if err != nil {
			return nil, err
		}
		return wire.StructDict(resp)
	})
}

// Update обновляет пакетную базу контейнера фоновой задачей.
func (w *DBusV2) Update(msg dbus.Message, container string) (uint32, *dbus.Error) {
	return w.startJob(msg, "Update", func(ctx context.Context) (wire.Dict, error) {
		resp, err := w.actions.Update(ctx, container)
		if err != nil {
			return nil, err
		}
		return wire.StructDict(resp)
	})
}

// Install ставит пакет в контейнер фоновой задачей.
func (w *DBusV2) Install(msg dbus.Message, container string, name string, options wire.Dict) (uint32, *dbus.Error) {
	var export bool
	if err := wire.ParseOptions(options, map[string]any{
		"export": &export,
	}); err != nil {
		return 0, wire.Error(err)
	}
	return w.startTransaction(msg, "Install", func(ctx context.Context) (wire.Dict, error) {
		resp, err := w.actions.Install(ctx, container, name, export)
		if err != nil {
			return nil, err
		}
		return wire.StructDict(resp)
	})
}

// Remove удаляет пакет из контейнера фоновой задачей.
func (w *DBusV2) Remove(msg dbus.Message, container string, name string, options wire.Dict) (uint32, *dbus.Error) {
	var onlyExport bool
	if err := wire.ParseOptions(options, map[string]any{
		"only_export": &onlyExport,
	}); err != nil {
		return 0, wire.Error(err)
	}
	return w.startTransaction(msg, "Remove", func(ctx context.Context) (wire.Dict, error) {
		resp, err := w.actions.Remove(ctx, container, name, onlyExport)
		if err != nil {
			return nil, err
		}
		return wire.StructDict(resp)
	})
}

// Info возвращает информацию о пакете контейнера.
func (w *DBusV2) Info(container string, name string) (wire.Dict, *dbus.Error) {
	resp, err := w.actions.Info(w.ctx, container, name)
	if err != nil {
		return nil, wire.Error(err)
	}
	return wire.Reply(wire.StructDict(resp.PackageInfo))
}

// Search ищет пакеты в контейнере.
func (w *DBusV2) Search(container string, text string) ([]wire.Dict, *dbus.Error) {
	resp, err := w.actions.Search(w.ctx, container, text)
	if err != nil {
		return nil, wire.Error(err)
	}
	return wire.Reply(wire.StructDicts(resp.Packages))
}

// List возвращает страницу пакетов контейнера по запросу и фильтрам.
func (w *DBusV2) List(container string, query wire.Dict, filters [][]wire.FilterRule) (uint32, []wire.Dict, *dbus.Error) {
	var sort, order string
	var limit, offset uint32
	var forceUpdate bool
	if err := wire.ParseOptions(query, map[string]any{
		"sort":         &sort,
		"order":        &order,
		"limit":        &limit,
		"offset":       &offset,
		"force_update": &forceUpdate,
	}); err != nil {
		return 0, nil, wire.Error(err)
	}

	pageSize, err := wire.PageLimit(limit)
	if err != nil {
		return 0, nil, wire.Error(err)
	}

	groups, err := wire.FilterGroups(filters, sandbox.DistroFilterConfig)
	if err != nil {
		return 0, nil, wire.Error(err)
	}

	resp, err := w.actions.List(w.ctx, ListParams{
		Container:   container,
		Sort:        sort,
		Order:       order,
		Limit:       pageSize,
		Offset:      int(offset),
		Filters:     groups,
		ForceUpdate: forceUpdate,
	})
	if err != nil {
		return 0, nil, wire.Error(err)
	}
	rows, err := wire.StructDicts(resp.Packages)
	if err != nil {
		return 0, nil, wire.Error(err)
	}
	return uint32(max(resp.TotalCount, 0)), rows, nil
}

// FilterFields возвращает описание полей фильтрации.
func (w *DBusV2) FilterFields() ([]wire.Dict, *dbus.Error) {
	resp, err := w.actions.GetFilterFields(w.ctx)
	if err != nil {
		return nil, wire.Error(err)
	}
	return wire.Reply(wire.StructDicts(resp))
}
