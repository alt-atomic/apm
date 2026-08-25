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
	"context"

	"altlinux.space/alt-atomic/apm/internal/common/app"
	"altlinux.space/alt-atomic/apm/internal/common/dbusv2"
	"altlinux.space/alt-atomic/apm/internal/common/dbusv2/authz"
	"altlinux.space/alt-atomic/apm/internal/common/dbusv2/jobs"
	"altlinux.space/alt-atomic/apm/internal/common/dbusv2/protocol"
	"altlinux.space/alt-atomic/apm/internal/common/dbusv2/wire"
	"altlinux.space/alt-atomic/apm/internal/common/polkit"
	"altlinux.space/alt-atomic/apm/internal/common/reply"

	"github.com/godbus/dbus/v5"
)

// DBusV2Module модуль интерфейса org.altlinux.APM2.Kernel.
func DBusV2Module(appConfig *app.Config, reporter *reply.Reporter) dbusv2.Module {
	return dbusv2.Module{
		Iface:         protocol.KernelIface,
		Introspection: kernelIntrospectionV2,
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

// DBusV2 DBus-объект интерфейса Kernel.
type DBusV2 struct {
	ctx     context.Context
	actions *Actions
	jobs    *jobs.Registry
	az      authz.Authorizer
}

// startJob авторизует отправителя и регистрирует фоновую задачу.
// Все мутирующие операции ядра — rpm-транзакции, отмена запрещена.
func (w *DBusV2) startJob(msg dbus.Message, kind string, fn func(ctx context.Context) (wire.Dict, error)) (uint32, *dbus.Error) {
	if err := w.az.Authorize(msg, protocol.ActionKernelManage); err != nil {
		return 0, wire.Error(err)
	}
	return w.jobs.StartNoCancel("kernel", kind, polkit.Sender(msg), fn), nil
}

// ListKernels возвращает список ядер флейвора.
func (w *DBusV2) ListKernels(flavour string, installedOnly bool) ([]wire.Dict, *dbus.Error) {
	resp, err := w.actions.ListKernels(w.ctx, flavour, installedOnly)
	if err != nil {
		return nil, wire.Error(err)
	}
	return wire.Reply(wire.StructDicts(resp.Kernels))
}

// Current возвращает текущее загруженное ядро.
func (w *DBusV2) Current() (wire.Dict, *dbus.Error) {
	resp, err := w.actions.GetCurrentKernel(w.ctx)
	if err != nil {
		return nil, wire.Error(err)
	}
	return wire.Reply(wire.StructDict(resp.Kernel))
}

// Install ставит ядро фоновой задачей.
func (w *DBusV2) Install(msg dbus.Message, flavour string, modules []string, options wire.Dict) (uint32, *dbus.Error) {
	var headers bool
	if err := wire.ParseOptions(options, map[string]any{
		"headers": &headers,
	}); err != nil {
		return 0, wire.Error(err)
	}
	return w.startJob(msg, "Install", func(ctx context.Context) (wire.Dict, error) {
		resp, err := w.actions.InstallKernel(ctx, flavour, modules, headers, false)
		if err != nil {
			return nil, err
		}
		return wire.StructDict(resp)
	})
}

// Update обновляет ядро фоновой задачей.
func (w *DBusV2) Update(msg dbus.Message, flavour string, modules []string, options wire.Dict) (uint32, *dbus.Error) {
	var headers bool
	if err := wire.ParseOptions(options, map[string]any{
		"headers": &headers,
	}); err != nil {
		return 0, wire.Error(err)
	}
	return w.startJob(msg, "Update", func(ctx context.Context) (wire.Dict, error) {
		resp, err := w.actions.UpdateKernel(ctx, flavour, modules, headers, false)
		if err != nil {
			return nil, err
		}
		return wire.StructDict(resp)
	})
}

// CheckInstall симулирует установку ядра.
func (w *DBusV2) CheckInstall(msg dbus.Message, flavour string, modules []string, options wire.Dict) (wire.Dict, *dbus.Error) {
	var headers bool
	if err := wire.ParseOptions(options, map[string]any{
		"headers": &headers,
	}); err != nil {
		return nil, wire.Error(err)
	}
	return wire.Reply(authz.Authorized(w.az, msg, protocol.ActionKernelManage, func() (wire.Dict, error) {
		resp, err := w.actions.InstallKernel(w.ctx, flavour, modules, headers, true)
		if err != nil {
			return nil, err
		}
		return wire.StructDict(resp)
	}))
}

// CheckUpdate симулирует обновление ядра.
func (w *DBusV2) CheckUpdate(msg dbus.Message, flavour string, modules []string, options wire.Dict) (wire.Dict, *dbus.Error) {
	var headers bool
	if err := wire.ParseOptions(options, map[string]any{
		"headers": &headers,
	}); err != nil {
		return nil, wire.Error(err)
	}
	return wire.Reply(authz.Authorized(w.az, msg, protocol.ActionKernelManage, func() (wire.Dict, error) {
		resp, err := w.actions.UpdateKernel(w.ctx, flavour, modules, headers, true)
		if err != nil {
			return nil, err
		}
		return wire.StructDict(resp)
	}))
}

// CleanOld удаляет старые ядра фоновой задачей.
func (w *DBusV2) CleanOld(msg dbus.Message, options wire.Dict) (uint32, *dbus.Error) {
	var noBackup bool
	if err := wire.ParseOptions(options, map[string]any{
		"no_backup": &noBackup,
	}); err != nil {
		return 0, wire.Error(err)
	}
	return w.startJob(msg, "CleanOld", func(ctx context.Context) (wire.Dict, error) {
		resp, err := w.actions.CleanOldKernels(ctx, noBackup, false)
		if err != nil {
			return nil, err
		}
		return wire.StructDict(resp)
	})
}

// CheckCleanOld симулирует удаление старых ядер.
func (w *DBusV2) CheckCleanOld(msg dbus.Message, options wire.Dict) (wire.Dict, *dbus.Error) {
	var noBackup bool
	if err := wire.ParseOptions(options, map[string]any{
		"no_backup": &noBackup,
	}); err != nil {
		return nil, wire.Error(err)
	}
	return wire.Reply(authz.Authorized(w.az, msg, protocol.ActionKernelManage, func() (wire.Dict, error) {
		resp, err := w.actions.CleanOldKernels(w.ctx, noBackup, true)
		if err != nil {
			return nil, err
		}
		return wire.StructDict(resp)
	}))
}

// ListModules возвращает ядро и его доступные модули.
func (w *DBusV2) ListModules(flavour string) (wire.Dict, *dbus.Error) {
	resp, err := w.actions.ListKernelModules(w.ctx, flavour)
	if err != nil {
		return nil, wire.Error(err)
	}
	return wire.Reply(wire.StructDict(resp))
}

// InstallModules ставит модули ядра фоновой задачей.
func (w *DBusV2) InstallModules(msg dbus.Message, flavour string, modules []string) (uint32, *dbus.Error) {
	return w.startJob(msg, "InstallModules", func(ctx context.Context) (wire.Dict, error) {
		resp, err := w.actions.InstallKernelModules(ctx, flavour, modules, false)
		if err != nil {
			return nil, err
		}
		return wire.StructDict(resp)
	})
}

// CheckInstallModules симулирует установку модулей ядра.
func (w *DBusV2) CheckInstallModules(msg dbus.Message, flavour string, modules []string) (wire.Dict, *dbus.Error) {
	return wire.Reply(authz.Authorized(w.az, msg, protocol.ActionKernelManage, func() (wire.Dict, error) {
		resp, err := w.actions.InstallKernelModules(w.ctx, flavour, modules, true)
		if err != nil {
			return nil, err
		}
		return wire.StructDict(resp)
	}))
}

// RemoveModules удаляет модули ядра фоновой задачей.
func (w *DBusV2) RemoveModules(msg dbus.Message, flavour string, modules []string) (uint32, *dbus.Error) {
	return w.startJob(msg, "RemoveModules", func(ctx context.Context) (wire.Dict, error) {
		resp, err := w.actions.RemoveKernelModules(ctx, flavour, modules, false)
		if err != nil {
			return nil, err
		}
		return wire.StructDict(resp)
	})
}

// CheckRemoveModules симулирует удаление модулей ядра.
func (w *DBusV2) CheckRemoveModules(msg dbus.Message, flavour string, modules []string) (wire.Dict, *dbus.Error) {
	return wire.Reply(authz.Authorized(w.az, msg, protocol.ActionKernelManage, func() (wire.Dict, error) {
		resp, err := w.actions.RemoveKernelModules(w.ctx, flavour, modules, true)
		if err != nil {
			return nil, err
		}
		return wire.StructDict(resp)
	}))
}
