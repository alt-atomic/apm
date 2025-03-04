package dbus

import (
	"apm/logger"
	"github.com/godbus/dbus"
	"time"
)

var Conn *dbus.Conn

// InitDBus устанавливает соединение с сессионной шиной DBus.
func InitDBus() {
	var err error
	Conn, err = dbus.SessionBus()
	if err != nil {
		logger.Log.Fatal("Ошибка подключения к DBus: %v", err)
	}
}

// SendNotificationResponse отправляет ответы через DBus.
func SendNotificationResponse(message string) {
	if Conn == nil {
		logger.Log.Debug("Соединение DBus не инициализировано")
		return
	}
	// Объектный путь, по которому отправляются сигналы
	objPath := dbus.ObjectPath("/com/application/APM")
	signalName := "com.application.APM.Notification"

	err := Conn.Emit(objPath, signalName, message)
	if err != nil {
		logger.Log.Debugf("Ошибка отправки уведомления: %v", err)
		return
	}

	time.Sleep(200 * time.Millisecond)
}
