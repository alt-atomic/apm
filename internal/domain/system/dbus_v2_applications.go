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
	"context"
	"fmt"

	"altlinux.space/alt-atomic/apm/internal/common/apmerr"
	"altlinux.space/alt-atomic/apm/internal/common/dbusv2"
	"altlinux.space/alt-atomic/apm/internal/common/dbusv2/authz"
	"altlinux.space/alt-atomic/apm/internal/common/dbusv2/jobs"
	"altlinux.space/alt-atomic/apm/internal/common/dbusv2/protocol"
	"altlinux.space/alt-atomic/apm/internal/common/dbusv2/wire"
	"altlinux.space/alt-atomic/apm/internal/common/polkit"
	"altlinux.space/alt-atomic/apm/internal/common/swcat"
	"altlinux.space/alt-atomic/apm/internal/domain/system/appstream"

	"github.com/godbus/dbus/v5"
)

// applicationsModuleV2 модуль интерфейса org.altlinux.APM2.Applications.
func applicationsModuleV2(actions *appstream.Actions) dbusv2.Module {
	return dbusv2.Module{
		Iface:         protocol.ApplicationsIface,
		Introspection: applicationsIntrospectionV2,
		Build: func(ctx context.Context, reg *jobs.Registry, az authz.Authorizer) any {
			return &ApplicationsV2{
				ctx:     ctx,
				actions: actions,
				jobs:    reg,
				az:      az,
			}
		},
	}
}

// ApplicationsV2 DBus-объект интерфейса Applications (каталог AppStream).
type ApplicationsV2 struct {
	ctx     context.Context
	actions *appstream.Actions
	jobs    *jobs.Registry
	az      authz.Authorizer
}

// guard проверяет единое право на управление каталогом приложений.
func (w *ApplicationsV2) guard(msg dbus.Message) *dbus.Error {
	return wire.Error(w.az.Authorize(msg, protocol.ActionApplicationsManage))
}

// Update обновляет каталог приложений фоновой задачей.
func (w *ApplicationsV2) Update(msg dbus.Message) (string, *dbus.Error) {
	if dbusErr := w.guard(msg); dbusErr != nil {
		return "", dbusErr
	}
	job := w.jobs.Start(jobs.ResourceHost, "applications", "Update", polkit.Sender(msg), protocol.ActionApplicationsManage,
		wire.JSONTask(func(ctx context.Context) (*appstream.UpdateResponse, error) {
			return w.actions.Update(ctx)
		}))
	return job, nil
}

// Info возвращает AppStream-компоненты пакета.
func (w *ApplicationsV2) Info(name string) (string, *dbus.Error) {
	resp, err := w.actions.Info(w.ctx, name)
	return wire.JSONReply(resp, err)
}

// List возвращает страницу AppStream-компонентов по запросу и фильтрам.
func (w *ApplicationsV2) List(requestJSON string) (string, *dbus.Error) {
	var request wire.ForceUpdateListRequest
	if err := wire.DecodeJSON(requestJSON, &request); err != nil {
		return "", wire.Error(err)
	}

	if request.ForceUpdate {
		return "", wire.Error(apmerr.New(apmerr.ErrorTypeValidation,
			fmt.Errorf("forceUpdate is not supported by Applications.List")))
	}

	page, err := request.Validate(swcat.FilterConfig)
	if err != nil {
		return "", wire.Error(err)
	}

	resp, err := w.actions.List(w.ctx, appstream.ListParams{
		Sort:    request.Sort,
		Order:   request.Order,
		Limit:   page.Limit,
		Offset:  page.Offset,
		Filters: page.Filters,
	})
	return wire.JSONReply(resp, err)
}

// Categories возвращает список категорий приложений.
func (w *ApplicationsV2) Categories() ([]string, *dbus.Error) {
	resp, err := w.actions.Categories(w.ctx)
	if err != nil {
		return nil, wire.Error(err)
	}
	return resp.Categories, nil
}

// FilterFields возвращает описание полей фильтрации каталога.
func (w *ApplicationsV2) FilterFields() (string, *dbus.Error) {
	resp, err := w.actions.GetFilterFields(w.ctx)
	return wire.JSONReply(resp, err)
}
