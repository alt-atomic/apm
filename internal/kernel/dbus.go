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
	"apm/internal/common/helper"
	"context"
	"encoding/json"

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

// InstallKernel – Установить ядро с указанным flavour
// doc_response: InstallUpdateKernelResponse
func (w *DBusWrapper) InstallKernel(sender dbus.Sender, flavour string, modules []string, includeHeaders bool, simulate bool, transaction string) (string, *dbus.Error) {
	if err := w.checkManagePermission(sender); err != nil {
		return "", err
	}

	ctx := context.WithValue(w.ctx, helper.TransactionKey, transaction)
	resp, err := w.actions.InstallKernel(ctx, flavour, modules, includeHeaders, simulate)
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
func (w *DBusWrapper) UpdateKernel(sender dbus.Sender, flavour string, modules []string, includeHeaders bool, simulate bool, transaction string) (string, *dbus.Error) {
	if err := w.checkManagePermission(sender); err != nil {
		return "", err
	}

	ctx := context.WithValue(w.ctx, helper.TransactionKey, transaction)
	resp, err := w.actions.UpdateKernel(ctx, flavour, modules, includeHeaders, simulate)
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
func (w *DBusWrapper) CleanOldKernels(sender dbus.Sender, noBackup bool, simulate bool, transaction string) (string, *dbus.Error) {
	if err := w.checkManagePermission(sender); err != nil {
		return "", err
	}

	ctx := context.WithValue(w.ctx, helper.TransactionKey, transaction)
	resp, err := w.actions.CleanOldKernels(ctx, noBackup, simulate)
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

// InstallKernelModules – Установить модули ядра
// doc_response: InstallKernelModulesResponse
func (w *DBusWrapper) InstallKernelModules(sender dbus.Sender, flavour string, modules []string, simulate bool, transaction string) (string, *dbus.Error) {
	if err := w.checkManagePermission(sender); err != nil {
		return "", err
	}

	ctx := context.WithValue(w.ctx, helper.TransactionKey, transaction)
	resp, err := w.actions.InstallKernelModules(ctx, flavour, modules, simulate)
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
func (w *DBusWrapper) RemoveKernelModules(sender dbus.Sender, flavour string, modules []string, simulate bool, transaction string) (string, *dbus.Error) {
	if err := w.checkManagePermission(sender); err != nil {
		return "", err
	}

	ctx := context.WithValue(w.ctx, helper.TransactionKey, transaction)
	resp, err := w.actions.RemoveKernelModules(ctx, flavour, modules, simulate)
	if err != nil {
		return "", dbus.MakeFailedError(err)
	}
	data, jerr := json.Marshal(resp)
	if jerr != nil {
		return "", dbus.MakeFailedError(jerr)
	}
	return string(data), nil
}
