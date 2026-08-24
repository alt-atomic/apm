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
	"errors"

	"altlinux.space/alt-atomic/apm/internal/common/apmerr"
	"altlinux.space/alt-atomic/apm/internal/common/app"
	_package "altlinux.space/alt-atomic/apm/internal/common/apt/package"
	"altlinux.space/alt-atomic/apm/internal/common/dbusv2"
	"altlinux.space/alt-atomic/apm/internal/common/dbusv2/authz"
	"altlinux.space/alt-atomic/apm/internal/common/dbusv2/jobs"
	"altlinux.space/alt-atomic/apm/internal/common/dbusv2/protocol"
	"altlinux.space/alt-atomic/apm/internal/common/dbusv2/wire"
	"altlinux.space/alt-atomic/apm/internal/common/reply"

	"github.com/godbus/dbus/v5"
)

// DBusV2Modules интерфейсы v2 домена system: Packages, Image, Applications.
func DBusV2Modules(appConfig *app.Config, reporter *reply.Reporter) []dbusv2.Module {
	return []dbusv2.Module{
		packagesModuleV2(appConfig, reporter),
		imageModuleV2(appConfig, reporter),
		applicationsModuleV2(appConfig, reporter),
	}
}

// packagesModuleV2 модуль интерфейса org.altlinux.APM2.Packages.
func packagesModuleV2(appConfig *app.Config, reporter *reply.Reporter) dbusv2.Module {
	return dbusv2.Module{
		Iface:         protocol.PackagesIface,
		Introspection: packagesIntrospectionV2,
		Build: func(ctx context.Context, reg *jobs.Registry, az authz.Authorizer) any {
			return &PackagesV2{
				ctx:     ctx,
				actions: NewActions(appConfig, reporter),
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

// filterRuleV2 одно условие фильтра в aa(sss).
type filterRuleV2 = wire.FilterRule

// startTransaction регистрирует неотменяемую rpm/apt-транзакцию.
func (w *PackagesV2) startTransaction(sender dbus.Sender, kind string, fn func(ctx context.Context) (wire.Dict, error)) (uint32, *dbus.Error) {
	if err := w.az.Authorize(sender, protocol.ActionPackagesManage); err != nil {
		return 0, wire.Error(err)
	}
	return w.jobs.StartNoCancel("packages", kind, string(sender), fn), nil
}

// Install ставит пакеты фоновой задачей.
func (w *PackagesV2) Install(sender dbus.Sender, packages []string, options wire.Dict) (uint32, *dbus.Error) {
	var downloadOnly, noUpdate bool
	if err := wire.ParseOptions(options, map[string]any{
		"download_only": &downloadOnly,
		"no_update":     &noUpdate,
	}); err != nil {
		return 0, wire.Error(err)
	}
	return w.startTransaction(sender, "Install", func(ctx context.Context) (wire.Dict, error) {
		resp, err := w.actions.Install(ctx, packages, true, downloadOnly, noUpdate)
		if err != nil {
			return nil, err
		}
		return wire.StructDict(resp)
	})
}

// Remove удаляет пакеты фоновой задачей.
func (w *PackagesV2) Remove(sender dbus.Sender, packages []string, options wire.Dict) (uint32, *dbus.Error) {
	var purge, depends bool
	if err := wire.ParseOptions(options, map[string]any{
		"purge":   &purge,
		"depends": &depends,
	}); err != nil {
		return 0, wire.Error(err)
	}
	return w.startTransaction(sender, "Remove", func(ctx context.Context) (wire.Dict, error) {
		resp, err := w.actions.Remove(ctx, packages, purge, depends, true)
		if err != nil {
			return nil, err
		}
		return wire.StructDict(resp)
	})
}

// Reinstall переустанавливает пакеты фоновой задачей.
func (w *PackagesV2) Reinstall(sender dbus.Sender, packages []string) (uint32, *dbus.Error) {
	return w.startTransaction(sender, "Reinstall", func(ctx context.Context) (wire.Dict, error) {
		resp, err := w.actions.Reinstall(ctx, packages, true)
		if err != nil {
			return nil, err
		}
		return wire.StructDict(resp)
	})
}

// Upgrade обновляет систему фоновой задачей.
func (w *PackagesV2) Upgrade(sender dbus.Sender, options wire.Dict) (uint32, *dbus.Error) {
	var downloadOnly bool
	if err := wire.ParseOptions(options, map[string]any{
		"download_only": &downloadOnly,
	}); err != nil {
		return 0, wire.Error(err)
	}
	return w.startTransaction(sender, "Upgrade", func(ctx context.Context) (wire.Dict, error) {
		resp, err := w.actions.Upgrade(ctx, downloadOnly)
		if err != nil {
			return nil, err
		}
		return wire.StructDict(resp)
	})
}

// Update обновляет список пакетов фоновой задачей.
func (w *PackagesV2) Update(sender dbus.Sender, options wire.Dict) (uint32, *dbus.Error) {
	var onlyDB bool
	if err := wire.ParseOptions(options, map[string]any{
		"only_db": &onlyDB,
	}); err != nil {
		return 0, wire.Error(err)
	}
	return w.startTransaction(sender, "Update", func(ctx context.Context) (wire.Dict, error) {
		resp, err := w.actions.Update(ctx, false, onlyDB)
		if err != nil {
			return nil, err
		}
		return wire.StructDict(resp)
	})
}

// CheckInstall симулирует установку.
func (w *PackagesV2) CheckInstall(sender dbus.Sender, packages []string) (wire.Dict, *dbus.Error) {
	return wire.Reply(authz.Authorized(w.az, sender, protocol.ActionPackagesManage, func() (wire.Dict, error) {
		resp, err := w.actions.CheckInstall(w.ctx, packages)
		if err != nil {
			return nil, err
		}
		return wire.StructDict(resp)
	}))
}

// CheckRemove симулирует удаление.
func (w *PackagesV2) CheckRemove(sender dbus.Sender, packages []string, options wire.Dict) (wire.Dict, *dbus.Error) {
	var purge, depends bool
	if err := wire.ParseOptions(options, map[string]any{
		"purge":   &purge,
		"depends": &depends,
	}); err != nil {
		return nil, wire.Error(err)
	}
	return wire.Reply(authz.Authorized(w.az, sender, protocol.ActionPackagesManage, func() (wire.Dict, error) {
		resp, err := w.actions.CheckRemove(w.ctx, packages, purge, depends)
		if err != nil {
			return nil, err
		}
		return wire.StructDict(resp)
	}))
}

// CheckUpgrade симулирует обновление системы.
func (w *PackagesV2) CheckUpgrade(sender dbus.Sender) (wire.Dict, *dbus.Error) {
	return wire.Reply(authz.Authorized(w.az, sender, protocol.ActionPackagesManage, func() (wire.Dict, error) {
		resp, err := w.actions.CheckUpgrade(w.ctx)
		if err != nil {
			return nil, err
		}
		return wire.StructDict(resp)
	}))
}

// List возвращает страницу пакетов по запросу и фильтрам.
// Строки короткие; full=true добавляет файлы, зависимости и описания.
func (w *PackagesV2) List(query wire.Dict, filters [][]filterRuleV2) (uint32, []wire.Dict, *dbus.Error) {
	var sort, order string
	var limit, offset uint32
	var forceUpdate, full bool
	if err := wire.ParseOptions(query, map[string]any{
		"sort":         &sort,
		"order":        &order,
		"limit":        &limit,
		"offset":       &offset,
		"force_update": &forceUpdate,
		"full":         &full,
	}); err != nil {
		return 0, nil, wire.Error(err)
	}

	pageSize, err := wire.PageLimit(limit)
	if err != nil {
		return 0, nil, wire.Error(err)
	}

	groups, err := wire.FilterGroups(filters, _package.SystemFilterConfig)
	if err != nil {
		return 0, nil, wire.Error(err)
	}

	resp, err := w.actions.List(w.ctx, ListParams{
		Sort:        sort,
		Order:       order,
		Limit:       pageSize,
		Offset:      int(offset),
		Filters:     groups,
		ForceUpdate: forceUpdate,
		Full:        full,
	})
	if err != nil {
		return 0, nil, wire.Error(err)
	}
	packages, err := packageRowsV2(w.actions, resp.Packages, full)
	if err != nil {
		return 0, nil, wire.Error(err)
	}
	return uint32(max(resp.TotalCount, 0)), packages, nil
}

// packageRowsV2 сериализует пакеты: короткое или полное представление.
func packageRowsV2(actions *Actions, packages []_package.Package, full bool) ([]wire.Dict, error) {
	if full {
		return wire.StructDicts(packages)
	}
	short, ok := actions.FormatPackageOutput(packages, false).([]ShortPackageResponse)
	if !ok {
		return nil, apmerr.New(apmerr.ErrorTypeValidation, errors.New("unexpected short package format"))
	}
	return wire.StructDicts(short)
}

// Info возвращает информацию о пакете.
func (w *PackagesV2) Info(name string) (wire.Dict, *dbus.Error) {
	resp, err := w.actions.Info(w.ctx, name)
	if err != nil {
		return nil, wire.Error(err)
	}
	return wire.Reply(wire.StructDict(resp.PackageInfo))
}

// MultiInfo возвращает информацию о нескольких пакетах.
func (w *PackagesV2) MultiInfo(names []string) ([]wire.Dict, []string, *dbus.Error) {
	resp, err := w.actions.MultiInfo(w.ctx, names)
	if err != nil {
		return nil, nil, wire.Error(err)
	}
	notFound := resp.NotFound
	if notFound == nil {
		notFound = []string{}
	}
	packages, err := wire.StructDicts(resp.Packages)
	if err != nil {
		return nil, nil, wire.Error(err)
	}
	return packages, notFound, nil
}

// Search ищет пакеты по подстроке имени; строки короткие — детали через Info.
func (w *PackagesV2) Search(text string, installed bool) ([]wire.Dict, *dbus.Error) {
	resp, err := w.actions.Search(w.ctx, text, installed)
	if err != nil {
		return nil, wire.Error(err)
	}
	return wire.Reply(packageRowsV2(w.actions, resp.Packages, false))
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
func (w *PackagesV2) FilterFields() ([]wire.Dict, *dbus.Error) {
	resp, err := w.actions.GetFilterFields(w.ctx)
	if err != nil {
		return nil, wire.Error(err)
	}
	return wire.Reply(wire.StructDicts(resp))
}

// AptConfig возвращает переопределения конфигурации APT.
func (w *PackagesV2) AptConfig() (map[string]string, *dbus.Error) {
	resp, err := w.actions.GetAptConfigOverrides()
	if err != nil {
		return nil, wire.Error(err)
	}
	return resp.Options, nil
}

// SetAptConfig устанавливает переопределения конфигурации APT.
func (w *PackagesV2) SetAptConfig(sender dbus.Sender, options map[string]string) *dbus.Error {
	return wire.Error(authz.Guard(w.az, sender, protocol.ActionPackagesManage, func() error {
		_, err := w.actions.SetAptConfigOverrides(options)
		return err
	}))
}
