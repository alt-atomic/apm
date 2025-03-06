package reply

import (
	"apm/lib"
	"encoding/json"
	"github.com/godbus/dbus"
	"runtime"
	"strings"
	"time"
)

type Notification struct {
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
		Event      string `json:"event"`
		EventName  string `json:"eventName"`
		EventState string `json:"eventState"`
	}

	taskName := parts[len(parts)-1]
	baseModel := Notification{Data: Model{Event: taskName, EventName: getTaskViewName(taskName), EventState: state}, Transaction: lib.Env.Transaction, Type: "event"}

	b, err := json.MarshalIndent(baseModel, "", "  ")
	if err != nil {
		lib.Log.Debug(err.Error())
	}
	UpdateTask(taskName, getTaskViewName(taskName), state)

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

func getTaskViewName(task string) string {
	switch task {
	case "os.SavePackagesToDB":
		return "Сохранение пакетов в базу"
	case "api.GetContainerList":
		return "Запрос списка контейнеров"
	case "api.ExportingApp":
		return "Экспорт пакета"
	case "api.GetContainerOsInfo":
		return "Запрос информации о контейнере"
	case "api.CreateContainer":
		return "Создание контейнера"
	case "api.RemoveContainer":
		return "Удаление контейнера"
	case "os.InstallPackage":
		return "Установка пакета"
	case "os.RemovePackage":
		return "Удаление пакета"
	case "os.GetPackages":
		return "Получение списка пакетов"
	case "os.GetPackageOwner":
		return "Определение владельца файла"
	case "os.GetPathByPackageName":
		return "Поиск путей пакета"
	case "os.GetInfoPackage":
		return "Получение информации о пакете"
	case "os.UpdatePackages":
		return "Обновление пакетов"
	case "os.GetPackagesQuery":
		return "Фильтрация пакетов"
	default:
		// Если имя задачи неизвестно, возвращаем его без изменений
		return task
	}
}
