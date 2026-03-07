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
	"apm/internal/common/http_server"
	"apm/internal/system/appstream"
	"context"
	_ "embed"
	"reflect"
)

//go:embed dbus.go
var dbusSource string

// getDocConfig возвращает конфигурацию документации D-Bus
func getDocConfig() dbus_doc.Config {
	responseTypes, methodResponses := dbus_doc.DeriveResponseTypes((*Actions)(nil))

	// добавление модуля приложений
	asResponseTypes, _ := dbus_doc.DeriveResponseTypes((*appstream.Actions)(nil))
	for name, typ := range asResponseTypes {
		if name == "APIResponse" {
			continue
		}
		responseTypes[name] = typ
	}
	methodResponses["ApplicationUpdate"] = reflect.TypeOf(appstream.UpdateResponse{}).Name()
	methodResponses["ApplicationInfo"] = reflect.TypeOf(appstream.InfoResponse{}).Name()
	methodResponses["ApplicationList"] = reflect.TypeOf(appstream.ListResponse{}).Name()
	methodResponses["ApplicationGetFilterFields"] = reflect.TypeOf(appstream.FilterFieldsAppStreamResponse{}).Name()
	methodResponses["ApplicationCategories"] = reflect.TypeOf(appstream.CategoriesResponse{}).Name()

	return dbus_doc.Config{
		ModuleName:      "System",
		DBusInterface:   "org.altlinux.APM.system",
		SourceCode:      dbusSource,
		DBusSession:     "system",
		ResponseTypes:   responseTypes,
		MethodResponses: methodResponses,
	}
}

// startDocServer запускает веб-сервер с D-Bus документацией
func startDocServer(ctx context.Context) error {
	gen := dbus_doc.NewGenerator(getDocConfig())
	return http_server.ServeHTML(ctx, "127.0.0.1:8085", gen.GenerateDBusDocHTML)
}
