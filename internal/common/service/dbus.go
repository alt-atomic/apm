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

package service

import (
	"apm/internal/common/app"
	apmcli "apm/internal/common/cli"
	"apm/internal/common/dbus_doc"
	"context"
	"fmt"
	"sync"

	"github.com/godbus/dbus/v5"
	"github.com/godbus/dbus/v5/introspect"
	"github.com/urfave/cli/v3"
)

const DBusObjectPath = "/org/altlinux/APM"

type BusType int

const (
	BusSystem BusType = iota
	BusSession
)

type DBusExport struct {
	Object     any
	PostExport func(context.Context)
}

type DBusModule struct {
	Interface string
	Build     func(ctx context.Context, conn *dbus.Conn) (DBusExport, error)
}

type DBusRunConfig struct {
	Bus     BusType
	Mode    apmcli.RootCheckMode
	Modules []DBusModule
}

func RunDBus(ctx context.Context, _ *cli.Command, appConfig *app.Config, cfg DBusRunConfig) error {
	appConfig.ConfigManager.SetFormat(app.FormatDBus)
	appConfig.ConfigManager.EnableVerbose()
	if err := apmcli.CheckRoot(cfg.Mode); err != nil {
		return err
	}

	if err := connectBus(appConfig, cfg.Bus); err != nil {
		return fmt.Errorf("connect dbus: %w", err)
	}
	conn := appConfig.DBusManager.GetConnection()

	interfaces := make(map[string]any, len(cfg.Modules))
	var postHooks []func(context.Context)
	for _, mod := range cfg.Modules {
		exp, err := mod.Build(ctx, conn)
		if err != nil {
			return fmt.Errorf("build %s: %w", mod.Interface, err)
		}
		if err = conn.Export(exp.Object, DBusObjectPath, mod.Interface); err != nil {
			return fmt.Errorf("export %s: %w", mod.Interface, err)
		}
		interfaces[mod.Interface] = exp.Object
		if exp.PostExport != nil {
			postHooks = append(postHooks, exp.PostExport)
		}
	}

	if err := conn.Export(
		introspect.Introspectable(dbus_doc.GenerateIntrospectXML(interfaces)),
		DBusObjectPath,
		"org.freedesktop.DBus.Introspectable",
	); err != nil {
		return fmt.Errorf("export introspectable: %w", err)
	}

	var wg sync.WaitGroup
	for _, hook := range postHooks {
		wg.Add(1)
		go func(h func(context.Context)) {
			defer wg.Done()
			h(ctx)
		}(hook)
	}

	<-ctx.Done()
	wg.Wait()
	return nil
}

func connectBus(appConfig *app.Config, bus BusType) error {
	switch bus {
	case BusSystem:
		return appConfig.DBusManager.ConnectSystemBus()
	case BusSession:
		return appConfig.DBusManager.ConnectSessionBus()
	default:
		return fmt.Errorf("unknown bus type: %d", bus)
	}
}
