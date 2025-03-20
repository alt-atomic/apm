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
	"apm/cmd/common/helper"
	"apm/cmd/common/icon"
	"apm/cmd/common/reply"
	"apm/cmd/distrobox"
	"apm/cmd/system"
	"apm/lib"
	"context"
	"fmt"
	"github.com/godbus/dbus/v5/introspect"
	"github.com/urfave/cli/v3"
	"os"
	"os/signal"
	"syscall"
)

var (
	ctx, globalCancel = context.WithCancel(context.Background())
)

func main() {
	defer cleanup()
	lib.Log.Debugln("Starting apm")

	lib.InitConfig()
	lib.InitLogger()
	lib.InitDatabase()
	lib.InitLocales()

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)

	go func() {
		sig := <-sigs
		switch sig {
		case syscall.SIGINT, syscall.SIGTERM:
			infoText := fmt.Sprintf("Получен корректный сигнал %s. Завершаем работу приложения", sig)
			lib.Log.Info(infoText)
			_ = reply.CliResponse(ctx, reply.APIResponse{
				Data: map[string]interface{}{
					"message": infoText,
				},
				Error: false,
			})
		default:
			infoText := fmt.Sprintf("Получен неожиданный сигнал %s. Завершаем работу приложения с ошибкой", sig)
			lib.Log.Error(infoText)
			_ = reply.CliResponse(ctx, reply.APIResponse{
				Data: map[string]interface{}{
					"message": infoText,
				},
				Error: true,
			})
		}

		cleanup()
		os.Exit(0)
	}()

	rootCommand := &cli.Command{
		Name:  "apm",
		Usage: "Atomic Package Manager",
		//EnableShellCompletion: true,
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    "format",
				Usage:   "Формат вывода: json, text",
				Aliases: []string{"f"},
				Value:   "text",
			},
			&cli.StringFlag{
				Name:    "transaction",
				Usage:   "Внутреннее свойство, добавляет транзакцию к выводу",
				Aliases: []string{"t"},
			},
		},
		Commands: []*cli.Command{
			{
				Name:  "dbus-user",
				Usage: "Запуск DBUS-сервиса com.application.APM",
				Action: func(ctx context.Context, cmd *cli.Command) error {
					err := lib.InitDBus(false)
					if err != nil {
						return err
					}

					distroActions := distrobox.NewActions()
					serviceIcon := icon.NewIconService(lib.GetDBKv())
					distroObj := distrobox.NewDBusWrapper(distroActions, serviceIcon)

					if err = lib.DBUSConn.Export(distroObj, "/com/application/APM", "com.application.distrobox"); err != nil {
						return err
					}

					if err = lib.DBUSConn.Export(
						introspect.Introspectable(helper.UserIntrospectXML),
						"/com/application/APM",
						"org.freedesktop.DBus.Introspectable",
					); err != nil {
						return err
					}

					lib.Env.Format = "dbus"

					go func() {
						err = serviceIcon.ReloadIcons(ctx)
						if err != nil {
							lib.Log.Error(err.Error())
						}
					}()

					select {}
				},
			},
			{
				Name:  "dbus-system",
				Usage: "Запуск DBUS-сервиса com.application.APM",
				Action: func(ctx context.Context, cmd *cli.Command) error {
					err := lib.InitDBus(true)
					if err != nil {
						return err
					}

					if syscall.Geteuid() != 0 {
						return fmt.Errorf("для запуска необходимы права администратора")
					}

					sysActions := system.NewActions()
					sysObj := system.NewDBusWrapper(sysActions)

					if err = lib.DBUSConn.Export(sysObj, "/com/application/APM", "com.application.system"); err != nil {
						return err
					}

					if err = lib.DBUSConn.Export(
						introspect.Introspectable(helper.SystemIntrospectXML),
						"/com/application/APM",
						"org.freedesktop.DBus.Introspectable",
					); err != nil {
						return err
					}

					lib.Env.Format = "dbus"

					select {}
				},
			},
			system.CommandList(),
			distrobox.CommandList(),
			{
				Name:      "help",
				Aliases:   []string{"h"},
				Usage:     "Показать список команд или справку по каждой команде",
				ArgsUsage: "[command]",
				HideHelp:  true,
			},
		},
	}

	rootCommand.Suggest = true
	if err := rootCommand.Run(ctx, os.Args); err != nil {
		lib.Log.Error(err.Error())

		_ = reply.CliResponse(ctx, reply.APIResponse{
			Data: map[string]interface{}{
				"message": err.Error(),
			},
			Error: true,
		})
	}
}

func cleanup() {
	lib.Log.Debugln("Завершаем работу приложения. Освобождаем ресурсы...")

	defer globalCancel()
	if dbKV := lib.CheckDBKv(); dbKV != nil {
		if err := dbKV.Close(); err != nil {
			lib.Log.Error("Ошибка при закрытии базы данных KV: ", err)
		}
	}

	if dbSQL := lib.CheckDB(); dbSQL != nil {
		if err := dbSQL.Close(); err != nil {
			lib.Log.Error("Ошибка при закрытии базы данных SQL: ", err)
		}
	}
}
