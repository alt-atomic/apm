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
				Name:      "install",
				Usage:     "Список пакетов на установку",
				ArgsUsage: "packages",
				Action: withGlobalWrapper(func(ctx context.Context, cmd *cli.Command) error {
					resp, err := NewActions().Install(ctx, cmd.Args().Slice())
					if err != nil {
						return reply.CliResponse(ctx, newErrorResponse(err.Error()))
					}

					return reply.CliResponse(ctx, resp)
				}),
			},
			{
				Name:      "remove",
				Usage:     "Список пакетов на удаление",
				Aliases:   []string{"rm"},
				ArgsUsage: "packages",
				Action: withGlobalWrapper(func(ctx context.Context, cmd *cli.Command) error {
					resp, err := NewActions().Remove(ctx, cmd.Args().Slice())
					if err != nil {
						return reply.CliResponse(ctx, newErrorResponse(err.Error()))
					}

					return reply.CliResponse(ctx, resp)
				}),
			},
			{
				Name:  "update",
				Usage: "Обновление пакетной базы",
				Action: withGlobalWrapper(func(ctx context.Context, cmd *cli.Command) error {
					resp, err := NewActions().Update(ctx)
					if err != nil {
						return reply.CliResponse(ctx, newErrorResponse(err.Error()))
					}

					return reply.CliResponse(ctx, resp)
				}),
			},
			{
				Name:      "info",
				Usage:     "Информация о пакете",
				ArgsUsage: "package",
				Action: withGlobalWrapper(func(ctx context.Context, cmd *cli.Command) error {
					resp, err := NewActions().Info(ctx, cmd.Args().First())
					if err != nil {
						return reply.CliResponse(ctx, newErrorResponse(err.Error()))
					}

					return reply.CliResponse(ctx, resp)
				}),
			},
			{
				Name:      "search",
				Usage:     "Поиск пакета по названию",
				ArgsUsage: "package",
				Flags: []cli.Flag{
					&cli.BoolFlag{
						Name:    "installed",
						Usage:   "Только установленные",
						Aliases: []string{"i"},
						Value:   false,
					},
				},
				Action: withGlobalWrapper(func(ctx context.Context, cmd *cli.Command) error {
					resp, err := NewActions().Search(ctx, cmd.Args().First(), cmd.Bool("installed"))
					if err != nil {
						return reply.CliResponse(ctx, newErrorResponse(err.Error()))
					}

					return reply.CliResponse(ctx, resp)
				}),
			},
			{
				Name:  "list",
				Usage: "Построение запроса для получения списка пакетов",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:  "sort",
						Usage: "Поле для сортировки (например, name, installed)",
					},
					&cli.StringFlag{
						Name:  "order",
						Usage: "Порядок сортировки: ASC или DESC",
						Value: "ASC",
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
					&cli.StringFlag{
						Name:  "filter-field",
						Usage: "Название поля для фильтрации (например, name, version, manager, section)",
					},
					&cli.StringFlag{
						Name:  "filter-value",
						Usage: "Значение для фильтрации по указанному полю",
					},
					&cli.BoolFlag{
						Name:  "force-update",
						Usage: "Принудительно обновить все пакеты перед запросом",
						Value: false,
					},
				},
				Action: withGlobalWrapper(func(ctx context.Context, cmd *cli.Command) error {
					params := ListParams{
						Sort:        cmd.String("sort"),
						Offset:      cmd.Int("offset"),
						Limit:       cmd.Int("limit"),
						FilterField: cmd.String("filter-field"),
						FilterValue: cmd.String("filter-value"),
						ForceUpdate: cmd.Bool("force-update"),
					}

					resp, err := NewActions().List(ctx, params)
					if err != nil {
						return reply.CliResponse(ctx, newErrorResponse(err.Error()))
					}

					return reply.CliResponse(ctx, resp)
				}),
			},
			{
				Name:    "image",
				Usage:   "Модуль для работы с образом",
				Aliases: []string{"i"},
				Hidden:  !lib.Env.IsAtomic,
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
