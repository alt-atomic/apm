package system

import (
	"apm/cmd/common/reply"
	"apm/lib"
	"context"
	"fmt"
	"github.com/urfave/cli/v3"
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

// checkRoot проверяет, запущен ли софт от имени root
func checkRoot() error {
	if syscall.Geteuid() != 0 {
		return fmt.Errorf("для выполнения необходимы права администратора, используйте sudo или su")
	}

	return nil
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
						Name:  "switch-local",
						Usage: "Ручное переключение на локальный образ",
						Action: withGlobalWrapper(func(ctx context.Context, cmd *cli.Command) error {
							resp, err := NewActions().ImageSwitchLocal(ctx)
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
				},
			},
		},
	}
}
