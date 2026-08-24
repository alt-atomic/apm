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
	"altlinux.space/alt-atomic/apm/internal/common/dbusv2/protocol"

	"github.com/godbus/dbus/v5"
)

// errorNames маппинг типов apmerr на имена DBus-ошибок v2.
// PERMISSION намеренно отдаётся стандартным именем AccessDenied.
var errorNames = map[string]string{
	apmerr.ErrorTypeDatabase:    protocol.ErrorPrefix + "Database",
	apmerr.ErrorTypeRepository:  protocol.ErrorPrefix + "Repository",
	apmerr.ErrorTypeApt:         protocol.ErrorPrefix + "Apt",
	apmerr.ErrorTypeValidation:  protocol.ErrorPrefix + "Validation",
	apmerr.ErrorTypePermission:  "org.freedesktop.DBus.Error.AccessDenied",
	apmerr.ErrorTypeCanceled:    protocol.ErrorPrefix + "Canceled",
	apmerr.ErrorTypeImage:       protocol.ErrorPrefix + "Image",
	apmerr.ErrorTypeKernel:      protocol.ErrorPrefix + "Kernel",
	apmerr.ErrorTypeContainer:   protocol.ErrorPrefix + "Container",
	apmerr.ErrorTypeNoOperation: protocol.ErrorPrefix + "NoOperation",
	apmerr.ErrorTypeNotFound:    protocol.ErrorPrefix + "NotFound",
}

// Error конвертирует ошибку приложения в именованную DBus-ошибку.
func Error(err error) *dbus.Error {
	if err == nil {
		return nil
	}
	if apmErr, ok := errors.AsType[apmerr.APMError](err); ok {
		if name, ok := errorNames[apmErr.Type]; ok {
			return &dbus.Error{Name: name, Body: []any{err.Error()}}
		}
	}
	return &dbus.Error{Name: protocol.ErrorPrefix + "Failed", Body: []any{err.Error()}}
}

// Reply адаптирует пару (результат, ошибка) к возврату DBus-метода.
func Reply[T any](v T, err error) (T, *dbus.Error) {
	return v, Error(err)
}
