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
	"maps"

	_package "altlinux.space/alt-atomic/apm/internal/common/apt/package"
	"altlinux.space/alt-atomic/apm/internal/common/dbusv2"
	"altlinux.space/alt-atomic/apm/internal/common/dbusv2/authz"
	"altlinux.space/alt-atomic/apm/internal/common/dbusv2/jobs"
	"altlinux.space/alt-atomic/apm/internal/common/dbusv2/protocol"
	"altlinux.space/alt-atomic/apm/internal/common/dbusv2/wire"
	"altlinux.space/alt-atomic/apm/internal/common/polkit"

	"github.com/godbus/dbus/v5"
)

// V2Modules интерфейсы v2 домена system: Packages, Image, Applications.
func (s *DBusServices) V2Modules() []dbusv2.Module {
	modules := []dbusv2.Module{
		packagesModuleV2(s.actions),
		applicationsModuleV2(s.appstreamActions),
	}
	if s.appConfig.ConfigManager.GetConfig().IsAtomic {
		modules = append(modules, imageModuleV2(s.actions))
	}
	return modules
}

// packagesModuleV2 модуль интерфейса org.altlinux.APM2.Packages.
func packagesModuleV2(actions *Actions) dbusv2.Module {
	return dbusv2.Module{
		Iface:         protocol.PackagesIface,
		Introspection: packagesIntrospectionV2,
		Build: func(ctx context.Context, reg *jobs.Registry, az authz.Authorizer) any {
			return &PackagesV2{
				ctx:     ctx,
				actions: actions,
				jobs:    reg,
				az:      az,
			}
		},
	}
}

// PackagesV2 DBus-объект интерфейса Packages.
type PackagesV2 struct {
	ctx     context.Context
	actions *Actions
	jobs    *jobs.Registry
	az      authz.Authorizer
}

type aptOperationOptions struct {
	AptConfig map[string]string `json:"aptConfig"`
}

// guard проверяет единое право на управление пакетами.
func (w *PackagesV2) guard(msg dbus.Message) *dbus.Error {
	return wire.Error(w.az.Authorize(msg, protocol.ActionPackagesManage))
}

// startJob регистрирует фоновую задачу домена.
// Отмена запрещена всем: вызов apt не прерывается на полпути.
func (w *PackagesV2) startJob(msg dbus.Message, kind string, aptConfig map[string]string, fn func(ctx context.Context) (string, error)) string {
	configSnapshot := maps.Clone(aptConfig)
	return w.jobs.StartNoCancel(jobs.ResourceHost, "packages", kind, polkit.Sender(msg), func(ctx context.Context) (string, error) {
		return fn(_package.WithAptConfigOverrides(ctx, configSnapshot))
	})
}

// Install ставит пакеты фоновой задачей.
func (w *PackagesV2) Install(msg dbus.Message, packages []string, optionsJSON string) (string, *dbus.Error) {
	if dbusErr := w.guard(msg); dbusErr != nil {
		return "", dbusErr
	}
	var options struct {
		aptOperationOptions
		DownloadOnly bool `json:"downloadOnly"`
		NoUpdate     bool `json:"noUpdate"`
	}
	if err := wire.DecodeOptions(optionsJSON, &options); err != nil {
		return "", wire.Error(err)
	}
	return w.startJob(msg, "Install", options.AptConfig, wire.JSONTask(func(ctx context.Context) (*InstallRemoveResponse, error) {
		return w.actions.Install(ctx, packages, true, options.DownloadOnly, options.NoUpdate)
	})), nil
}

// Remove удаляет пакеты фоновой задачей.
func (w *PackagesV2) Remove(msg dbus.Message, packages []string, optionsJSON string) (string, *dbus.Error) {
	if dbusErr := w.guard(msg); dbusErr != nil {
		return "", dbusErr
	}
	var options struct {
		aptOperationOptions
		Purge   bool `json:"purge"`
		Depends bool `json:"depends"`
	}
	if err := wire.DecodeOptions(optionsJSON, &options); err != nil {
		return "", wire.Error(err)
	}
	return w.startJob(msg, "Remove", options.AptConfig, wire.JSONTask(func(ctx context.Context) (*InstallRemoveResponse, error) {
		return w.actions.Remove(ctx, packages, options.Purge, options.Depends, true)
	})), nil
}

// Reinstall переустанавливает пакеты фоновой задачей.
func (w *PackagesV2) Reinstall(msg dbus.Message, packages []string, optionsJSON string) (string, *dbus.Error) {
	if dbusErr := w.guard(msg); dbusErr != nil {
		return "", dbusErr
	}
	var options aptOperationOptions
	if err := wire.DecodeOptions(optionsJSON, &options); err != nil {
		return "", wire.Error(err)
	}
	return w.startJob(msg, "Reinstall", options.AptConfig, wire.JSONTask(func(ctx context.Context) (*InstallRemoveResponse, error) {
		return w.actions.Reinstall(ctx, packages, true)
	})), nil
}

// Upgrade обновляет систему фоновой задачей.
func (w *PackagesV2) Upgrade(msg dbus.Message, optionsJSON string) (string, *dbus.Error) {
	if dbusErr := w.guard(msg); dbusErr != nil {
		return "", dbusErr
	}
	var options struct {
		aptOperationOptions
		DownloadOnly bool `json:"downloadOnly"`
	}
	if err := wire.DecodeOptions(optionsJSON, &options); err != nil {
		return "", wire.Error(err)
	}
	return w.startJob(msg, "Upgrade", options.AptConfig, wire.JSONTask(func(ctx context.Context) (*UpgradeResponse, error) {
		return w.actions.Upgrade(ctx, options.DownloadOnly)
	})), nil
}

// Update обновляет список пакетов фоновой задачей.
func (w *PackagesV2) Update(msg dbus.Message, optionsJSON string) (string, *dbus.Error) {
	if dbusErr := w.guard(msg); dbusErr != nil {
		return "", dbusErr
	}
	var options struct {
		aptOperationOptions
		OnlyDB bool `json:"onlyDB"`
	}
	if err := wire.DecodeOptions(optionsJSON, &options); err != nil {
		return "", wire.Error(err)
	}
	return w.startJob(msg, "Update", options.AptConfig, wire.JSONTask(func(ctx context.Context) (*UpdateResponse, error) {
		return w.actions.Update(ctx, false, options.OnlyDB)
	})), nil
}

// CheckInstall симулирует установку фоновой задачей.
func (w *PackagesV2) CheckInstall(msg dbus.Message, packages []string, optionsJSON string) (string, *dbus.Error) {
	if dbusErr := w.guard(msg); dbusErr != nil {
		return "", dbusErr
	}
	var options aptOperationOptions
	if err := wire.DecodeOptions(optionsJSON, &options); err != nil {
		return "", wire.Error(err)
	}
	return w.startJob(msg, "CheckInstall", options.AptConfig, wire.JSONTask(func(ctx context.Context) (*CheckResponse, error) {
		return w.actions.CheckInstall(ctx, packages)
	})), nil
}

// CheckRemove симулирует удаление фоновой задачей.
func (w *PackagesV2) CheckRemove(msg dbus.Message, packages []string, optionsJSON string) (string, *dbus.Error) {
	if dbusErr := w.guard(msg); dbusErr != nil {
		return "", dbusErr
	}
	var options struct {
		aptOperationOptions
		Purge   bool `json:"purge"`
		Depends bool `json:"depends"`
	}
	if err := wire.DecodeOptions(optionsJSON, &options); err != nil {
		return "", wire.Error(err)
	}
	return w.startJob(msg, "CheckRemove", options.AptConfig, wire.JSONTask(func(ctx context.Context) (*CheckResponse, error) {
		return w.actions.CheckRemove(ctx, packages, options.Purge, options.Depends)
	})), nil
}

// CheckUpgrade симулирует обновление системы фоновой задачей.
func (w *PackagesV2) CheckUpgrade(msg dbus.Message, optionsJSON string) (string, *dbus.Error) {
	if dbusErr := w.guard(msg); dbusErr != nil {
		return "", dbusErr
	}
	var options aptOperationOptions
	if err := wire.DecodeOptions(optionsJSON, &options); err != nil {
		return "", wire.Error(err)
	}
	return w.startJob(msg, "CheckUpgrade", options.AptConfig, wire.JSONTask(func(ctx context.Context) (*CheckResponse, error) {
		return w.actions.CheckUpgrade(ctx)
	})), nil
}

// List возвращает JSON существующего ListResponse.
func (w *PackagesV2) List(requestJSON string) (string, *dbus.Error) {
	var request wire.ListRequest
	if err := wire.DecodeJSON(requestJSON, &request); err != nil {
		return "", wire.Error(err)
	}

	page, err := request.Validate(_package.SystemFilterConfig)
	if err != nil {
		return "", wire.Error(err)
	}

	resp, err := w.actions.List(w.ctx, ListParams{
		Sort:    request.Sort,
		Order:   request.Order,
		Limit:   page.Limit,
		Offset:  page.Offset,
		Filters: page.Filters,
		Full:    true,
	})
	return wire.JSONReply(resp, err)
}

// Info возвращает информацию о пакете.
func (w *PackagesV2) Info(name string) (string, *dbus.Error) {
	resp, err := w.actions.Info(w.ctx, name)
	return wire.JSONReply(resp, err)
}

// MultiInfo возвращает информацию о нескольких пакетах.
func (w *PackagesV2) MultiInfo(names []string) (string, *dbus.Error) {
	resp, err := w.actions.MultiInfo(w.ctx, names)
	return wire.JSONReply(resp, err)
}

// Search ищет пакеты по подстроке имени; строки короткие — детали через Info.
func (w *PackagesV2) Search(text string, installed bool) (string, *dbus.Error) {
	resp, err := w.actions.Search(w.ctx, text, installed)
	return wire.JSONReply(resp, err)
}

// Sections возвращает список секций пакетов.
func (w *PackagesV2) Sections() ([]string, *dbus.Error) {
	resp, err := w.actions.Sections(w.ctx)
	if err != nil {
		return nil, wire.Error(err)
	}
	return resp.Sections, nil
}

// FilterFields возвращает описание полей фильтрации.
func (w *PackagesV2) FilterFields() (string, *dbus.Error) {
	resp, err := w.actions.GetFilterFields(w.ctx)
	return wire.JSONReply(resp, err)
}
