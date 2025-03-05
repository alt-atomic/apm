package dbus

import (
	"apm/lib"
	"encoding/json"
	"github.com/godbus/dbus"
	"runtime"
	"strings"
	"time"
)

type DBUSNotification struct {
	Data        interface{} `json:"data"`
	Transaction string      `json:"transaction,omitempty"`
	Type        string      `json:"type,omitempty"`
}

var (
	STATE_BEFORE = "BEFORE"
	STATE_AFTER  = "AFTER"
)

// SendFuncNameDBUS отправляет название функции в DBUS для отслеживания состояния
func SendFuncNameDBUS(state string) {
	if lib.Env.Format != "dbus" {
		return
	}

	pc, _, _, ok := runtime.Caller(1)
	if !ok {
		return
	}
	fn := runtime.FuncForPC(pc)
	if fn == nil {
		return
	}
	fullName := fn.Name()

	parts := strings.Split(fullName, "/")

	type Model struct {
		EventName string `json:"event_name"`
		EventType string `json:"event_type"`
	}

	baseModel := DBUSNotification{Data: Model{EventName: parts[len(parts)-1], EventType: state}, Transaction: lib.Env.Transaction, Type: "event"}

	b, err := json.MarshalIndent(baseModel, "", "  ")
	if err != nil {
		lib.Log.Debug(err.Error())
	}

	SendNotificationResponse(string(b))
}

// SendNotificationResponse отправляет ответы через DBus.
func SendNotificationResponse(message string) {
	if lib.Env.Format != "dbus" {
		return
	}

	if lib.DBUSConn == nil {
		lib.Log.Debug("Соединение DBus не инициализировано")
		time.Sleep(100 * time.Millisecond)

		if lib.DBUSConn == nil {
			return
		}
	}

	// Объектный путь, по которому отправляются сигналы
	objPath := dbus.ObjectPath("/com/application/APM")
	signalName := "com.application.APM.Notification"

	err := lib.DBUSConn.Emit(objPath, signalName, message)
	if err != nil {
		lib.Log.Debugf("Ошибка отправки уведомления: %v", err)
	}
}
