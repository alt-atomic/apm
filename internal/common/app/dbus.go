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

package app

import (
	"fmt"

	"github.com/godbus/dbus/v5"
)

// DBusManager управляет соединениями с DBus
type DBusManager interface {
	GetConnection() *dbus.Conn
	ConnectSystemBus() error
	ConnectSessionBus() error
	Close() error
	IsConnected() bool
}

// dbusManagerImpl реализация DBusManager
type dbusManagerImpl struct {
	conn      *dbus.Conn
	connected bool
}

// NewDBusManager создает новый менеджер DBus
func NewDBusManager() DBusManager {
	return &dbusManagerImpl{}
}

// GetConnection возвращает текущее соединение
func (dm *dbusManagerImpl) GetConnection() *dbus.Conn {
	return dm.conn
}

// ConnectSystemBus подключается к системной шине DBus
func (dm *dbusManagerImpl) ConnectSystemBus() error {
	return dm.connect(true)
}

// ConnectSessionBus подключается к пользовательской шине DBus
func (dm *dbusManagerImpl) ConnectSessionBus() error {
	return dm.connect(false)
}

// Connect внутренний метод для подключения
func (dm *dbusManagerImpl) connect(isSystem bool) error {
	var err error

	// Подключаемся к нужной шине
	if isSystem {
		dm.conn, err = dbus.ConnectSystemBus()
	} else {
		dm.conn, err = dbus.ConnectSessionBus()
	}
	if err != nil {
		return fmt.Errorf(T_("failed to connect to DBus: %w"), err)
	}

	// Регистрируем имя сервиса
	reply, err := dm.conn.RequestName("org.altlinux.APM", dbus.NameFlagDoNotQueue)
	if err != nil {
		_ = dm.conn.Close()
		dm.conn = nil
		return fmt.Errorf(T_("failed to request DBus name: %w"), err)
	}

	if reply != dbus.RequestNameReplyPrimaryOwner {
		_ = dm.conn.Close()
		dm.conn = nil
		return fmt.Errorf(T_("Interface org.altlinux.APM is already in use"))
	}

	dm.connected = true
	Log.Debug("DBus connection established")

	return nil
}

// Close закрывает соединение с DBus
func (dm *dbusManagerImpl) Close() error {
	if dm.conn != nil {
		err := dm.conn.Close()
		dm.conn = nil
		dm.connected = false
		if err != nil {
			return fmt.Errorf(T_("failed to close DBus connection: %w"), err)
		}
		Log.Debug("DBus connection closed")
	}
	return nil
}

// IsConnected проверяет, установлено ли соединение
func (dm *dbusManagerImpl) IsConnected() bool {
	return dm.connected && dm.conn != nil
}
