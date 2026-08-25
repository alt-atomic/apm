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

package polkit

import (
	"errors"
	"fmt"

	"altlinux.space/alt-atomic/apm/internal/common/app"

	"github.com/godbus/dbus/v5"
)

// allowUserInteraction разрешает polkit показать диалог аутентификации.
const allowUserInteraction = uint32(1)

// Sender возвращает уникальное имя отправителя сообщения.
func Sender(msg dbus.Message) string {
	name, _ := msg.Headers[dbus.FieldSender].Value().(string)
	return name
}

// Check выполняет проверку доступа через Polkit.
// Диалог аутентификации поднимается, только если клиент разрешил интерактив.
func Check(conn *dbus.Conn, msg dbus.Message, actionID string) error {
	sender := Sender(msg)
	uid, err := senderUID(conn, sender)
	if err != nil {
		return err
	}
	if uid == 0 {
		return nil
	}

	var flags uint32
	if msg.Flags&dbus.FlagAllowInteractiveAuthorization != 0 {
		flags = allowUserInteraction
	}

	subject := struct {
		Kind    string
		Details map[string]dbus.Variant
	}{
		Kind:    "system-bus-name",
		Details: map[string]dbus.Variant{"name": dbus.MakeVariant(sender)},
	}

	var reply struct {
		Granted   bool
		Challenge bool
		Details   map[string]string
	}
	call := conn.Object("org.freedesktop.PolicyKit1", "/org/freedesktop/PolicyKit1/Authority").Call(
		"org.freedesktop.PolicyKit1.Authority.CheckAuthorization", 0,
		subject, actionID, map[string]string{}, flags, "",
	)
	if call.Err != nil {
		return fmt.Errorf(app.T_("polkit dbus failure: %w"), call.Err)
	}
	if err = call.Store(&reply); err != nil {
		return fmt.Errorf(app.T_("polkit unpack failure: %w"), err)
	}
	if !reply.Granted {
		if reply.Challenge && flags == 0 {
			return fmt.Errorf(app.T_("authentication required for action %s: repeat the call with interactive authorization allowed"), actionID)
		}
		return fmt.Errorf(app.T_("not authorized by polkit (action=%s)"), actionID)
	}

	return nil
}

// senderUID запрашивает у шины uid владельца соединения.
func senderUID(conn *dbus.Conn, sender string) (uint32, error) {
	var creds map[string]dbus.Variant
	err := conn.BusObject().
		Call("org.freedesktop.DBus.GetConnectionCredentials", 0, sender).
		Store(&creds)
	if err != nil {
		return 0, fmt.Errorf(app.T_("Failed to get sender credentials: %w"), err)
	}

	uid, ok := creds["UnixUserID"].Value().(uint32)
	if !ok {
		return 0, errors.New(app.T_("Sender credentials have no UnixUserID"))
	}

	return uid, nil
}
