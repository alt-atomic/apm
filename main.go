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
	"apm/internal/common/app"
	"apm/internal/common/binding/apt"
	aptLib "apm/internal/common/binding/apt/lib"
	"apm/internal/common/dbus_doc"
	"apm/internal/common/helper"
	"apm/internal/common/http_server"
	"apm/internal/common/icon"
	"apm/internal/common/reply"
	"apm/internal/common/version"
	"apm/internal/distrobox"
	"apm/internal/kernel"
	"apm/internal/repo"
	"apm/internal/system"
	"context"
	"errors"
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
	appConfig         *app.Config
)

func main() {
	var errInitial error
	appConfig, errInitial = app.InitializeAppDefault()
	if errInitial != nil {
		panic(errInitial)
	}
	defer cleanup()

	helper.SetupHelpTemplates()
	app.Log.Debug("Starting apm…")

	setupSignalHandling()
	ctx = context.WithValue(ctx, app.AppConfigKey, appConfig)

	systemCommands := system.CommandList(ctx)
	distroboxCommands := distrobox.CommandList(ctx)
	kernelCommands := kernel.CommandList(ctx)
	repoCommands := repo.CommandList(ctx)

	cmds := []*cli.Command{
		{
			Name:   "dbus-session",
			Usage:  app.T_("Start session D-Bus service org.altlinux.APM"),
			Action: sessionDbus,
			Flags: []cli.Flag{
				&cli.BoolFlag{
					Name:    "verbose",
					Aliases: []string{"v"},
					Usage:   app.T_("Enable verbose logging to stdout"),
				},
			},
		},
		{
			Name:   "dbus-system",
			Usage:  app.T_("Start system D-Bus service org.altlinux.APM"),
			Action: systemDbus,
			Flags: []cli.Flag{
				&cli.BoolFlag{
					Name:    "verbose",
					Aliases: []string{"v"},
					Usage:   app.T_("Enable verbose logging to stdout"),
				},
			},
		},
		{
			Name:   "http-server",
			Usage:  app.T_("Start HTTP API server"),
			Action: httpServer,
			Flags: []cli.Flag{
				&cli.StringFlag{
					Name:    "listen",
					Aliases: []string{"l"},
					Usage:   app.T_("Listen address (host:port)"),
					Value:   "127.0.0.1:8080",
				},
				&cli.StringFlag{
					Name:    "socket",
					Aliases: []string{"s"},
					Usage:   app.T_("Unix socket path"),
				},
				&cli.StringFlag{
					Name:  "api-token",
					Usage: app.T_("API token for authentication"),
				},
			},
		},
		systemCommands,
		repoCommands,
	}

	if appConfig.ConfigManager.GetConfig().ExistDistrobox {
		cmds = append(cmds, distroboxCommands)
	}

	if !appConfig.ConfigManager.GetConfig().IsAtomic {
		cmds = append(cmds, kernelCommands)
	}

	cmds = append(
		cmds,
		[]*cli.Command{
			{
				Name:      "help",
				Aliases:   []string{"h"},
				Usage:     app.T_("Show the list of commands or help for each command"),
				ArgsUsage: app.T_("[command]"),
				HideHelp:  true,
			},
			{
				Name:      "version",
				Aliases:   []string{"v"},
				Usage:     app.T_("Print version"),
				ArgsUsage: app.T_("[command]"),
				Action:    printVersion,
			},
		}...,
	)

	// Основная команда приложения
	rootCommand := &cli.Command{
		Name:    "apm",
		Usage:   "Atomic Package Manager",
		Version: version.ParseVersion(appConfig.ConfigManager.GetConfig().Version).Value,
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    "format",
				Usage:   app.T_("Output format: json, text"),
				Aliases: []string{"f"},
				Value:   "text",
			},
			&cli.StringFlag{
				Name:    "transaction",
				Usage:   app.T_("Internal property, adds the transaction to the output"),
				Aliases: []string{"t"},
			},
		},
		Commands: cmds,
	}

	applyCommandSetting(rootCommand)

	if err := rootCommand.Run(ctx, os.Args); err != nil {
		os.Exit(1)
	}
}

// setupSignalHandling настраивает обработку системных сигналов
func setupSignalHandling() {
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)

	go func() {
		sig := <-sigs

		switch sig {
		case syscall.SIGINT, syscall.SIGTERM:
			infoText := fmt.Sprintf(app.T_("Recieved correct signal %s. Stopping application…"), sig)
			app.Log.Info(infoText)

			cleanup()
			cliError(errors.New(infoText))

		default:
			infoText := fmt.Sprintf(app.T_("Unexpected signal %s received. Terminating the application with an error."), sig)
			app.Log.Error(infoText)

			cleanup()
			cliError(errors.New(infoText))
		}
		code := 1
		if s, ok := sig.(syscall.Signal); ok {
			switch s {
			case syscall.SIGINT:
				code = 130
			case syscall.SIGTERM:
				code = 143
			default:
				code = 128 + int(s)
			}
		}
		os.Exit(code)
	}()
}

func applyCommandSetting(cliCommand *cli.Command) {
	cliCommand.CommandNotFound = func(ctx context.Context, cmd *cli.Command, name string) {
		appConfig.ConfigManager.SetFormat(cmd.String("format"))
		msg := fmt.Sprintf(app.T_("Unknown command: %s. See 'apm help'"), name)
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
	if cmd.Bool("verbose") {
		app.Log.EnableStdoutLogging()
	}
	if syscall.Geteuid() == 0 {
		errPermission := app.T_("Elevated rights are not allowed to perform this action. Please do not use sudo or su")
		cliError(errors.New(errPermission))
		return errors.New(errPermission)
	}
	defer cleanup()
	err := appConfig.DBusManager.ConnectSessionBus()
	if err != nil {
		app.Log.Error("ConnectSessionBus failed: ", err)
		cliError(err)
		return err
	}

	distroActions := distrobox.NewActions(appConfig)
	serviceIcon := icon.NewIconService(appConfig.DatabaseManager.GetKeyValueDB(), appConfig.ConfigManager.GetConfig().CommandPrefix)
	distroObj := distrobox.NewDBusWrapper(distroActions, serviceIcon, ctx)

	// Экспортируем в D-Bus
	if err = appConfig.DBusManager.GetConnection().Export(distroObj, "/org/altlinux/APM", "org.altlinux.APM.distrobox"); err != nil {
		return err
	}

	// Собираем интерфейсы для интроспекции
	interfaces := map[string]any{
		"org.altlinux.APM.distrobox": distroObj,
	}

	if err = appConfig.DBusManager.GetConnection().Export(
		introspect.Introspectable(dbus_doc.GenerateIntrospectXML(interfaces)),
		"/org/altlinux/APM",
		"org.freedesktop.DBus.Introspectable",
	); err != nil {
		return err
	}

	appConfig.ConfigManager.SetFormat("dbus_doc")

	// Параллельно обновляем иконки
	go func() {
		err = serviceIcon.ReloadIcons(ctx)
		if err != nil {
			app.Log.Error(err.Error())
		}
	}()

	// Блокируем до сигнала
	select {}
}

func systemDbus(ctx context.Context, cmd *cli.Command) error {
	appConfig.ConfigManager.SetFormat(cmd.String("format"))
	if cmd.Bool("verbose") {
		app.Log.EnableStdoutLogging()
	}
	if syscall.Geteuid() != 0 {
		errPermission := app.T_("Elevated rights are required to perform this action. Please use sudo or su")
		cliError(errors.New(errPermission))
		return errors.New(errPermission)
	}

	defer cleanup()
	err := appConfig.DBusManager.ConnectSystemBus()
	if err != nil {
		cliError(err)
		return err
	}

	if syscall.Geteuid() != 0 {
		return errors.New(app.T_("Administrator privileges are required to start"))
	}

	sysActions := system.NewActions(appConfig)
	conn, _ := dbus.SystemBus()

	// Экспортируем system методы в D-Bus
	sysObj := system.NewDBusWrapper(sysActions, conn, ctx)
	if err = appConfig.DBusManager.GetConnection().Export(sysObj, "/org/altlinux/APM", "org.altlinux.APM.system"); err != nil {
		return err
	}

	// Собираем интерфейсы для интроспекции
	interfaces := map[string]any{
		"org.altlinux.APM.system": sysObj,
	}

	// Экспортируем kernel методы только для не-атомарных систем
	if !appConfig.ConfigManager.GetConfig().IsAtomic {
		kernelActions := kernel.NewActions(appConfig)
		kernelObj := kernel.NewDBusWrapper(kernelActions, conn, ctx)
		if err = appConfig.DBusManager.GetConnection().Export(kernelObj, "/org/altlinux/APM", "org.altlinux.APM.kernel"); err != nil {
			return err
		}
		interfaces["org.altlinux.APM.kernel"] = kernelObj
	}

	// Экспортируем repo методы в D-Bus
	repoActions := repo.NewActions(appConfig)
	repoObj := repo.NewDBusWrapper(repoActions, conn, ctx)
	if err = appConfig.DBusManager.GetConnection().Export(repoObj, "/org/altlinux/APM", "org.altlinux.APM.repo"); err != nil {
		return err
	}
	interfaces["org.altlinux.APM.repo"] = repoObj

	if err = appConfig.DBusManager.GetConnection().Export(
		introspect.Introspectable(dbus_doc.GenerateIntrospectXML(interfaces)),
		"/org/altlinux/APM",
		"org.freedesktop.DBus.Introspectable",
	); err != nil {
		return err
	}

	appConfig.ConfigManager.SetFormat("dbus_doc")

	// Блокируем до сигнала
	select {}
}

func httpServer(ctx context.Context, cmd *cli.Command) error {
	appConfig.ConfigManager.SetFormat(cmd.String("format"))
	app.Log.EnableStdoutLogging()

	if syscall.Geteuid() != 0 {
		errPermission := app.T_("Elevated rights are required to perform this action. Please use sudo or su")
		cliError(errors.New(errPermission))
		return errors.New(errPermission)
	}

	defer cleanup()

	config := http_server.DefaultConfig()
	if listen := cmd.String("listen"); listen != "" {
		config.ListenAddr = listen
	}
	if socket := cmd.String("socket"); socket != "" {
		config.UnixSocket = socket
	}
	if token := cmd.String("api-token"); token != "" {
		config.APIToken = token
	}

	server := http_server.NewServer(config, appConfig)

	server.RegisterHealthCheck()
	server.RegisterAPIInfo(
		appConfig.ConfigManager.GetConfig().IsAtomic,
		appConfig.ConfigManager.GetConfig().ExistDistrobox,
		!appConfig.ConfigManager.GetConfig().IsAtomic,
	)

	sysActions := system.NewActions(appConfig)

	// Создаём HTTP wrapper и регистрируем маршруты напрямую
	httpWrapper := system.NewHTTPWrapper(sysActions, appConfig, ctx)
	httpWrapper.RegisterRoutes(server.GetMux(), appConfig.ConfigManager.GetConfig().IsAtomic)

	// Регистрируем endpoints в registry для OpenAPI документации и проверки прав
	registry := http_server.NewRegistry()
	registry.RegisterResponseTypes(system.GetHTTPResponseTypes())
	registry.RegisterEndpoints(system.GetHTTPEndpoints())
	server.SetRegistry(registry)

	// Регистрируем OpenAPI документацию из registry
	server.RegisterOpenAPIFromRegistry(http_server.NewOpenAPIGenerator(registry, appConfig.ConfigManager.GetConfig().Version, appConfig.ConfigManager.GetConfig().IsAtomic))

	err := server.Start(ctx)
	if err != nil {
		cliError(err)
	}
	return err
}

func printVersion(_ context.Context, _ *cli.Command) error {
	v := version.ParseVersion(appConfig.ConfigManager.GetConfig().Version)
	fmt.Printf("%s version %s\n", "apm", v.Value)
	return nil
}

func cliError(err error) {
	if err == nil {
		return
	}

	_ = reply.CliResponse(ctx, reply.APIResponse{
		Data: map[string]interface{}{
			"message": err.Error(),
		},
		Error: true,
	})
}

func cleanup() {
	if appConfig != nil {
		defer func(appConfig *app.Config) {
			closeApp(appConfig)
		}(appConfig)
	}

	defer globalCancel()
}

func closeApp(appConfig *app.Config) {
	if appConfig == nil {
		return
	}

	aptLib.WaitIdle()
	defer apt.Close()

	// Закрываем DBus соединение
	if appConfig.DBusManager != nil {
		if err := appConfig.DBusManager.Close(); err != nil {
			app.Log.Errorf(app.T_("failed to close DBus: %w"), err)
		}
	}

	// Закрываем базы данных
	if appConfig.DatabaseManager != nil {
		if err := appConfig.DatabaseManager.Close(); err != nil {
			app.Log.Errorf(app.T_("failed to close databases: %w"), err)
		}
	}
}
