// Atomic Package Manager
// Copyright (C) 2025 Дмитрий Удалов dmitry@udalov.online
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package repo

import (
	"apm/internal/common/app"
	"apm/internal/common/reply"
	"apm/internal/common/wrapper"
	"context"
	"fmt"

	"github.com/urfave/cli/v3"
)

// newErrorResponse создаёт ответ с ошибкой и указанным сообщением
func newErrorResponse(message string) reply.APIResponse {
	app.Log.Error(message)

	return reply.APIResponse{
		Data:  map[string]interface{}{"message": message},
		Error: true,
	}
}

var withGlobalWrapper = wrapper.WithOptions(wrapper.NoRootCheck, NewActions, newErrorResponse)
var withRootCheckWrapper = wrapper.WithOptions(wrapper.RequireRoot, NewActions, newErrorResponse)

// completeBranches возвращает функцию автодополнения для веток
func completeBranches() func(ctx context.Context, cmd *cli.Command) {
	return func(ctx context.Context, cmd *cli.Command) {
		appConfig := app.GetAppConfig(ctx)
		actions := NewActions(appConfig)
		branches := actions.repoService.GetBranches()
		for _, branch := range branches {
			fmt.Println(branch)
		}
	}
}

// CommandList возвращает команду repo со всеми подкомандами
func CommandList(ctx context.Context) *cli.Command {
	appConfig := app.GetAppConfig(ctx)

	return &cli.Command{
		Name:            "repo",
		Aliases:         []string{"r"},
		Usage:           app.T_("Repository management"),
		HideHelpCommand: true,
		Commands: []*cli.Command{
			{
				Name:      "list",
				Usage:     app.T_("List repositories"),
				ArgsUsage: "[-a]",
				Flags: []cli.Flag{
					&cli.BoolFlag{
						Name:    "all",
						Usage:   app.T_("Show all repositories including inactive"),
						Aliases: []string{"a"},
						Value:   false,
					},
				},
				Action: withGlobalWrapper(func(ctx context.Context, cmd *cli.Command, actions *Actions) error {
					resp, err := actions.List(ctx, cmd.Bool("all"))
					if err != nil {
						return reply.CliResponse(ctx, newErrorResponse(err.Error()))
					}
					return reply.CliResponse(ctx, *resp)
				}),
			},
			{
				Name:      "add",
				Usage:     app.T_("Add repository (branch/task/URL). For branch archive: add <branch> <date>"),
				ArgsUsage: "<source> [YYYYMMDD|YYYY/MM/DD]",
				Flags: []cli.Flag{
					&cli.BoolFlag{
						Name:    "simulate",
						Usage:   app.T_("Simulate adding without making changes"),
						Aliases: []string{"s"},
						Value:   false,
					},
				},
				Action: withRootCheckWrapper(func(ctx context.Context, cmd *cli.Command, actions *Actions) error {
					var source string
					for i := 0; i < cmd.NArg(); i++ {
						if i > 0 {
							source += " "
						}
						source += cmd.Args().Get(i)
					}

					resp, err := actions.Add(ctx, source, cmd.Bool("simulate"))
					if err != nil {
						return reply.CliResponse(ctx, newErrorResponse(err.Error()))
					}
					return reply.CliResponse(ctx, *resp)
				}),
				ShellComplete: completeBranches(),
			},
			{
				Name:      "remove",
				Aliases:   []string{"rm"},
				Usage:     app.T_("Remove repository. Use 'all' to remove all repositories, or specify branch/task/URL to remove specific repository"),
				ArgsUsage: "<source|all>",
				Flags: []cli.Flag{
					&cli.BoolFlag{
						Name:    "simulate",
						Usage:   app.T_("Simulate removal without making changes"),
						Aliases: []string{"s"},
						Value:   false,
					},
				},
				Action: withRootCheckWrapper(func(ctx context.Context, cmd *cli.Command, actions *Actions) error {
					var source string
					for i := 0; i < cmd.NArg(); i++ {
						if i > 0 {
							source += " "
						}
						source += cmd.Args().Get(i)
					}

					resp, err := actions.Remove(ctx, source, cmd.Bool("simulate"))
					if err != nil {
						return reply.CliResponse(ctx, newErrorResponse(err.Error()))
					}
					return reply.CliResponse(ctx, *resp)
				}),
				ShellComplete: completeBranches(),
			},
			{
				Name:      "set",
				Usage:     app.T_("Set branch (removes all existing and adds specified branch). For branch archive: set <branch> <date>"),
				ArgsUsage: "<branch> [YYYYMMDD|YYYY/MM/DD]",
				Flags: []cli.Flag{
					&cli.BoolFlag{
						Name:    "simulate",
						Usage:   app.T_("Simulate setting without making changes"),
						Aliases: []string{"s"},
						Value:   false,
					},
				},
				Action: withRootCheckWrapper(func(ctx context.Context, cmd *cli.Command, actions *Actions) error {
					var branch string
					for i := 0; i < cmd.NArg(); i++ {
						if i > 0 {
							branch += " "
						}
						branch += cmd.Args().Get(i)
					}

					resp, err := actions.Set(ctx, branch, cmd.Bool("simulate"))
					if err != nil {
						return reply.CliResponse(ctx, newErrorResponse(err.Error()))
					}
					return reply.CliResponse(ctx, *resp)
				}),
				ShellComplete: completeBranches(),
			},
			{
				Name:  "clean",
				Usage: app.T_("Remove all cdrom and task repositories"),
				Flags: []cli.Flag{
					&cli.BoolFlag{
						Name:    "simulate",
						Usage:   app.T_("Simulate cleaning without making changes"),
						Aliases: []string{"s"},
						Value:   false,
					},
				},
				Action: withRootCheckWrapper(func(ctx context.Context, cmd *cli.Command, actions *Actions) error {
					resp, err := actions.Clean(ctx, cmd.Bool("simulate"))
					if err != nil {
						return reply.CliResponse(ctx, newErrorResponse(err.Error()))
					}
					return reply.CliResponse(ctx, *resp)
				}),
			},
			{
				Name:  "branches",
				Usage: app.T_("List available branches"),
				Action: withGlobalWrapper(func(ctx context.Context, cmd *cli.Command, actions *Actions) error {
					resp, err := actions.GetBranches(ctx)
					if err != nil {
						return reply.CliResponse(ctx, newErrorResponse(err.Error()))
					}
					return reply.CliResponse(ctx, *resp)
				}),
			},
			{
				Name:      "task",
				Usage:     app.T_("Show packages in task"),
				ArgsUsage: "<task_number>",
				Action: withGlobalWrapper(func(ctx context.Context, cmd *cli.Command, actions *Actions) error {
					resp, err := actions.GetTaskPackages(ctx, cmd.Args().First())
					if err != nil {
						return reply.CliResponse(ctx, newErrorResponse(err.Error()))
					}
					return reply.CliResponse(ctx, *resp)
				}),
			},
			{
				Name:  "dbus-doc",
				Usage: app.T_("Show dbus online documentation"),
				Action: withGlobalWrapper(func(ctx context.Context, cmd *cli.Command, actions *Actions) error {
					reply.StopSpinner(appConfig)
					return actions.GenerateOnlineDoc(ctx)
				}),
			},
		},
	}
}
