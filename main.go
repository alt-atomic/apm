package main

import (
	"apm/cmd/common/helper"
	"apm/cmd/common/reply"
	"apm/cmd/distrobox"
	"apm/cmd/system"
	"apm/lib"
	"context"
	"github.com/godbus/dbus/v5/introspect"
	"github.com/urfave/cli/v3"
	"os"
)

func main() {
	lib.Log.Debugln("Starting apm")

	lib.InitConfig()
	lib.InitLogger()
	lib.InitDatabase()

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
				Name:    "dbus-service",
				Usage:   "Запуск DBUS-сервиса com.application.APM",
				Aliases: []string{"dbus"},
				Action: func(ctx context.Context, cmd *cli.Command) error {
					err := lib.InitDBus()
					if err != nil {
						return err
					}

					distroActions := distrobox.NewActions()
					distroObj := distrobox.NewDBusWrapper(distroActions)

					sysActions := system.NewActions()
					sysObj := system.NewDBusWrapper(sysActions)

					if err := lib.DBUSConn.Export(distroObj, "/com/application/APM", "com.application.distrobox"); err != nil {
						return err
					}
					if err := lib.DBUSConn.Export(sysObj, "/com/application/APM", "com.application.system"); err != nil {
						return err
					}

					if err := lib.DBUSConn.Export(
						introspect.Introspectable(helper.CombinedIntrospectXML),
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
	if err := rootCommand.Run(context.Background(), os.Args); err != nil {
		lib.Log.Error(err.Error())

		_ = reply.CliResponse(reply.APIResponse{
			Data: map[string]interface{}{
				"message": err.Error(),
			},
			Error: true,
		})
	}
}
