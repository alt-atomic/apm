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

package kernel

import (
	"apm/internal/common/dbus_doc"
	"apm/internal/common/reply"
	"context"
	_ "embed"
	"reflect"
)

//go:embed dbus.go
var dbusSource string

// getDocConfig возвращает конфигурацию документации для kernel модуля
func getDocConfig() dbus_doc.Config {
	return dbus_doc.Config{
		ModuleName:    "Kernel",
		DBusInterface: "org.altlinux.APM.kernel",
		ServerPort:    "8082",
		DBusWrapper:   (*DBusWrapper)(nil),
		SourceCode:    dbusSource,
		DBusSession:   "system",
		ResponseTypes: map[string]reflect.Type{
			"APIResponse":                  reflect.TypeOf(reply.APIResponse{}),
			"ListKernelsResponse":          reflect.TypeOf(ListKernelsResponse{}),
			"GetCurrentKernelResponse":     reflect.TypeOf(GetCurrentKernelResponse{}),
			"InstallUpdateKernelResponse":  reflect.TypeOf(InstallUpdateKernelResponse{}),
			"CleanOldKernelsResponse":      reflect.TypeOf(CleanOldKernelsResponse{}),
			"ListKernelModulesResponse":    reflect.TypeOf(ListKernelModulesResponse{}),
			"InstallKernelModulesResponse": reflect.TypeOf(InstallKernelModulesResponse{}),
			"RemoveKernelModulesResponse":  reflect.TypeOf(RemoveKernelModulesResponse{}),
		},
	}
}

// startDocServer запускает веб-сервер с документацией
func startDocServer(ctx context.Context) error {
	generator := dbus_doc.NewGenerator(getDocConfig())
	return generator.StartDocServer(ctx)
}
