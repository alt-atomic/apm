package lib

import (
	"github.com/godbus/dbus"
)

var DBUSConn *dbus.Conn

// InitDBus устанавливает соединение с сессионной шиной DBus.
func InitDBus() {
	var err error
	DBUSConn, err = dbus.SessionBus()
	if err != nil {
		Log.Fatal("Ошибка подключения к DBus: %v", err)
	}
}
