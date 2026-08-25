package apmerr

import (
	"errors"

	"github.com/godbus/dbus/v5"
)

// DBusError создаёт типизированную DBus ошибку на основе APMError.
// Если ошибка не является APMError, возвращается стандартная dbus.Error.
func DBusError(err error) *dbus.Error {
	if apmErr, ok := errors.AsType[APMError](err); ok {
		return &dbus.Error{
			Name: apmErr.DBusErrorName(),
			Body: []any{err.Error()},
		}
	}
	return dbus.MakeFailedError(err)
}
