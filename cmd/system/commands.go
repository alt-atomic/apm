package system

import (
	"apm/cmd/common/reply"
	"apm/lib"
	"context"
	"github.com/urfave/cli/v3"
	"time"
)

// newErrorResponse создаёт ответ с ошибкой и указанным сообщением.
func newErrorResponse(message string) reply.APIResponse {
	lib.Log.Error(message)

	return reply.APIResponse{
		Data:  map[string]interface{}{"message": message},
		Error: true,
	}
}

func withSpinnerWrapper(action cli.ActionFunc) cli.ActionFunc {
	return func(ctx context.Context, cmd *cli.Command) error {
		lib.Env.Format = cmd.String("format")
		lib.Env.Transaction = cmd.String("transaction")

		reply.CreateSpinner()
		err := action(ctx, cmd)
		reply.StopSpinner()

		return err
	}
}

func withGlobalWrapper(action cli.ActionFunc) cli.ActionFunc {
	return func(ctx context.Context, cmd *cli.Command) error {

		lib.Env.Format = cmd.String("format")
		lib.Env.Transaction = cmd.String("transaction")
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
				Usage: "Обновить и синхронизировать списки установленных пакетов с хостом",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:     "container",
						Usage:    "Название контейнера",
						Required: true,
						Aliases:  []string{"c"},
					},
				},
				Action: withSpinnerWrapper(func(ctx context.Context, cmd *cli.Command) error {
					// Спиннер уже крутится
					type Test struct {
						Container string `json:"container"`
						Message   string `json:"message"`
					}
					// Долгая работа
					time.Sleep(3 * time.Second)

					// Теперь ничего не ломается при выводе
					resp := reply.APIResponse{
						Data: map[string]interface{}{
							"message": Test{Container: "123", Message: "123"},
							"test":    Test{Container: "2", Message: "3"},
						},
						Error: false,
					}

					return reply.CliResponse(cmd, resp)
				}),
			},
			{
				Name:  "update",
				Usage: "Информация о пакете",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:     "container",
						Usage:    "Название контейнера",
						Aliases:  []string{"c"},
						Required: true,
					},
				},
				//Action: withGlobalWrapper(func(ctx context.Context, cmd *cli.Command) error {
				//
				//}),
			},
			{
				Name:  "search",
				Usage: "Поиск пакета по названию",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:     "container",
						Usage:    "Название контейнера",
						Aliases:  []string{"c"},
						Required: true,
					},
				},
				//Action: withGlobalWrapper(func(ctx context.Context, cmd *cli.Command) error {
				//
				//}),
			},
			{
				Name:    "remove",
				Usage:   "Поиск пакета по названию",
				Aliases: []string{"rm"},
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:     "container",
						Usage:    "Название контейнера",
						Aliases:  []string{"c"},
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
