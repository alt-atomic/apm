package apmerr

import (
	"errors"

	"github.com/godbus/dbus/v5"
)

// DBusError создаёт типизированную DBus ошибку на основе APMError.
// Если ошибка не является APMError, возвращается стандартная dbus.Error.
func DBusError(err error) *dbus.Error {
	var apmErr APMError
	if errors.As(err, &apmErr) {
		return &dbus.Error{
			Name: apmErr.DBusErrorName(),
			Body: []any{err.Error()},
		}
	}
	return dbus.MakeFailedError(err)
}
