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

package dbusv1

import (
	"context"
	"fmt"

	"altlinux.space/alt-atomic/apm/internal/common/reply"

	"github.com/godbus/dbus/v5"
	"github.com/godbus/dbus/v5/introspect"
)

// Path объектный путь API v1.
const Path = dbus.ObjectPath("/org/altlinux/APM")

// Object экспортируемый объект интерфейса и необязательная фоновая инициализация.
type Object struct {
	Value      any
	PostExport func(context.Context)
}

// Module один доменный интерфейс API v1.
type Module struct {
	Interface string
	Build     func(ctx context.Context, conn *dbus.Conn) (Object, error)
}

// Setup конфигурация экспорта API v1.
type Setup struct {
	Reporter *reply.Reporter
	Modules  []Module
}

// Export экспортирует интерфейсы API v1, их introspection и подписывает
// Reporter на рассылку совместимых сигналов Notification.
func (s Setup) Export(ctx context.Context, conn *dbus.Conn) error {
	if len(s.Modules) == 0 {
		return nil
	}

	interfaces := make(map[string]any, len(s.Modules))
	for _, m := range s.Modules {
		obj, err := m.Build(ctx, conn)
		if err != nil {
			return fmt.Errorf("build %s: %w", m.Interface, err)
		}
		if err = conn.Export(obj.Value, Path, m.Interface); err != nil {
			return fmt.Errorf("export %s: %w", m.Interface, err)
		}
		interfaces[m.Interface] = obj.Value
		if obj.PostExport != nil {
			go obj.PostExport(ctx)
		}
	}

	if err := conn.Export(
		introspect.Introspectable(generateIntrospectXML(interfaces)),
		Path,
		"org.freedesktop.DBus.Introspectable",
	); err != nil {
		return fmt.Errorf("export introspectable: %w", err)
	}

	s.Reporter.AddSink(NewSink(conn))
	return nil
}
