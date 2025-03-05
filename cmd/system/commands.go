package system

import (
	"apm/cmd/distrobox/dbus_event"
	"apm/lib"
	"context"
	"encoding/json"
	"fmt"
	"github.com/urfave/cli/v3"
	"time"
)

// APIResponse описывает формат ответа
type APIResponse struct {
	Data        interface{} `json:"data"`
	Error       bool        `json:"error"`
	Transaction string      `json:"transaction,omitempty"`
}

// newErrorResponse создаёт ответ с ошибкой и указанным сообщением.
func newErrorResponse(message string) APIResponse {
	lib.Log.Error(message)

	return APIResponse{
		Data:  map[string]interface{}{"message": message},
		Error: true,
	}
}

func response(cmd *cli.Command, resp APIResponse) error {
	format := cmd.String("format")
	resp.Transaction = cmd.String("transaction")

	switch format {
	case "dbus":
		if !resp.Error {
			if dataMap, ok := resp.Data.(map[string]interface{}); ok {
				delete(dataMap, "message")
			}
		}

		b, err := json.MarshalIndent(resp, "", "  ")
		if err != nil {
			return err
		}
		dbus_event.SendNotificationResponse(string(b))
		fmt.Println(string(b))
		time.Sleep(200 * time.Millisecond)
	case "json":
		if !resp.Error {
			if dataMap, ok := resp.Data.(map[string]interface{}); ok {
				delete(dataMap, "message")
			}
		}

		b, err := json.MarshalIndent(resp, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(b))
	default:
		var message string
		switch data := resp.Data.(type) {
		case map[string]interface{}:
			if m, ok := data["message"]; ok {
				if str, ok := m.(string); ok {
					message = str
				} else {
					message = fmt.Sprintf("%v", m)
				}
			}
		case map[string]string:
			message = data["message"]
		case string:
			message = data
		default:
			message = fmt.Sprintf("%v", resp.Data)
		}
		fmt.Println(message)
	}

	return nil
}

func withGlobalWrapper(action cli.ActionFunc) cli.ActionFunc {
	return func(ctx context.Context, cmd *cli.Command) error {
		dbus_event.FORMAT = cmd.String("format")
		dbus_event.TRANSACTION = cmd.String("transaction")
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
				//Action: withGlobalWrapper(func(ctx context.Context, cmd *cli.Command) error {
				//
				//}),
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
				Name:  "rm",
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
		},
	}
}
