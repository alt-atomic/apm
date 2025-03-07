package system

import (
	"apm/cmd/common/reply"
	"apm/cmd/system/converter"
	"apm/cmd/system/service"
	"apm/lib"
	"context"
	"fmt"
	"github.com/urfave/cli/v3"
	"os"
	"syscall"
)

// newErrorResponse создаёт ответ с ошибкой и указанным сообщением.
func newErrorResponse(message string) reply.APIResponse {
	lib.Log.Error(message)

	return reply.APIResponse{
		Data:  map[string]interface{}{"message": message},
		Error: true,
	}
}

// checkRoot проверяет, запущен ли установщик от имени root
func checkRoot() error {
	if syscall.Geteuid() != 0 {
		return fmt.Errorf("для выполнения необходимы права администратора, используйте sudo или su")
	}

	return nil
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

					packageInfo, err := service.GetPackageInfo(packageName)
					if err != nil {
						return reply.CliResponse(newErrorResponse(err.Error()))
					}

					return reply.CliResponse(reply.APIResponse{
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
			{
				Name:    "image",
				Usage:   "Модуль для работы с образом",
				Aliases: []string{"i"},
				Commands: []*cli.Command{
					{
						Name:  "generate",
						Usage: "Принудительная генерация локального образа",
						Flags: []cli.Flag{
							&cli.BoolFlag{
								Name:  "switch",
								Usage: "Переключиться на локальный образ",
								Value: false,
							},
						},
						Action: withGlobalWrapper(func(ctx context.Context, cmd *cli.Command) error {
							err := checkRoot()
							if err != nil {
								return reply.CliResponse(newErrorResponse(err.Error()))
							}

							config, err := converter.ParseConfig()
							if err != nil {
								return reply.CliResponse(newErrorResponse(err.Error()))
							}

							dockerStr, err := config.GenerateDockerfile()
							if err != nil {
								return reply.CliResponse(newErrorResponse(err.Error()))
							}

							err = os.WriteFile("test.txt", []byte(dockerStr), 0644)
							if err != nil {
								return reply.CliResponse(newErrorResponse(err.Error()))
							}
							return reply.CliResponse(reply.APIResponse{
								Data: map[string]interface{}{
									"message": "Конфигурация образа",
									"config":  config,
								},
								Error: false,
							})
						}),
					},
					{
						Name:  "update",
						Usage: "Обновление образа",
						//Action: withGlobalWrapper(func(ctx context.Context, cmd *cli.Command) error {
						//	image, err := os.GetActiveImage()
						//
						//	if err != nil {
						//
						//	}
						//}),
					},
					{
						Name:  "switch",
						Usage: "Переключение образа",
						//Action: withGlobalWrapper(func(ctx context.Context, cmd *cli.Command) error {
						//
						//}),
					},
				},
			},
		},
	}
}
