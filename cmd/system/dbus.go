package system

import (
	"context"
	"encoding/json"
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
func (w *DBusWrapper) Install(packageName []string, transaction string) (string, *dbus.Error) {
	ctx := context.WithValue(context.Background(), "transaction", transaction)
	resp, err := w.actions.Install(ctx, packageName)
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

// Remove – обёртка над Actions.Remove.
func (w *DBusWrapper) Remove(packages []string, transaction string) (string, *dbus.Error) {
	ctx := context.WithValue(context.Background(), "transaction", transaction)
	resp, err := w.actions.Remove(ctx, packages)
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
