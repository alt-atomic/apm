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
	"encoding/json"
	"fmt"

	"altlinux.space/alt-atomic/apm/internal/common/imagesvc"
	"altlinux.space/alt-atomic/apm/internal/common/polkit"

	"altlinux.space/alt-atomic/apm/internal/common/apmerr"
	"altlinux.space/alt-atomic/apm/internal/common/app"
	_package "altlinux.space/alt-atomic/apm/internal/common/apt/package"
	"altlinux.space/alt-atomic/apm/internal/common/dbusv1"
	"altlinux.space/alt-atomic/apm/internal/common/filter"
	"altlinux.space/alt-atomic/apm/internal/common/helper"
	"altlinux.space/alt-atomic/apm/internal/common/reply"
	"altlinux.space/alt-atomic/apm/internal/common/swcat"
	"altlinux.space/alt-atomic/apm/internal/domain/system/appstream"

	"github.com/godbus/dbus/v5"
)

const DBusInterface = "org.altlinux.APM.system"

func DBusFactory(appConfig *app.Config, reporter *reply.Reporter) dbusv1.Module {
	return dbusv1.Module{
		Interface: DBusInterface,
		Build: func(ctx context.Context, conn *dbus.Conn) (dbusv1.Object, error) {
			actions := NewActions(appConfig, reporter)
			return dbusv1.Object{Value: NewDBusWrapper(actions, conn, ctx)}, nil
		},
	}
}

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
		appstreamActions: appstream.NewActions(a.appConfig, a.reporter),
		conn:             c,
		ctx:              ctx,
	}
}

// checkManagePermission проверяет права org.altlinux.APM.manage
func (w *DBusWrapper) checkManagePermission(msg dbus.Message) *dbus.Error {
	if err := polkit.Check(w.conn, msg, "org.altlinux.APM.manage"); err != nil {
		return dbus.MakeFailedError(err)
	}
	return nil
}

// Install устанавливает пакеты.
func (w *DBusWrapper) Install(msg dbus.Message, packages []string, downloadOnly bool, transaction string, background bool) (string, *dbus.Error) {
	if err := w.checkManagePermission(msg); err != nil {
		return "", err
	}

	if transaction == "" {
		transaction = helper.GenerateTransactionID()
	}

	if background {
		ctx := context.WithValue(w.ctx, helper.TransactionKey, transaction)
		go func() {
			resp, err := w.actions.Install(ctx, packages, true, downloadOnly, false)
			w.actions.reporter.SendTaskResult(ctx, reply.EventSystemInstall, resp, err)
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
	resp, err := w.actions.Install(ctx, packages, true, downloadOnly, false)
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
func (w *DBusWrapper) Remove(msg dbus.Message, packages []string, purge bool, depends bool, transaction string, background bool) (string, *dbus.Error) {
	if err := w.checkManagePermission(msg); err != nil {
		return "", err
	}

	if transaction == "" {
		transaction = helper.GenerateTransactionID()
	}

	if background {
		ctx := context.WithValue(w.ctx, helper.TransactionKey, transaction)
		go func() {
			resp, err := w.actions.Remove(ctx, packages, purge, depends, true)
			w.actions.reporter.SendTaskResult(ctx, reply.EventSystemRemove, resp, err)
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
func (w *DBusWrapper) Update(msg dbus.Message, transaction string, background bool) (string, *dbus.Error) {
	if err := w.checkManagePermission(msg); err != nil {
		return "", err
	}

	if transaction == "" {
		transaction = helper.GenerateTransactionID()
	}

	if background {
		ctx := context.WithValue(w.ctx, helper.TransactionKey, transaction)
		go func() {
			resp, err := w.actions.Update(ctx, false, false)
			w.actions.reporter.SendTaskResult(ctx, reply.EventSystemUpdate, resp, err)
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

// List выполняет продвинутый поиск пакетов по фильтру. filtersJSON — JSON-объект {"filters","orFilters"}.
func (w *DBusWrapper) List(sort string, order string, limit int, offset int, filtersJSON string, forceUpdate bool, transaction string) (string, *dbus.Error) {
	ctx := context.WithValue(w.ctx, helper.TransactionKey, transaction)
	if limit <= 0 {
		limit = 10
	}

	body, err := filter.ParseJSONBody(filtersJSON)
	if err != nil {
		return "", apmerr.DBusError(apmerr.New(apmerr.ErrorTypeValidation, err))
	}

	groups, err := _package.SystemFilterConfig.ValidateBody(body)
	if err != nil {
		return "", apmerr.DBusError(apmerr.New(apmerr.ErrorTypeValidation, err))
	}

	params := ListParams{
		Sort:        sort,
		Order:       order,
		Limit:       limit,
		Offset:      offset,
		Filters:     groups,
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
func (w *DBusWrapper) CheckUpgrade(msg dbus.Message, transaction string, background bool) (string, *dbus.Error) {
	if err := w.checkManagePermission(msg); err != nil {
		return "", err
	}

	if transaction == "" {
		transaction = helper.GenerateTransactionID()
	}

	if background {
		ctx := context.WithValue(w.ctx, helper.TransactionKey, transaction)
		go func() {
			resp, err := w.actions.CheckUpgrade(ctx)
			w.actions.reporter.SendTaskResult(ctx, reply.EventSystemCheckUpgrade, resp, err)
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
func (w *DBusWrapper) Upgrade(msg dbus.Message, downloadOnly bool, transaction string, background bool) (string, *dbus.Error) {
	if err := w.checkManagePermission(msg); err != nil {
		return "", err
	}

	if transaction == "" {
		transaction = helper.GenerateTransactionID()
	}

	if background {
		ctx := context.WithValue(w.ctx, helper.TransactionKey, transaction)
		go func() {
			resp, err := w.actions.Upgrade(ctx, downloadOnly)
			w.actions.reporter.SendTaskResult(ctx, reply.EventSystemUpgrade, resp, err)
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
func (w *DBusWrapper) CheckInstall(msg dbus.Message, packages []string, transaction string, background bool) (string, *dbus.Error) {
	if err := w.checkManagePermission(msg); err != nil {
		return "", err
	}

	if transaction == "" {
		transaction = helper.GenerateTransactionID()
	}

	if background {
		ctx := context.WithValue(w.ctx, helper.TransactionKey, transaction)
		go func() {
			resp, err := w.actions.CheckInstall(ctx, packages)
			w.actions.reporter.SendTaskResult(ctx, reply.EventSystemCheckInstall, resp, err)
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
func (w *DBusWrapper) CheckRemove(msg dbus.Message, packages []string, depends bool, transaction string, background bool) (string, *dbus.Error) {
	if err := w.checkManagePermission(msg); err != nil {
		return "", err
	}

	if transaction == "" {
		transaction = helper.GenerateTransactionID()
	}

	if background {
		ctx := context.WithValue(w.ctx, helper.TransactionKey, transaction)
		go func() {
			resp, err := w.actions.CheckRemove(ctx, packages, false, depends)
			w.actions.reporter.SendTaskResult(ctx, reply.EventSystemCheckRemove, resp, err)
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
func (w *DBusWrapper) ImageApply(msg dbus.Message, transaction string, background bool, pullImage bool, noCache bool, configPath string, workdir string) (string, *dbus.Error) {
	if err := w.checkManagePermission(msg); err != nil {
		return "", err
	}

	if transaction == "" {
		transaction = helper.GenerateTransactionID()
	}

	hostCache := !noCache

	if background {
		ctx := context.WithValue(w.ctx, helper.TransactionKey, transaction)
		go func() {
			resp, err := w.actions.ImageApply(ctx, pullImage, hostCache, true, configPath, workdir)
			w.actions.reporter.SendTaskResult(ctx, reply.EventSystemImageApply, resp, err)
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
	resp, err := w.actions.ImageApply(ctx, pullImage, hostCache, true, configPath, workdir)
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
func (w *DBusWrapper) ImageHistory(msg dbus.Message, transaction string, imageName string, limit int, offset int) (string, *dbus.Error) {
	if err := w.checkManagePermission(msg); err != nil {
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

// ImageSwitch переключает систему на другой базовый образ.
func (w *DBusWrapper) ImageSwitch(msg dbus.Message, transaction string, background bool, image string, pullImage bool, noCache bool) (string, *dbus.Error) {
	if err := w.checkManagePermission(msg); err != nil {
		return "", err
	}

	if transaction == "" {
		transaction = helper.GenerateTransactionID()
	}

	hostCache := !noCache

	if background {
		ctx := context.WithValue(w.ctx, helper.TransactionKey, transaction)
		go func() {
			resp, err := w.actions.ImageSwitch(ctx, image, pullImage, hostCache)
			w.actions.reporter.SendTaskResult(ctx, reply.EventSystemImageSwitch, resp, err)
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
	resp, err := w.actions.ImageSwitch(ctx, image, pullImage, hostCache)
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
func (w *DBusWrapper) ImageUpdate(msg dbus.Message, transaction string, background bool, noCache bool) (string, *dbus.Error) {
	if err := w.checkManagePermission(msg); err != nil {
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
			w.actions.reporter.SendTaskResult(ctx, reply.EventSystemImageUpdate, resp, err)
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
func (w *DBusWrapper) ImageStatus(msg dbus.Message, transaction string) (string, *dbus.Error) {
	if err := w.checkManagePermission(msg); err != nil {
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

// ImageSyncGroups синхронизирует группы пользователей из конфигурации.
func (w *DBusWrapper) ImageSyncGroups(msg dbus.Message, transaction string) (string, *dbus.Error) {
	if err := w.checkManagePermission(msg); err != nil {
		return "", err
	}
	ctx := context.WithValue(w.ctx, helper.TransactionKey, transaction)
	resp, err := w.actions.ImageSyncGroups(ctx)
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
func (w *DBusWrapper) ApplicationUpdate(msg dbus.Message, transaction string, background bool) (string, *dbus.Error) {
	if err := w.checkManagePermission(msg); err != nil {
		return "", err
	}

	if transaction == "" {
		transaction = helper.GenerateTransactionID()
	}

	if background {
		ctx := context.WithValue(w.ctx, helper.TransactionKey, transaction)
		go func() {
			resp, err := w.appstreamActions.Update(ctx)
			w.actions.reporter.SendTaskResult(ctx, reply.EventApplicationUpdate, resp, err)
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

// ApplicationList возвращает список приложений с фильтрами. filtersJSON — JSON-объект {"filters","orFilters"}.
func (w *DBusWrapper) ApplicationList(sort, order string, limit, offset int, filtersJSON, transaction string) (string, *dbus.Error) {
	ctx := context.WithValue(w.ctx, helper.TransactionKey, transaction)
	if limit <= 0 {
		limit = 10
	}

	body, err := filter.ParseJSONBody(filtersJSON)
	if err != nil {
		return "", apmerr.DBusError(apmerr.New(apmerr.ErrorTypeValidation, err))
	}

	groups, err := swcat.FilterConfig.ValidateBody(body)
	if err != nil {
		return "", apmerr.DBusError(apmerr.New(apmerr.ErrorTypeValidation, err))
	}

	params := appstream.ListParams{
		Sort:    sort,
		Order:   order,
		Limit:   limit,
		Offset:  offset,
		Filters: groups,
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
func (w *DBusWrapper) SetAptConfigOverrides(msg dbus.Message, options map[string]string) (string, *dbus.Error) {
	if err := w.checkManagePermission(msg); err != nil {
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
func (w *DBusWrapper) ImageSaveConfig(msg dbus.Message, config string) (string, *dbus.Error) {
	if err := w.checkManagePermission(msg); err != nil {
		return "", err
	}

	configObject := imagesvc.Config{}
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

// BackgroundTaskResponse — совместимый ответ API v1 при запуске фоновой задачи.
type BackgroundTaskResponse struct {
	Message     string `json:"message"`
	Transaction string `json:"transaction"`
}
