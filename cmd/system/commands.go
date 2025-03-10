package system

import (
	"apm/cmd/common/reply"
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
		ctx = context.WithValue(ctx, "transaction", cmd.String("transaction"))

		reply.CreateSpinner()
		return action(ctx, cmd)
	}
}

func CommandList() *cli.Command {
	return &cli.Command{
		Name:    "system",
		Aliases: []string{"s"},
		Usage:   "Управление системными пакетами",
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
				Action: withGlobalWrapper(func(ctx context.Context, cmd *cli.Command) error {
					resp, err := NewActions().Install(ctx, cmd.String("package"))
					if err != nil {
						return reply.CliResponse(ctx, newErrorResponse(err.Error()))
					}

					return reply.CliResponse(ctx, resp)
				}),
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
				//Action: withGlobalWrapper(func(ctx context.Context, cmd *cli.Command) error {
				//	packageName := cmd.String("package")
				//
				//	packageInfo, err := service.GetPackageInfo(packageName)
				//	if err != nil {
				//		return reply.CliResponse(newErrorResponse(err.Error()))
				//	}
				//
				//	return reply.CliResponse(reply.APIResponse{
				//		Data: map[string]interface{}{
				//			"message": "Информация о пакете",
				//			"package": packageInfo,
				//		},
				//		Error: false,
				//	})
				//}),
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
			{
				Name:    "image",
				Usage:   "Модуль для работы с образом",
				Aliases: []string{"i"},
				Commands: []*cli.Command{
					{
						Name:  "apply",
						Usage: "Применить изменения к хосту",
						Action: withGlobalWrapper(func(ctx context.Context, cmd *cli.Command) error {
							resp, err := NewActions().ImageApply(ctx)
							if err != nil {
								return reply.CliResponse(ctx, newErrorResponse(err.Error()))
							}

							return reply.CliResponse(ctx, resp)
						}),
					},
					{
						Name:  "status",
						Usage: "Статус образа",
						Action: withGlobalWrapper(func(ctx context.Context, cmd *cli.Command) error {
							resp, err := NewActions().ImageStatus(ctx)
							if err != nil {
								return reply.CliResponse(ctx, newErrorResponse(err.Error()))
							}

							return reply.CliResponse(ctx, resp)
						}),
					},
					{
						Name:  "update",
						Usage: "Обновление образа",
						Action: withGlobalWrapper(func(ctx context.Context, cmd *cli.Command) error {
							resp, err := NewActions().ImageUpdate(ctx)
							if err != nil {
								return reply.CliResponse(ctx, newErrorResponse(err.Error()))
							}

							return reply.CliResponse(ctx, resp)
						}),
					},
					{
						Name:  "history",
						Usage: "История изменений образа",
						Flags: []cli.Flag{
							&cli.StringFlag{
								Name:  "image",
								Usage: "Фильтрация по названию образа",
							},
							&cli.IntFlag{
								Name:  "limit",
								Usage: "Лимит выборки",
								Value: 10,
							},
							&cli.IntFlag{
								Name:  "offset",
								Usage: "Смещение выборки",
								Value: 0,
							},
						},
						Action: withGlobalWrapper(func(ctx context.Context, cmd *cli.Command) error {
							resp, err := NewActions().ImageHistory(ctx, cmd.String("image"), cmd.Int("limit"), cmd.Int("offset"))
							if err != nil {
								return reply.CliResponse(ctx, newErrorResponse(err.Error()))
							}

							return reply.CliResponse(ctx, resp)
						}),
					},
				},
			},
		},
	}
}
