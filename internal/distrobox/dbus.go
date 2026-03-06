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
	"apm/internal/common/apmerr"
	"apm/internal/common/app"
	"apm/internal/common/filter"
	"apm/internal/common/helper"
	"apm/internal/common/reply"
	"apm/internal/distrobox/service"
	"context"
	"encoding/json"

	"github.com/godbus/dbus/v5"
)

// DBusWrapper предоставляет обёртку для действий с контейнерами через DBus.
type DBusWrapper struct {
	actions *Actions
	ctx     context.Context
}

// NewDBusWrapper создаёт новую обёртку над actions.
func NewDBusWrapper(a *Actions, ctx context.Context) *DBusWrapper {
	return &DBusWrapper{actions: a, ctx: ctx}
}

// GetIconByPackage возвращает иконку приложения. Параметр container можно передать пустым.
func (w *DBusWrapper) GetIconByPackage(packageName string, container string) ([]byte, *dbus.Error) {
	ctx := context.WithValue(w.ctx, helper.TransactionKey, "")
	data, err := w.actions.GetIconByPackage(ctx, packageName, container)
	if err != nil {
		return nil, apmerr.DBusError(err)
	}

	return data, nil
}

// GetFilterFields возвращает список полей фильтрации для динамического построения фильтров в интерфейсе.
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

// Update обновляет пакеты.
func (w *DBusWrapper) Update(container string, transaction string, background bool) (string, *dbus.Error) {
	if transaction == "" {
		transaction = helper.GenerateTransactionID()
	}

	if background {
		ctx := context.WithValue(w.ctx, helper.TransactionKey, transaction)
		go func() {
			resp, err := w.actions.Update(ctx, container)
			reply.SendTaskResult(ctx, reply.EventDistroUpdate, resp, err)
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
	resp, err := w.actions.Update(ctx, container)
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
func (w *DBusWrapper) Info(container string, packageName string, transaction string) (string, *dbus.Error) {
	ctx := context.WithValue(w.ctx, helper.TransactionKey, transaction)
	resp, err := w.actions.Info(ctx, container, packageName)
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
func (w *DBusWrapper) Search(container string, packageName string, transaction string) (string, *dbus.Error) {
	ctx := context.WithValue(w.ctx, helper.TransactionKey, transaction)
	resp, err := w.actions.Search(ctx, container, packageName)
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
func (w *DBusWrapper) List(container string, sort string, order string, limit int, offset int, filtersJSON string, forceUpdate bool, transaction string) (string, *dbus.Error) {
	ctx := context.WithValue(w.ctx, helper.TransactionKey, transaction)
	if limit <= 0 {
		limit = 50
	}

	var filters []filter.Filter
	if filtersJSON != "" {
		if err := json.Unmarshal([]byte(filtersJSON), &filters); err != nil {
			return "", apmerr.DBusError(apmerr.New(apmerr.ErrorTypeValidation, err))
		}
	}

	validated, err := service.DistroFilterConfig.Validate(filters)
	if err != nil {
		return "", apmerr.DBusError(apmerr.New(apmerr.ErrorTypeValidation, err))
	}

	params := ListParams{
		Container:   container,
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

// Install устанавливает пакет.
func (w *DBusWrapper) Install(container string, packageName string, export bool, transaction string) (string, *dbus.Error) {
	ctx := context.WithValue(w.ctx, helper.TransactionKey, transaction)
	resp, err := w.actions.Install(ctx, container, packageName, export)
	if err != nil {
		return "", apmerr.DBusError(err)
	}
	data, jerr := json.Marshal(reply.OK(resp))
	if jerr != nil {
		return "", dbus.MakeFailedError(jerr)
	}
	return string(data), nil
}

// Remove удаляет пакет.
func (w *DBusWrapper) Remove(container string, packageName string, onlyExport bool, transaction string) (string, *dbus.Error) {
	ctx := context.WithValue(w.ctx, helper.TransactionKey, transaction)
	resp, err := w.actions.Remove(ctx, container, packageName, onlyExport)
	if err != nil {
		return "", apmerr.DBusError(err)
	}
	data, jerr := json.Marshal(reply.OK(resp))
	if jerr != nil {
		return "", dbus.MakeFailedError(jerr)
	}
	return string(data), nil
}

// ContainerList возвращает список контейнеров.
func (w *DBusWrapper) ContainerList(transaction string) (string, *dbus.Error) {
	ctx := context.WithValue(w.ctx, helper.TransactionKey, transaction)
	resp, err := w.actions.ContainerList(ctx)
	if err != nil {
		return "", apmerr.DBusError(err)
	}
	data, jerr := json.Marshal(reply.OK(resp))
	if jerr != nil {
		return "", dbus.MakeFailedError(jerr)
	}
	return string(data), nil
}

// ContainerAdd добавляет контейнер.
func (w *DBusWrapper) ContainerAdd(image, name, additionalPackages, initHooks string, transaction string, background bool) (string, *dbus.Error) {
	if transaction == "" {
		transaction = helper.GenerateTransactionID()
	}

	if background {
		ctx := context.WithValue(w.ctx, helper.TransactionKey, transaction)
		go func() {
			resp, err := w.actions.ContainerAdd(ctx, image, name, additionalPackages, initHooks)
			reply.SendTaskResult(ctx, reply.EventDistroContainerAdd, resp, err)
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
	resp, err := w.actions.ContainerAdd(ctx, image, name, additionalPackages, initHooks)
	if err != nil {
		return "", apmerr.DBusError(err)
	}
	data, jerr := json.Marshal(reply.OK(resp))
	if jerr != nil {
		return "", dbus.MakeFailedError(jerr)
	}
	return string(data), nil
}

// ContainerRemove удаляет контейнер.
func (w *DBusWrapper) ContainerRemove(name string, transaction string) (string, *dbus.Error) {
	ctx := context.WithValue(w.ctx, helper.TransactionKey, transaction)
	resp, err := w.actions.ContainerRemove(ctx, name)
	if err != nil {
		return "", apmerr.DBusError(err)
	}
	data, jerr := json.Marshal(reply.OK(resp))
	if jerr != nil {
		return "", dbus.MakeFailedError(jerr)
	}
	return string(data), nil
}
