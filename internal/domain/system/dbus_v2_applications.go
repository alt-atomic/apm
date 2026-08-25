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

	"altlinux.space/alt-atomic/apm/internal/common/app"
	"altlinux.space/alt-atomic/apm/internal/common/dbusv2"
	"altlinux.space/alt-atomic/apm/internal/common/dbusv2/authz"
	"altlinux.space/alt-atomic/apm/internal/common/dbusv2/jobs"
	"altlinux.space/alt-atomic/apm/internal/common/dbusv2/protocol"
	"altlinux.space/alt-atomic/apm/internal/common/dbusv2/wire"
	"altlinux.space/alt-atomic/apm/internal/common/polkit"
	"altlinux.space/alt-atomic/apm/internal/common/reply"
	"altlinux.space/alt-atomic/apm/internal/common/swcat"
	"altlinux.space/alt-atomic/apm/internal/domain/system/appstream"

	"github.com/godbus/dbus/v5"
)

// applicationsModuleV2 модуль интерфейса org.altlinux.APM2.Applications.
func applicationsModuleV2(appConfig *app.Config, reporter *reply.Reporter) dbusv2.Module {
	return dbusv2.Module{
		Iface:         protocol.ApplicationsIface,
		Introspection: applicationsIntrospectionV2,
		Build: func(ctx context.Context, reg *jobs.Registry, az authz.Authorizer) any {
			return &ApplicationsV2{
				ctx:     ctx,
				actions: appstream.NewActions(appConfig, reporter),
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

// Update обновляет каталог приложений фоновой задачей.
func (w *ApplicationsV2) Update(msg dbus.Message) (uint32, *dbus.Error) {
	if err := w.az.Authorize(msg, protocol.ActionApplicationsManage); err != nil {
		return 0, wire.Error(err)
	}
	job := w.jobs.Start("applications", "Update", polkit.Sender(msg), protocol.ActionApplicationsManage,
		func(ctx context.Context) (wire.Dict, error) {
			resp, err := w.actions.Update(ctx)
			if err != nil {
				return nil, err
			}
			return wire.StructDict(resp)
		})
	return job, nil
}

// Info возвращает AppStream-компоненты пакета.
func (w *ApplicationsV2) Info(name string) (wire.Dict, *dbus.Error) {
	resp, err := w.actions.Info(w.ctx, name)
	if err != nil {
		return nil, wire.Error(err)
	}
	return wire.Reply(wire.StructDict(resp))
}

// List возвращает страницу AppStream-компонентов по запросу и фильтрам.
func (w *ApplicationsV2) List(query wire.Dict, filters [][]filterRuleV2) (uint32, []wire.Dict, *dbus.Error) {
	var sort, order string
	var limit, offset uint32
	if err := wire.ParseOptions(query, map[string]any{
		"sort":   &sort,
		"order":  &order,
		"limit":  &limit,
		"offset": &offset,
	}); err != nil {
		return 0, nil, wire.Error(err)
	}

	pageSize, err := wire.PageLimit(limit)
	if err != nil {
		return 0, nil, wire.Error(err)
	}

	groups, err := wire.FilterGroups(filters, swcat.FilterConfig)
	if err != nil {
		return 0, nil, wire.Error(err)
	}

	resp, err := w.actions.List(w.ctx, appstream.ListParams{
		Sort:    sort,
		Order:   order,
		Limit:   pageSize,
		Offset:  int(offset),
		Filters: groups,
	})
	if err != nil {
		return 0, nil, wire.Error(err)
	}

	rows, err := wire.StructDicts(resp.Components)
	if err != nil {
		return 0, nil, wire.Error(err)
	}
	return uint32(max(resp.TotalCount, 0)), rows, nil
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
func (w *ApplicationsV2) FilterFields() ([]wire.Dict, *dbus.Error) {
	resp, err := w.actions.GetFilterFields(w.ctx)
	if err != nil {
		return nil, wire.Error(err)
	}
	return wire.Reply(wire.StructDicts(resp))
}
