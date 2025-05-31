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

package reply

import (
	"apm/lib"
	"context"
	"encoding/json"
	"errors"
	"runtime"
	"strings"

	"github.com/godbus/dbus/v5"
)

// EventData содержит данные события.
type EventData struct {
	Name            string  `json:"name"`
	View            string  `json:"message"`
	State           string  `json:"state"`
	Type            string  `json:"type"`
	ProgressPercent float64 `json:"progress"`
	ProgressDone    string  `json:"progressDone"`
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

// WithProgressDoneText задаёт текст в конце прогресса.
func WithProgressDoneText(text string) NotificationOption {
	return func(ed *EventData) {
		ed.ProgressDone = text
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
			errText := lib.T_("Failed to retrieve call information")
			lib.Log.Error(errors.New(errText))
			return
		}
		fn := runtime.FuncForPC(pc)
		if fn == nil {
			errText := lib.T_("FuncForPC returned nil")
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

	UpdateTask(eventType, eventData.Name, eventData.View, eventData.State, eventData.ProgressPercent, eventData.ProgressDone)
	SendNotificationResponse(string(b))
}

// SendNotificationResponse отправляет ответы через DBus.
func SendNotificationResponse(message string) {
	if lib.Env.Format != "dbus" {
		return
	}

	if lib.DBUSConn == nil {
		lib.Log.Error(lib.T_("DBus connection is not initialized"))
		return
	}

	objPath := dbus.ObjectPath("/org/altlinux/APM")
	signalName := "org.altlinux.APM.Notification"

	err := lib.DBUSConn.Emit(objPath, signalName, message)
	if err != nil {
		lib.Log.Error(lib.T_("Error sending notification: %v"), err)
	}
}

func getTaskText(task string) string {
	switch task {
	case "distro.SavePackagesToDB":
		return lib.T_("Saving packages to the database")
	case "distro.GetContainerList":
		return lib.T_("Requesting list of containers")
	case "distro.ExportingApp":
		return lib.T_("Exporting package")
	case "distro.GetContainerOsInfo":
		return lib.T_("Requesting container information")
	case "distro.CreateContainer":
		return lib.T_("Creating container")
	case "distro.RemoveContainer":
		return lib.T_("Deleting container")
	case "distro.InstallPackage":
		return lib.T_("Installing package")
	case "distro.RemovePackage":
		return lib.T_("Removing package")
	case "distro.GetPackages":
		return lib.T_("Retrieving list of packages")
	case "distro.GetPackageOwner":
		return lib.T_("Determining file owner")
	case "distro.GetPathByPackageName":
		return lib.T_("Searching package paths")
	case "distro.GetInfoPackage":
		return lib.T_("Retrieving package information")
	case "distro.UpdatePackages":
		return lib.T_("Updating packages")
	case "distro.GetPackagesQuery":
		return lib.T_("Filtering packages")
	case "system.Working":
		return lib.T_("Working with packages")
	case "system.Upgrade":
		return lib.T_("System update")
	case "system.Check":
		return lib.T_("Analyzing packages")
	case "system.Update":
		return lib.T_("General update process")
	case "system.UpdateALR":
		return lib.T_("Loading package list from ALR repository")
	case "system.AptUpdate":
		return lib.T_("Loading package list from ALT repository")
	case "system.SavePackagesToDB":
		return lib.T_("Saving packages to the database")
	case "system.SaveImageToDB":
		return lib.T_("Saving image history to the database")
	case "system.BuildImage":
		return lib.T_("Building local image")
	case "system.SwitchImage":
		return lib.T_("Switching to local image")
	case "system.CheckAndUpdateBaseImage":
		return lib.T_("General Image Update Process")
	case "system.bootcUpgrade":
		return lib.T_("Downloading base image update")
	case "system.pruneOldImages":
		return lib.T_("Cleaning up old images")
	case "system.updateAllPackagesDB":
		return lib.T_("Synchronizing database")
	default:
		// If the task name is unknown, we return it unchanged.
		return task
	}
}
