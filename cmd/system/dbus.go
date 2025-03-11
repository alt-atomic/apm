package system

import (
	"context"
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

// Install – обёртка над Actions.Install.
func (w *DBusWrapper) Install(packages []string, applyAtomic bool, transaction string) (string, *dbus.Error) {
	ctx := context.WithValue(context.Background(), "transaction", transaction)
	resp, err := w.actions.Install(ctx, packages, applyAtomic)
	if err != nil {
		return "", dbus.MakeFailedError(err)
	}
	data, jerr := json.Marshal(resp)
	if jerr != nil {
		return "", dbus.MakeFailedError(jerr)
	}
	return string(data), nil
}

// Remove – обёртка над Actions.Remove.
func (w *DBusWrapper) Remove(packages []string, applyAtomic bool, transaction string) (string, *dbus.Error) {
	ctx := context.WithValue(context.Background(), "transaction", transaction)
	resp, err := w.actions.Remove(ctx, packages, applyAtomic)
	if err != nil {
		return "", dbus.MakeFailedError(err)
	}
	data, jerr := json.Marshal(resp)
	if jerr != nil {
		return "", dbus.MakeFailedError(jerr)
	}
	return string(data), nil
}

// Update – обёртка над Actions.Update.
func (w *DBusWrapper) Update(transaction string) (string, *dbus.Error) {
	ctx := context.WithValue(context.Background(), "transaction", transaction)
	resp, err := w.actions.Update(ctx)
	if err != nil {
		return "", dbus.MakeFailedError(err)
	}
	data, jerr := json.Marshal(resp)
	if jerr != nil {
		return "", dbus.MakeFailedError(jerr)
	}
	return string(data), nil
}

// List – обёртка над Actions.List.
func (w *DBusWrapper) List(paramsJSON string, transaction string) (string, *dbus.Error) {
	ctx := context.WithValue(context.Background(), "transaction", transaction)
	var params ListParams
	if err := json.Unmarshal([]byte(paramsJSON), &params); err != nil {
		return "", dbus.MakeFailedError(fmt.Errorf("не удалось разобрать JSON: %w", err))
	}

	resp, err := w.actions.List(ctx, params)
	if err != nil {
		return "", dbus.MakeFailedError(err)
	}
	data, jerr := json.Marshal(resp)
	if jerr != nil {
		return "", dbus.MakeFailedError(jerr)
	}
	return string(data), nil
}

// Info – обёртка над Actions.Info.
func (w *DBusWrapper) Info(packageName string, transaction string) (string, *dbus.Error) {
	ctx := context.WithValue(context.Background(), "transaction", transaction)
	resp, err := w.actions.Info(ctx, packageName)
	if err != nil {
		return "", dbus.MakeFailedError(err)
	}
	data, jerr := json.Marshal(resp)
	if jerr != nil {
		return "", dbus.MakeFailedError(jerr)
	}
	return string(data), nil
}

// CheckInstall – обёртка над Actions.CheckInstall.
func (w *DBusWrapper) CheckInstall(packages []string, transaction string) (string, *dbus.Error) {
	ctx := context.WithValue(context.Background(), "transaction", transaction)
	resp, err := w.actions.CheckInstall(ctx, packages)
	if err != nil {
		return "", dbus.MakeFailedError(err)
	}
	data, jerr := json.Marshal(resp)
	if jerr != nil {
		return "", dbus.MakeFailedError(jerr)
	}
	return string(data), nil
}

// CheckRemove – обёртка над Actions.CheckRemove.
func (w *DBusWrapper) CheckRemove(packages []string, transaction string) (string, *dbus.Error) {
	ctx := context.WithValue(context.Background(), "transaction", transaction)
	resp, err := w.actions.CheckRemove(ctx, packages)
	if err != nil {
		return "", dbus.MakeFailedError(err)
	}
	data, jerr := json.Marshal(resp)
	if jerr != nil {
		return "", dbus.MakeFailedError(jerr)
	}
	return string(data), nil
}

// Search – обёртка над Actions.Search.
func (w *DBusWrapper) Search(packageName string, transaction string, installed bool) (string, *dbus.Error) {
	ctx := context.WithValue(context.Background(), "transaction", transaction)
	resp, err := w.actions.Search(ctx, packageName, installed)
	if err != nil {
		return "", dbus.MakeFailedError(err)
	}
	data, jerr := json.Marshal(resp)
	if jerr != nil {
		return "", dbus.MakeFailedError(jerr)
	}
	return string(data), nil
}

// ImageApply – обёртка над Actions.Apply.
func (w *DBusWrapper) ImageApply(transaction string) (string, *dbus.Error) {
	ctx := context.WithValue(context.Background(), "transaction", transaction)
	resp, err := w.actions.ImageApply(ctx)
	if err != nil {
		return "", dbus.MakeFailedError(err)
	}
	data, jerr := json.Marshal(resp)
	if jerr != nil {
		return "", dbus.MakeFailedError(jerr)
	}
	return string(data), nil
}

// ImageHistory – обёртка над Actions.ImageHistory.
func (w *DBusWrapper) ImageHistory(transaction string, imageName string, limit int64, offset int64) (string, *dbus.Error) {
	ctx := context.WithValue(context.Background(), "transaction", transaction)
	resp, err := w.actions.ImageHistory(ctx, imageName, limit, offset)
	if err != nil {
		return "", dbus.MakeFailedError(err)
	}
	data, jerr := json.Marshal(resp)
	if jerr != nil {
		return "", dbus.MakeFailedError(jerr)
	}
	return string(data), nil
}

// ImageUpdate – обёртка над Actions.ImageUpdate.
func (w *DBusWrapper) ImageUpdate(transaction string) (string, *dbus.Error) {
	ctx := context.WithValue(context.Background(), "transaction", transaction)
	resp, err := w.actions.ImageUpdate(ctx)
	if err != nil {
		return "", dbus.MakeFailedError(err)
	}
	data, jerr := json.Marshal(resp)
	if jerr != nil {
		return "", dbus.MakeFailedError(jerr)
	}
	return string(data), nil
}

// ImageStatus – обёртка над Actions.ImageStatus.
func (w *DBusWrapper) ImageStatus(transaction string) (string, *dbus.Error) {
	ctx := context.WithValue(context.Background(), "transaction", transaction)
	resp, err := w.actions.ImageStatus(ctx)
	if err != nil {
		return "", dbus.MakeFailedError(err)
	}
	data, jerr := json.Marshal(resp)
	if jerr != nil {
		return "", dbus.MakeFailedError(jerr)
	}
	return string(data), nil
}
