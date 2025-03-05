package system

import (
	"apm/cmd/common/reply"
	"apm/cmd/system/os"
	"apm/lib"
	"context"
	"github.com/urfave/cli/v3"
)

// newErrorResponse создаёт ответ с ошибкой и указанным сообщением.
func newErrorResponse(message string) reply.APIResponse {
	lib.Log.Error(message)

	return reply.APIResponse{
		Data:  map[string]interface{}{"message": message},
		Error: true,
	}
}

func withGlobalWrapper(action cli.ActionFunc) cli.ActionFunc {
	return func(ctx context.Context, cmd *cli.Command) error {
		lib.Env.Format = cmd.String("format")
		lib.Env.Transaction = cmd.String("transaction")

		reply.CreateSpinner()
		return action(ctx, cmd)
	}
}

func CommandList() *cli.Command {
	return &cli.Command{
		Name:    "system",
		Aliases: []string{"s"},
		Usage:   "Управление системными пакетами",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    "format",
				Usage:   "Формат вывода: json, text, dbus (com.application.APM)",
				Aliases: []string{"f"},
				Value:   "text",
			},
		},
		Commands: []*cli.Command{
			{
				Name:  "install",
				Usage: "Установка пакета в систему",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:     "package",
						Usage:    "Название пакета. Необходимо указать",
						Aliases:  []string{"p"},
						Required: true,
					},
				},
				//Action: withGlobalWrapper(func(ctx context.Context, cmd *cli.Command) error {
				//
				//}),
			},
			{
				Name:  "update",
				Usage: "Обновление пакетной базы",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:     "package",
						Usage:    "Название пакета. Необходимо указать",
						Aliases:  []string{"p"},
						Required: true,
					},
				},
				//Action: withGlobalWrapper(func(ctx context.Context, cmd *cli.Command) error {
				//
				//}),
			},
			{
				Name:  "info",
				Usage: "Информация о пакете",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:     "package",
						Usage:    "Название пакета. Необходимо указать",
						Aliases:  []string{"p"},
						Required: true,
					},
				},
				Action: withGlobalWrapper(func(ctx context.Context, cmd *cli.Command) error {
					packageName := cmd.String("package")

					packageInfo, err := os.GetPackageInfo(packageName)
					if err != nil {
						return reply.CliResponse(cmd, newErrorResponse(err.Error()))
					}

					return reply.CliResponse(cmd, reply.APIResponse{
						Data: map[string]interface{}{
							"message": "Информация о пакете",
							"package": packageInfo,
						},
						Error: false,
					})
				}),
			},
			{
				Name:  "search",
				Usage: "Поиск пакета по названию",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:     "package",
						Usage:    "Название пакета. Необходимо указать",
						Aliases:  []string{"p"},
						Required: true,
					},
				},
				//Action: withGlobalWrapper(func(ctx context.Context, cmd *cli.Command) error {
				//	packageInfo, error := os.GetPackageInfo()
				//
				//	return reply.CliResponse(cmd, resp)
				//}),
			},
			{
				Name:    "remove",
				Usage:   "Удаление пакета",
				Aliases: []string{"rm"},
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:     "package",
						Usage:    "Название пакета. Необходимо указать",
						Aliases:  []string{"p"},
						Required: true,
					},
				},
				//Action: withGlobalWrapper(func(ctx context.Context, cmd *cli.Command) error {
				//
				//}),
			},
		},
	}
}
