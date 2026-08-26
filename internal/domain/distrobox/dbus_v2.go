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

	"altlinux.space/alt-atomic/apm/internal/common/dbusv2"
	"altlinux.space/alt-atomic/apm/internal/common/dbusv2/authz"
	"altlinux.space/alt-atomic/apm/internal/common/dbusv2/jobs"
	"altlinux.space/alt-atomic/apm/internal/common/dbusv2/protocol"
	"altlinux.space/alt-atomic/apm/internal/common/dbusv2/wire"
	"altlinux.space/alt-atomic/apm/internal/common/polkit"
	"altlinux.space/alt-atomic/apm/internal/common/sandbox"

	"github.com/godbus/dbus/v5"
)

// V2Modules интерфейсы v2 сессионной шины: Distrobox и Icons.
func (s *DBusServices) V2Modules() []dbusv2.Module {
	return []dbusv2.Module{
		distroboxModuleV2(s.actions),
		iconsModuleV2(s.actions),
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
func (w *DBusV2) startJob(msg dbus.Message, kind string, fn func(ctx context.Context) (string, error)) string {
	return w.jobs.Start("distrobox", kind, polkit.Sender(msg), "", fn)
}

// startTransaction регистрирует неотменяемую пакетную транзакцию.
func (w *DBusV2) startTransaction(msg dbus.Message, kind string, fn func(ctx context.Context) (string, error)) string {
	return w.jobs.StartNoCancel("distrobox", kind, polkit.Sender(msg), fn)
}

// ContainerList возвращает список контейнеров.
func (w *DBusV2) ContainerList() (string, *dbus.Error) {
	resp, err := w.actions.ContainerList(w.ctx)
	return wire.JSONReply(resp, err)
}

// ContainerAdd создаёт контейнер фоновой задачей.
func (w *DBusV2) ContainerAdd(msg dbus.Message, image string, name string, optionsJSON string) (string, *dbus.Error) {
	var options struct {
		AdditionalPackages string `json:"additionalPackages"`
		InitHooks          string `json:"initHooks"`
	}
	if err := wire.DecodeOptions(optionsJSON, &options); err != nil {
		return "", wire.Error(err)
	}
	return w.startJob(msg, "ContainerAdd", wire.JSONTask(func(ctx context.Context) (*ContainerAddResponse, error) {
		return w.actions.ContainerAdd(ctx, image, name, options.AdditionalPackages, options.InitHooks)
	})), nil
}

// ContainerRemove удаляет контейнер фоновой задачей.
func (w *DBusV2) ContainerRemove(msg dbus.Message, name string) (string, *dbus.Error) {
	return w.startJob(msg, "ContainerRemove", wire.JSONTask(func(ctx context.Context) (*ContainerRemoveResponse, error) {
		return w.actions.ContainerRemove(ctx, name)
	})), nil
}

// Update обновляет пакетную базу контейнера фоновой задачей.
func (w *DBusV2) Update(msg dbus.Message, container string) (string, *dbus.Error) {
	return w.startJob(msg, "Update", wire.JSONTask(func(ctx context.Context) (*UpdateResponse, error) {
		return w.actions.Update(ctx, container)
	})), nil
}

// Install ставит пакет в контейнер фоновой задачей.
func (w *DBusV2) Install(msg dbus.Message, container string, name string, optionsJSON string) (string, *dbus.Error) {
	var options struct {
		Export bool `json:"export"`
	}
	if err := wire.DecodeOptions(optionsJSON, &options); err != nil {
		return "", wire.Error(err)
	}
	return w.startTransaction(msg, "Install", wire.JSONTask(func(ctx context.Context) (*InstallResponse, error) {
		return w.actions.Install(ctx, container, name, options.Export)
	})), nil
}

// Remove удаляет пакет из контейнера фоновой задачей.
func (w *DBusV2) Remove(msg dbus.Message, container string, name string, optionsJSON string) (string, *dbus.Error) {
	var options struct {
		OnlyExport bool `json:"onlyExport"`
	}
	if err := wire.DecodeOptions(optionsJSON, &options); err != nil {
		return "", wire.Error(err)
	}
	return w.startTransaction(msg, "Remove", wire.JSONTask(func(ctx context.Context) (*RemoveResponse, error) {
		return w.actions.Remove(ctx, container, name, options.OnlyExport)
	})), nil
}

// Info возвращает информацию о пакете контейнера.
func (w *DBusV2) Info(container string, name string) (string, *dbus.Error) {
	resp, err := w.actions.Info(w.ctx, container, name)
	return wire.JSONReply(resp, err)
}

// Search ищет пакеты в контейнере.
func (w *DBusV2) Search(container string, text string) (string, *dbus.Error) {
	resp, err := w.actions.Search(w.ctx, container, text)
	return wire.JSONReply(resp, err)
}

// List возвращает страницу пакетов контейнера по запросу и фильтрам.
func (w *DBusV2) List(container string, requestJSON string) (string, *dbus.Error) {
	var request wire.ListRequest
	if err := wire.DecodeJSON(requestJSON, &request); err != nil {
		return "", wire.Error(err)
	}

	page, err := request.Validate(sandbox.DistroFilterConfig)
	if err != nil {
		return "", wire.Error(err)
	}

	resp, err := w.actions.List(w.ctx, ListParams{
		Container:   container,
		Sort:        request.Sort,
		Order:       request.Order,
		Limit:       page.Limit,
		Offset:      page.Offset,
		Filters:     page.Filters,
		ForceUpdate: request.ForceUpdate,
	})
	return wire.JSONReply(resp, err)
}

// FilterFields возвращает описание полей фильтрации.
func (w *DBusV2) FilterFields() (string, *dbus.Error) {
	resp, err := w.actions.GetFilterFields(w.ctx)
	return wire.JSONReply(resp, err)
}
