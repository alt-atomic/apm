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

// Package dbusv2 — сборка нативного DBus API v2: экспорт интерфейсов,
// реестр задач, properties и introspection на пути /org/altlinux/APM2.
package dbusv2

import (
	"context"
	"fmt"

	"altlinux.space/alt-atomic/apm/internal/common/app"
	"altlinux.space/alt-atomic/apm/internal/common/dbusv2/authz"
	"altlinux.space/alt-atomic/apm/internal/common/dbusv2/jobs"
	"altlinux.space/alt-atomic/apm/internal/common/dbusv2/protocol"
	"altlinux.space/alt-atomic/apm/internal/common/reply"

	"github.com/godbus/dbus/v5"
	"github.com/godbus/dbus/v5/introspect"
	"github.com/godbus/dbus/v5/prop"
)

// Module один доменный интерфейс API v2.
type Module struct {
	Iface         string
	Introspection introspect.Interface
	Build         func(ctx context.Context, reg *jobs.Registry, az authz.Authorizer) any
	// PostExport запускается в фоне после экспорта интерфейса (прогрев кэшей и т.п.).
	PostExport func(ctx context.Context)
}

// Setup конфигурация экспорта API v2.
type Setup struct {
	Reporter *reply.Reporter
	// UsePolkit true — авторизация через polkit (системная шина), false — AllowAll.
	UsePolkit bool
	// Props константные свойства: интерфейс → имя → значение.
	Props   map[string]map[string]any
	Modules []Module

	registry *jobs.Registry
}

// jobPrefix уникальное имя соединения на шине.
func jobPrefix(conn *dbus.Conn) string {
	names := conn.Names()
	if len(names) == 0 {
		return ""
	}
	return names[0]
}

// Export экспортирует интерфейсы API, свойства и introspection на соединении демона.
func (s *Setup) Export(ctx context.Context, conn *dbus.Conn) error {
	emit := func(member string, values ...any) {
		if err := conn.Emit(protocol.Path, protocol.JobsIface+"."+member, values...); err != nil {
			app.Log.Error("dbusv2 emit failed: ", err)
		}
	}
	reg := jobs.NewRegistry(ctx, jobPrefix(conn), emit)
	s.registry = reg
	s.Reporter.AddSink(jobs.NewSink(reg))

	az := authz.AllowAll
	if s.UsePolkit {
		az = authz.Polkit(conn)
	}

	interfaces := []introspect.Interface{introspect.IntrospectData, jobsIntrospection}

	jobsAPI := &JobsAPI{reg: reg, az: az}
	if err := conn.Export(jobsAPI, protocol.Path, protocol.JobsIface); err != nil {
		return fmt.Errorf("export %s: %w", protocol.JobsIface, err)
	}

	for _, m := range s.Modules {
		obj := m.Build(ctx, reg, az)
		if err := conn.Export(obj, protocol.Path, m.Iface); err != nil {
			return fmt.Errorf("export %s: %w", m.Iface, err)
		}
		interfaces = append(interfaces, m.Introspection)
		if m.PostExport != nil {
			go m.PostExport(ctx)
		}
	}

	if len(s.Props) > 0 {
		propMap := prop.Map{}
		for iface, values := range s.Props {
			entry := map[string]*prop.Prop{}
			for name, value := range values {
				entry[name] = &prop.Prop{Value: value, Emit: prop.EmitConst}
			}
			propMap[iface] = entry
		}
		if _, err := prop.Export(conn, protocol.Path, propMap); err != nil {
			return fmt.Errorf("export properties: %w", err)
		}
		interfaces = append(interfaces, prop.IntrospectData)
	}

	node := &introspect.Node{Interfaces: interfaces}
	if err := conn.Export(introspect.NewIntrospectable(node), protocol.Path, "org.freedesktop.DBus.Introspectable"); err != nil {
		return fmt.Errorf("export introspectable: %w", err)
	}

	return nil
}

// Shutdown завершает реестр фоновых задач перед закрытием соединения и БД.
func (s *Setup) Shutdown() {
	if s.registry != nil {
		s.registry.Shutdown()
	}
}
