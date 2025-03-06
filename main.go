package main

import (
	"apm/cmd/common/reply"
	"apm/cmd/distrobox"
	"apm/cmd/system"
	"apm/lib"
	"context"
	"github.com/urfave/cli/v3"
	"os"
)

func main() {
	//path, err := converter.ParseConfig("example.yml")
	//fmt.Println(path[1])
	//if err != nil {
	//	fmt.Println(err)
	//}
	//return
	lib.Log.Debugln("Starting apm")

	lib.InitConfig()
	lib.InitLogger()
	lib.InitDatabase()
	go lib.InitDBus()

	rootCommand := &cli.Command{
		Name:  "apm",
		Usage: "Atomic Packages Manager",
		//EnableShellCompletion: true,
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
