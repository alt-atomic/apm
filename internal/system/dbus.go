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
	"apm/internal/common/app"
	"apm/internal/common/build"
	"apm/internal/common/helper"
	"apm/internal/common/reply"
	"context"
	"encoding/json"
	"fmt"

	"github.com/godbus/dbus/v5"
)

// DBusWrapper – обёртка для системных действий, предназначенная для экспорта через DBus.
type DBusWrapper struct {
	conn    *dbus.Conn
	actions *Actions
	ctx     context.Context
}

// NewDBusWrapper создаёт новую обёртку над actions
func NewDBusWrapper(a *Actions, c *dbus.Conn, ctx context.Context) *DBusWrapper {
	return &DBusWrapper{actions: a, conn: c, ctx: ctx}
}

// checkManagePermission проверяет права org.altlinux.APM.manage
func (w *DBusWrapper) checkManagePermission(sender dbus.Sender) *dbus.Error {
	if err := helper.PolkitCheck(w.conn, sender, "org.altlinux.APM.manage"); err != nil {
		return dbus.MakeFailedError(err)
	}
	return nil
}

// Install – Установка пакетов
// doc_response: InstallRemoveResponse
func (w *DBusWrapper) Install(sender dbus.Sender, packages []string, transaction string, background bool) (string, *dbus.Error) {
	if err := w.checkManagePermission(sender); err != nil {
		return "", err
	}

	if transaction == "" {
		transaction = generateTransactionID()
	}

	if background {
		ctx := context.WithValue(w.ctx, helper.TransactionKey, transaction)
		go func() {
			resp, err := w.actions.Install(ctx, packages, true)
			var data interface{}
			if resp != nil {
				data = resp.Data
			}
			reply.SendTaskResult(ctx, "system.Install", data, err)
		}()

		bgResp := BackgroundTaskResponse{
			Message:     app.T_("Task started in background"),
			Transaction: transaction,
		}
		data, jerr := json.Marshal(bgResp)
		if jerr != nil {
			return "", dbus.MakeFailedError(jerr)
		}
		return string(data), nil
	}

	// Синхронное выполнение
	ctx := context.WithValue(w.ctx, helper.TransactionKey, transaction)
	resp, err := w.actions.Install(ctx, packages, true)
	if err != nil {
		return "", dbus.MakeFailedError(err)
	}
	data, jerr := json.Marshal(resp)
	if jerr != nil {
		return "", dbus.MakeFailedError(jerr)
	}
	return string(data), nil
}

// Remove – Удаление пакетов
// doc_response: InstallRemoveResponse
func (w *DBusWrapper) Remove(sender dbus.Sender, packages []string, purge bool, depends bool, transaction string, background bool) (string, *dbus.Error) {
	if err := w.checkManagePermission(sender); err != nil {
		return "", err
	}

	if transaction == "" {
		transaction = generateTransactionID()
	}

	if background {
		ctx := context.WithValue(w.ctx, helper.TransactionKey, transaction)
		go func() {
			resp, err := w.actions.Remove(ctx, packages, purge, depends, true)
			var data interface{}
			if resp != nil {
				data = resp.Data
			}
			reply.SendTaskResult(ctx, "system.Remove", data, err)
		}()

		bgResp := BackgroundTaskResponse{
			Message:     app.T_("Task started in background"),
			Transaction: transaction,
		}
		data, jerr := json.Marshal(bgResp)
		if jerr != nil {
			return "", dbus.MakeFailedError(jerr)
		}
		return string(data), nil
	}

	// Синхронное выполнение
	ctx := context.WithValue(w.ctx, helper.TransactionKey, transaction)
	resp, err := w.actions.Remove(ctx, packages, purge, depends, true)
	if err != nil {
		return "", dbus.MakeFailedError(err)
	}
	data, jerr := json.Marshal(resp)
	if jerr != nil {
		return "", dbus.MakeFailedError(jerr)
	}
	return string(data), nil
}

// GetFilterFields - Список полей фильтрации для метода list, помогает динамически строить фильтры в интерфейсе
// doc_response: GetFilterFieldsResponse
func (w *DBusWrapper) GetFilterFields(transaction string) (string, *dbus.Error) {
	ctx := context.WithValue(w.ctx, helper.TransactionKey, transaction)
	resp, err := w.actions.GetFilterFields(ctx)
	if err != nil {
		return "", dbus.MakeFailedError(err)
	}

	data, jerr := json.Marshal(resp)
	if jerr != nil {
		return "", dbus.MakeFailedError(jerr)
	}

	return string(data), nil
}

// Update – Обновление системы
// doc_response: UpdateResponse
func (w *DBusWrapper) Update(sender dbus.Sender, transaction string, background bool) (string, *dbus.Error) {
	if err := w.checkManagePermission(sender); err != nil {
		return "", err
	}

	if transaction == "" {
		transaction = generateTransactionID()
	}

	if background {
		ctx := context.WithValue(w.ctx, helper.TransactionKey, transaction)
		go func() {
			resp, err := w.actions.Update(ctx, false)
			var data interface{}
			if resp != nil {
				data = resp.Data
			}
			reply.SendTaskResult(ctx, "system.Update", data, err)
		}()

		bgResp := BackgroundTaskResponse{
			Message:     app.T_("Task started in background"),
			Transaction: transaction,
		}
		data, jerr := json.Marshal(bgResp)
		if jerr != nil {
			return "", dbus.MakeFailedError(jerr)
		}
		return string(data), nil
	}

	// Синхронное выполнение
	ctx := context.WithValue(w.ctx, helper.TransactionKey, transaction)
	resp, err := w.actions.Update(ctx, false)
	if err != nil {
		return "", dbus.MakeFailedError(err)
	}
	data, jerr := json.Marshal(resp)
	if jerr != nil {
		return "", dbus.MakeFailedError(jerr)
	}
	return string(data), nil
}

// List – Продвинутый поиск пакетов по фильтру
// doc_response: ListResponse
func (w *DBusWrapper) List(sort string, order string, limit int, offset int, filters []string, forceUpdate bool, transaction string) (string, *dbus.Error) {
	ctx := context.WithValue(w.ctx, helper.TransactionKey, transaction)
	if limit <= 0 {
		limit = 10
	}
	params := ListParams{
		Sort:        sort,
		Order:       order,
		Limit:       limit,
		Offset:      offset,
		Filters:     filters,
		ForceUpdate: forceUpdate,
	}

	resp, err := w.actions.List(ctx, params, true)
	if err != nil {
		return "", dbus.MakeFailedError(err)
	}
	data, jerr := json.Marshal(resp)
	if jerr != nil {
		return "", dbus.MakeFailedError(jerr)
	}
	return string(data), nil
}

// Info – Получить информацию о пакете
// doc_response: InfoResponse
func (w *DBusWrapper) Info(packageName string, transaction string) (string, *dbus.Error) {
	ctx := context.WithValue(w.ctx, helper.TransactionKey, transaction)
	resp, err := w.actions.Info(ctx, packageName, true)
	if err != nil {
		return "", dbus.MakeFailedError(err)
	}
	data, jerr := json.Marshal(resp)
	if jerr != nil {
		return "", dbus.MakeFailedError(jerr)
	}
	return string(data), nil
}

// CheckUpgrade – Проверить обновление
// doc_response: CheckResponse
func (w *DBusWrapper) CheckUpgrade(sender dbus.Sender, transaction string, background bool) (string, *dbus.Error) {
	if err := w.checkManagePermission(sender); err != nil {
		return "", err
	}

	if transaction == "" {
		transaction = generateTransactionID()
	}

	if background {
		ctx := context.WithValue(w.ctx, helper.TransactionKey, transaction)
		go func() {
			resp, err := w.actions.CheckUpgrade(ctx)
			var data interface{}
			if resp != nil {
				data = resp.Data
			}
			reply.SendTaskResult(ctx, "system.CheckUpgrade", data, err)
		}()

		bgResp := BackgroundTaskResponse{
			Message:     app.T_("Task started in background"),
			Transaction: transaction,
		}
		data, jerr := json.Marshal(bgResp)
		if jerr != nil {
			return "", dbus.MakeFailedError(jerr)
		}
		return string(data), nil
	}

	// Синхронное выполнение
	ctx := context.WithValue(w.ctx, helper.TransactionKey, transaction)
	resp, err := w.actions.CheckUpgrade(ctx)
	if err != nil {
		return "", dbus.MakeFailedError(err)
	}
	data, jerr := json.Marshal(resp)
	if jerr != nil {
		return "", dbus.MakeFailedError(jerr)
	}
	return string(data), nil
}

// Upgrade – Обновить систему (для не-атомарных систем)
// doc_response: UpgradeResponse
func (w *DBusWrapper) Upgrade(sender dbus.Sender, transaction string, background bool) (string, *dbus.Error) {
	if err := w.checkManagePermission(sender); err != nil {
		return "", err
	}

	if transaction == "" {
		transaction = generateTransactionID()
	}

	if background {
		ctx := context.WithValue(w.ctx, helper.TransactionKey, transaction)
		go func() {
			resp, err := w.actions.Upgrade(ctx)
			var data interface{}
			if resp != nil {
				data = resp.Data
			}
			reply.SendTaskResult(ctx, "system.Upgrade", data, err)
		}()

		bgResp := BackgroundTaskResponse{
			Message:     app.T_("Task started in background"),
			Transaction: transaction,
		}
		data, jerr := json.Marshal(bgResp)
		if jerr != nil {
			return "", dbus.MakeFailedError(jerr)
		}
		return string(data), nil
	}

	// Синхронное выполнение
	ctx := context.WithValue(w.ctx, helper.TransactionKey, transaction)
	resp, err := w.actions.Upgrade(ctx)
	if err != nil {
		return "", dbus.MakeFailedError(err)
	}
	data, jerr := json.Marshal(resp)
	if jerr != nil {
		return "", dbus.MakeFailedError(jerr)
	}
	return string(data), nil
}

// CheckInstall – Проверить установку пакетов
// doc_response: CheckResponse
func (w *DBusWrapper) CheckInstall(sender dbus.Sender, packages []string, transaction string, background bool) (string, *dbus.Error) {
	if err := w.checkManagePermission(sender); err != nil {
		return "", err
	}

	if transaction == "" {
		transaction = generateTransactionID()
	}

	if background {
		ctx := context.WithValue(w.ctx, helper.TransactionKey, transaction)
		go func() {
			resp, err := w.actions.CheckInstall(ctx, packages)
			var data interface{}
			if resp != nil {
				data = resp.Data
			}
			reply.SendTaskResult(ctx, "system.CheckInstall", data, err)
		}()

		bgResp := BackgroundTaskResponse{
			Message:     app.T_("Task started in background"),
			Transaction: transaction,
		}
		data, jerr := json.Marshal(bgResp)
		if jerr != nil {
			return "", dbus.MakeFailedError(jerr)
		}
		return string(data), nil
	}

	// Синхронное выполнение
	ctx := context.WithValue(w.ctx, helper.TransactionKey, transaction)
	resp, err := w.actions.CheckInstall(ctx, packages)
	if err != nil {
		return "", dbus.MakeFailedError(err)
	}
	data, jerr := json.Marshal(resp)
	if jerr != nil {
		return "", dbus.MakeFailedError(jerr)
	}
	return string(data), nil
}

// CheckRemove – Проверить удаление пакетов
// doc_response: CheckResponse
func (w *DBusWrapper) CheckRemove(sender dbus.Sender, packages []string, depends bool, transaction string, background bool) (string, *dbus.Error) {
	if err := w.checkManagePermission(sender); err != nil {
		return "", err
	}

	if transaction == "" {
		transaction = generateTransactionID()
	}

	if background {
		ctx := context.WithValue(w.ctx, helper.TransactionKey, transaction)
		go func() {
			resp, err := w.actions.CheckRemove(ctx, packages, false, depends)
			var data interface{}
			if resp != nil {
				data = resp.Data
			}
			reply.SendTaskResult(ctx, "system.CheckRemove", data, err)
		}()

		bgResp := BackgroundTaskResponse{
			Message:     app.T_("Task started in background"),
			Transaction: transaction,
		}
		data, jerr := json.Marshal(bgResp)
		if jerr != nil {
			return "", dbus.MakeFailedError(jerr)
		}
		return string(data), nil
	}

	// Синхронное выполнение
	ctx := context.WithValue(w.ctx, helper.TransactionKey, transaction)
	resp, err := w.actions.CheckRemove(ctx, packages, false, depends)
	if err != nil {
		return "", dbus.MakeFailedError(err)
	}
	data, jerr := json.Marshal(resp)
	if jerr != nil {
		return "", dbus.MakeFailedError(jerr)
	}
	return string(data), nil
}

// Search – Простой! Поиск пакетов
// doc_response: ListResponse
func (w *DBusWrapper) Search(sender dbus.Sender, packageName string, transaction string, installed bool) (string, *dbus.Error) {
	if err := w.checkManagePermission(sender); err != nil {
		return "", err
	}
	ctx := context.WithValue(w.ctx, helper.TransactionKey, transaction)
	resp, err := w.actions.Search(ctx, packageName, installed, true)
	if err != nil {
		return "", dbus.MakeFailedError(err)
	}
	data, jerr := json.Marshal(resp)
	if jerr != nil {
		return "", dbus.MakeFailedError(jerr)
	}
	return string(data), nil
}

// ImageApply – Декларативно применить настройки image.yml к образу хост-системы
// doc_response: ImageApplyResponse
func (w *DBusWrapper) ImageApply(sender dbus.Sender, transaction string, background bool) (string, *dbus.Error) {
	if err := w.checkManagePermission(sender); err != nil {
		return "", err
	}

	if transaction == "" {
		transaction = generateTransactionID()
	}

	if background {
		ctx := context.WithValue(w.ctx, helper.TransactionKey, transaction)
		go func() {
			resp, err := w.actions.ImageApply(ctx)
			var data interface{}
			if resp != nil {
				data = resp.Data
			}
			reply.SendTaskResult(ctx, "system.ImageApply", data, err)
		}()

		bgResp := BackgroundTaskResponse{
			Message:     app.T_("Task started in background"),
			Transaction: transaction,
		}
		data, jerr := json.Marshal(bgResp)
		if jerr != nil {
			return "", dbus.MakeFailedError(jerr)
		}
		return string(data), nil
	}

	// Синхронное выполнение
	ctx := context.WithValue(w.ctx, helper.TransactionKey, transaction)
	resp, err := w.actions.ImageApply(ctx)
	if err != nil {
		return "", dbus.MakeFailedError(err)
	}
	data, jerr := json.Marshal(resp)
	if jerr != nil {
		return "", dbus.MakeFailedError(jerr)
	}
	return string(data), nil
}

// ImageHistory – История обновлений
// doc_response: ImageHistoryResponse
func (w *DBusWrapper) ImageHistory(sender dbus.Sender, transaction string, imageName string, limit int, offset int) (string, *dbus.Error) {
	if err := w.checkManagePermission(sender); err != nil {
		return "", err
	}
	ctx := context.WithValue(w.ctx, helper.TransactionKey, transaction)
	resp, err := w.actions.ImageHistory(ctx, imageName, limit, offset)
	if err != nil {
		return "", dbus.MakeFailedError(err)
	}
	data, jerr := json.Marshal(resp)
	if jerr != nil {
		return "", dbus.MakeFailedError(jerr)
	}
	return string(data), nil
}

// ImageUpdate – Обновить образ системы
// doc_response: ImageUpdateResponse
func (w *DBusWrapper) ImageUpdate(sender dbus.Sender, transaction string, background bool) (string, *dbus.Error) {
	if err := w.checkManagePermission(sender); err != nil {
		return "", err
	}

	if transaction == "" {
		transaction = generateTransactionID()
	}

	if background {
		ctx := context.WithValue(w.ctx, helper.TransactionKey, transaction)
		go func() {
			resp, err := w.actions.ImageUpdate(ctx)
			var data interface{}
			if resp != nil {
				data = resp.Data
			}
			reply.SendTaskResult(ctx, "system.ImageUpdate", data, err)
		}()

		bgResp := BackgroundTaskResponse{
			Message:     app.T_("Task started in background"),
			Transaction: transaction,
		}
		data, jerr := json.Marshal(bgResp)
		if jerr != nil {
			return "", dbus.MakeFailedError(jerr)
		}
		return string(data), nil
	}

	// Синхронное выполнение
	ctx := context.WithValue(w.ctx, helper.TransactionKey, transaction)
	resp, err := w.actions.ImageUpdate(ctx)
	if err != nil {
		return "", dbus.MakeFailedError(err)
	}
	data, jerr := json.Marshal(resp)
	if jerr != nil {
		return "", dbus.MakeFailedError(jerr)
	}
	return string(data), nil
}

// ImageStatus – Проверить статус образа
// doc_response: ImageStatusResponse
func (w *DBusWrapper) ImageStatus(sender dbus.Sender, transaction string) (string, *dbus.Error) {
	if err := w.checkManagePermission(sender); err != nil {
		return "", err
	}
	ctx := context.WithValue(w.ctx, helper.TransactionKey, transaction)
	resp, err := w.actions.ImageStatus(ctx)
	if err != nil {
		return "", dbus.MakeFailedError(err)
	}
	data, jerr := json.Marshal(resp)
	if jerr != nil {
		return "", dbus.MakeFailedError(jerr)
	}
	return string(data), nil
}

// ImageGetConfig - Получить текущий конфиг image.yml
// doc_response: ImageConfigResponse
func (w *DBusWrapper) ImageGetConfig() (string, *dbus.Error) {
	resp, err := w.actions.ImageGetConfig(w.ctx)
	if err != nil {
		return "", dbus.MakeFailedError(err)
	}
	data, jerr := json.Marshal(resp)
	if jerr != nil {
		return "", dbus.MakeFailedError(jerr)
	}
	return string(data), nil
}

// ImageSaveConfig - Проверить и сохранить новый конфиг image.yml
// doc_response: ImageConfigResponse
func (w *DBusWrapper) ImageSaveConfig(config string) (string, *dbus.Error) {
	configObject := build.Config{}
	if err := json.Unmarshal([]byte(config), &configObject); err != nil {
		return "", dbus.MakeFailedError(fmt.Errorf(app.T_("Failed to parse JSON: %w"), err))
	}
	resp, err := w.actions.ImageSaveConfig(w.ctx, configObject)
	if err != nil {
		return "", dbus.MakeFailedError(err)
	}
	data, jerr := json.Marshal(resp)
	if jerr != nil {
		return "", dbus.MakeFailedError(jerr)
	}
	return string(data), nil
}
