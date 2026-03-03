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
	"apm/internal/common/apmerr"
	"apm/internal/common/app"
	"apm/internal/common/helper"
	"apm/internal/common/reply"
	"context"
	"encoding/json"

	"github.com/godbus/dbus/v5"
)

// DBusWrapper предоставляет обёртку для действий с ядрами через DBus.
type DBusWrapper struct {
	conn    *dbus.Conn
	actions *Actions
	ctx     context.Context
}

// NewDBusWrapper создаёт новую обёртку над actions.
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

// ListKernels возвращает список доступных ядер.
func (w *DBusWrapper) ListKernels(flavour string, installedOnly bool, transaction string) (string, *dbus.Error) {
	ctx := context.WithValue(w.ctx, helper.TransactionKey, transaction)
	resp, err := w.actions.ListKernels(ctx, flavour, installedOnly)
	if err != nil {
		return "", apmerr.DBusError(err)
	}
	data, jerr := json.Marshal(reply.OK(resp))
	if jerr != nil {
		return "", dbus.MakeFailedError(jerr)
	}
	return string(data), nil
}

// GetCurrentKernel возвращает информацию о текущем ядре.
func (w *DBusWrapper) GetCurrentKernel(transaction string) (string, *dbus.Error) {
	ctx := context.WithValue(w.ctx, helper.TransactionKey, transaction)
	resp, err := w.actions.GetCurrentKernel(ctx)
	if err != nil {
		return "", apmerr.DBusError(err)
	}
	data, jerr := json.Marshal(reply.OK(resp))
	if jerr != nil {
		return "", dbus.MakeFailedError(jerr)
	}
	return string(data), nil
}

// CheckInstallKernel проверяет возможность установки ядра.
func (w *DBusWrapper) CheckInstallKernel(sender dbus.Sender, flavour string, modules []string, includeHeaders bool, transaction string, background bool) (string, *dbus.Error) {
	if err := w.checkManagePermission(sender); err != nil {
		return "", err
	}

	if transaction == "" {
		transaction = helper.GenerateTransactionID()
	}

	if background {
		ctx := context.WithValue(w.ctx, helper.TransactionKey, transaction)
		go func() {
			resp, err := w.actions.InstallKernel(ctx, flavour, modules, includeHeaders, true)
			reply.SendTaskResult(ctx, reply.EventKernelCheckInstall, resp, err)
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
	resp, err := w.actions.InstallKernel(ctx, flavour, modules, includeHeaders, true)
	if err != nil {
		return "", apmerr.DBusError(err)
	}
	data, jerr := json.Marshal(reply.OK(resp))
	if jerr != nil {
		return "", dbus.MakeFailedError(jerr)
	}
	return string(data), nil
}

// InstallKernel устанавливает ядро.
func (w *DBusWrapper) InstallKernel(sender dbus.Sender, flavour string, modules []string, includeHeaders bool, transaction string, background bool) (string, *dbus.Error) {
	if err := w.checkManagePermission(sender); err != nil {
		return "", err
	}

	if transaction == "" {
		transaction = helper.GenerateTransactionID()
	}

	if background {
		ctx := context.WithValue(w.ctx, helper.TransactionKey, transaction)
		go func() {
			resp, err := w.actions.InstallKernel(ctx, flavour, modules, includeHeaders, false)
			reply.SendTaskResult(ctx, reply.EventKernelInstall, resp, err)
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
	resp, err := w.actions.InstallKernel(ctx, flavour, modules, includeHeaders, false)
	if err != nil {
		return "", apmerr.DBusError(err)
	}
	data, jerr := json.Marshal(reply.OK(resp))
	if jerr != nil {
		return "", dbus.MakeFailedError(jerr)
	}
	return string(data), nil
}

// CheckUpdateKernel проверяет возможность обновления ядра.
func (w *DBusWrapper) CheckUpdateKernel(sender dbus.Sender, flavour string, modules []string, includeHeaders bool, transaction string, background bool) (string, *dbus.Error) {
	if err := w.checkManagePermission(sender); err != nil {
		return "", err
	}

	if transaction == "" {
		transaction = helper.GenerateTransactionID()
	}

	if background {
		ctx := context.WithValue(w.ctx, helper.TransactionKey, transaction)
		go func() {
			resp, err := w.actions.UpdateKernel(ctx, flavour, modules, includeHeaders, true)
			reply.SendTaskResult(ctx, reply.EventKernelCheckUpdate, resp, err)
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
	resp, err := w.actions.UpdateKernel(ctx, flavour, modules, includeHeaders, true)
	if err != nil {
		return "", apmerr.DBusError(err)
	}
	data, jerr := json.Marshal(reply.OK(resp))
	if jerr != nil {
		return "", dbus.MakeFailedError(jerr)
	}
	return string(data), nil
}

// UpdateKernel обновляет ядро.
func (w *DBusWrapper) UpdateKernel(sender dbus.Sender, flavour string, modules []string, includeHeaders bool, transaction string, background bool) (string, *dbus.Error) {
	if err := w.checkManagePermission(sender); err != nil {
		return "", err
	}

	if transaction == "" {
		transaction = helper.GenerateTransactionID()
	}

	if background {
		ctx := context.WithValue(w.ctx, helper.TransactionKey, transaction)
		go func() {
			resp, err := w.actions.UpdateKernel(ctx, flavour, modules, includeHeaders, false)
			reply.SendTaskResult(ctx, reply.EventKernelUpdate, resp, err)
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
	resp, err := w.actions.UpdateKernel(ctx, flavour, modules, includeHeaders, false)
	if err != nil {
		return "", apmerr.DBusError(err)
	}
	data, jerr := json.Marshal(reply.OK(resp))
	if jerr != nil {
		return "", dbus.MakeFailedError(jerr)
	}
	return string(data), nil
}

// CheckCleanOldKernels проверяет возможность удаления старых ядер.
func (w *DBusWrapper) CheckCleanOldKernels(sender dbus.Sender, noBackup bool, transaction string, background bool) (string, *dbus.Error) {
	if err := w.checkManagePermission(sender); err != nil {
		return "", err
	}

	if transaction == "" {
		transaction = helper.GenerateTransactionID()
	}

	if background {
		ctx := context.WithValue(w.ctx, helper.TransactionKey, transaction)
		go func() {
			resp, err := w.actions.CleanOldKernels(ctx, noBackup, true)
			reply.SendTaskResult(ctx, reply.EventKernelCheckClean, resp, err)
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
	resp, err := w.actions.CleanOldKernels(ctx, noBackup, true)
	if err != nil {
		return "", apmerr.DBusError(err)
	}
	data, jerr := json.Marshal(reply.OK(resp))
	if jerr != nil {
		return "", dbus.MakeFailedError(jerr)
	}
	return string(data), nil
}

// CleanOldKernels удаляет старые ядра.
func (w *DBusWrapper) CleanOldKernels(sender dbus.Sender, noBackup bool, transaction string, background bool) (string, *dbus.Error) {
	if err := w.checkManagePermission(sender); err != nil {
		return "", err
	}

	if transaction == "" {
		transaction = helper.GenerateTransactionID()
	}

	if background {
		ctx := context.WithValue(w.ctx, helper.TransactionKey, transaction)
		go func() {
			resp, err := w.actions.CleanOldKernels(ctx, noBackup, false)
			reply.SendTaskResult(ctx, reply.EventKernelClean, resp, err)
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
	resp, err := w.actions.CleanOldKernels(ctx, noBackup, false)
	if err != nil {
		return "", apmerr.DBusError(err)
	}
	data, jerr := json.Marshal(reply.OK(resp))
	if jerr != nil {
		return "", dbus.MakeFailedError(jerr)
	}
	return string(data), nil
}

// ListKernelModules возвращает список модулей ядра.
func (w *DBusWrapper) ListKernelModules(flavour string, transaction string) (string, *dbus.Error) {
	ctx := context.WithValue(w.ctx, helper.TransactionKey, transaction)
	resp, err := w.actions.ListKernelModules(ctx, flavour)
	if err != nil {
		return "", apmerr.DBusError(err)
	}
	data, jerr := json.Marshal(reply.OK(resp))
	if jerr != nil {
		return "", dbus.MakeFailedError(jerr)
	}
	return string(data), nil
}

// CheckInstallKernelModules проверяет возможность установки модулей ядра.
func (w *DBusWrapper) CheckInstallKernelModules(sender dbus.Sender, flavour string, modules []string, transaction string, background bool) (string, *dbus.Error) {
	if err := w.checkManagePermission(sender); err != nil {
		return "", err
	}

	if transaction == "" {
		transaction = helper.GenerateTransactionID()
	}

	if background {
		ctx := context.WithValue(w.ctx, helper.TransactionKey, transaction)
		go func() {
			resp, err := w.actions.InstallKernelModules(ctx, flavour, modules, true)
			reply.SendTaskResult(ctx, reply.EventKernelCheckInstallMods, resp, err)
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
	resp, err := w.actions.InstallKernelModules(ctx, flavour, modules, true)
	if err != nil {
		return "", apmerr.DBusError(err)
	}
	data, jerr := json.Marshal(reply.OK(resp))
	if jerr != nil {
		return "", dbus.MakeFailedError(jerr)
	}
	return string(data), nil
}

// InstallKernelModules устанавливает модули ядра.
func (w *DBusWrapper) InstallKernelModules(sender dbus.Sender, flavour string, modules []string, transaction string, background bool) (string, *dbus.Error) {
	if err := w.checkManagePermission(sender); err != nil {
		return "", err
	}

	if transaction == "" {
		transaction = helper.GenerateTransactionID()
	}

	if background {
		ctx := context.WithValue(w.ctx, helper.TransactionKey, transaction)
		go func() {
			resp, err := w.actions.InstallKernelModules(ctx, flavour, modules, false)
			reply.SendTaskResult(ctx, reply.EventKernelInstallMods, resp, err)
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
	resp, err := w.actions.InstallKernelModules(ctx, flavour, modules, false)
	if err != nil {
		return "", apmerr.DBusError(err)
	}
	data, jerr := json.Marshal(reply.OK(resp))
	if jerr != nil {
		return "", dbus.MakeFailedError(jerr)
	}
	return string(data), nil
}

// CheckRemoveKernelModules проверяет возможность удаления модулей ядра.
func (w *DBusWrapper) CheckRemoveKernelModules(sender dbus.Sender, flavour string, modules []string, transaction string, background bool) (string, *dbus.Error) {
	if err := w.checkManagePermission(sender); err != nil {
		return "", err
	}

	if transaction == "" {
		transaction = helper.GenerateTransactionID()
	}

	if background {
		ctx := context.WithValue(w.ctx, helper.TransactionKey, transaction)
		go func() {
			resp, err := w.actions.RemoveKernelModules(ctx, flavour, modules, true)
			reply.SendTaskResult(ctx, reply.EventKernelCheckRemoveMods, resp, err)
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
	resp, err := w.actions.RemoveKernelModules(ctx, flavour, modules, true)
	if err != nil {
		return "", apmerr.DBusError(err)
	}
	data, jerr := json.Marshal(reply.OK(resp))
	if jerr != nil {
		return "", dbus.MakeFailedError(jerr)
	}
	return string(data), nil
}

// RemoveKernelModules удаляет модули ядра.
func (w *DBusWrapper) RemoveKernelModules(sender dbus.Sender, flavour string, modules []string, transaction string, background bool) (string, *dbus.Error) {
	if err := w.checkManagePermission(sender); err != nil {
		return "", err
	}

	if transaction == "" {
		transaction = helper.GenerateTransactionID()
	}

	if background {
		ctx := context.WithValue(w.ctx, helper.TransactionKey, transaction)
		go func() {
			resp, err := w.actions.RemoveKernelModules(ctx, flavour, modules, false)
			reply.SendTaskResult(ctx, reply.EventKernelRemoveMods, resp, err)
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
	resp, err := w.actions.RemoveKernelModules(ctx, flavour, modules, false)
	if err != nil {
		return "", apmerr.DBusError(err)
	}
	data, jerr := json.Marshal(reply.OK(resp))
	if jerr != nil {
		return "", dbus.MakeFailedError(jerr)
	}
	return string(data), nil
}
