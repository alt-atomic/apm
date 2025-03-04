package main

import (
	"apm/config"
	"apm/database"
	"apm/dbus"
	"context"
	"github.com/urfave/cli/v3"
	"os"

	"apm/cmd/distrobox"
	"apm/logger"
)

func main() {
	logger.Log.Debugln("Starting apm")

	config.InitConfig()
	logger.InitLogger()
	database.InitDatabase()
	go dbus.InitDBus()

	rootCommand := &cli.Command{
		Name:  "apm",
		Usage: "Atomic Packages Manager",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    "format",
				Usage:   "Формат вывода: json, text, dbus (com.application.APM)",
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

	if err := rootCommand.Run(context.Background(), os.Args); err != nil {
		logger.Log.Error(err.Error())
	}
}
