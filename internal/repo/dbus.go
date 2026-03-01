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

package repo

import (
	"apm/internal/common/apmerr"
	"apm/internal/common/helper"
	"apm/internal/common/reply"
	"context"
	"encoding/json"

	"github.com/godbus/dbus/v5"
)

// DBusWrapper – обёртка для действий с репозиториями, предназначенная для экспорта через DBus
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

// List – Получить список репозиториев
func (w *DBusWrapper) List(all bool, transaction string) (string, *dbus.Error) {
	ctx := context.WithValue(w.ctx, helper.TransactionKey, transaction)
	resp, err := w.actions.List(ctx, all)
	if err != nil {
		return "", apmerr.DBusError(err)
	}
	data, jerr := json.Marshal(reply.OK(resp))
	if jerr != nil {
		return "", dbus.MakeFailedError(jerr)
	}
	return string(data), nil
}

// Add – Добавить репозиторий
func (w *DBusWrapper) Add(sender dbus.Sender, source, date, transaction string) (string, *dbus.Error) {
	if err := w.checkManagePermission(sender); err != nil {
		return "", err
	}
	ctx := context.WithValue(w.ctx, helper.TransactionKey, transaction)
	// Для DBus source - это одна строка (формат sources.list или имя ветки/задачи)
	resp, err := w.actions.Add(ctx, []string{source}, date)
	if err != nil {
		return "", apmerr.DBusError(err)
	}
	data, jerr := json.Marshal(reply.OK(resp))
	if jerr != nil {
		return "", dbus.MakeFailedError(jerr)
	}
	return string(data), nil
}

// Remove – Удалить репозиторий
func (w *DBusWrapper) Remove(sender dbus.Sender, source, date, transaction string) (string, *dbus.Error) {
	if err := w.checkManagePermission(sender); err != nil {
		return "", err
	}
	ctx := context.WithValue(w.ctx, helper.TransactionKey, transaction)
	// Для DBus source - это одна строка (формат sources.list или имя ветки/задачи)
	resp, err := w.actions.Remove(ctx, []string{source}, date)
	if err != nil {
		return "", apmerr.DBusError(err)
	}
	data, jerr := json.Marshal(reply.OK(resp))
	if jerr != nil {
		return "", dbus.MakeFailedError(jerr)
	}
	return string(data), nil
}

// Set – Установить ветку (удалить все и добавить указанную)
func (w *DBusWrapper) Set(sender dbus.Sender, branch, date, transaction string) (string, *dbus.Error) {
	if err := w.checkManagePermission(sender); err != nil {
		return "", err
	}
	ctx := context.WithValue(w.ctx, helper.TransactionKey, transaction)
	resp, err := w.actions.Set(ctx, branch, date)
	if err != nil {
		return "", apmerr.DBusError(err)
	}
	data, jerr := json.Marshal(reply.OK(resp))
	if jerr != nil {
		return "", dbus.MakeFailedError(jerr)
	}
	return string(data), nil
}

// Clean – Удалить cdrom и task репозитории
func (w *DBusWrapper) Clean(sender dbus.Sender, transaction string) (string, *dbus.Error) {
	if err := w.checkManagePermission(sender); err != nil {
		return "", err
	}
	ctx := context.WithValue(w.ctx, helper.TransactionKey, transaction)
	resp, err := w.actions.Clean(ctx)
	if err != nil {
		return "", apmerr.DBusError(err)
	}
	data, jerr := json.Marshal(reply.OK(resp))
	if jerr != nil {
		return "", dbus.MakeFailedError(jerr)
	}
	return string(data), nil
}

// GetBranches – Получить список доступных веток
func (w *DBusWrapper) GetBranches() (string, *dbus.Error) {
	resp, err := w.actions.GetBranches(w.ctx)
	if err != nil {
		return "", apmerr.DBusError(err)
	}
	data, jerr := json.Marshal(reply.OK(resp))
	if jerr != nil {
		return "", dbus.MakeFailedError(jerr)
	}
	return string(data), nil
}

// GetTaskPackages – Получить список пакетов из задачи
func (w *DBusWrapper) GetTaskPackages(taskNum string, transaction string) (string, *dbus.Error) {
	ctx := context.WithValue(w.ctx, helper.TransactionKey, transaction)
	resp, err := w.actions.GetTaskPackages(ctx, taskNum)
	if err != nil {
		return "", apmerr.DBusError(err)
	}
	data, jerr := json.Marshal(reply.OK(resp))
	if jerr != nil {
		return "", dbus.MakeFailedError(jerr)
	}
	return string(data), nil
}

// CheckAdd – Симулировать добавление репозитория
func (w *DBusWrapper) CheckAdd(source, date, transaction string) (string, *dbus.Error) {
	ctx := context.WithValue(w.ctx, helper.TransactionKey, transaction)
	// Для DBus source - это одна строка (формат sources.list или имя ветки/задачи)
	resp, err := w.actions.CheckAdd(ctx, []string{source}, date)
	if err != nil {
		return "", apmerr.DBusError(err)
	}
	data, jerr := json.Marshal(reply.OK(resp))
	if jerr != nil {
		return "", dbus.MakeFailedError(jerr)
	}
	return string(data), nil
}

// CheckRemove – Симулировать удаление репозитория
func (w *DBusWrapper) CheckRemove(source, date, transaction string) (string, *dbus.Error) {
	ctx := context.WithValue(w.ctx, helper.TransactionKey, transaction)
	// Для DBus source - это одна строка (формат sources.list или имя ветки/задачи)
	resp, err := w.actions.CheckRemove(ctx, []string{source}, date)
	if err != nil {
		return "", apmerr.DBusError(err)
	}
	data, jerr := json.Marshal(reply.OK(resp))
	if jerr != nil {
		return "", dbus.MakeFailedError(jerr)
	}
	return string(data), nil
}

// CheckSet – Симулировать установку ветки
func (w *DBusWrapper) CheckSet(branch, date, transaction string) (string, *dbus.Error) {
	ctx := context.WithValue(w.ctx, helper.TransactionKey, transaction)
	resp, err := w.actions.CheckSet(ctx, branch, date)
	if err != nil {
		return "", apmerr.DBusError(err)
	}
	data, jerr := json.Marshal(reply.OK(resp))
	if jerr != nil {
		return "", dbus.MakeFailedError(jerr)
	}
	return string(data), nil
}

// CheckClean – Симулировать очистку cdrom и task репозиториев
func (w *DBusWrapper) CheckClean(transaction string) (string, *dbus.Error) {
	ctx := context.WithValue(w.ctx, helper.TransactionKey, transaction)
	resp, err := w.actions.CheckClean(ctx)
	if err != nil {
		return "", apmerr.DBusError(err)
	}
	data, jerr := json.Marshal(reply.OK(resp))
	if jerr != nil {
		return "", dbus.MakeFailedError(jerr)
	}
	return string(data), nil
}
