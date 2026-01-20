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

package system

import (
	"apm/internal/common/dbus_doc"
	"apm/internal/common/reply"
	"context"
	_ "embed"
	"reflect"
)

//go:embed dbus.go
var dbusSource string

//go:embed actions.go
var actionsSource string

// responseTypes общие типы ответов для D-Bus и HTTP API
var responseTypes = map[string]reflect.Type{
	"APIResponse":             reflect.TypeOf(reply.APIResponse{}),
	"InstallRemoveResponse":   reflect.TypeOf(InstallRemoveResponse{}),
	"GetFilterFieldsResponse": reflect.TypeOf(GetFilterFieldsResponse{}),
	"UpdateResponse":          reflect.TypeOf(UpdateResponse{}),
	"ListResponse":            reflect.TypeOf(ListResponse{}),
	"InfoResponse":            reflect.TypeOf(InfoResponse{}),
	"CheckResponse":           reflect.TypeOf(CheckResponse{}),
	"UpgradeResponse":         reflect.TypeOf(UpgradeResponse{}),
	"SearchResponse":          reflect.TypeOf(SearchResponse{}),
	"ImageApplyResponse":      reflect.TypeOf(ImageApplyResponse{}),
	"ImageHistoryResponse":    reflect.TypeOf(ImageHistoryResponse{}),
	"ImageUpdateResponse":     reflect.TypeOf(ImageUpdateResponse{}),
	"ImageStatusResponse":     reflect.TypeOf(ImageStatusResponse{}),
	"ImageConfigResponse":     reflect.TypeOf(ImageConfigResponse{}),
}

// GetActionsSourceCode возвращает исходный код actions.go для парсинга аннотаций
func GetActionsSourceCode() string {
	return actionsSource
}

// GetHTTPResponseTypes возвращает типы ответов для генерации OpenAPI схем
func GetHTTPResponseTypes() map[string]reflect.Type {
	return responseTypes
}

// getDocConfig возвращает конфигурацию документации D-Bus
func getDocConfig() dbus_doc.Config {
	return dbus_doc.Config{
		ModuleName:    "System",
		DBusInterface: "org.altlinux.APM.system",
		ServerPort:    "8081",
		DBusWrapper:   (*DBusWrapper)(nil),
		SourceCode:    dbusSource,
		DBusSession:   "system",
		ResponseTypes: responseTypes,
	}
}

// startDocServer запускает веб-сервер с D-Bus документацией
func startDocServer(ctx context.Context) error {
	generator := dbus_doc.NewGenerator(getDocConfig())
	return generator.StartDocServer(ctx)
}
