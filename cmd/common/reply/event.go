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
	Name            string  `json:"name"`
	View            string  `json:"message"`
	State           string  `json:"state"`
	Type            string  `json:"type"`
	ProgressPercent float64 `json:"progress"`
	Transaction     string  `json:"transaction,omitempty"`
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
func WithProgressPercent(percent float64) NotificationOption {
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
		ed.View = getTaskText(ed.Name)
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

	eventType := "PROGRESS"
	if eventData.Type != "PROGRESS" {
		eventType = "TASK"
	}

	UpdateTask(eventType, eventData.Name, eventData.View, eventData.State, eventData.ProgressPercent)
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

func getTaskText(task string) string {
	switch task {
	case "distro.SavePackagesToDB":
		return "Сохранение пакетов в базу"
	case "distro.GetContainerList":
		return "Запрос списка контейнеров"
	case "distro.ExportingApp":
		return "Экспорт пакета"
	case "distro.GetContainerOsInfo":
		return "Запрос информации о контейнере"
	case "distro.CreateContainer":
		return "Создание контейнера"
	case "distro.RemoveContainer":
		return "Удаление контейнера"
	case "distro.InstallPackage":
		return "Установка пакета"
	case "distro.RemovePackage":
		return "Удаление пакета"
	case "distro.GetPackages":
		return "Получение списка пакетов"
	case "distro.GetPackageOwner":
		return "Определение владельца файла"
	case "distro.GetPathByPackageName":
		return "Поиск путей пакета"
	case "distro.GetInfoPackage":
		return "Получение информации о пакете"
	case "distro.UpdatePackages":
		return "Обновление пакетов"
	case "distro.GetPackagesQuery":
		return "Фильтрация пакетов"
	case "system.Working":
		return "Работа над пакетами"
	case "system.Check":
		return "Анализ пакетов"
	case "system.Update":
		return "Общий процесс обновления"
	case "system.AptUpdate":
		return "Загрузка списка пакетов из репозитория ALT"
	case "system.SavePackagesToDB":
		return "Сохранение пакетов в базу"
	case "system.SaveImageToDB":
		return "Сохранение истории образа в базу данных"
	case "system.BuildImage":
		return "Сборка локального образа"
	case "system.SwitchImage":
		return "Переключение на локальный образ"
	case "system.CheckAndUpdateBaseImage":
		return "Проверка обновления"
	case "system.bootcUpgrade":
		return "Загрузка обновления базового образа"
	case "system.pruneOldImages":
		return "Очистка старых образов"
	case "system.updateAllPackagesDB":
		return "Синхронизация базы данных"
	default:
		// Если имя задачи неизвестно, возвращаем его без изменений
		return task
	}
}
