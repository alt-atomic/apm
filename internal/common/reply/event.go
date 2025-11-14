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
	"apm/internal/common/app"
	"apm/internal/common/helper"
	"context"
	"encoding/json"

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

	// Если имя события не задано
	if ed.Name == "" {
		ed.Name = "unknown"
	}

	if ed.View == "" {
		ed.View = getTaskText(ed.Name)
	}

	SendFuncNameDBUS(ctx, &ed)
}

// SendFuncNameDBUS отправляет уведомление через DBUS.
func SendFuncNameDBUS(ctx context.Context, eventData *EventData) {
	appConfig := app.GetAppConfig(ctx)
	txVal := ctx.Value(helper.TransactionKey)
	txStr, ok := txVal.(string)
	if ok {
		eventData.Transaction = txStr
	}

	eventType := "PROGRESS"
	if eventData.Type != "PROGRESS" {
		eventType = "TASK"
	}

	UpdateTask(appConfig, eventType, eventData.Name, eventData.View, eventData.State, eventData.ProgressPercent, eventData.ProgressDone)
	if appConfig.ConfigManager.GetConfig().Format != "dbus" {
		return
	}

	SendNotificationResponse(eventData, appConfig.DBusManager.GetConnection())
}

// SendNotificationResponse отправляет ответы через DBus.
func SendNotificationResponse(eventData *EventData, dbusConn *dbus.Conn) {
	message, err := json.MarshalIndent(eventData, "", "  ")
	if err != nil {
		app.Log.Debug(err.Error())
	}

	if dbusConn == nil {
		app.Log.Error(app.T_("DBus connection is not initialized"))
		return
	}

	objPath := dbus.ObjectPath("/org/altlinux/APM")
	signalName := "org.altlinux.APM.Notification"

	err = dbusConn.Emit(objPath, signalName, string(message))
	if err != nil {
		app.Log.Error(app.T_("Error sending notification: %v"), err)
	}
}

func getTaskText(task string) string {
	switch task {
	case "distro.SavePackagesToDB":
		return app.T_("Saving packages to the database")
	case "distro.GetContainerList":
		return app.T_("Requesting list of containers")
	case "distro.ExportingApp":
		return app.T_("Exporting package")
	case "distro.GetContainerOsInfo":
		return app.T_("Requesting container information")
	case "distro.CreateContainer":
		return app.T_("Creating container")
	case "distro.RemoveContainer":
		return app.T_("Deleting container")
	case "distro.InstallPackage":
		return app.T_("Installing package")
	case "distro.RemovePackage":
		return app.T_("Removing package")
	case "distro.GetPackages":
		return app.T_("Retrieving list of packages")
	case "distro.GetPackageOwner":
		return app.T_("Determining file owner")
	case "distro.GetPathByPackageName":
		return app.T_("Searching package paths")
	case "distro.GetInfoPackage":
		return app.T_("Retrieving package information")
	case "distro.UpdatePackages":
		return app.T_("Updating packages")
	case "distro.GetPackagesQuery":
		return app.T_("Filtering packages")
	case "system.Working":
		return app.T_("Working with packages")
	case "system.Upgrade":
		return app.T_("System update")
	case "system.Check":
		return app.T_("Analyzing packages")
	case "system.Update":
		return app.T_("General update process")
	case "system.UpdateKernel":
		return app.T_("General update kernel")
	case "system.UpdateSTPLR":
		return app.T_("Loading package list from STPLR repository")
	case "system.AptUpdate":
		return app.T_("Loading package list from ALT repository")
	case "system.SavePackagesToDB":
		return app.T_("Saving packages to the database")
	case "system.SaveImageToDB":
		return app.T_("Saving image history to the database")
	case "system.BuildImage":
		return app.T_("Building local image")
	case "system.SwitchImage":
		return app.T_("Switching to local image")
	case "system.CheckAndUpdateBaseImage":
		return app.T_("General Image Update Process")
	case "system.bootcUpgrade":
		return app.T_("Downloading base image update")
	case "system.pruneOldImages":
		return app.T_("Cleaning up old images")
	case "system.updateAllPackagesDB":
		return app.T_("Synchronizing database")
	case "system.UpdateAppStream":
		return app.T_("Update information about applications")
	case "kernel.CurrentKernel":
		return app.T_("Get current kernel")
	case "kernel.ListKernels":
		return app.T_("Get list kernels")
	case "kernel.InstallKernel":
		return app.T_("Install kernel")
	case "kernel.InstallModules":
		return app.T_("Install kernel modules")
	case "kernel.RemovePackage":
		return app.T_("Remove packages")
	case "kernel.CheckRemovePackage":
		return app.T_("Simulate Remove packages")
	default:
		return task
	}
}
