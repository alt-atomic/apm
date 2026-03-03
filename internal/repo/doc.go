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

package repo

import (
	"apm/internal/common/dbus_doc"
	"apm/internal/common/http_server"
	"context"
	_ "embed"
)

//go:embed dbus.go
var dbusSource string

// getDocConfig возвращает конфигурацию документации для модуля repo
func getDocConfig() dbus_doc.Config {
	responseTypes, methodResponses := dbus_doc.DeriveResponseTypes((*Actions)(nil))
	return dbus_doc.Config{
		ModuleName:      "Repo",
		DBusInterface:   "org.altlinux.APM.repo",
		SourceCode:      dbusSource,
		DBusSession:     "system",
		ResponseTypes:   responseTypes,
		MethodResponses: methodResponses,
	}
}

// startDocServer запускает веб-сервер с документацией
func startDocServer(ctx context.Context) error {
	gen := dbus_doc.NewGenerator(getDocConfig())
	return http_server.ServeHTML(ctx, "127.0.0.1:8084", gen.GenerateDBusDocHTML)
}
