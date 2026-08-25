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
	"context"
	"fmt"

	"altlinux.space/alt-atomic/apm/internal/common/app"
	apmcli "altlinux.space/alt-atomic/apm/internal/common/cli"

	"github.com/godbus/dbus/v5"
	"github.com/urfave/cli/v3"
)

type BusType int

const (
	BusSystem BusType = iota
	BusSession
)

type DBusAPI interface {
	Export(ctx context.Context, conn *dbus.Conn) error
}

type DBusRunConfig struct {
	Bus  BusType
	Mode apmcli.RootCheckMode
	APIs []DBusAPI
}

// RunDBus поднимает DBus-демон: соединение, экспорт API, ожидание останова.
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

	for _, api := range cfg.APIs {
		if err := api.Export(ctx, conn); err != nil {
			return fmt.Errorf("export dbus api: %w", err)
		}
	}

	<-ctx.Done()
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
