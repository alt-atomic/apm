package lib

import (
	"github.com/godbus/dbus/v5"
)

// DBUSConn – глобальное соединение с DBus.
var DBUSConn *dbus.Conn

// InitDBus устанавливает соединение с сессионной шиной DBus
// и регистрирует имя сервиса.
func InitDBus() {
	var err error
	DBUSConn, err = dbus.ConnectSessionBus()
	if err != nil {
		Log.Error("Ошибка подключения к DBus: %v", err)
		return
	}

	reply, err := DBUSConn.RequestName("com.application.APM", dbus.NameFlagDoNotQueue)
	if err != nil {
		Log.Error("Ошибка запроса имени сервиса: %v", err)
		return
	}

	if reply != dbus.RequestNameReplyPrimaryOwner {
		Log.Error("Имя сервиса уже занято!")
		return
	}
}
