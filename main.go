package main

import (
	"apm/cmd/common/helper"
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

func main() {
	lib.Log.Debugln("Starting apm")

	lib.InitConfig()
	lib.InitLogger()
	lib.InitDatabase()

	ctx := context.Background()
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		sig := <-sigs
		infoText := fmt.Sprintf("Получен сигнал %s. Завершаем работу приложения...", sig)

		lib.Log.Error(infoText)
		_ = reply.CliResponse(ctx, reply.APIResponse{
			Data: map[string]interface{}{
				"message": infoText,
			},
			Error: true,
		})

		os.Exit(0)
	}()

	rootCommand := &cli.Command{
		Name:  "apm",
		Usage: "Atomic Packages Manager",
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
					distroObj := distrobox.NewDBusWrapper(distroActions)

					if err := lib.DBUSConn.Export(distroObj, "/com/application/APM", "com.application.distrobox"); err != nil {
						return err
					}

					if err := lib.DBUSConn.Export(
						introspect.Introspectable(helper.UserIntrospectXML),
						"/com/application/APM",
						"org.freedesktop.DBus.Introspectable",
					); err != nil {
						return err
					}

					lib.Env.Format = "dbus"
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

					if err := lib.DBUSConn.Export(sysObj, "/com/application/APM", "com.application.system"); err != nil {
						return err
					}

					if err := lib.DBUSConn.Export(
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
