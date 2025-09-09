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

package system

import (
	"apm/internal/common/config"
	"apm/internal/common/helper"
	"apm/internal/common/reply"
	"context"
	"fmt"
	"strings"
	"syscall"

	"github.com/urfave/cli/v3"
)

// newErrorResponse создаёт ответ с ошибкой и указанным сообщением.
func newErrorResponse(ctx context.Context, message string) reply.APIResponse {
	appConfig := config.GetAppConfig(ctx)
	appConfig.Logger.Error(message)

	return reply.APIResponse{
		Data:  map[string]interface{}{"message": message},
		Error: true,
	}
}

func findPkgWithInstalled(installed bool) func(ctx context.Context, cmd *cli.Command) {
	return func(ctx context.Context, cmd *cli.Command) {
		appConfig := config.GetAppConfig(ctx)
		args := cmd.Args().Slice()

		// Текущий токен — последний позиционный аргумент (если есть)
		var currentToken string
		if len(args) > 0 {
			currentToken = args[len(args)-1]
		}
		currentToken = strings.TrimSpace(currentToken)
		if currentToken == "" {
			// Пользователь ещё ничего не ввёл — не предлагаем варианты
			return
		}

		base := strings.TrimRight(currentToken, "+-")
		if base == "" {
			return
		}
		suffix := currentToken[len(base):]

		exclude := make(map[string]struct{}, len(args))
		for i := 0; i < len(args)-1; i++ {
			exclude[strings.TrimRight(strings.TrimSpace(args[i]), "+-")] = struct{}{}
		}

		like := base + "%"
		svc := NewActions(appConfig).serviceAptDatabase
		if svc == nil {
			return
		}

		pkgs, _ := svc.SearchPackagesMultiLimit(ctx, like, 200, installed)

		// Избегаем самоповторов и дубликатов в выводе
		printed := make(map[string]struct{}, len(pkgs))
		for _, p := range pkgs {
			if _, seen := exclude[p.Name]; seen {
				continue
			}
			candidate := p.Name + suffix
			if strings.EqualFold(candidate, currentToken) {
				continue
			}
			if _, dup := printed[candidate]; dup {
				continue
			}
			printed[candidate] = struct{}{}
			fmt.Println(candidate)
		}
	}
}

// findPkgInfoOnlyFirstArg — как обычный поиск, но только для первого аргумента
func findPkgInfoOnlyFirstArg() func(ctx context.Context, cmd *cli.Command) {
	return func(ctx context.Context, cmd *cli.Command) {
		if cmd.NArg() >= 2 {
			return
		}
		findPkgWithInstalled(false)(ctx, cmd)
	}
}

func wrapperWithOptions(requireRoot bool) func(func(context.Context, *cli.Command, *Actions) error) cli.ActionFunc {
	return func(actionFunc func(context.Context, *cli.Command, *Actions) error) cli.ActionFunc {
		return func(ctx context.Context, cmd *cli.Command) error {
			appConfig := config.GetAppConfig(ctx)
			appConfig.ConfigManager.SetFormat(cmd.String("format"))
			ctx = context.WithValue(ctx, helper.TransactionKey, cmd.String("transaction"))

			if requireRoot && syscall.Geteuid() != 0 {
				return reply.CliResponse(ctx, newErrorResponse(ctx, appConfig.Translator.T_("Elevated rights are required to perform this action. Please use sudo or su")))
			}

			actions := NewActions(appConfig)

			reply.CreateSpinner()
			return actionFunc(ctx, cmd, actions)
		}
	}
}

var withGlobalWrapper = wrapperWithOptions(false)
var withRootCheckWrapper = wrapperWithOptions(true)

func CommandList(ctx context.Context) *cli.Command {
	appConfig := config.GetAppConfig(ctx)

	return &cli.Command{
		Name:            "system",
		Aliases:         []string{"s"},
		Usage:           appConfig.Translator.T_("System package management"),
		HideHelpCommand: true,
		Commands: []*cli.Command{
			{
				Name:      "install",
				Usage:     appConfig.Translator.T_("Package list for installation. The format package- package+ is supported."),
				ArgsUsage: "packages",
				Action: withRootCheckWrapper(func(ctx context.Context, cmd *cli.Command, actions *Actions) error {
					resp, err := actions.Install(ctx, cmd.Args().Slice())
					if err != nil {
						return reply.CliResponse(ctx, newErrorResponse(ctx, err.Error()))
					}

					return reply.CliResponse(ctx, *resp)
				}),
				ShellComplete: findPkgWithInstalled(false),
			},
			{
				Name:      "remove",
				Usage:     appConfig.Translator.T_("List of packages to remove"),
				ArgsUsage: "packages",
				Flags: []cli.Flag{
					&cli.BoolFlag{
						Name:    "purge",
						Usage:   appConfig.Translator.T_("Delete all files"),
						Aliases: []string{"p"},
						Value:   false,
					},
				},
				Action: withRootCheckWrapper(func(ctx context.Context, cmd *cli.Command, actions *Actions) error {
					resp, err := actions.Remove(ctx, cmd.Args().Slice(), cmd.Bool("purge"))
					if err != nil {
						return reply.CliResponse(ctx, newErrorResponse(ctx, err.Error()))
					}

					return reply.CliResponse(ctx, *resp)
				}),
				ShellComplete: findPkgWithInstalled(true),
			},
			{
				Name:  "update",
				Usage: appConfig.Translator.T_("Updating package database"),
				Action: withRootCheckWrapper(func(ctx context.Context, cmd *cli.Command, actions *Actions) error {
					resp, err := actions.Update(ctx)
					if err != nil {
						return reply.CliResponse(ctx, newErrorResponse(ctx, err.Error()))
					}

					return reply.CliResponse(ctx, *resp)
				}),
			},
			{
				Name:  "upgrade",
				Usage: appConfig.Translator.T_("General system upgrade"),
				Action: withRootCheckWrapper(func(ctx context.Context, cmd *cli.Command, actions *Actions) error {
					var resp *reply.APIResponse
					var err error
					if appConfig.ConfigManager.GetConfig().IsAtomic {
						resp, err = actions.ImageUpdate(ctx)
					} else {
						resp, err = actions.Upgrade(ctx)
					}

					if err != nil {
						return reply.CliResponse(ctx, newErrorResponse(ctx, err.Error()))
					}

					return reply.CliResponse(ctx, *resp)
				}),
			},
			{
				Name:      "info",
				Usage:     appConfig.Translator.T_("Package information"),
				ArgsUsage: "package",
				Flags: []cli.Flag{
					&cli.BoolFlag{
						Name:  "full",
						Usage: appConfig.Translator.T_("Full output of information"),
						Value: false,
					},
				},
				Action: withGlobalWrapper(func(ctx context.Context, cmd *cli.Command, actions *Actions) error {
					resp, err := actions.Info(ctx, cmd.Args().First(), cmd.Bool("full"))
					if err != nil {
						return reply.CliResponse(ctx, newErrorResponse(ctx, err.Error()))
					}

					return reply.CliResponse(ctx, *resp)
				}),
				ShellComplete: findPkgInfoOnlyFirstArg(),
			},
			{
				Name:      "search",
				Usage:     appConfig.Translator.T_("Quick package search by name"),
				ArgsUsage: "package",
				Flags: []cli.Flag{
					&cli.BoolFlag{
						Name:    "installed",
						Usage:   appConfig.Translator.T_("Only installed"),
						Aliases: []string{"i"},
						Value:   false,
					},
					&cli.BoolFlag{
						Name:  "full",
						Usage: appConfig.Translator.T_("Full information output"),
						Value: false,
					},
				},
				Action: withGlobalWrapper(func(ctx context.Context, cmd *cli.Command, actions *Actions) error {
					resp, err := actions.Search(ctx, cmd.Args().First(), cmd.Bool("installed"), cmd.Bool("full"))
					if err != nil {
						return reply.CliResponse(ctx, newErrorResponse(ctx, err.Error()))
					}

					return reply.CliResponse(ctx, *resp)
				}),
			},
			{
				Name:  "list",
				Usage: appConfig.Translator.T_("Building a query to get a list of packages"),
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:  "sort",
						Usage: appConfig.Translator.T_("Sort packages by field, example fields: name, section"),
					},
					&cli.StringFlag{
						Name:  "order",
						Usage: appConfig.Translator.T_("Sorting order: ASC or DESC"),
						Value: "ASC",
					},
					&cli.IntFlag{
						Name:  "limit",
						Usage: appConfig.Translator.T_("Maximum number of records to return"),
						Value: 10,
					},
					&cli.IntFlag{
						Name:  "offset",
						Usage: appConfig.Translator.T_("Starting position (offset) for the result set"),
						Value: 0,
					},
					&cli.StringSliceFlag{
						Name:  "filter",
						Usage: appConfig.Translator.T_("Filter in the format key=value. The flag can be specified multiple times, for example: --filter name=zip --filter installed=true"),
					},
					&cli.BoolFlag{
						Name:  "force-update",
						Usage: appConfig.Translator.T_("Force update all packages before query"),
						Value: false,
					},
					&cli.BoolFlag{
						Name:  "full",
						Usage: appConfig.Translator.T_("Full information output"),
						Value: false,
					},
				},
				Action: withGlobalWrapper(func(ctx context.Context, cmd *cli.Command, actions *Actions) error {
					params := ListParams{
						Sort:        cmd.String("sort"),
						Order:       cmd.String("order"),
						Offset:      cmd.Int("offset"),
						Limit:       cmd.Int("limit"),
						Filters:     cmd.StringSlice("filter"),
						ForceUpdate: cmd.Bool("force-update"),
					}

					resp, err := actions.List(ctx, params, cmd.Bool("full"))
					if err != nil {
						return reply.CliResponse(ctx, newErrorResponse(ctx, err.Error()))
					}

					return reply.CliResponse(ctx, *resp)
				}),
			},
			{
				Name:  "dbus-doc",
				Usage: appConfig.Translator.T_("Show dbus online documentation"),
				Action: withGlobalWrapper(func(ctx context.Context, cmd *cli.Command, actions *Actions) error {
					reply.StopSpinner()
					return actions.GenerateOnlineDoc(ctx)
				}),
			},
			{
				Name:    "image",
				Usage:   appConfig.Translator.T_("Module for working with the image"),
				Aliases: []string{"i"},
				Hidden:  !appConfig.ConfigManager.GetConfig().IsAtomic,
				Commands: []*cli.Command{
					{
						Name:  "apply",
						Usage: appConfig.Translator.T_("Apply changes to the host"),
						Action: withRootCheckWrapper(func(ctx context.Context, cmd *cli.Command, actions *Actions) error {
							resp, err := actions.ImageApply(ctx)
							if err != nil {
								return reply.CliResponse(ctx, newErrorResponse(ctx, err.Error()))
							}

							return reply.CliResponse(ctx, *resp)
						}),
					},
					{
						Name:  "status",
						Usage: appConfig.Translator.T_("Image status"),
						Action: withRootCheckWrapper(func(ctx context.Context, cmd *cli.Command, actions *Actions) error {
							resp, err := actions.ImageStatus(ctx)
							if err != nil {
								return reply.CliResponse(ctx, newErrorResponse(ctx, err.Error()))
							}

							return reply.CliResponse(ctx, *resp)
						}),
					},
					{
						Name:  "update",
						Usage: appConfig.Translator.T_("Image update"),
						Action: withRootCheckWrapper(func(ctx context.Context, cmd *cli.Command, actions *Actions) error {
							resp, err := actions.ImageUpdate(ctx)
							if err != nil {
								return reply.CliResponse(ctx, newErrorResponse(ctx, err.Error()))
							}

							return reply.CliResponse(ctx, *resp)
						}),
					},
					{
						Name:  "history",
						Usage: appConfig.Translator.T_("Image changes history"),
						Flags: []cli.Flag{
							&cli.StringFlag{
								Name:  "image",
								Usage: appConfig.Translator.T_("Filter by image name"),
							},
							&cli.IntFlag{
								Name:  "limit",
								Usage: appConfig.Translator.T_("Maximum number of records to return"),
								Value: 10,
							},
							&cli.IntFlag{
								Name:  "offset",
								Usage: appConfig.Translator.T_("Starting position (offset) for the result set"),
								Value: 0,
							},
						},
						Action: withRootCheckWrapper(func(ctx context.Context, cmd *cli.Command, actions *Actions) error {
							resp, err := actions.ImageHistory(ctx, cmd.String("image"), cmd.Int("limit"), cmd.Int("offset"))
							if err != nil {
								return reply.CliResponse(ctx, newErrorResponse(ctx, err.Error()))
							}

							return reply.CliResponse(ctx, *resp)
						}),
					},
				},
			},
		},
	}
}
