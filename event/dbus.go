package event

import (
	"apm/logger"
	"github.com/godbus/dbus"
)

var DBUSConn *dbus.Conn

// InitDBus устанавливает соединение с сессионной шиной DBus.
func InitDBus() {
	var err error
	DBUSConn, err = dbus.SessionBus()
	if err != nil {
		logger.Log.Fatal("Ошибка подключения к DBus: %v", err)
	}
}
