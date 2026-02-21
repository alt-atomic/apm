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

// WebSocketBroadcaster интерфейс для отправки событий через WebSocket
type WebSocketBroadcaster interface {
	BroadcastEvent(event interface{})
}

var wsHub WebSocketBroadcaster

// SetWebSocketHub устанавливает WebSocket hub для отправки событий
func SetWebSocketHub(hub WebSocketBroadcaster) {
	wsHub = hub
}

// EventData содержит данные события.
type EventData struct {
	Name            string  `json:"name"`
	View            string  `json:"message"`
	State           string  `json:"state"`
	Type            string  `json:"type"`
	ProgressPercent float64 `json:"progress"`
	ProgressDone    string  `json:"progressDone"`
	Transaction     string  `json:"transaction"`
}

const (
	EventTypeNotification = "NOTIFICATION"
	EventTypeProgress     = "PROGRESS"
	EventTypeTaskResult   = "TASK_RESULT"
)

const (
	StateBefore = "BEFORE"
	StateAfter  = "AFTER"
)

// Имена событий — константы для использования в WithEventName.
const (
	// Distrobox
	EventDistroSavePackagesToDB = "distro.SavePackagesToDB"
	EventDistroGetContainerList = "distro.GetContainerList"
	EventDistroExportingApp     = "distro.ExportingApp"
	EventDistroGetContainerInfo = "distro.GetContainerOsInfo"
	EventDistroCreateContainer  = "distro.CreateContainer"
	EventDistroRemoveContainer  = "distro.RemoveContainer"
	EventDistroInstallPackage   = "distro.InstallPackage"
	EventDistroRemovePackage    = "distro.RemovePackage"
	EventDistroGetPackages      = "distro.GetPackages"
	EventDistroGetPackageOwner  = "distro.GetPackageOwner"
	EventDistroGetPathByPkg     = "distro.GetPathByPackageName"
	EventDistroGetInfoPackage   = "distro.GetInfoPackage"
	EventDistroUpdatePackages   = "distro.UpdatePackages"
	EventDistroGetPackagesQuery = "distro.GetPackagesQuery"

	// System
	EventSystemWorking              = "system.Working"
	EventSystemUpgrade              = "system.Upgrade"
	EventSystemCheck                = "system.Check"
	EventSystemUpdate               = "system.Update"
	EventSystemUpdateKernel         = "system.UpdateKernel"
	EventSystemUpdateSTPLR          = "system.UpdateSTPLR"
	EventSystemAptUpdate            = "system.AptUpdate"
	EventSystemSavePackagesToDB     = "system.SavePackagesToDB"
	EventSystemSaveImageToDB        = "system.SaveImageToDB"
	EventSystemBuildImage           = "system.BuildImage"
	EventSystemSwitchImage          = "system.SwitchImage"
	EventSystemCheckUpdateBaseImage = "system.CheckAndUpdateBaseImage"
	EventSystemBootcUpgrade         = "system.bootcUpgrade"
	EventSystemPruneOldImages       = "system.pruneOldImages"
	EventSystemUpdateAllPackagesDB  = "system.updateAllPackagesDB"
	EventSystemUpdateAppStream      = "system.UpdateAppStream"
	EventSystemDownloadProgress     = "system.downloadProgress"
	EventSystemPullImage            = "system.pullImage"

	// Bootc
	EventBootcLayers   = "service.bootc-layers"
	EventBootcDownload = "service.bootc-download"

	// Kernel
	EventKernelCurrent     = "kernel.CurrentKernel"
	EventKernelList        = "kernel.ListKernels"
	EventKernelInstall     = "kernel.InstallKernel"
	EventKernelInstallMods = "kernel.InstallModules"
	EventKernelRemove      = "kernel.RemovePackage"
	EventKernelCheckRemove = "kernel.CheckRemovePackage"
)


// TaskResultEvent содержит результат фоновой задачи
type TaskResultEvent struct {
	Type        string      `json:"type"`
	Name        string      `json:"name"`
	Transaction string      `json:"transaction,omitempty"`
	Data        interface{} `json:"data"`
	Error       bool        `json:"error"`
}

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

	UpdateTask(appConfig, eventData.Type, eventData.Name, eventData.View, eventData.State, eventData.ProgressPercent, eventData.ProgressDone)

	format := appConfig.ConfigManager.GetConfig().Format
	switch format {
	case app.FormatDBus:
		SendNotificationResponse(eventData, appConfig.DBusManager.GetConnection())
	case app.FormatHTTP:
		SendWebSocketNotification(eventData)
	}
}

// SendNotificationResponse отправляет ответы через DBus.
func SendNotificationResponse(eventData *EventData, dbusConn *dbus.Conn) {
	message, err := json.Marshal(eventData)
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

// SendWebSocketNotification отправляет событие через WebSocket
func SendWebSocketNotification(eventData *EventData) {
	if wsHub == nil {
		app.Log.Debug("WebSocket hub is not initialized")
		return
	}
	wsHub.BroadcastEvent(eventData)
}

// SendTaskResult отправляет результат фоновой задачи через WebSocket и D-Bus
func SendTaskResult(ctx context.Context, taskName string, data interface{}, taskErr error) {
	appConfig := app.GetAppConfig(ctx)

	txVal := ctx.Value(helper.TransactionKey)
	txStr, _ := txVal.(string)

	event := TaskResultEvent{
		Type:        EventTypeTaskResult,
		Name:        taskName,
		Transaction: txStr,
		Data:        data,
		Error:       false,
	}

	if taskErr != nil {
		event.Error = true
		event.Data = map[string]interface{}{
			"message": taskErr.Error(),
		}
	}

	format := appConfig.ConfigManager.GetConfig().Format
	switch format {
	case app.FormatDBus:
		SendTaskResultDBus(&event, appConfig.DBusManager.GetConnection())
	case app.FormatHTTP:
		SendTaskResultWebSocket(&event)
	}
}

// SendTaskResultWebSocket отправляет результат задачи через WebSocket
func SendTaskResultWebSocket(event *TaskResultEvent) {
	if wsHub == nil {
		app.Log.Debug("WebSocket hub is not initialized")
		return
	}
	wsHub.BroadcastEvent(event)
}

// SendTaskResultDBus отправляет результат задачи через D-Bus сигнал
func SendTaskResultDBus(event *TaskResultEvent, dbusConn *dbus.Conn) {
	message, err := json.Marshal(event)
	if err != nil {
		app.Log.Debug(err.Error())
		return
	}

	if dbusConn == nil {
		app.Log.Error(app.T_("DBus connection is not initialized"))
		return
	}

	objPath := dbus.ObjectPath("/org/altlinux/APM")
	signalName := "org.altlinux.APM.Notification"

	err = dbusConn.Emit(objPath, signalName, string(message))
	if err != nil {
		app.Log.Error(app.T_("Error sending task result: %v"), err)
	}
}

func getTaskText(task string) string {
	switch task {
	case EventDistroSavePackagesToDB:
		return app.T_("Saving packages to the database")
	case EventDistroGetContainerList:
		return app.T_("Requesting list of containers")
	case EventDistroExportingApp:
		return app.T_("Exporting package")
	case EventDistroGetContainerInfo:
		return app.T_("Requesting container information")
	case EventDistroCreateContainer:
		return app.T_("Creating container")
	case EventDistroRemoveContainer:
		return app.T_("Deleting container")
	case EventDistroInstallPackage:
		return app.T_("Installing package")
	case EventDistroRemovePackage:
		return app.T_("Removing package")
	case EventDistroGetPackages:
		return app.T_("Retrieving list of packages")
	case EventDistroGetPackageOwner:
		return app.T_("Determining file owner")
	case EventDistroGetPathByPkg:
		return app.T_("Searching package paths")
	case EventDistroGetInfoPackage:
		return app.T_("Retrieving package information")
	case EventDistroUpdatePackages:
		return app.T_("Updating packages")
	case EventDistroGetPackagesQuery:
		return app.T_("Filtering packages")
	case EventSystemWorking:
		return app.T_("Working with packages")
	case EventSystemUpgrade:
		return app.T_("System update")
	case EventSystemCheck:
		return app.T_("Analyzing packages")
	case EventSystemUpdate:
		return app.T_("General update process")
	case EventSystemUpdateKernel:
		return app.T_("General update kernel")
	case EventSystemUpdateSTPLR:
		return app.T_("Loading package list from STPLR repository")
	case EventSystemAptUpdate:
		return app.T_("Loading package list from ALT repository")
	case EventSystemSavePackagesToDB:
		return app.T_("Saving packages to the database")
	case EventSystemSaveImageToDB:
		return app.T_("Saving image history to the database")
	case EventSystemBuildImage:
		return app.T_("Building local image")
	case EventSystemSwitchImage:
		return app.T_("Switching to local image")
	case EventSystemCheckUpdateBaseImage:
		return app.T_("General Image Update Process")
	case EventSystemBootcUpgrade:
		return app.T_("Downloading base image update")
	case EventSystemPruneOldImages:
		return app.T_("Cleaning up old images")
	case EventSystemUpdateAllPackagesDB:
		return app.T_("Synchronizing database")
	case EventSystemUpdateAppStream:
		return app.T_("Update information about applications")
	case EventSystemDownloadProgress:
		return app.T_("Downloading packages")
	case EventSystemPullImage:
		return app.T_("Downloading image")
	case EventBootcLayers:
		return app.T_("Fetching layers")
	case EventBootcDownload:
		return app.T_("Downloading update")
	case EventKernelCurrent:
		return app.T_("Get current kernel")
	case EventKernelList:
		return app.T_("Get list kernels")
	case EventKernelInstall:
		return app.T_("Install kernel")
	case EventKernelInstallMods:
		return app.T_("Install kernel modules")
	case EventKernelRemove:
		return app.T_("Remove packages")
	case EventKernelCheckRemove:
		return app.T_("Simulate Remove packages")
	default:
		return task
	}
}
