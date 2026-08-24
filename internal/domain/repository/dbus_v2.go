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
	"context"

	"altlinux.space/alt-atomic/apm/internal/common/app"
	"altlinux.space/alt-atomic/apm/internal/common/dbusv2"
	"altlinux.space/alt-atomic/apm/internal/common/dbusv2/authz"
	"altlinux.space/alt-atomic/apm/internal/common/dbusv2/jobs"
	"altlinux.space/alt-atomic/apm/internal/common/dbusv2/protocol"
	"altlinux.space/alt-atomic/apm/internal/common/dbusv2/wire"
	"altlinux.space/alt-atomic/apm/internal/common/reply"

	"github.com/godbus/dbus/v5"
)

// DBusV2Module модуль интерфейса org.altlinux.APM2.Repo.
func DBusV2Module(appConfig *app.Config, reporter *reply.Reporter) dbusv2.Module {
	return dbusv2.Module{
		Iface:         protocol.RepoIface,
		Introspection: repoIntrospectionV2,
		Build: func(ctx context.Context, reg *jobs.Registry, az authz.Authorizer) any {
			return &DBusV2{
				ctx:     ctx,
				actions: NewActions(appConfig, reporter),
				jobs:    reg,
				az:      az,
			}
		},
	}
}

// DBusV2 DBus-объект интерфейса Repo.
type DBusV2 struct {
	ctx     context.Context
	actions *Actions
	jobs    *jobs.Registry
	az      authz.Authorizer
}

// List возвращает подключённые репозитории.
func (w *DBusV2) List(all bool) ([]wire.Dict, *dbus.Error) {
	resp, err := w.actions.List(w.ctx, all)
	if err != nil {
		return nil, wire.Error(err)
	}
	return wire.Reply(wire.StructDicts(resp.Repositories))
}

// Branches возвращает список веток ALT.
func (w *DBusV2) Branches() ([]string, *dbus.Error) {
	resp, err := w.actions.GetBranches(w.ctx)
	if err != nil {
		return nil, wire.Error(err)
	}
	return resp.Branches, nil
}

// TaskPackages запрашивает пакеты задачи сборочницы фоновой задачей:
// поход в сеть медленный. Чтение — без polkit, отмена чужим — через repo.manage.
func (w *DBusV2) TaskPackages(sender dbus.Sender, task string) (uint32, *dbus.Error) {
	return w.jobs.Start("repo", "TaskPackages", string(sender), protocol.ActionRepoManage,
		func(ctx context.Context) (wire.Dict, error) {
			resp, err := w.actions.GetTaskPackages(ctx, task)
			if err != nil {
				return nil, err
			}
			return wire.StructDict(resp)
		}), nil
}

// TestTask симулирует установку задачи сборочницы фоновой задачей:
// временно подключает репозиторий задачи, обновляет индексы и считает изменения.
func (w *DBusV2) TestTask(sender dbus.Sender, task string) (uint32, *dbus.Error) {
	if err := w.az.Authorize(sender, protocol.ActionRepoManage); err != nil {
		return 0, wire.Error(err)
	}
	return w.jobs.StartNoCancel("repo", "TestTask", string(sender),
		func(ctx context.Context) (wire.Dict, error) {
			resp, err := w.actions.TestTask(ctx, task)
			if err != nil {
				return nil, err
			}
			return wire.StructDict(resp)
		}), nil
}

// Add подключает источники.
func (w *DBusV2) Add(sender dbus.Sender, sources []string, date string) (wire.Dict, *dbus.Error) {
	return wire.Reply(authz.Authorized(w.az, sender, protocol.ActionRepoManage, func() (wire.Dict, error) {
		resp, err := w.actions.Add(w.ctx, sources, date)
		if err != nil {
			return nil, err
		}
		return wire.StructDict(resp)
	}))
}

// Remove отключает источники.
func (w *DBusV2) Remove(sender dbus.Sender, sources []string, date string) (wire.Dict, *dbus.Error) {
	return wire.Reply(authz.Authorized(w.az, sender, protocol.ActionRepoManage, func() (wire.Dict, error) {
		resp, err := w.actions.Remove(w.ctx, sources, date)
		if err != nil {
			return nil, err
		}
		return wire.StructDict(resp)
	}))
}

// SetBranch переключает ветку репозитория.
func (w *DBusV2) SetBranch(sender dbus.Sender, branch string, date string) (wire.Dict, *dbus.Error) {
	return wire.Reply(authz.Authorized(w.az, sender, protocol.ActionRepoManage, func() (wire.Dict, error) {
		resp, err := w.actions.Set(w.ctx, branch, date)
		if err != nil {
			return nil, err
		}
		return wire.StructDict(resp)
	}))
}

// Clean удаляет все подключённые источники.
func (w *DBusV2) Clean(sender dbus.Sender) (wire.Dict, *dbus.Error) {
	return wire.Reply(authz.Authorized(w.az, sender, protocol.ActionRepoManage, func() (wire.Dict, error) {
		resp, err := w.actions.Clean(w.ctx)
		if err != nil {
			return nil, err
		}
		return wire.StructDict(resp)
	}))
}

// CheckAdd симулирует подключение источников.
func (w *DBusV2) CheckAdd(sender dbus.Sender, sources []string, date string) (wire.Dict, *dbus.Error) {
	return wire.Reply(authz.Authorized(w.az, sender, protocol.ActionRepoManage, func() (wire.Dict, error) {
		resp, err := w.actions.CheckAdd(w.ctx, sources, date)
		if err != nil {
			return nil, err
		}
		return wire.StructDict(resp)
	}))
}

// CheckRemove симулирует отключение источников.
func (w *DBusV2) CheckRemove(sender dbus.Sender, sources []string, date string) (wire.Dict, *dbus.Error) {
	return wire.Reply(authz.Authorized(w.az, sender, protocol.ActionRepoManage, func() (wire.Dict, error) {
		resp, err := w.actions.CheckRemove(w.ctx, sources, date)
		if err != nil {
			return nil, err
		}
		return wire.StructDict(resp)
	}))
}

// CheckSetBranch симулирует переключение ветки.
func (w *DBusV2) CheckSetBranch(sender dbus.Sender, branch string, date string) (wire.Dict, *dbus.Error) {
	return wire.Reply(authz.Authorized(w.az, sender, protocol.ActionRepoManage, func() (wire.Dict, error) {
		resp, err := w.actions.CheckSet(w.ctx, branch, date)
		if err != nil {
			return nil, err
		}
		return wire.StructDict(resp)
	}))
}

// CheckClean симулирует удаление всех источников.
func (w *DBusV2) CheckClean(sender dbus.Sender) (wire.Dict, *dbus.Error) {
	return wire.Reply(authz.Authorized(w.az, sender, protocol.ActionRepoManage, func() (wire.Dict, error) {
		resp, err := w.actions.CheckClean(w.ctx)
		if err != nil {
			return nil, err
		}
		return wire.StructDict(resp)
	}))
}
