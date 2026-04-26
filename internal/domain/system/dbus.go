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
	"apm/internal/common/apmerr"
	"apm/internal/common/app"
	_package "apm/internal/common/apt/package"
	"apm/internal/common/build"
	"apm/internal/common/filter"
	"apm/internal/common/helper"
	"apm/internal/common/reply"
	"apm/internal/common/swcat"
	"apm/internal/domain/system/appstream"
	"context"
	"encoding/json"
	"fmt"

	"github.com/godbus/dbus/v5"
)

// DBusWrapper предоставляет обёртку для системных действий, предназначенную для экспорта через DBus.
type DBusWrapper struct {
	conn             *dbus.Conn
	actions          *Actions
	appstreamActions *appstream.Actions
	ctx              context.Context
}

// NewDBusWrapper создаёт новую обёртку над actions
func NewDBusWrapper(a *Actions, c *dbus.Conn, ctx context.Context) *DBusWrapper {
	return &DBusWrapper{
		actions:          a,
		appstreamActions: appstream.NewActions(a.appConfig),
		conn:             c,
		ctx:              ctx,
	}
}

// checkManagePermission проверяет права org.altlinux.APM.manage
func (w *DBusWrapper) checkManagePermission(sender dbus.Sender) *dbus.Error {
	if err := helper.PolkitCheck(w.conn, sender, "org.altlinux.APM.manage"); err != nil {
		return dbus.MakeFailedError(err)
	}
	return nil
}

// Install устанавливает пакеты.
func (w *DBusWrapper) Install(sender dbus.Sender, packages []string, downloadOnly bool, transaction string, background bool) (string, *dbus.Error) {
	if err := w.checkManagePermission(sender); err != nil {
		return "", err
	}

	if transaction == "" {
		transaction = helper.GenerateTransactionID()
	}

	if background {
		ctx := context.WithValue(w.ctx, helper.TransactionKey, transaction)
		go func() {
			resp, err := w.actions.Install(ctx, packages, true, downloadOnly)
			reply.SendTaskResult(ctx, reply.EventSystemInstall, resp, err)
		}()

		bgResp := BackgroundTaskResponse{
			Message:     app.T_("Task started in background"),
			Transaction: transaction,
		}
		data, jerr := json.Marshal(reply.OK(bgResp))
		if jerr != nil {
			return "", dbus.MakeFailedError(jerr)
		}
		return string(data), nil
	}

	// Синхронное выполнение
	ctx := context.WithValue(w.ctx, helper.TransactionKey, transaction)
	resp, err := w.actions.Install(ctx, packages, true, downloadOnly)
	if err != nil {
		return "", apmerr.DBusError(err)
	}
	data, jerr := json.Marshal(reply.OK(resp))
	if jerr != nil {
		return "", dbus.MakeFailedError(jerr)
	}
	return string(data), nil
}

// Remove удаляет пакеты.
func (w *DBusWrapper) Remove(sender dbus.Sender, packages []string, purge bool, depends bool, transaction string, background bool) (string, *dbus.Error) {
	if err := w.checkManagePermission(sender); err != nil {
		return "", err
	}

	if transaction == "" {
		transaction = helper.GenerateTransactionID()
	}

	if background {
		ctx := context.WithValue(w.ctx, helper.TransactionKey, transaction)
		go func() {
			resp, err := w.actions.Remove(ctx, packages, purge, depends, true)
			reply.SendTaskResult(ctx, reply.EventSystemRemove, resp, err)
		}()

		bgResp := BackgroundTaskResponse{
			Message:     app.T_("Task started in background"),
			Transaction: transaction,
		}
		data, jerr := json.Marshal(reply.OK(bgResp))
		if jerr != nil {
			return "", dbus.MakeFailedError(jerr)
		}
		return string(data), nil
	}

	// Синхронное выполнение
	ctx := context.WithValue(w.ctx, helper.TransactionKey, transaction)
	resp, err := w.actions.Remove(ctx, packages, purge, depends, true)
	if err != nil {
		return "", apmerr.DBusError(err)
	}
	data, jerr := json.Marshal(reply.OK(resp))
	if jerr != nil {
		return "", dbus.MakeFailedError(jerr)
	}
	return string(data), nil
}

// GetFilterFields возвращает список полей фильтрации для метода list, помогающий динамически строить фильтры в интерфейсе.
func (w *DBusWrapper) GetFilterFields(transaction string) (string, *dbus.Error) {
	ctx := context.WithValue(w.ctx, helper.TransactionKey, transaction)
	resp, err := w.actions.GetFilterFields(ctx)
	if err != nil {
		return "", apmerr.DBusError(err)
	}

	data, jerr := json.Marshal(reply.OK(resp))
	if jerr != nil {
		return "", dbus.MakeFailedError(jerr)
	}

	return string(data), nil
}

// Update обновляет систему.
func (w *DBusWrapper) Update(sender dbus.Sender, transaction string, background bool) (string, *dbus.Error) {
	if err := w.checkManagePermission(sender); err != nil {
		return "", err
	}

	if transaction == "" {
		transaction = helper.GenerateTransactionID()
	}

	if background {
		ctx := context.WithValue(w.ctx, helper.TransactionKey, transaction)
		go func() {
			resp, err := w.actions.Update(ctx, false, false)
			reply.SendTaskResult(ctx, reply.EventSystemUpdate, resp, err)
		}()

		bgResp := BackgroundTaskResponse{
			Message:     app.T_("Task started in background"),
			Transaction: transaction,
		}
		data, jerr := json.Marshal(reply.OK(bgResp))
		if jerr != nil {
			return "", dbus.MakeFailedError(jerr)
		}
		return string(data), nil
	}

	// Синхронное выполнение
	ctx := context.WithValue(w.ctx, helper.TransactionKey, transaction)
	resp, err := w.actions.Update(ctx, false, false)
	if err != nil {
		return "", apmerr.DBusError(err)
	}
	data, jerr := json.Marshal(reply.OK(resp))
	if jerr != nil {
		return "", dbus.MakeFailedError(jerr)
	}
	return string(data), nil
}

// List выполняет продвинутый поиск пакетов по фильтру. filtersJSON - это JSON-строка вида [{"field":"name","op":"like","value":"fire"}]
func (w *DBusWrapper) List(sort string, order string, limit int, offset int, filtersJSON string, forceUpdate bool, transaction string) (string, *dbus.Error) {
	ctx := context.WithValue(w.ctx, helper.TransactionKey, transaction)
	if limit <= 0 {
		limit = 10
	}

	var filters []filter.Filter
	if filtersJSON != "" {
		if err := json.Unmarshal([]byte(filtersJSON), &filters); err != nil {
			return "", apmerr.DBusError(apmerr.New(apmerr.ErrorTypeValidation, err))
		}
	}

	validated, err := _package.SystemFilterConfig.Validate(filters)
	if err != nil {
		return "", apmerr.DBusError(apmerr.New(apmerr.ErrorTypeValidation, err))
	}

	params := ListParams{
		Sort:        sort,
		Order:       order,
		Limit:       limit,
		Offset:      offset,
		Filters:     validated,
		ForceUpdate: forceUpdate,
	}

	resp, err := w.actions.List(ctx, params)
	if err != nil {
		return "", apmerr.DBusError(err)
	}
	data, jerr := json.Marshal(reply.OK(resp))
	if jerr != nil {
		return "", dbus.MakeFailedError(jerr)
	}
	return string(data), nil
}

// Info возвращает информацию о пакете.
func (w *DBusWrapper) Info(packageName string, transaction string) (string, *dbus.Error) {
	ctx := context.WithValue(w.ctx, helper.TransactionKey, transaction)
	resp, err := w.actions.Info(ctx, packageName)
	if err != nil {
		return "", apmerr.DBusError(err)
	}
	data, jerr := json.Marshal(reply.OK(resp))
	if jerr != nil {
		return "", dbus.MakeFailedError(jerr)
	}
	return string(data), nil
}

// MultiInfo возвращает информацию о нескольких пакетах.
func (w *DBusWrapper) MultiInfo(packages []string, transaction string) (string, *dbus.Error) {
	ctx := context.WithValue(w.ctx, helper.TransactionKey, transaction)
	resp, err := w.actions.MultiInfo(ctx, packages)
	if err != nil {
		return "", apmerr.DBusError(err)
	}
	data, jerr := json.Marshal(reply.OK(resp))
	if jerr != nil {
		return "", dbus.MakeFailedError(jerr)
	}
	return string(data), nil
}

// CheckUpgrade проверяет возможность обновления.
func (w *DBusWrapper) CheckUpgrade(sender dbus.Sender, transaction string, background bool) (string, *dbus.Error) {
	if err := w.checkManagePermission(sender); err != nil {
		return "", err
	}

	if transaction == "" {
		transaction = helper.GenerateTransactionID()
	}

	if background {
		ctx := context.WithValue(w.ctx, helper.TransactionKey, transaction)
		go func() {
			resp, err := w.actions.CheckUpgrade(ctx)
			reply.SendTaskResult(ctx, reply.EventSystemCheckUpgrade, resp, err)
		}()

		bgResp := BackgroundTaskResponse{
			Message:     app.T_("Task started in background"),
			Transaction: transaction,
		}
		data, jerr := json.Marshal(reply.OK(bgResp))
		if jerr != nil {
			return "", dbus.MakeFailedError(jerr)
		}
		return string(data), nil
	}

	// Синхронное выполнение
	ctx := context.WithValue(w.ctx, helper.TransactionKey, transaction)
	resp, err := w.actions.CheckUpgrade(ctx)
	if err != nil {
		return "", apmerr.DBusError(err)
	}
	data, jerr := json.Marshal(reply.OK(resp))
	if jerr != nil {
		return "", dbus.MakeFailedError(jerr)
	}
	return string(data), nil
}

// Upgrade обновляет систему (для не-атомарных систем).
func (w *DBusWrapper) Upgrade(sender dbus.Sender, downloadOnly bool, transaction string, background bool) (string, *dbus.Error) {
	if err := w.checkManagePermission(sender); err != nil {
		return "", err
	}

	if transaction == "" {
		transaction = helper.GenerateTransactionID()
	}

	if background {
		ctx := context.WithValue(w.ctx, helper.TransactionKey, transaction)
		go func() {
			resp, err := w.actions.Upgrade(ctx, downloadOnly)
			reply.SendTaskResult(ctx, reply.EventSystemUpgrade, resp, err)
		}()

		bgResp := BackgroundTaskResponse{
			Message:     app.T_("Task started in background"),
			Transaction: transaction,
		}
		data, jerr := json.Marshal(reply.OK(bgResp))
		if jerr != nil {
			return "", dbus.MakeFailedError(jerr)
		}
		return string(data), nil
	}

	// Синхронное выполнение
	ctx := context.WithValue(w.ctx, helper.TransactionKey, transaction)
	resp, err := w.actions.Upgrade(ctx, downloadOnly)
	if err != nil {
		return "", apmerr.DBusError(err)
	}
	data, jerr := json.Marshal(reply.OK(resp))
	if jerr != nil {
		return "", dbus.MakeFailedError(jerr)
	}
	return string(data), nil
}

// CheckInstall проверяет возможность установки пакетов.
func (w *DBusWrapper) CheckInstall(sender dbus.Sender, packages []string, transaction string, background bool) (string, *dbus.Error) {
	if err := w.checkManagePermission(sender); err != nil {
		return "", err
	}

	if transaction == "" {
		transaction = helper.GenerateTransactionID()
	}

	if background {
		ctx := context.WithValue(w.ctx, helper.TransactionKey, transaction)
		go func() {
			resp, err := w.actions.CheckInstall(ctx, packages)
			reply.SendTaskResult(ctx, reply.EventSystemCheckInstall, resp, err)
		}()

		bgResp := BackgroundTaskResponse{
			Message:     app.T_("Task started in background"),
			Transaction: transaction,
		}
		data, jerr := json.Marshal(reply.OK(bgResp))
		if jerr != nil {
			return "", dbus.MakeFailedError(jerr)
		}
		return string(data), nil
	}

	// Синхронное выполнение
	ctx := context.WithValue(w.ctx, helper.TransactionKey, transaction)
	resp, err := w.actions.CheckInstall(ctx, packages)
	if err != nil {
		return "", apmerr.DBusError(err)
	}
	data, jerr := json.Marshal(reply.OK(resp))
	if jerr != nil {
		return "", dbus.MakeFailedError(jerr)
	}
	return string(data), nil
}

// CheckRemove проверяет возможность удаления пакетов.
func (w *DBusWrapper) CheckRemove(sender dbus.Sender, packages []string, depends bool, transaction string, background bool) (string, *dbus.Error) {
	if err := w.checkManagePermission(sender); err != nil {
		return "", err
	}

	if transaction == "" {
		transaction = helper.GenerateTransactionID()
	}

	if background {
		ctx := context.WithValue(w.ctx, helper.TransactionKey, transaction)
		go func() {
			resp, err := w.actions.CheckRemove(ctx, packages, false, depends)
			reply.SendTaskResult(ctx, reply.EventSystemCheckRemove, resp, err)
		}()

		bgResp := BackgroundTaskResponse{
			Message:     app.T_("Task started in background"),
			Transaction: transaction,
		}
		data, jerr := json.Marshal(reply.OK(bgResp))
		if jerr != nil {
			return "", dbus.MakeFailedError(jerr)
		}
		return string(data), nil
	}

	// Синхронное выполнение
	ctx := context.WithValue(w.ctx, helper.TransactionKey, transaction)
	resp, err := w.actions.CheckRemove(ctx, packages, false, depends)
	if err != nil {
		return "", apmerr.DBusError(err)
	}
	data, jerr := json.Marshal(reply.OK(resp))
	if jerr != nil {
		return "", dbus.MakeFailedError(jerr)
	}
	return string(data), nil
}

// Search выполняет простой поиск пакетов.
func (w *DBusWrapper) Search(packageName string, transaction string, installed bool) (string, *dbus.Error) {
	ctx := context.WithValue(w.ctx, helper.TransactionKey, transaction)
	resp, err := w.actions.Search(ctx, packageName, installed)
	if err != nil {
		return "", apmerr.DBusError(err)
	}
	data, jerr := json.Marshal(reply.OK(resp))
	if jerr != nil {
		return "", dbus.MakeFailedError(jerr)
	}
	return string(data), nil
}

// ImageApply декларативно применяет настройки image.yml к образу хост-системы.
func (w *DBusWrapper) ImageApply(sender dbus.Sender, transaction string, background bool, pullImage bool, noCache bool, configPath string, workdir string) (string, *dbus.Error) {
	if err := w.checkManagePermission(sender); err != nil {
		return "", err
	}

	if transaction == "" {
		transaction = helper.GenerateTransactionID()
	}

	hostCache := !noCache

	if background {
		ctx := context.WithValue(w.ctx, helper.TransactionKey, transaction)
		go func() {
			resp, err := w.actions.ImageApply(ctx, pullImage, hostCache, configPath, workdir)
			reply.SendTaskResult(ctx, reply.EventSystemImageApply, resp, err)
		}()

		bgResp := BackgroundTaskResponse{
			Message:     app.T_("Task started in background"),
			Transaction: transaction,
		}
		data, jerr := json.Marshal(reply.OK(bgResp))
		if jerr != nil {
			return "", dbus.MakeFailedError(jerr)
		}
		return string(data), nil
	}

	// Синхронное выполнение
	ctx := context.WithValue(w.ctx, helper.TransactionKey, transaction)
	resp, err := w.actions.ImageApply(ctx, pullImage, hostCache, configPath, workdir)
	if err != nil {
		return "", apmerr.DBusError(err)
	}
	data, jerr := json.Marshal(reply.OK(resp))
	if jerr != nil {
		return "", dbus.MakeFailedError(jerr)
	}
	return string(data), nil
}

// ImageHistory возвращает историю обновлений.
func (w *DBusWrapper) ImageHistory(sender dbus.Sender, transaction string, imageName string, limit int, offset int) (string, *dbus.Error) {
	if err := w.checkManagePermission(sender); err != nil {
		return "", err
	}
	ctx := context.WithValue(w.ctx, helper.TransactionKey, transaction)
	resp, err := w.actions.ImageHistory(ctx, imageName, limit, offset)
	if err != nil {
		return "", apmerr.DBusError(err)
	}
	data, jerr := json.Marshal(reply.OK(resp))
	if jerr != nil {
		return "", dbus.MakeFailedError(jerr)
	}
	return string(data), nil
}

// ImageUpdate обновляет образ системы.
func (w *DBusWrapper) ImageUpdate(sender dbus.Sender, transaction string, background bool, noCache bool) (string, *dbus.Error) {
	if err := w.checkManagePermission(sender); err != nil {
		return "", err
	}

	if transaction == "" {
		transaction = helper.GenerateTransactionID()
	}

	hostCache := !noCache

	if background {
		ctx := context.WithValue(w.ctx, helper.TransactionKey, transaction)
		go func() {
			resp, err := w.actions.ImageUpdate(ctx, hostCache)
			reply.SendTaskResult(ctx, reply.EventSystemImageUpdate, resp, err)
		}()

		bgResp := BackgroundTaskResponse{
			Message:     app.T_("Task started in background"),
			Transaction: transaction,
		}
		data, jerr := json.Marshal(reply.OK(bgResp))
		if jerr != nil {
			return "", dbus.MakeFailedError(jerr)
		}
		return string(data), nil
	}

	// Синхронное выполнение
	ctx := context.WithValue(w.ctx, helper.TransactionKey, transaction)
	resp, err := w.actions.ImageUpdate(ctx, hostCache)
	if err != nil {
		return "", apmerr.DBusError(err)
	}
	data, jerr := json.Marshal(reply.OK(resp))
	if jerr != nil {
		return "", dbus.MakeFailedError(jerr)
	}
	return string(data), nil
}

// ImageStatus проверяет статус образа.
func (w *DBusWrapper) ImageStatus(sender dbus.Sender, transaction string) (string, *dbus.Error) {
	if err := w.checkManagePermission(sender); err != nil {
		return "", err
	}
	ctx := context.WithValue(w.ctx, helper.TransactionKey, transaction)
	resp, err := w.actions.ImageStatus(ctx)
	if err != nil {
		return "", apmerr.DBusError(err)
	}
	data, jerr := json.Marshal(reply.OK(resp))
	if jerr != nil {
		return "", dbus.MakeFailedError(jerr)
	}
	return string(data), nil
}

// ImageGetConfig возвращает текущую конфигурацию image.yml.
func (w *DBusWrapper) ImageGetConfig() (string, *dbus.Error) {
	resp, err := w.actions.ImageGetConfig(w.ctx)
	if err != nil {
		return "", apmerr.DBusError(err)
	}
	data, jerr := json.Marshal(reply.OK(resp))
	if jerr != nil {
		return "", dbus.MakeFailedError(jerr)
	}
	return string(data), nil
}

// ApplicationUpdate загружает и сохраняет данные приложений.
func (w *DBusWrapper) ApplicationUpdate(sender dbus.Sender, transaction string, background bool) (string, *dbus.Error) {
	if err := w.checkManagePermission(sender); err != nil {
		return "", err
	}

	if transaction == "" {
		transaction = helper.GenerateTransactionID()
	}

	if background {
		ctx := context.WithValue(w.ctx, helper.TransactionKey, transaction)
		go func() {
			resp, err := w.appstreamActions.Update(ctx)
			reply.SendTaskResult(ctx, reply.EventApplicationUpdate, resp, err)
		}()

		bgResp := BackgroundTaskResponse{
			Message:     app.T_("Task started in background"),
			Transaction: transaction,
		}
		data, jerr := json.Marshal(reply.OK(bgResp))
		if jerr != nil {
			return "", dbus.MakeFailedError(jerr)
		}
		return string(data), nil
	}

	ctx := context.WithValue(w.ctx, helper.TransactionKey, transaction)
	resp, err := w.appstreamActions.Update(ctx)
	if err != nil {
		return "", apmerr.DBusError(err)
	}
	data, jerr := json.Marshal(reply.OK(resp))
	if jerr != nil {
		return "", dbus.MakeFailedError(jerr)
	}
	return string(data), nil
}

// ApplicationInfo возвращает данные приложения для конкретного пакета.
func (w *DBusWrapper) ApplicationInfo(pkgname, transaction string) (string, *dbus.Error) {
	ctx := context.WithValue(w.ctx, helper.TransactionKey, transaction)
	resp, err := w.appstreamActions.Info(ctx, pkgname)
	if err != nil {
		return "", apmerr.DBusError(err)
	}
	data, jerr := json.Marshal(reply.OK(resp))
	if jerr != nil {
		return "", dbus.MakeFailedError(jerr)
	}
	return string(data), nil
}

// ApplicationList возвращает список приложений с фильтрами.
func (w *DBusWrapper) ApplicationList(sort, order string, limit, offset int, filtersJSON, transaction string) (string, *dbus.Error) {
	ctx := context.WithValue(w.ctx, helper.TransactionKey, transaction)
	if limit <= 0 {
		limit = 10
	}

	var filters []filter.Filter
	if filtersJSON != "" {
		if err := json.Unmarshal([]byte(filtersJSON), &filters); err != nil {
			return "", apmerr.DBusError(apmerr.New(apmerr.ErrorTypeValidation, err))
		}
	}

	validated, err := swcat.FilterConfig.Validate(filters)
	if err != nil {
		return "", apmerr.DBusError(apmerr.New(apmerr.ErrorTypeValidation, err))
	}

	params := appstream.ListParams{
		Sort:    sort,
		Order:   order,
		Limit:   limit,
		Offset:  offset,
		Filters: validated,
	}

	resp, err := w.appstreamActions.List(ctx, params)
	if err != nil {
		return "", apmerr.DBusError(err)
	}
	data, jerr := json.Marshal(reply.OK(resp))
	if jerr != nil {
		return "", dbus.MakeFailedError(jerr)
	}
	return string(data), nil
}

// ApplicationGetFilterFields возвращает список полей фильтрации приложений.
func (w *DBusWrapper) ApplicationGetFilterFields(transaction string) (string, *dbus.Error) {
	ctx := context.WithValue(w.ctx, helper.TransactionKey, transaction)
	resp, err := w.appstreamActions.GetFilterFields(ctx)
	if err != nil {
		return "", apmerr.DBusError(err)
	}
	data, jerr := json.Marshal(reply.OK(resp))
	if jerr != nil {
		return "", dbus.MakeFailedError(jerr)
	}
	return string(data), nil
}

// Sections возвращает список уникальных секций пакетов.
func (w *DBusWrapper) Sections(transaction string) (string, *dbus.Error) {
	ctx := context.WithValue(w.ctx, helper.TransactionKey, transaction)
	resp, err := w.actions.Sections(ctx)
	if err != nil {
		return "", apmerr.DBusError(err)
	}
	data, jerr := json.Marshal(reply.OK(resp))
	if jerr != nil {
		return "", dbus.MakeFailedError(jerr)
	}
	return string(data), nil
}

// ApplicationCategories возвращает список уникальных категорий приложений.
func (w *DBusWrapper) ApplicationCategories(transaction string) (string, *dbus.Error) {
	ctx := context.WithValue(w.ctx, helper.TransactionKey, transaction)
	resp, err := w.appstreamActions.Categories(ctx)
	if err != nil {
		return "", apmerr.DBusError(err)
	}
	data, jerr := json.Marshal(reply.OK(resp))
	if jerr != nil {
		return "", dbus.MakeFailedError(jerr)
	}
	return string(data), nil
}

// SetAptConfigOverrides устанавливает переопределения конфигурации APT, сохраняющиеся между запросами.
func (w *DBusWrapper) SetAptConfigOverrides(sender dbus.Sender, options map[string]string) (string, *dbus.Error) {
	if err := w.checkManagePermission(sender); err != nil {
		return "", err
	}
	resp, err := w.actions.SetAptConfigOverrides(options)
	if err != nil {
		return "", apmerr.DBusError(err)
	}
	data, jerr := json.Marshal(reply.OK(resp))
	if jerr != nil {
		return "", dbus.MakeFailedError(jerr)
	}
	return string(data), nil
}

// GetAptConfigOverrides возвращает текущие переопределения конфигурации APT.
func (w *DBusWrapper) GetAptConfigOverrides() (string, *dbus.Error) {
	resp, err := w.actions.GetAptConfigOverrides()
	if err != nil {
		return "", apmerr.DBusError(err)
	}
	data, jerr := json.Marshal(reply.OK(resp))
	if jerr != nil {
		return "", dbus.MakeFailedError(jerr)
	}
	return string(data), nil
}

// ImageSaveConfig проверяет и сохраняет новую конфигурацию image.yml.
func (w *DBusWrapper) ImageSaveConfig(sender dbus.Sender, config string) (string, *dbus.Error) {
	if err := w.checkManagePermission(sender); err != nil {
		return "", err
	}

	configObject := build.Config{}
	if err := json.Unmarshal([]byte(config), &configObject); err != nil {
		return "", dbus.MakeFailedError(fmt.Errorf(app.T_("Failed to parse JSON: %w"), err))
	}
	resp, err := w.actions.ImageSaveConfig(w.ctx, configObject)
	if err != nil {
		return "", apmerr.DBusError(err)
	}
	data, jerr := json.Marshal(reply.OK(resp))
	if jerr != nil {
		return "", dbus.MakeFailedError(jerr)
	}
	return string(data), nil
}
