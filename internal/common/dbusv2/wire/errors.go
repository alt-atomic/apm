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

package wire

import (
	"errors"

	"altlinux.space/alt-atomic/apm/internal/common/apmerr"

	"github.com/godbus/dbus/v5"
)

// accessDeniedError стандартное имя отказа в доступе.
const accessDeniedError = "org.freedesktop.DBus.Error.AccessDenied"

// Error конвертирует ошибку приложения в именованную DBus-ошибку.
func Error(err error) *dbus.Error {
	if err == nil {
		return nil
	}
	if apmErr, ok := errors.AsType[apmerr.APMError](err); ok && apmErr.Type == apmerr.ErrorTypePermission {
		return &dbus.Error{Name: accessDeniedError, Body: []any{err.Error()}}
	}
	return apmerr.DBusError(err)
}
