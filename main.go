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
	"apm/internal/common/config"
	"apm/internal/common/helper"
	"apm/internal/common/icon"
	"apm/internal/common/reply"
	"apm/internal/distrobox"
	"apm/internal/kernel"
	"apm/internal/system"
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/godbus/dbus/v5"

	"github.com/godbus/dbus/v5/introspect"
	"github.com/urfave/cli/v3"
)

var (
	ctx, globalCancel = context.WithCancel(context.Background())
	appConfig         *config.AppConfig
)

func main() {
	var errInitial error
	appConfig, errInitial = config.InitializeAppDefault()
	cliError(errInitial)

	helper.SetupHelpTemplates()

	appConfig.Logger.Debug("Starting apm…")
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)

	go func() {
		sig := <-sigs

		switch sig {
		case syscall.SIGINT, syscall.SIGTERM:
			infoText := fmt.Sprintf(appConfig.Translator.T_("Recieved correct signal %s. Stopping application…"), sig)
			appConfig.Logger.Info(infoText)
			cliError(errors.New(infoText))

		default:
			infoText := fmt.Sprintf(appConfig.Translator.T_("Unexpected signal %s received. Terminating the application with an error."), sig)
			appConfig.Logger.Error(infoText)
			cliError(errors.New(infoText))
		}

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
	ctx = context.WithValue(ctx, config.AppConfigKey, appConfig)

	systemCommands := system.CommandList(ctx)
	distroboxCommands := distrobox.CommandList()
	kernelCommands := kernel.CommandList()

	// Основная команда приложения
	rootCommand := &cli.Command{
		Name:    "apm",
		Usage:   "Atomic Package Manager",
		Version: appConfig.ConfigManager.GetConfig().Version,
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    "format",
				Usage:   appConfig.Translator.T_("Output format: json, text"),
				Aliases: []string{"f"},
				Value:   "text",
			},
			&cli.StringFlag{
				Name:    "transaction",
				Usage:   appConfig.Translator.T_("Internal property, adds the transaction to the output"),
				Aliases: []string{"t"},
			},
		},
		Commands: []*cli.Command{
			{
				Name:   "dbus-session",
				Usage:  appConfig.Translator.T_("Start session D-Bus service org.altlinux.APM"),
				Action: sessionDbus,
			},
			{
				Name:   "dbus-system",
				Usage:  appConfig.Translator.T_("Start system D-Bus service org.altlinux.APM"),
				Action: systemDbus,
			},
			systemCommands,
			distroboxCommands,
			kernelCommands,
			{
				Name:      "help",
				Aliases:   []string{"h"},
				Usage:     appConfig.Translator.T_("Show the list of commands or help for each command"),
				ArgsUsage: appConfig.Translator.T_("[command]"),
				HideHelp:  true,
			},
		},
	}

	applyCommandSetting(rootCommand)

	if err := rootCommand.Run(ctx, os.Args); err != nil {
		cleanup()
		os.Exit(1)
	}
}

func applyCommandSetting(cliCommand *cli.Command) {
	cliCommand.CommandNotFound = func(ctx context.Context, cmd *cli.Command, name string) {
		appConfig.ConfigManager.SetFormat(cmd.String("format"))
		msg := fmt.Sprintf(appConfig.Translator.T_("Unknown command: %s. See 'apm help'"), name)
		cliError(errors.New(msg))
	}
	cliCommand.HideHelpCommand = true
	cliCommand.EnableShellCompletion = true
	cliCommand.Suggest = true

	for _, sub := range cliCommand.Commands {
		applyCommandSetting(sub)
	}
}

func sessionDbus(ctx context.Context, cmd *cli.Command) error {
	appConfig.ConfigManager.SetFormat(cmd.String("format"))
	if syscall.Geteuid() == 0 {
		errPermission := appConfig.Translator.T_("Elevated rights are not allowed to perform this action. Please do not use sudo or su")
		cliError(errors.New(errPermission))
		return fmt.Errorf(errPermission)
	}
	defer cleanup()
	err := appConfig.DBusManager.ConnectSessionBus()
	if err != nil {
		appConfig.Logger.Error("ConnectSessionBus failed: ", err)
		cliError(err)
		return err
	}

	distroActions := distrobox.NewActions()
	serviceIcon := icon.NewIconService(appConfig.DatabaseManager.GetKeyValueDB())
	distroObj := distrobox.NewDBusWrapper(distroActions, serviceIcon)

	// Экспортируем в D-Bus
	if err = appConfig.DBusManager.GetConnection().Export(distroObj, "/org/altlinux/APM", "org.altlinux.APM.distrobox"); err != nil {
		return err
	}

	if err = appConfig.DBusManager.GetConnection().Export(
		introspect.Introspectable(helper.GetUserIntrospectXML()),
		"/org/altlinux/APM",
		"org.freedesktop.DBus.Introspectable",
	); err != nil {
		return err
	}

	appConfig.ConfigManager.SetFormat("dbus")

	// Параллельно обновляем иконки
	go func() {
		err = serviceIcon.ReloadIcons(ctx)
		if err != nil {
			appConfig.Logger.Error(err.Error())
		}
	}()

	// Блокируем до сигнала
	select {}
}

func systemDbus(_ context.Context, cmd *cli.Command) error {
	appConfig.ConfigManager.SetFormat(cmd.String("format"))
	if syscall.Geteuid() != 0 {
		errPermission := appConfig.Translator.T_("Elevated rights are required to perform this action. Please use sudo or su")
		cliError(errors.New(errPermission))
		return fmt.Errorf(errPermission)
	}

	defer cleanup()
	err := appConfig.DBusManager.ConnectSystemBus()
	if err != nil {
		cliError(err)
		return err
	}

	if syscall.Geteuid() != 0 {
		return fmt.Errorf(appConfig.Translator.T_("Administrator privileges are required to start"))
	}

	sysActions := system.NewActions(appConfig)
	conn, _ := dbus.SystemBus()
	sysObj := system.NewDBusWrapper(sysActions, conn)

	// Экспортируем в D-Bus
	if err = appConfig.DBusManager.GetConnection().Export(sysObj, "/org/altlinux/APM", "org.altlinux.APM.system"); err != nil {
		return err
	}

	if err = appConfig.DBusManager.GetConnection().Export(
		introspect.Introspectable(helper.GetSystemIntrospectXML()),
		"/org/altlinux/APM",
		"org.freedesktop.DBus.Introspectable",
	); err != nil {
		return err
	}

	appConfig.ConfigManager.SetFormat("dbus")

	// Блокируем до сигнала
	select {}
}

func cliError(err error) {
	if err == nil {
		return
	}

	errCli := reply.CliResponse(ctx, reply.APIResponse{
		Data: map[string]interface{}{
			"message": err.Error(),
		},
		Error: true,
	})
	if errCli != nil {
		log.Fatal(errCli)
	}
}

func cleanup() {
	if appConfig != nil {
		appConfig.Logger.Debug(appConfig.Translator.T_("Terminating the application. Releasing resources…"))
		defer func(appConfig *config.AppConfig) {
			err := config.CleanupApp(appConfig)
			if err != nil {
				appConfig.Logger.Error(err)
			}
		}(appConfig)
	}

	defer globalCancel()
}
