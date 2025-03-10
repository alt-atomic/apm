package reply

import (
	"apm/lib"
	"context"
	"encoding/json"
	"errors"
	"github.com/godbus/dbus/v5"
	"runtime"
	"strings"
)

// EventData содержит данные события.
type EventData struct {
	Name            string `json:"name"`
	View            string `json:"message"`
	State           string `json:"state"`
	Type            string `json:"type"`
	ProgressPercent int    `json:"progress"`
	Transaction     string `json:"transaction,omitempty"`
}

var (
	EventTypeNotification = "NOTIFICATION"
	EventTypeProgress     = "PROGRESS"

	StateBefore = "BEFORE"
	StateAfter  = "AFTER"
)

// NotificationOption — функция-опция для настройки EventData.
type NotificationOption func(*EventData)

// WithEventName задаёт имя события.
func WithEventName(name string) NotificationOption {
	return func(ed *EventData) {
		ed.Name = name
	}
}

// WithEventView задаёт текст отображения события
func WithEventView(name string) NotificationOption {
	return func(ed *EventData) {
		ed.View = name
	}
}

// WithProgress указывает, что событие является прогрессом.
func WithProgress(isProgress bool) NotificationOption {
	return func(ed *EventData) {
		if isProgress {
			ed.Type = EventTypeProgress
		} else {
			ed.Type = EventTypeNotification
		}
	}
}

// WithProgressPercent задаёт процент выполнения.
func WithProgressPercent(percent int) NotificationOption {
	return func(ed *EventData) {
		ed.ProgressPercent = percent
	}
}

// CreateEventNotification создаёт EventData, используя заданное состояние и опции.
func CreateEventNotification(ctx context.Context, state string, opts ...NotificationOption) {
	// Устанавливаем значения по умолчанию.
	ed := EventData{
		Name:            "",
		State:           state,
		Type:            EventTypeNotification,
		ProgressPercent: 0,
	}

	// Применяем переданные опции.
	for _, opt := range opts {
		opt(&ed)
	}

	// Если имя события не задано, определяем его через runtime
	if ed.Name == "" {
		pc, _, _, ok := runtime.Caller(1)
		if !ok {
			errText := "не удалось получить информацию о вызове"
			lib.Log.Error(errors.New(errText))
			return
		}
		fn := runtime.FuncForPC(pc)
		if fn == nil {
			errText := "FuncForPC вернул nil"
			lib.Log.Error(errors.New(errText))
			return
		}
		fullName := fn.Name()
		parts := strings.Split(fullName, "/")
		ed.Name = parts[len(parts)-1]
	}

	if ed.View == "" {
		ed.View = getTaskViewName(ed.Name)
	}

	SendFuncNameDBUS(ctx, ed)
}

// SendFuncNameDBUS отправляет уведомление через DBUS.
func SendFuncNameDBUS(ctx context.Context, eventData EventData) {
	txVal := ctx.Value("transaction")
	txStr, ok := txVal.(string)
	if ok {
		eventData.Transaction = txStr
	}

	b, err := json.MarshalIndent(eventData, "", "  ")
	if err != nil {
		lib.Log.Debug(err.Error())
	}
	UpdateTask(eventData.Name, eventData.View, eventData.State)

	SendNotificationResponse(string(b))
}

// SendNotificationResponse отправляет ответы через DBus.
func SendNotificationResponse(message string) {
	if lib.Env.Format != "dbus" {
		return
	}

	if lib.DBUSConn == nil {
		lib.Log.Error("Соединение DBus не инициализировано")
		return
	}

	objPath := dbus.ObjectPath("/com/application/APM")
	signalName := "com.application.APM.Notification"

	err := lib.DBUSConn.Emit(objPath, signalName, message)
	if err != nil {
		lib.Log.Error("Ошибка отправки уведомления: %v", err)
	}
}

func getTaskViewName(task string) string {
	switch task {
	case "apt.(*Actions).Install":
		return "Установка пакетов"
	case "apt.(*Actions).Check":
		return "Анализ пакетов"
	case "apt.(*Actions).Update":
		return "Анализ полученных пакетов"
	case "apt.SavePackagesToDB":
		return "Сохранение пакетов в базу"
	case "apt.aptUpdate":
		return "Загружается список пакетов"
	case "service.CheckAndUpdateBaseImage":
		return "Проверка обновления"
	case "service.bootcUpgrade":
		return "Загрузка обновления базового образа"
	case "service.SwitchImage":
		return "Переключение на локальный образ"
	case "service.BuildImage":
		return "Сборка локального образа"
	case "service.SavePackagesToDB":
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
	case "service.InstallPackage":
		return "Установка пакета"
	case "service.RemovePackage":
		return "Удаление пакета"
	case "service.GetPackages":
		return "Получение списка пакетов"
	case "service.GetPackageOwner":
		return "Определение владельца файла"
	case "service.GetPathByPackageName":
		return "Поиск путей пакета"
	case "service.GetInfoPackage":
		return "Получение информации о пакете"
	case "service.UpdatePackages":
		return "Обновление пакетов"
	case "service.GetPackagesQuery":
		return "Фильтрация пакетов"
	default:
		// Если имя задачи неизвестно, возвращаем его без изменений
		return task
	}
}
