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

package distrobox

import (
	"apm/internal/common/dbus_doc"
	"apm/internal/common/reply"
	"context"
	_ "embed"
	"reflect"
)

//go:embed dbus.go
var dbusSource string

// getDocConfig возвращает конфигурацию документации для distrobox модуля
func getDocConfig() dbus_doc.Config {
	return dbus_doc.Config{
		ModuleName:    "Distrobox",
		DBusInterface: "org.altlinux.APM.distrobox",
		ServerPort:    "8082",
		DBusWrapper:   (*DBusWrapper)(nil),
		SourceCode:    dbusSource,
		DBusSession:   "session",
		ResponseTypes: map[string]reflect.Type{
			"APIResponse":             reflect.TypeOf(reply.APIResponse{}),
			"UpdateResponse":          reflect.TypeOf(UpdateResponse{}),
			"InfoResponse":            reflect.TypeOf(InfoResponse{}),
			"SearchResponse":          reflect.TypeOf(SearchResponse{}),
			"ListResponse":            reflect.TypeOf(ListResponse{}),
			"InstallResponse":         reflect.TypeOf(InstallResponse{}),
			"RemoveResponse":          reflect.TypeOf(RemoveResponse{}),
			"ContainerListResponse":   reflect.TypeOf(ContainerListResponse{}),
			"ContainerAddResponse":    reflect.TypeOf(ContainerAddResponse{}),
			"ContainerRemoveResponse": reflect.TypeOf(ContainerRemoveResponse{}),
			"GetFilterFieldsResponse": reflect.TypeOf(GetFilterFieldsResponse{}),
		},
	}
}

// startDocServer запускает веб-сервер с документацией
func startDocServer(ctx context.Context) error {
	generator := dbus_doc.NewGenerator(getDocConfig())
	return generator.StartDocServer(ctx)
}
