package distrobox

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
		Name:    "distrobox",
		Aliases: []string{"d"},
		Usage:   "Управление пакетами и контейнерами и контейнерами distrobox",
		Commands: []*cli.Command{
			{
				Name:  "update",
				Usage: "Обновить и синхронизировать списки установленных пакетов с хостом",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:     "container",
						Usage:    "Название контейнера",
						Required: true,
						Aliases:  []string{"c"},
					},
				},
				Action: withGlobalWrapper(func(ctx context.Context, cmd *cli.Command) error {
					resp, err := NewActions().Update(ctx, cmd.String("container"))
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
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:     "container",
						Usage:    "Название контейнера. Необходимо указать",
						Aliases:  []string{"c"},
						Required: true,
					},
				},
				Action: withGlobalWrapper(func(ctx context.Context, cmd *cli.Command) error {
					resp, err := NewActions().Info(ctx, cmd.String("container"), cmd.Args().First())
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
					&cli.StringFlag{
						Name:     "container",
						Usage:    "Название контейнера. Необходимо указать",
						Aliases:  []string{"c"},
						Required: true,
					},
				},
				Action: withGlobalWrapper(func(ctx context.Context, cmd *cli.Command) error {
					resp, err := NewActions().Search(ctx, cmd.String("container"), cmd.Args().First())
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
						Name:     "container",
						Usage:    "Название контейнера. Необходимо указать",
						Aliases:  []string{"c"},
						Required: true,
					},
					&cli.StringFlag{
						Name:  "sort",
						Usage: "Поле для сортировки (например, packageName, version)",
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
						Usage: "Название поля для фильтрации (например, packageName, version, manager)",
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
						Container:   cmd.String("container"),
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
				Name:      "install",
				Usage:     "Установить пакет",
				ArgsUsage: "package",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:     "container",
						Usage:    "Название контейнера. Необходимо указать",
						Aliases:  []string{"c"},
						Required: true,
					},
					&cli.BoolFlag{
						Name:  "export",
						Usage: "Экспортировать пакет",
						Value: true,
					},
				},
				Action: withGlobalWrapper(func(ctx context.Context, cmd *cli.Command) error {
					resp, err := NewActions().Install(ctx, cmd.String("container"), cmd.Args().First(), cmd.Bool("export"))
					if err != nil {
						return reply.CliResponse(ctx, newErrorResponse(err.Error()))
					}

					return reply.CliResponse(ctx, resp)
				}),
			},
			{
				Name:      "remove",
				Usage:     "Удалить пакет",
				ArgsUsage: "package",
				Aliases:   []string{"rm"},
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:     "container",
						Usage:    "Название контейнера. Необходимо указать",
						Aliases:  []string{"c"},
						Required: true,
					},
					&cli.BoolFlag{
						Name:  "only-export",
						Usage: "Удалить только экспорт, оставить пакет в контейнере",
						Value: false,
					},
				},
				Action: withGlobalWrapper(func(ctx context.Context, cmd *cli.Command) error {
					resp, err := NewActions().Remove(ctx, cmd.String("container"), cmd.Args().First(), cmd.Bool("only-export"))
					if err != nil {
						return reply.CliResponse(ctx, newErrorResponse(err.Error()))
					}

					return reply.CliResponse(ctx, resp)
				}),
			},
			{
				Name:    "container",
				Usage:   "Модуль для работы с контейнерами",
				Aliases: []string{"c"},
				Commands: []*cli.Command{
					{
						Name:  "list",
						Usage: "Список контейнеров",
						Action: withGlobalWrapper(func(ctx context.Context, cmd *cli.Command) error {
							resp, err := NewActions().ContainerList(ctx)
							if err != nil {
								return reply.CliResponse(ctx, newErrorResponse(err.Error()))
							}

							return reply.CliResponse(ctx, resp)
						}),
					},
					{
						Name:  "add",
						Usage: "Добавить контейнер",
						Flags: []cli.Flag{
							&cli.StringFlag{
								Name:     "image",
								Usage:    "Ссылка на образ. Необходимо указать",
								Required: true,
							},
							&cli.StringFlag{
								Name:     "name",
								Usage:    "Название контейнера. Необходимо указать",
								Required: true,
							},
							&cli.StringFlag{
								Name:  "additional-packages",
								Usage: "Список пакетов для установки",
								Value: "zsh",
							},
							&cli.StringFlag{
								Name:  "init-hooks",
								Usage: "Вызов хука для выполнения команд",
							},
						},
						Action: withGlobalWrapper(func(ctx context.Context, cmd *cli.Command) error {
							imageVal := cmd.String("image")
							nameVal := cmd.String("name")
							addPkgVal := cmd.String("additional-packages")
							hookVal := cmd.String("init-hooks")

							resp, err := NewActions().ContainerAdd(ctx, imageVal, nameVal, addPkgVal, hookVal)
							if err != nil {
								return reply.CliResponse(ctx, newErrorResponse(err.Error()))
							}

							return reply.CliResponse(ctx, resp)
						}),
					},
					{
						Name:    "remove",
						Usage:   "Удалить контейнер",
						Aliases: []string{"rm"},
						Flags: []cli.Flag{
							&cli.StringFlag{
								Name:     "name",
								Usage:    "Название контейнера. Необходимо указать",
								Required: true,
							},
						},
						Action: withGlobalWrapper(func(ctx context.Context, cmd *cli.Command) error {
							resp, err := NewActions().ContainerRemove(ctx, cmd.String("name"))
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
