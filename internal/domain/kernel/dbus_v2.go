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

// guard проверяет единое право на управление ядром.
func (w *DBusV2) guard(msg dbus.Message) *dbus.Error {
	return wire.Error(w.az.Authorize(msg, protocol.ActionKernelManage))
}

// startJob авторизует отправителя и регистрирует фоновую задачу.
// Все мутирующие операции ядра — rpm-транзакции, отмена запрещена.
func (w *DBusV2) startJob(msg dbus.Message, kind string, fn func(ctx context.Context) (string, error)) (uint32, *dbus.Error) {
	if dbusErr := w.guard(msg); dbusErr != nil {
		return 0, dbusErr
	}
	return w.jobs.StartNoCancel("kernel", kind, polkit.Sender(msg), fn), nil
}

// ListKernels возвращает список ядер флейвора.
func (w *DBusV2) ListKernels(flavour string, installedOnly bool) (string, *dbus.Error) {
	resp, err := w.actions.ListKernels(w.ctx, flavour, installedOnly)
	return wire.JSONReply(resp, err)
}

// Current возвращает текущее загруженное ядро.
func (w *DBusV2) Current() (string, *dbus.Error) {
	resp, err := w.actions.GetCurrentKernel(w.ctx)
	return wire.JSONReply(resp, err)
}

// Install ставит ядро фоновой задачей.
func (w *DBusV2) Install(msg dbus.Message, flavour string, modules []string, optionsJSON string) (uint32, *dbus.Error) {
	var options struct {
		Headers bool `json:"headers"`
	}
	if err := wire.DecodeOptions(optionsJSON, &options); err != nil {
		return 0, wire.Error(err)
	}
	return w.startJob(msg, "Install", wire.JSONTask(func(ctx context.Context) (*InstallUpdateKernelResponse, error) {
		return w.actions.InstallKernel(ctx, flavour, modules, options.Headers, false)
	}))
}

// Update обновляет ядро фоновой задачей.
func (w *DBusV2) Update(msg dbus.Message, flavour string, modules []string, optionsJSON string) (uint32, *dbus.Error) {
	var options struct {
		Headers bool `json:"headers"`
	}
	if err := wire.DecodeOptions(optionsJSON, &options); err != nil {
		return 0, wire.Error(err)
	}
	return w.startJob(msg, "Update", wire.JSONTask(func(ctx context.Context) (*InstallUpdateKernelResponse, error) {
		return w.actions.UpdateKernel(ctx, flavour, modules, options.Headers, false)
	}))
}

// CheckInstall симулирует установку ядра.
func (w *DBusV2) CheckInstall(msg dbus.Message, flavour string, modules []string, optionsJSON string) (string, *dbus.Error) {
	var options struct {
		Headers bool `json:"headers"`
	}
	if err := wire.DecodeOptions(optionsJSON, &options); err != nil {
		return "", wire.Error(err)
	}
	if dbusErr := w.guard(msg); dbusErr != nil {
		return "", dbusErr
	}
	resp, err := w.actions.InstallKernel(w.ctx, flavour, modules, options.Headers, true)
	return wire.JSONReply(resp, err)
}

// CheckUpdate симулирует обновление ядра.
func (w *DBusV2) CheckUpdate(msg dbus.Message, flavour string, modules []string, optionsJSON string) (string, *dbus.Error) {
	var options struct {
		Headers bool `json:"headers"`
	}
	if err := wire.DecodeOptions(optionsJSON, &options); err != nil {
		return "", wire.Error(err)
	}
	if dbusErr := w.guard(msg); dbusErr != nil {
		return "", dbusErr
	}
	resp, err := w.actions.UpdateKernel(w.ctx, flavour, modules, options.Headers, true)
	return wire.JSONReply(resp, err)
}

// CleanOld удаляет старые ядра фоновой задачей.
func (w *DBusV2) CleanOld(msg dbus.Message, optionsJSON string) (uint32, *dbus.Error) {
	var options struct {
		NoBackup bool `json:"noBackup"`
	}
	if err := wire.DecodeOptions(optionsJSON, &options); err != nil {
		return 0, wire.Error(err)
	}
	return w.startJob(msg, "CleanOld", wire.JSONTask(func(ctx context.Context) (*CleanOldKernelsResponse, error) {
		return w.actions.CleanOldKernels(ctx, options.NoBackup, false)
	}))
}

// CheckCleanOld симулирует удаление старых ядер.
func (w *DBusV2) CheckCleanOld(msg dbus.Message, optionsJSON string) (string, *dbus.Error) {
	var options struct {
		NoBackup bool `json:"noBackup"`
	}
	if err := wire.DecodeOptions(optionsJSON, &options); err != nil {
		return "", wire.Error(err)
	}
	if dbusErr := w.guard(msg); dbusErr != nil {
		return "", dbusErr
	}
	resp, err := w.actions.CleanOldKernels(w.ctx, options.NoBackup, true)
	return wire.JSONReply(resp, err)
}

// ListModules возвращает ядро и его доступные модули.
func (w *DBusV2) ListModules(flavour string) (string, *dbus.Error) {
	resp, err := w.actions.ListKernelModules(w.ctx, flavour)
	return wire.JSONReply(resp, err)
}

// InstallModules ставит модули ядра фоновой задачей.
func (w *DBusV2) InstallModules(msg dbus.Message, flavour string, modules []string) (uint32, *dbus.Error) {
	return w.startJob(msg, "InstallModules", wire.JSONTask(func(ctx context.Context) (*InstallKernelModulesResponse, error) {
		return w.actions.InstallKernelModules(ctx, flavour, modules, false)
	}))
}

// CheckInstallModules симулирует установку модулей ядра.
func (w *DBusV2) CheckInstallModules(msg dbus.Message, flavour string, modules []string) (string, *dbus.Error) {
	if dbusErr := w.guard(msg); dbusErr != nil {
		return "", dbusErr
	}
	resp, err := w.actions.InstallKernelModules(w.ctx, flavour, modules, true)
	return wire.JSONReply(resp, err)
}

// RemoveModules удаляет модули ядра фоновой задачей.
func (w *DBusV2) RemoveModules(msg dbus.Message, flavour string, modules []string) (uint32, *dbus.Error) {
	return w.startJob(msg, "RemoveModules", wire.JSONTask(func(ctx context.Context) (*RemoveKernelModulesResponse, error) {
		return w.actions.RemoveKernelModules(ctx, flavour, modules, false)
	}))
}

// CheckRemoveModules симулирует удаление модулей ядра.
func (w *DBusV2) CheckRemoveModules(msg dbus.Message, flavour string, modules []string) (string, *dbus.Error) {
	if dbusErr := w.guard(msg); dbusErr != nil {
		return "", dbusErr
	}
	resp, err := w.actions.RemoveKernelModules(w.ctx, flavour, modules, true)
	return wire.JSONReply(resp, err)
}
