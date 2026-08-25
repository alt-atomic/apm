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

// Package authz — авторизация вызовов API v2 через polkit.
package authz

import (
	"altlinux.space/alt-atomic/apm/internal/common/apmerr"
	"altlinux.space/alt-atomic/apm/internal/common/polkit"

	"github.com/godbus/dbus/v5"
)

// Authorizer проверяет право отправителя на действие.
type Authorizer interface {
	Authorize(msg dbus.Message, action string) error
}

// Func адаптирует функцию к Authorizer.
type Func func(msg dbus.Message, action string) error

func (f Func) Authorize(msg dbus.Message, action string) error {
	return f(msg, action)
}

// AllowAll пропускает всех: для сессионной шины.
var AllowAll Authorizer = Func(func(dbus.Message, string) error { return nil })

// Polkit возвращает Authorizer поверх polkit для системной шины.
func Polkit(conn *dbus.Conn) Authorizer {
	return Func(func(msg dbus.Message, action string) error {
		if err := polkit.Check(conn, msg, action); err != nil {
			return apmerr.New(apmerr.ErrorTypePermission, err)
		}
		return nil
	})
}

// Guard выполняет fn только после успешной авторизации.
func Guard(a Authorizer, msg dbus.Message, action string, fn func() error) error {
	if err := a.Authorize(msg, action); err != nil {
		return err
	}
	return fn()
}

// Authorized выполняет fn с результатом только после успешной авторизации.
func Authorized[T any](a Authorizer, msg dbus.Message, action string, fn func() (T, error)) (T, error) {
	if err := a.Authorize(msg, action); err != nil {
		var zero T
		return zero, err
	}
	return fn()
}
