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

package distrobox

import (
	"apm/cmd/common/icon"
	"apm/lib"
	"context"
	"encoding/json"
	"fmt"

	"github.com/godbus/dbus/v5"
)

// DBusWrapper – обёртка для системных действий, предназначенная для экспорта через DBus.
type DBusWrapper struct {
	actions     *Actions
	iconService *icon.Service
}

// NewDBusWrapper создаёт новую обёртку над actions
func NewDBusWrapper(a *Actions, i *icon.Service) *DBusWrapper {
	return &DBusWrapper{actions: a, iconService: i}
}

// GetIconByPackage обёртка над actions.GetFilterFields
func (w *DBusWrapper) GetIconByPackage(packageName string, container string) ([]byte, *dbus.Error) {
	bytes, err := w.iconService.GetIcon(packageName, container)
	if err != nil {
		return nil, dbus.MakeFailedError(err)
	}

	return bytes, nil
}

// GetFilterFields обёртка над actions.GetFilterFields
func (w *DBusWrapper) GetFilterFields(container string, transaction string) (string, *dbus.Error) {
	ctx := context.WithValue(context.Background(), "transaction", transaction)
	resp, err := w.actions.GetFilterFields(ctx, container)
	if err != nil {
		return "", dbus.MakeFailedError(err)
	}

	data, jerr := json.Marshal(resp)
	if jerr != nil {
		return "", dbus.MakeFailedError(jerr)
	}

	return string(data), nil
}

// Update обёртка над actions.Update
func (w *DBusWrapper) Update(container string, transaction string) (string, *dbus.Error) {
	ctx := context.WithValue(context.Background(), "transaction", transaction)
	resp, err := w.actions.Update(ctx, container)
	if err != nil {
		return "", dbus.MakeFailedError(err)
	}
	data, jerr := json.Marshal(resp)
	if jerr != nil {
		return "", dbus.MakeFailedError(jerr)
	}
	return string(data), nil
}

// Info обёртка над actions.Info
func (w *DBusWrapper) Info(container string, packageName string, transaction string) (string, *dbus.Error) {
	ctx := context.WithValue(context.Background(), "transaction", transaction)
	resp, err := w.actions.Info(ctx, container, packageName)
	if err != nil {
		return "", dbus.MakeFailedError(err)
	}
	data, jerr := json.Marshal(resp)
	if jerr != nil {
		return "", dbus.MakeFailedError(jerr)
	}
	return string(data), nil
}

// Search обёртка над actions.Search
func (w *DBusWrapper) Search(container string, packageName string, transaction string) (string, *dbus.Error) {
	ctx := context.WithValue(context.Background(), "transaction", transaction)
	resp, err := w.actions.Search(ctx, container, packageName)
	if err != nil {
		return "", dbus.MakeFailedError(err)
	}
	data, jerr := json.Marshal(resp)
	if jerr != nil {
		return "", dbus.MakeFailedError(jerr)
	}
	return string(data), nil
}

// List принимает JSON‑строку с параметрами ListParams, а возвращает JSON с reply.APIResponse.
func (w *DBusWrapper) List(paramsJSON string, transaction string) (string, *dbus.Error) {
	ctx := context.WithValue(context.Background(), "transaction", transaction)
	var params ListParams
	if err := json.Unmarshal([]byte(paramsJSON), &params); err != nil {
		return "", dbus.MakeFailedError(fmt.Errorf(lib.T_("Failed to parse JSON: %w"), err))
	}

	resp, err := w.actions.List(ctx, params)
	if err != nil {
		return "", dbus.MakeFailedError(err)
	}
	data, jerr := json.Marshal(resp)
	if jerr != nil {
		return "", dbus.MakeFailedError(jerr)
	}
	return string(data), nil
}

// Install обёртка над actions.Install
func (w *DBusWrapper) Install(container string, packageName string, export bool, transaction string) (string, *dbus.Error) {
	ctx := context.WithValue(context.Background(), "transaction", transaction)
	resp, err := w.actions.Install(ctx, container, packageName, export)
	if err != nil {
		return "", dbus.MakeFailedError(err)
	}
	data, jerr := json.Marshal(resp)
	if jerr != nil {
		return "", dbus.MakeFailedError(jerr)
	}
	return string(data), nil
}

// Remove обёртка над actions.Remove
func (w *DBusWrapper) Remove(container string, packageName string, onlyExport bool, transaction string) (string, *dbus.Error) {
	ctx := context.WithValue(context.Background(), "transaction", transaction)
	resp, err := w.actions.Remove(ctx, container, packageName, onlyExport)
	if err != nil {
		return "", dbus.MakeFailedError(err)
	}
	data, jerr := json.Marshal(resp)
	if jerr != nil {
		return "", dbus.MakeFailedError(jerr)
	}
	return string(data), nil
}

// ContainerList обёртка над actions.ContainerList
func (w *DBusWrapper) ContainerList(transaction string) (string, *dbus.Error) {
	ctx := context.WithValue(context.Background(), "transaction", transaction)
	resp, err := w.actions.ContainerList(ctx)
	if err != nil {
		return "", dbus.MakeFailedError(err)
	}
	data, jerr := json.Marshal(resp)
	if jerr != nil {
		return "", dbus.MakeFailedError(jerr)
	}
	return string(data), nil
}

// ContainerAdd обёртка над actions.ContainerAdd
func (w *DBusWrapper) ContainerAdd(image, name, additionalPackages, initHooks string, transaction string) (string, *dbus.Error) {
	ctx := context.WithValue(context.Background(), "transaction", transaction)
	resp, err := w.actions.ContainerAdd(ctx, image, name, additionalPackages, initHooks)
	if err != nil {
		return "", dbus.MakeFailedError(err)
	}
	data, jerr := json.Marshal(resp)
	if jerr != nil {
		return "", dbus.MakeFailedError(jerr)
	}
	return string(data), nil
}

// ContainerRemove обёртка над actions.ContainerRemove
func (w *DBusWrapper) ContainerRemove(name string, transaction string) (string, *dbus.Error) {
	ctx := context.WithValue(context.Background(), "transaction", transaction)
	resp, err := w.actions.ContainerRemove(ctx, name)
	if err != nil {
		return "", dbus.MakeFailedError(err)
	}
	data, jerr := json.Marshal(resp)
	if jerr != nil {
		return "", dbus.MakeFailedError(jerr)
	}
	return string(data), nil
}
