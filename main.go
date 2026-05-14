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
	apmcli "apm/internal/common/cli"
	"apm/internal/common/reply"
	"apm/internal/common/service"
	"apm/internal/domain/distrobox"
	"apm/internal/domain/kernel"
	"apm/internal/domain/repository"
	"apm/internal/domain/system"
	"context"
	"errors"
	"fmt"
	"os"
	"sync"

	"github.com/urfave/cli/v3"
)

const (
	defaultSystemHTTPListen  = "127.0.0.1:8080"
	defaultSessionHTTPListen = "127.0.0.1:8082"
)

type appRuntime struct {
	config   *app.Config
	reporter *reply.Reporter
	ctx      context.Context
	once     sync.Once
}

func main() {
	os.Exit(run())
}

func run() int {
	cfg, err := app.InitializeAppDefault()
	if err != nil {
		_, _ = fmt.Fprintln(os.Stderr, "Initialization error:", err)
		return 1
	}

	ctx, cancel := apmcli.InstallSignalHandler(context.Background())
	defer cancel()

	rt := &appRuntime{
		config:   cfg,
		reporter: reply.NewReporter(cfg),
		ctx:      ctx,
	}
	defer rt.cleanup()

	apmcli.SetupHelpTemplates()
	app.Log.Debug("Starting apm…")

	rootCommand := &cli.Command{
		Name:        "apm",
		Usage:       "Atomic Package Manager",
		Description: apmcli.AppDescription(),
		Version:     rt.config.ConfigManager.GetParsedVersion().Value,
		Flags:       apmcli.RootFlags(),
		Commands:    rt.buildCommands(),
	}

	apmcli.ApplyCommandSettings(rootCommand, apmcli.CommandHooks{
		OnNotFound: func(_ context.Context, cmd *cli.Command, name string) {
			rt.config.ConfigManager.SetFormat(cmd.String("format"))
			rt.cliError(fmt.Errorf(app.T_("Unknown command: %s. See 'apm help'"), name))
		},
		OnUsageError: func(err error) error {
			rt.cliError(apmcli.TranslateUsageError(err))
			return err
		},
	})

	if err := rootCommand.Run(rt.ctx, os.Args); err != nil {
		return 1
	}

	return 0
}

func (rt *appRuntime) buildCommands() []*cli.Command {
	cfg := rt.config.ConfigManager.GetConfig()

	commands := []*cli.Command{
		apmcli.NewDBusCommand("dbus-session", app.T_("Start session D-Bus service org.altlinux.APM"), rt.sessionDbus),
		apmcli.NewDBusCommand("dbus-system", app.T_("Start system D-Bus service org.altlinux.APM"), rt.systemDbus),
		apmcli.NewHTTPCommand("http-server", app.T_("Start system HTTP API server"), defaultSystemHTTPListen, rt.httpServer),
		apmcli.NewHTTPCommand("http-session", app.T_("Start session HTTP API"), defaultSessionHTTPListen, rt.httpSession),
		system.CommandList(rt.config, rt.reporter),
		repository.CommandList(rt.config, rt.reporter),
	}
	if cfg.ExistDistrobox {
		commands = append(commands, distrobox.CommandList(rt.config, rt.reporter))
	}
	if !cfg.IsAtomic {
		commands = append(commands, kernel.CommandList(rt.config, rt.reporter))
	}
	return append(commands, apmcli.HelpCommand(), apmcli.VersionCommand(rt.printVersion))
}

func (rt *appRuntime) sessionDbus(ctx context.Context, cmd *cli.Command) error {
	return rt.reportError(service.RunDBus(ctx, cmd, rt.config, service.DBusRunConfig{
		Bus:  service.BusSession,
		Mode: apmcli.ForbidRoot,
		Modules: []service.DBusModule{
			distrobox.DBusFactory(rt.config, rt.reporter),
		},
	}))
}

func (rt *appRuntime) systemDbus(ctx context.Context, cmd *cli.Command) error {
	cfg := rt.config.ConfigManager.GetConfig()
	modules := []service.DBusModule{
		system.DBusFactory(rt.config, rt.reporter),
		repository.DBusFactory(rt.config, rt.reporter),
	}
	if !cfg.IsAtomic {
		modules = append(modules, kernel.DBusFactory(rt.config, rt.reporter))
	}
	return rt.reportError(service.RunDBus(ctx, cmd, rt.config, service.DBusRunConfig{
		Bus:     service.BusSystem,
		Mode:    apmcli.RequireRoot,
		Modules: modules,
	}))
}

func (rt *appRuntime) httpServer(ctx context.Context, cmd *cli.Command) error {
	cfg := rt.config.ConfigManager.GetConfig()
	return rt.reportError(service.RunHTTP(ctx, cmd, rt.config, service.HTTPRunConfig{
		Mode: apmcli.RequireRoot,
		APIInfo: service.APIInfo{
			IsAtomic:     cfg.IsAtomic,
			HasDistrobox: cfg.ExistDistrobox,
			HasKernel:    !cfg.IsAtomic,
		},
		Modules: []service.HTTPModule{
			system.HTTPFactory(rt.config, rt.reporter, cfg.IsAtomic),
			repository.HTTPFactory(rt.config, rt.reporter),
		},
	}))
}

func (rt *appRuntime) httpSession(ctx context.Context, cmd *cli.Command) error {
	if !rt.config.ConfigManager.GetConfig().ExistDistrobox {
		return rt.reportError(errors.New(app.T_("Distrobox is not installed")))
	}
	return rt.reportError(service.RunHTTP(ctx, cmd, rt.config, service.HTTPRunConfig{
		Mode:    apmcli.ForbidRoot,
		APIInfo: service.APIInfo{HasDistrobox: true},
		Modules: []service.HTTPModule{
			distrobox.HTTPFactory(rt.config, rt.reporter),
		},
	}))
}

func (rt *appRuntime) reportError(err error) error {
	if err != nil {
		rt.cliError(err)
	}
	return err
}

func (rt *appRuntime) printVersion(_ context.Context, _ *cli.Command) error {
	fmt.Printf("%s version %s\n", "apm", rt.config.ConfigManager.GetParsedVersion().Value)
	return nil
}

func (rt *appRuntime) cliError(err error) {
	if err == nil {
		return
	}
	_ = rt.reporter.CliResponse(rt.ctx, reply.ErrorResponseFromError(err))
}

func (rt *appRuntime) cleanup() {
	rt.once.Do(func() {
		if rt.config != nil {
			reply.StopSpinner(rt.config)
			closeApp(rt.config)
		}
	})
}

func closeApp(appConfig *app.Config) {
	if appConfig == nil {
		return
	}

	aptLib.WaitIdle()
	defer apt.Close()

	if appConfig.DBusManager != nil {
		if err := appConfig.DBusManager.Close(); err != nil {
			app.Log.Errorf(app.T_("failed to close DBus: %w"), err)
		}
	}
	if appConfig.DatabaseManager != nil {
		if err := appConfig.DatabaseManager.Close(); err != nil {
			app.Log.Errorf(app.T_("failed to close databases: %w"), err)
		}
	}
}
