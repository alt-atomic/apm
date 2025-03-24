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

package lib

import (
	"fmt"

	"github.com/godbus/dbus/v5"
)

// DBUSConn – глобальное соединение с DBus.
var DBUSConn *dbus.Conn

// InitDBus устанавливает соединение с сессионной шиной DBus
func InitDBus(isSystem bool) error {
	var err error
	if isSystem {
		DBUSConn, err = dbus.ConnectSystemBus()
	} else {
		DBUSConn, err = dbus.ConnectSessionBus()
	}
	if err != nil {
		return err
	}

	reply, err := DBUSConn.RequestName("com.application.APM", dbus.NameFlagDoNotQueue)
	if err != nil {
		return err
	}

	if reply != dbus.RequestNameReplyPrimaryOwner {
		return fmt.Errorf(T_("Interface com.application.APM is already in use"))
	}

	return nil
}
