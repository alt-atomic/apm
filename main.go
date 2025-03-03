package main

import (
	"apm/config"
	"apm/database"
	"context"
	"github.com/urfave/cli/v3"
	"os"

	"apm/cmd/distrobox"
	"apm/logger"
)

func main() {
	config.InitConfig()
	logger.InitLogger()
	database.InitDatabase()

	logger.Log.Debugln("Starting apm")
	rootCommand := &cli.Command{
		Name:  "apm",
		Usage: "Atomic Packages Manager",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    "format",
				Usage:   "Формат вывода: json, text",
				Aliases: []string{"f"},
				Value:   "text",
			},
		},
		Commands: []*cli.Command{
			distrobox.CommandList(),
			{
				Name:      "help",
				Aliases:   []string{"h"},
				Usage:     "Показывать список команд или справку по каждой команде",
				ArgsUsage: "[command]",
				HideHelp:  true,
			},
		},
	}

	if err := rootCommand.Run(context.Background(), os.Args); err != nil {
		logger.Log.Error(err.Error())
	}
}
