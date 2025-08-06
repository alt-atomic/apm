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
	"apm/internal/common/helper"
	"apm/internal/common/reply"
	"apm/lib"
	"context"
	"encoding/json"
	"fmt"

	"github.com/godbus/dbus/v5"
)

// DBusWrapper – обёртка для системных действий, предназначенная для экспорта через DBus.
type DBusWrapper struct {
	conn    *dbus.Conn
	actions *Actions
}

// NewDBusWrapper создаёт новую обёртку над actions
func NewDBusWrapper(a *Actions, c *dbus.Conn) *DBusWrapper {
	return &DBusWrapper{actions: a, conn: c}
}

// checkManagePermission проверяет права org.altlinux.APM.manage
func (w *DBusWrapper) checkManagePermission(sender dbus.Sender) *dbus.Error {
	if err := helper.PolkitCheck(w.conn, sender, "org.altlinux.APM.manage"); err != nil {
		return dbus.MakeFailedError(err)
	}
	return nil
}

// Install – обёртка над Actions.Install.
func (w *DBusWrapper) Install(sender dbus.Sender, packages []string, applyAtomic bool, transaction string) (string, *dbus.Error) {
	if err := w.checkManagePermission(sender); err != nil {
		return "", err
	}
	ctx := context.WithValue(context.Background(), "transaction", transaction)
	resp, err := w.actions.Install(ctx, packages, applyAtomic)
	if err != nil {
		return "", dbus.MakeFailedError(err)
	}
	data, jerr := json.Marshal(resp)
	if jerr != nil {
		return "", dbus.MakeFailedError(jerr)
	}
	return string(data), nil
}

// Remove – обёртка над Actions.Remove.
func (w *DBusWrapper) Remove(sender dbus.Sender, packages []string, applyAtomic bool, transaction string) (string, *dbus.Error) {
	if err := w.checkManagePermission(sender); err != nil {
		return "", err
	}
	ctx := context.WithValue(context.Background(), "transaction", transaction)
	resp, err := w.actions.Remove(ctx, packages, applyAtomic)
	if err != nil {
		return "", dbus.MakeFailedError(err)
	}
	data, jerr := json.Marshal(resp)
	if jerr != nil {
		return "", dbus.MakeFailedError(jerr)
	}
	return string(data), nil
}

// GetFilterFields обёртка над actions.GetFilterFields
func (w *DBusWrapper) GetFilterFields(transaction string) (string, *dbus.Error) {
	ctx := context.WithValue(context.Background(), "transaction", transaction)
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

// Update – обёртка над Actions.Update.
func (w *DBusWrapper) Update(sender dbus.Sender, transaction string) (string, *dbus.Error) {
	if err := w.checkManagePermission(sender); err != nil {
		return "", err
	}
	ctx := context.WithValue(context.Background(), "transaction", transaction)
	resp, err := w.actions.Update(ctx)
	if err != nil {
		return "", dbus.MakeFailedError(err)
	}
	data, jerr := json.Marshal(resp)
	if jerr != nil {
		return "", dbus.MakeFailedError(jerr)
	}
	return string(data), nil
}

// List – обёртка над Actions.List.
func (w *DBusWrapper) List(paramsJSON string, transaction string) (string, *dbus.Error) {
	ctx := context.WithValue(context.Background(), "transaction", transaction)
	params := ListParams{
		Limit:       10,
		Offset:      0,
		ForceUpdate: false,
	}
	if err := json.Unmarshal([]byte(paramsJSON), &params); err != nil {
		return "", dbus.MakeFailedError(fmt.Errorf(lib.T_("Failed to parse JSON: %w"), err))
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

// Info – обёртка над Actions.Info.
func (w *DBusWrapper) Info(packageName string, transaction string) (string, *dbus.Error) {
	ctx := context.WithValue(context.Background(), "transaction", transaction)
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

// UpdateKernel – обёртка над Actions.UpdateKernel.
func (w *DBusWrapper) UpdateKernel(sender dbus.Sender, transaction string) (string, *dbus.Error) {
	if err := w.checkManagePermission(sender); err != nil {
		return "", err
	}
	ctx := context.WithValue(context.Background(), "transaction", transaction)
	resp, err := w.actions.UpdateKernel(ctx)
	if err != nil {
		return "", dbus.MakeFailedError(err)
	}
	data, jerr := json.Marshal(resp)
	if jerr != nil {
		return "", dbus.MakeFailedError(jerr)
	}
	return string(data), nil
}

// CheckUpdateKernel – обёртка над Actions.CheckUpdateKernel.
func (w *DBusWrapper) CheckUpdateKernel(sender dbus.Sender, transaction string) (string, *dbus.Error) {
	if err := w.checkManagePermission(sender); err != nil {
		return "", err
	}
	ctx := context.WithValue(context.Background(), "transaction", transaction)
	resp, err := w.actions.CheckUpdateKernel(ctx)
	if err != nil {
		return "", dbus.MakeFailedError(err)
	}
	data, jerr := json.Marshal(resp)
	if jerr != nil {
		return "", dbus.MakeFailedError(jerr)
	}
	return string(data), nil
}

// CheckUpgrade – обёртка над Actions.CheckUpgrade.
func (w *DBusWrapper) CheckUpgrade(sender dbus.Sender, transaction string) (string, *dbus.Error) {
	if err := w.checkManagePermission(sender); err != nil {
		return "", err
	}
	ctx := context.WithValue(context.Background(), "transaction", transaction)
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

// Upgrade – обёртка над Actions.Upgrade или Actions.ImageUpdate.
func (w *DBusWrapper) Upgrade(sender dbus.Sender, transaction string) (string, *dbus.Error) {
	if err := w.checkManagePermission(sender); err != nil {
		return "", err
	}
	ctx := context.WithValue(context.Background(), "transaction", transaction)
	var resp *reply.APIResponse
	var err error
	if lib.Env.IsAtomic {
		resp, err = w.actions.ImageUpdate(ctx)
	} else {
		resp, err = w.actions.Upgrade(ctx)
	}
	if err != nil {
		return "", dbus.MakeFailedError(err)
	}
	data, jerr := json.Marshal(resp)
	if jerr != nil {
		return "", dbus.MakeFailedError(jerr)
	}
	return string(data), nil
}

// CheckInstall – обёртка над Actions.CheckInstall.
func (w *DBusWrapper) CheckInstall(sender dbus.Sender, packages []string, transaction string) (string, *dbus.Error) {
	if err := w.checkManagePermission(sender); err != nil {
		return "", err
	}
	ctx := context.WithValue(context.Background(), "transaction", transaction)
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

// CheckRemove – обёртка над Actions.CheckRemove.
func (w *DBusWrapper) CheckRemove(sender dbus.Sender, packages []string, transaction string) (string, *dbus.Error) {
	if err := w.checkManagePermission(sender); err != nil {
		return "", err
	}
	ctx := context.WithValue(context.Background(), "transaction", transaction)
	resp, err := w.actions.CheckRemove(ctx, packages)
	if err != nil {
		return "", dbus.MakeFailedError(err)
	}
	data, jerr := json.Marshal(resp)
	if jerr != nil {
		return "", dbus.MakeFailedError(jerr)
	}
	return string(data), nil
}

// Search – обёртка над Actions.Search.
func (w *DBusWrapper) Search(sender dbus.Sender, packageName string, transaction string, installed bool) (string, *dbus.Error) {
	if err := w.checkManagePermission(sender); err != nil {
		return "", err
	}
	ctx := context.WithValue(context.Background(), "transaction", transaction)
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

// ImageApply – обёртка над Actions.Apply.
func (w *DBusWrapper) ImageApply(sender dbus.Sender, transaction string) (string, *dbus.Error) {
	if err := w.checkManagePermission(sender); err != nil {
		return "", err
	}
	ctx := context.WithValue(context.Background(), "transaction", transaction)
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

// ImageHistory – обёртка над Actions.ImageHistory.
func (w *DBusWrapper) ImageHistory(sender dbus.Sender, transaction string, imageName string, limit int, offset int) (string, *dbus.Error) {
	if err := w.checkManagePermission(sender); err != nil {
		return "", err
	}
	ctx := context.WithValue(context.Background(), "transaction", transaction)
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

// ImageUpdate – обёртка над Actions.ImageUpdate.
func (w *DBusWrapper) ImageUpdate(sender dbus.Sender, transaction string) (string, *dbus.Error) {
	if err := w.checkManagePermission(sender); err != nil {
		return "", err
	}
	ctx := context.WithValue(context.Background(), "transaction", transaction)
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

// ImageStatus – обёртка над Actions.ImageStatus.
func (w *DBusWrapper) ImageStatus(sender dbus.Sender, transaction string) (string, *dbus.Error) {
	if err := w.checkManagePermission(sender); err != nil {
		return "", err
	}
	ctx := context.WithValue(context.Background(), "transaction", transaction)
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
