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
	"apm/internal/common/app"
	"apm/internal/common/helper"
	"apm/internal/common/reply"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	"github.com/godbus/dbus/v5"
)

// DBusWrapper – обёртка для kernel действий, предназначенная для экспорта через DBus.
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

// generateTransactionID генерирует уникальный ID транзакции
func generateTransactionID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return fmt.Sprintf("%d-%s", time.Now().UnixNano(), hex.EncodeToString(b))
}

// ListKernels – Получить список доступных ядер
// doc_response: ListKernelsResponse
func (w *DBusWrapper) ListKernels(flavour string, installedOnly bool, transaction string) (string, *dbus.Error) {
	ctx := context.WithValue(w.ctx, helper.TransactionKey, transaction)
	resp, err := w.actions.ListKernels(ctx, flavour, installedOnly)
	if err != nil {
		return "", dbus.MakeFailedError(err)
	}
	data, jerr := json.Marshal(resp)
	if jerr != nil {
		return "", dbus.MakeFailedError(jerr)
	}
	return string(data), nil
}

// GetCurrentKernel – Получить информацию о текущем ядре
// doc_response: GetCurrentKernelResponse
func (w *DBusWrapper) GetCurrentKernel(transaction string) (string, *dbus.Error) {
	ctx := context.WithValue(w.ctx, helper.TransactionKey, transaction)
	resp, err := w.actions.GetCurrentKernel(ctx)
	if err != nil {
		return "", dbus.MakeFailedError(err)
	}
	data, jerr := json.Marshal(resp)
	if jerr != nil {
		return "", dbus.MakeFailedError(jerr)
	}
	return string(data), nil
}

// CheckInstallKernel – Проверить установку ядра (симуляция)
// doc_response: InstallUpdateKernelResponse
func (w *DBusWrapper) CheckInstallKernel(sender dbus.Sender, flavour string, modules []string, includeHeaders bool, transaction string, background bool) (string, *dbus.Error) {
	if err := w.checkManagePermission(sender); err != nil {
		return "", err
	}

	if transaction == "" {
		transaction = generateTransactionID()
	}

	if background {
		ctx := context.WithValue(w.ctx, helper.TransactionKey, transaction)
		go func() {
			resp, err := w.actions.InstallKernel(ctx, flavour, modules, includeHeaders, true)
			var data interface{}
			if resp != nil {
				data = resp.Data
			}
			reply.SendTaskResult(ctx, "kernel.CheckInstallKernel", data, err)
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

	ctx := context.WithValue(w.ctx, helper.TransactionKey, transaction)
	resp, err := w.actions.InstallKernel(ctx, flavour, modules, includeHeaders, true)
	if err != nil {
		return "", dbus.MakeFailedError(err)
	}
	data, jerr := json.Marshal(resp)
	if jerr != nil {
		return "", dbus.MakeFailedError(jerr)
	}
	return string(data), nil
}

// InstallKernel – Установить ядро с указанным flavour
// doc_response: InstallUpdateKernelResponse
func (w *DBusWrapper) InstallKernel(sender dbus.Sender, flavour string, modules []string, includeHeaders bool, transaction string, background bool) (string, *dbus.Error) {
	if err := w.checkManagePermission(sender); err != nil {
		return "", err
	}

	if transaction == "" {
		transaction = generateTransactionID()
	}

	if background {
		ctx := context.WithValue(w.ctx, helper.TransactionKey, transaction)
		go func() {
			resp, err := w.actions.InstallKernel(ctx, flavour, modules, includeHeaders, false)
			var data interface{}
			if resp != nil {
				data = resp.Data
			}
			reply.SendTaskResult(ctx, "kernel.InstallKernel", data, err)
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

	ctx := context.WithValue(w.ctx, helper.TransactionKey, transaction)
	resp, err := w.actions.InstallKernel(ctx, flavour, modules, includeHeaders, false)
	if err != nil {
		return "", dbus.MakeFailedError(err)
	}
	data, jerr := json.Marshal(resp)
	if jerr != nil {
		return "", dbus.MakeFailedError(jerr)
	}
	return string(data), nil
}

// CheckUpdateKernel – Проверить обновление ядра (симуляция)
// doc_response: InstallUpdateKernelResponse
func (w *DBusWrapper) CheckUpdateKernel(sender dbus.Sender, flavour string, modules []string, includeHeaders bool, transaction string, background bool) (string, *dbus.Error) {
	if err := w.checkManagePermission(sender); err != nil {
		return "", err
	}

	if transaction == "" {
		transaction = generateTransactionID()
	}

	if background {
		ctx := context.WithValue(w.ctx, helper.TransactionKey, transaction)
		go func() {
			resp, err := w.actions.UpdateKernel(ctx, flavour, modules, includeHeaders, true)
			var data interface{}
			if resp != nil {
				data = resp.Data
			}
			reply.SendTaskResult(ctx, "kernel.CheckUpdateKernel", data, err)
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

	ctx := context.WithValue(w.ctx, helper.TransactionKey, transaction)
	resp, err := w.actions.UpdateKernel(ctx, flavour, modules, includeHeaders, true)
	if err != nil {
		return "", dbus.MakeFailedError(err)
	}
	data, jerr := json.Marshal(resp)
	if jerr != nil {
		return "", dbus.MakeFailedError(jerr)
	}
	return string(data), nil
}

// UpdateKernel – Обновить ядро до последней версии
// doc_response: InstallUpdateKernelResponse
func (w *DBusWrapper) UpdateKernel(sender dbus.Sender, flavour string, modules []string, includeHeaders bool, transaction string, background bool) (string, *dbus.Error) {
	if err := w.checkManagePermission(sender); err != nil {
		return "", err
	}

	if transaction == "" {
		transaction = generateTransactionID()
	}

	if background {
		ctx := context.WithValue(w.ctx, helper.TransactionKey, transaction)
		go func() {
			resp, err := w.actions.UpdateKernel(ctx, flavour, modules, includeHeaders, false)
			var data interface{}
			if resp != nil {
				data = resp.Data
			}
			reply.SendTaskResult(ctx, "kernel.UpdateKernel", data, err)
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

	ctx := context.WithValue(w.ctx, helper.TransactionKey, transaction)
	resp, err := w.actions.UpdateKernel(ctx, flavour, modules, includeHeaders, false)
	if err != nil {
		return "", dbus.MakeFailedError(err)
	}
	data, jerr := json.Marshal(resp)
	if jerr != nil {
		return "", dbus.MakeFailedError(jerr)
	}
	return string(data), nil
}

// CheckCleanOldKernels – Проверить удаление старых ядер (симуляция)
// doc_response: CleanOldKernelsResponse
func (w *DBusWrapper) CheckCleanOldKernels(sender dbus.Sender, noBackup bool, transaction string, background bool) (string, *dbus.Error) {
	if err := w.checkManagePermission(sender); err != nil {
		return "", err
	}

	if transaction == "" {
		transaction = generateTransactionID()
	}

	if background {
		ctx := context.WithValue(w.ctx, helper.TransactionKey, transaction)
		go func() {
			resp, err := w.actions.CleanOldKernels(ctx, noBackup, true)
			var data interface{}
			if resp != nil {
				data = resp.Data
			}
			reply.SendTaskResult(ctx, "kernel.CheckCleanOldKernels", data, err)
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

	ctx := context.WithValue(w.ctx, helper.TransactionKey, transaction)
	resp, err := w.actions.CleanOldKernels(ctx, noBackup, true)
	if err != nil {
		return "", dbus.MakeFailedError(err)
	}
	data, jerr := json.Marshal(resp)
	if jerr != nil {
		return "", dbus.MakeFailedError(jerr)
	}
	return string(data), nil
}

// CleanOldKernels – Удалить старые ядра
// doc_response: CleanOldKernelsResponse
func (w *DBusWrapper) CleanOldKernels(sender dbus.Sender, noBackup bool, transaction string, background bool) (string, *dbus.Error) {
	if err := w.checkManagePermission(sender); err != nil {
		return "", err
	}

	if transaction == "" {
		transaction = generateTransactionID()
	}

	if background {
		ctx := context.WithValue(w.ctx, helper.TransactionKey, transaction)
		go func() {
			resp, err := w.actions.CleanOldKernels(ctx, noBackup, false)
			var data interface{}
			if resp != nil {
				data = resp.Data
			}
			reply.SendTaskResult(ctx, "kernel.CleanOldKernels", data, err)
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

	ctx := context.WithValue(w.ctx, helper.TransactionKey, transaction)
	resp, err := w.actions.CleanOldKernels(ctx, noBackup, false)
	if err != nil {
		return "", dbus.MakeFailedError(err)
	}
	data, jerr := json.Marshal(resp)
	if jerr != nil {
		return "", dbus.MakeFailedError(jerr)
	}
	return string(data), nil
}

// ListKernelModules – Получить список модулей для ядра
// doc_response: ListKernelModulesResponse
func (w *DBusWrapper) ListKernelModules(flavour string, transaction string) (string, *dbus.Error) {
	ctx := context.WithValue(w.ctx, helper.TransactionKey, transaction)
	resp, err := w.actions.ListKernelModules(ctx, flavour)
	if err != nil {
		return "", dbus.MakeFailedError(err)
	}
	data, jerr := json.Marshal(resp)
	if jerr != nil {
		return "", dbus.MakeFailedError(jerr)
	}
	return string(data), nil
}

// CheckInstallKernelModules – Проверить установку модулей ядра (симуляция)
// doc_response: InstallKernelModulesResponse
func (w *DBusWrapper) CheckInstallKernelModules(sender dbus.Sender, flavour string, modules []string, transaction string, background bool) (string, *dbus.Error) {
	if err := w.checkManagePermission(sender); err != nil {
		return "", err
	}

	if transaction == "" {
		transaction = generateTransactionID()
	}

	if background {
		ctx := context.WithValue(w.ctx, helper.TransactionKey, transaction)
		go func() {
			resp, err := w.actions.InstallKernelModules(ctx, flavour, modules, true)
			var data interface{}
			if resp != nil {
				data = resp.Data
			}
			reply.SendTaskResult(ctx, "kernel.CheckInstallKernelModules", data, err)
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

	ctx := context.WithValue(w.ctx, helper.TransactionKey, transaction)
	resp, err := w.actions.InstallKernelModules(ctx, flavour, modules, true)
	if err != nil {
		return "", dbus.MakeFailedError(err)
	}
	data, jerr := json.Marshal(resp)
	if jerr != nil {
		return "", dbus.MakeFailedError(jerr)
	}
	return string(data), nil
}

// InstallKernelModules – Установить модули ядра
// doc_response: InstallKernelModulesResponse
func (w *DBusWrapper) InstallKernelModules(sender dbus.Sender, flavour string, modules []string, transaction string, background bool) (string, *dbus.Error) {
	if err := w.checkManagePermission(sender); err != nil {
		return "", err
	}

	if transaction == "" {
		transaction = generateTransactionID()
	}

	if background {
		ctx := context.WithValue(w.ctx, helper.TransactionKey, transaction)
		go func() {
			resp, err := w.actions.InstallKernelModules(ctx, flavour, modules, false)
			var data interface{}
			if resp != nil {
				data = resp.Data
			}
			reply.SendTaskResult(ctx, "kernel.InstallKernelModules", data, err)
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

	ctx := context.WithValue(w.ctx, helper.TransactionKey, transaction)
	resp, err := w.actions.InstallKernelModules(ctx, flavour, modules, false)
	if err != nil {
		return "", dbus.MakeFailedError(err)
	}
	data, jerr := json.Marshal(resp)
	if jerr != nil {
		return "", dbus.MakeFailedError(jerr)
	}
	return string(data), nil
}

// CheckRemoveKernelModules – Проверить удаление модулей ядра (симуляция)
// doc_response: RemoveKernelModulesResponse
func (w *DBusWrapper) CheckRemoveKernelModules(sender dbus.Sender, flavour string, modules []string, transaction string, background bool) (string, *dbus.Error) {
	if err := w.checkManagePermission(sender); err != nil {
		return "", err
	}

	if transaction == "" {
		transaction = generateTransactionID()
	}

	if background {
		ctx := context.WithValue(w.ctx, helper.TransactionKey, transaction)
		go func() {
			resp, err := w.actions.RemoveKernelModules(ctx, flavour, modules, true)
			var data interface{}
			if resp != nil {
				data = resp.Data
			}
			reply.SendTaskResult(ctx, "kernel.CheckRemoveKernelModules", data, err)
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

	ctx := context.WithValue(w.ctx, helper.TransactionKey, transaction)
	resp, err := w.actions.RemoveKernelModules(ctx, flavour, modules, true)
	if err != nil {
		return "", dbus.MakeFailedError(err)
	}
	data, jerr := json.Marshal(resp)
	if jerr != nil {
		return "", dbus.MakeFailedError(jerr)
	}
	return string(data), nil
}

// RemoveKernelModules – Удалить модули ядра
// doc_response: RemoveKernelModulesResponse
func (w *DBusWrapper) RemoveKernelModules(sender dbus.Sender, flavour string, modules []string, transaction string, background bool) (string, *dbus.Error) {
	if err := w.checkManagePermission(sender); err != nil {
		return "", err
	}

	if transaction == "" {
		transaction = generateTransactionID()
	}

	if background {
		ctx := context.WithValue(w.ctx, helper.TransactionKey, transaction)
		go func() {
			resp, err := w.actions.RemoveKernelModules(ctx, flavour, modules, false)
			var data interface{}
			if resp != nil {
				data = resp.Data
			}
			reply.SendTaskResult(ctx, "kernel.RemoveKernelModules", data, err)
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

	ctx := context.WithValue(w.ctx, helper.TransactionKey, transaction)
	resp, err := w.actions.RemoveKernelModules(ctx, flavour, modules, false)
	if err != nil {
		return "", dbus.MakeFailedError(err)
	}
	data, jerr := json.Marshal(resp)
	if jerr != nil {
		return "", dbus.MakeFailedError(jerr)
	}
	return string(data), nil
}
