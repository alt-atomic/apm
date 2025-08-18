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

package main

import (
	"apm/internal/common/binding/apt"
	aptLib "apm/internal/common/binding/apt/lib"
	"apm/internal/common/helper"
	"apm/internal/common/icon"
	"apm/internal/common/reply"
	"apm/internal/distrobox"
	"apm/internal/system"
	"apm/lib"
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/godbus/dbus/v5"

	"github.com/godbus/dbus/v5/introspect"
	"github.com/urfave/cli/v3"
)

var (
	ctx, globalCancel = context.WithCancel(context.Background())
)

func main() {
	lib.Log.Debugln("Starting apm…")

	errInitial := lib.InitConfig()
	if errInitial != nil {
		_ = reply.CliResponse(ctx, reply.APIResponse{
			Data: map[string]interface{}{
				"message": errInitial.Error(),
			},
			Error: true,
		})
	}

	lib.InitLogger()
	lib.InitLocales()
	helper.SetupHelpTemplates()

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)

	go func() {
		sig := <-sigs

		switch sig {
		case syscall.SIGINT, syscall.SIGTERM:
			infoText := fmt.Sprintf(lib.T_("Recieved correct signal %s. Stopping application…"), sig)
			lib.Log.Info(infoText)
			_ = reply.CliResponse(ctx, reply.APIResponse{
				Data: map[string]interface{}{
					"message": infoText,
				},
				Error: true,
			})
		default:
			infoText := fmt.Sprintf(lib.T_("Unexpected signal %s received. Terminating the application with an error."), sig)
			lib.Log.Error(infoText)
			_ = reply.CliResponse(ctx, reply.APIResponse{
				Data: map[string]interface{}{
					"message": infoText,
				},
				Error: true,
			})
		}

		aptLib.WaitIdle()
		cleanup()
		code := 1
		if s, ok := sig.(syscall.Signal); ok {
			if s == syscall.SIGINT {
				code = 130
			} else if s == syscall.SIGTERM {
				code = 143
			} else {
				code = 128 + int(s)
			}
		}
		os.Exit(code)
	}()

	systemCommands := system.CommandList()
	distroboxCommands := distrobox.CommandList()

	// Основная команда приложения
	rootCommand := &cli.Command{
		Name:  "apm",
		Usage: "Atomic Package Manager",
		Version: lib.Env.Version,
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    "format",
				Usage:   lib.T_("Output format: json, text"),
				Aliases: []string{"f"},
				Value:   "text",
			},
			&cli.StringFlag{
				Name:    "transaction",
				Usage:   lib.T_("Internal property, adds the transaction to the output"),
				Aliases: []string{"t"},
			},
		},
		Commands: []*cli.Command{
			{
				Name:   "dbus-session",
				Usage:  lib.T_("Start session D-Bus service org.altlinux.APM"),
				Action: sessionDbus,
			},
			{
				Name:   "dbus-system",
				Usage:  lib.T_("Start system D-Bus service org.altlinux.APM"),
				Action: systemDbus,
			},
			systemCommands,
			distroboxCommands,
			{
				Name:      "help",
				Aliases:   []string{"h"},
				Usage:     lib.T_("Show the list of commands or help for each command"),
				ArgsUsage: lib.T_("[command]"),
				HideHelp:  true,
			},
		},
	}

	applyCommandSetting(rootCommand)
	applyCommandSetting(distroboxCommands)
	applyCommandSetting(systemCommands)

	if err := rootCommand.Run(ctx, os.Args); err != nil {
		cleanup()
		os.Exit(1)
	}
}

func applyCommandSetting(cliCommand *cli.Command) {
	cliCommand.CommandNotFound = func(ctx context.Context, cmd *cli.Command, name string) {
		lib.Env.Format = cmd.String("format")
		msg := fmt.Sprintf(lib.T_("Unknown command: %s. See 'apm help'"), name)
		_ = reply.CliResponse(ctx, reply.APIResponse{Data: map[string]interface{}{"message": msg}, Error: true})
	}
	cliCommand.EnableShellCompletion = true
	cliCommand.Suggest = true

	for _, sub := range cliCommand.Commands {
		applyCommandSetting(sub)
	}
}

func sessionDbus(ctx context.Context, cmd *cli.Command) error {
	lib.Env.Format = cmd.String("format")
	if syscall.Geteuid() == 0 {
		errPermission := lib.T_("Elevated rights are not allowed to perform this action. Please do not use sudo or su")
		_ = reply.CliResponse(ctx, reply.APIResponse{
			Data: map[string]interface{}{
				"message": errPermission,
			},
			Error: true,
		})
		return fmt.Errorf(errPermission)
	}
	defer cleanup()
	err := lib.InitDBus(false)
	if err != nil {
		lib.Log.Errorf("InitDBus failed: %v", err)
		_ = reply.CliResponse(ctx, reply.APIResponse{
			Data: map[string]interface{}{
				"message": err.Error(),
			},
			Error: true,
		})
		return err
	}

	distroActions := distrobox.NewActions()
	serviceIcon := icon.NewIconService(lib.GetDBKv())
	distroObj := distrobox.NewDBusWrapper(distroActions, serviceIcon)

	// Экспортируем в D-Bus
	if err = lib.DBUSConn.Export(distroObj, "/org/altlinux/APM", "org.altlinux.APM.distrobox"); err != nil {
		return err
	}

	if err = lib.DBUSConn.Export(
		introspect.Introspectable(helper.GetUserIntrospectXML()),
		"/org/altlinux/APM",
		"org.freedesktop.DBus.Introspectable",
	); err != nil {
		return err
	}

	lib.Env.Format = "dbus"

	// Параллельно обновляем иконки
	go func() {
		err = serviceIcon.ReloadIcons(ctx)
		if err != nil {
			lib.Log.Error(err.Error())
		}
	}()

	// Блокируем до сигнала
	select {}
}

func systemDbus(ctx context.Context, cmd *cli.Command) error {
	lib.Env.Format = cmd.String("format")
	if syscall.Geteuid() != 0 {
		errPermission := lib.T_("Elevated rights are required to perform this action. Please use sudo or su")
		_ = reply.CliResponse(ctx, reply.APIResponse{
			Data: map[string]interface{}{
				"message": errPermission,
			},
			Error: true,
		})
		return fmt.Errorf(errPermission)
	}

	defer cleanup()
	err := lib.InitDBus(true)
	if err != nil {
		lib.Log.Errorf("InitDBus failed: %v", err)
		_ = reply.CliResponse(ctx, reply.APIResponse{
			Data: map[string]interface{}{
				"message": err.Error(),
			},
			Error: true,
		})
		return err
	}

	if syscall.Geteuid() != 0 {
		return fmt.Errorf(lib.T_("Administrator privileges are required to start"))
	}

	sysActions := system.NewActions()
	conn, _ := dbus.SystemBus()
	sysObj := system.NewDBusWrapper(sysActions, conn)

	// Экспортируем в D-Bus
	if err = lib.DBUSConn.Export(sysObj, "/org/altlinux/APM", "org.altlinux.APM.system"); err != nil {
		return err
	}

	if err = lib.DBUSConn.Export(
		introspect.Introspectable(helper.GetSystemIntrospectXML()),
		"/org/altlinux/APM",
		"org.freedesktop.DBus.Introspectable",
	); err != nil {
		return err
	}

	lib.Env.Format = "dbus"

	// Блокируем до сигнала
	select {}
}

func cleanup() {
	lib.Log.Debugln(lib.T_("Terminating the application. Releasing resources…"))

	defer apt.Close() // закрываем экземпляр APT system
	defer globalCancel()
	if dbKV := lib.CheckDBKv(); dbKV != nil {
		if err := dbKV.Close(); err != nil {
			lib.Log.Error(lib.T_("Error closing KV database: "), err)
		}
	}

	if dbSQL := lib.CheckDB(); dbSQL != nil {
		if err := dbSQL.Close(); err != nil {
			lib.Log.Error(lib.T_("Error closing SQL database: "), err)
		}
	}
}
