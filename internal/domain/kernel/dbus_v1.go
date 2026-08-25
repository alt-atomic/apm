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
	"encoding/json"

	"altlinux.space/alt-atomic/apm/internal/common/apmerr"
	"altlinux.space/alt-atomic/apm/internal/common/app"
	"altlinux.space/alt-atomic/apm/internal/common/dbusv1"
	"altlinux.space/alt-atomic/apm/internal/common/helper"
	"altlinux.space/alt-atomic/apm/internal/common/polkit"
	"altlinux.space/alt-atomic/apm/internal/common/reply"

	"github.com/godbus/dbus/v5"
)

const DBusInterface = "org.altlinux.APM.kernel"

func DBusFactory(appConfig *app.Config, reporter *reply.Reporter) dbusv1.Module {
	return dbusv1.Module{
		Interface: DBusInterface,
		Build: func(ctx context.Context, conn *dbus.Conn) (dbusv1.Object, error) {
			actions := NewActions(appConfig, reporter)
			return dbusv1.Object{Value: NewDBusWrapper(actions, conn, ctx)}, nil
		},
	}
}

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
func (w *DBusWrapper) checkManagePermission(msg dbus.Message) *dbus.Error {
	if err := polkit.Check(w.conn, msg, "org.altlinux.APM.manage"); err != nil {
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
func (w *DBusWrapper) CheckInstallKernel(msg dbus.Message, flavour string, modules []string, includeHeaders bool, transaction string, background bool) (string, *dbus.Error) {
	if err := w.checkManagePermission(msg); err != nil {
		return "", err
	}

	if transaction == "" {
		transaction = helper.GenerateTransactionID()
	}

	if background {
		ctx := context.WithValue(w.ctx, helper.TransactionKey, transaction)
		go func() {
			resp, err := w.actions.InstallKernel(ctx, flavour, modules, includeHeaders, true)
			w.actions.reporter.SendTaskResult(ctx, reply.EventKernelCheckInstall, resp, err)
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
func (w *DBusWrapper) InstallKernel(msg dbus.Message, flavour string, modules []string, includeHeaders bool, transaction string, background bool) (string, *dbus.Error) {
	if err := w.checkManagePermission(msg); err != nil {
		return "", err
	}

	if transaction == "" {
		transaction = helper.GenerateTransactionID()
	}

	if background {
		ctx := context.WithValue(w.ctx, helper.TransactionKey, transaction)
		go func() {
			resp, err := w.actions.InstallKernel(ctx, flavour, modules, includeHeaders, false)
			w.actions.reporter.SendTaskResult(ctx, reply.EventKernelInstall, resp, err)
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
func (w *DBusWrapper) CheckUpdateKernel(msg dbus.Message, flavour string, modules []string, includeHeaders bool, transaction string, background bool) (string, *dbus.Error) {
	if err := w.checkManagePermission(msg); err != nil {
		return "", err
	}

	if transaction == "" {
		transaction = helper.GenerateTransactionID()
	}

	if background {
		ctx := context.WithValue(w.ctx, helper.TransactionKey, transaction)
		go func() {
			resp, err := w.actions.UpdateKernel(ctx, flavour, modules, includeHeaders, true)
			w.actions.reporter.SendTaskResult(ctx, reply.EventKernelCheckUpdate, resp, err)
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
func (w *DBusWrapper) UpdateKernel(msg dbus.Message, flavour string, modules []string, includeHeaders bool, transaction string, background bool) (string, *dbus.Error) {
	if err := w.checkManagePermission(msg); err != nil {
		return "", err
	}

	if transaction == "" {
		transaction = helper.GenerateTransactionID()
	}

	if background {
		ctx := context.WithValue(w.ctx, helper.TransactionKey, transaction)
		go func() {
			resp, err := w.actions.UpdateKernel(ctx, flavour, modules, includeHeaders, false)
			w.actions.reporter.SendTaskResult(ctx, reply.EventKernelUpdate, resp, err)
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
func (w *DBusWrapper) CheckCleanOldKernels(msg dbus.Message, noBackup bool, transaction string, background bool) (string, *dbus.Error) {
	if err := w.checkManagePermission(msg); err != nil {
		return "", err
	}

	if transaction == "" {
		transaction = helper.GenerateTransactionID()
	}

	if background {
		ctx := context.WithValue(w.ctx, helper.TransactionKey, transaction)
		go func() {
			resp, err := w.actions.CleanOldKernels(ctx, noBackup, true)
			w.actions.reporter.SendTaskResult(ctx, reply.EventKernelCheckClean, resp, err)
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
func (w *DBusWrapper) CleanOldKernels(msg dbus.Message, noBackup bool, transaction string, background bool) (string, *dbus.Error) {
	if err := w.checkManagePermission(msg); err != nil {
		return "", err
	}

	if transaction == "" {
		transaction = helper.GenerateTransactionID()
	}

	if background {
		ctx := context.WithValue(w.ctx, helper.TransactionKey, transaction)
		go func() {
			resp, err := w.actions.CleanOldKernels(ctx, noBackup, false)
			w.actions.reporter.SendTaskResult(ctx, reply.EventKernelClean, resp, err)
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
func (w *DBusWrapper) CheckInstallKernelModules(msg dbus.Message, flavour string, modules []string, transaction string, background bool) (string, *dbus.Error) {
	if err := w.checkManagePermission(msg); err != nil {
		return "", err
	}

	if transaction == "" {
		transaction = helper.GenerateTransactionID()
	}

	if background {
		ctx := context.WithValue(w.ctx, helper.TransactionKey, transaction)
		go func() {
			resp, err := w.actions.InstallKernelModules(ctx, flavour, modules, true)
			w.actions.reporter.SendTaskResult(ctx, reply.EventKernelCheckInstallMods, resp, err)
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
func (w *DBusWrapper) InstallKernelModules(msg dbus.Message, flavour string, modules []string, transaction string, background bool) (string, *dbus.Error) {
	if err := w.checkManagePermission(msg); err != nil {
		return "", err
	}

	if transaction == "" {
		transaction = helper.GenerateTransactionID()
	}

	if background {
		ctx := context.WithValue(w.ctx, helper.TransactionKey, transaction)
		go func() {
			resp, err := w.actions.InstallKernelModules(ctx, flavour, modules, false)
			w.actions.reporter.SendTaskResult(ctx, reply.EventKernelInstallMods, resp, err)
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
func (w *DBusWrapper) CheckRemoveKernelModules(msg dbus.Message, flavour string, modules []string, transaction string, background bool) (string, *dbus.Error) {
	if err := w.checkManagePermission(msg); err != nil {
		return "", err
	}

	if transaction == "" {
		transaction = helper.GenerateTransactionID()
	}

	if background {
		ctx := context.WithValue(w.ctx, helper.TransactionKey, transaction)
		go func() {
			resp, err := w.actions.RemoveKernelModules(ctx, flavour, modules, true)
			w.actions.reporter.SendTaskResult(ctx, reply.EventKernelCheckRemoveMods, resp, err)
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
func (w *DBusWrapper) RemoveKernelModules(msg dbus.Message, flavour string, modules []string, transaction string, background bool) (string, *dbus.Error) {
	if err := w.checkManagePermission(msg); err != nil {
		return "", err
	}

	if transaction == "" {
		transaction = helper.GenerateTransactionID()
	}

	if background {
		ctx := context.WithValue(w.ctx, helper.TransactionKey, transaction)
		go func() {
			resp, err := w.actions.RemoveKernelModules(ctx, flavour, modules, false)
			w.actions.reporter.SendTaskResult(ctx, reply.EventKernelRemoveMods, resp, err)
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

// BackgroundTaskResponse — совместимый ответ API v1 при запуске фоновой задачи.
type BackgroundTaskResponse struct {
	Message     string `json:"message"`
	Transaction string `json:"transaction"`
}
