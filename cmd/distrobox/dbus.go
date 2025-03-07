package distrobox

import (
	"encoding/json"
	"fmt"
	"github.com/godbus/dbus/v5"
)

// DBusWrapper – обёртка для системных действий, предназначенная для экспорта через DBus.
type DBusWrapper struct {
	actions *Actions
}

// NewDBusWrapper создаёт новую обёртку над actions
func NewDBusWrapper(a *Actions) *DBusWrapper {
	return &DBusWrapper{actions: a}
}

// Update обёртка над actions.Update
func (w *DBusWrapper) Update(container string) (string, *dbus.Error) {
	resp, err := w.actions.Update(container)
	if err != nil {
		return "", dbus.MakeFailedError(err)
	}
	data, jerr := json.Marshal(resp)
	if jerr != nil {
		return "", dbus.MakeFailedError(jerr)
	}
	return string(data), nil
}

// Info обёртка над actions.Info
func (w *DBusWrapper) Info(container string, packageName string) (string, *dbus.Error) {
	resp, err := w.actions.Info(container, packageName)
	if err != nil {
		return "", dbus.MakeFailedError(err)
	}
	data, jerr := json.Marshal(resp)
	if jerr != nil {
		return "", dbus.MakeFailedError(jerr)
	}
	return string(data), nil
}

// Search обёртка над actions.Search
func (w *DBusWrapper) Search(container string, packageName string) (string, *dbus.Error) {
	resp, err := w.actions.Search(container, packageName)
	if err != nil {
		return "", dbus.MakeFailedError(err)
	}
	data, jerr := json.Marshal(resp)
	if jerr != nil {
		return "", dbus.MakeFailedError(jerr)
	}
	return string(data), nil
}

// List принимает JSON‑строку с параметрами ListParams, а возвращает JSON с reply.APIResponse.
func (w *DBusWrapper) List(paramsJSON string) (string, *dbus.Error) {
	var params ListParams
	if err := json.Unmarshal([]byte(paramsJSON), &params); err != nil {
		return "", dbus.MakeFailedError(fmt.Errorf("не удалось разобрать JSON: %w", err))
	}

	resp, err := w.actions.List(params)
	if err != nil {
		return "", dbus.MakeFailedError(err)
	}
	data, jerr := json.Marshal(resp)
	if jerr != nil {
		return "", dbus.MakeFailedError(jerr)
	}
	return string(data), nil
}

// Install обёртка над actions.Install
func (w *DBusWrapper) Install(container string, packageName string, export bool) (string, *dbus.Error) {
	resp, err := w.actions.Install(container, packageName, export)
	if err != nil {
		return "", dbus.MakeFailedError(err)
	}
	data, jerr := json.Marshal(resp)
	if jerr != nil {
		return "", dbus.MakeFailedError(jerr)
	}
	return string(data), nil
}

// Remove обёртка над actions.Remove
func (w *DBusWrapper) Remove(container string, packageName string, onlyExport bool) (string, *dbus.Error) {
	resp, err := w.actions.Remove(container, packageName, onlyExport)
	if err != nil {
		return "", dbus.MakeFailedError(err)
	}
	data, jerr := json.Marshal(resp)
	if jerr != nil {
		return "", dbus.MakeFailedError(jerr)
	}
	return string(data), nil
}

// ContainerList обёртка над actions.ContainerList
func (w *DBusWrapper) ContainerList() (string, *dbus.Error) {
	resp, err := w.actions.ContainerList()
	if err != nil {
		return "", dbus.MakeFailedError(err)
	}
	data, jerr := json.Marshal(resp)
	if jerr != nil {
		return "", dbus.MakeFailedError(jerr)
	}
	return string(data), nil
}

// ContainerAdd обёртка над actions.ContainerAdd
func (w *DBusWrapper) ContainerAdd(image, name, additionalPackages, initHooks string) (string, *dbus.Error) {
	resp, err := w.actions.ContainerAdd(image, name, additionalPackages, initHooks)
	if err != nil {
		return "", dbus.MakeFailedError(err)
	}
	data, jerr := json.Marshal(resp)
	if jerr != nil {
		return "", dbus.MakeFailedError(jerr)
	}
	return string(data), nil
}

// ContainerRemove обёртка над actions.ContainerRemove
func (w *DBusWrapper) ContainerRemove(name string) (string, *dbus.Error) {
	resp, err := w.actions.ContainerRemove(name)
	if err != nil {
		return "", dbus.MakeFailedError(err)
	}
	data, jerr := json.Marshal(resp)
	if jerr != nil {
		return "", dbus.MakeFailedError(jerr)
	}
	return string(data), nil
}
