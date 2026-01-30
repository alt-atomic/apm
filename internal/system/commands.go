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
	"apm/internal/common/app"
	"apm/internal/common/reply"
	"apm/internal/common/wrapper"
	"context"
	"fmt"
	"strings"

	"github.com/urfave/cli/v3"
)

// newErrorResponse создаёт ответ с ошибкой и указанным сообщением.
func newErrorResponse(message string) reply.APIResponse {
	app.Log.Error(message)

	return reply.APIResponse{
		Data:  map[string]interface{}{"message": message},
		Error: true,
	}
}

func findPkgWithInstalled(installed bool) func(ctx context.Context, cmd *cli.Command) {
	return func(ctx context.Context, cmd *cli.Command) {
		appConfig := app.GetAppConfig(ctx)
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

var withGlobalWrapper = wrapper.WithOptions(wrapper.NoRootCheck, NewActions, newErrorResponse)
var withRootCheckWrapper = wrapper.WithOptions(wrapper.RequireRoot, NewActions, newErrorResponse)

func CommandList(ctx context.Context) *cli.Command {
	appConfig := app.GetAppConfig(ctx)

	imageCmds := []*cli.Command{
		{
			Name:  "build",
			Usage: app.T_("Build image"),
			Flags: []cli.Flag{
				&cli.IntFlag{
					Name:   "flat-index",
					Usage:  "Execute specific flattened module by index (internal use)",
					Value:  -1,
					Hidden: true,
				},
				&cli.StringFlag{
					Name:   "config",
					Usage:  "Path to image.yml (internal use)",
					Hidden: true,
				},
				&cli.StringFlag{
					Name:   "resources",
					Usage:  "Path to resources directory (internal use)",
					Hidden: true,
				},
			},
			Action: withGlobalWrapper(func(ctx context.Context, cmd *cli.Command, actions *Actions) error {
				flatIndex := cmd.Int("flat-index")
				configPath := cmd.String("config")
				resourcesPath := cmd.String("resources")
				resp, err := actions.ImageBuildWithOptions(ctx, flatIndex, configPath, resourcesPath)
				if err != nil {
					return reply.CliResponse(ctx, newErrorResponse(err.Error()))
				}

				return reply.CliResponse(ctx, *resp)
			}),
		},
		{
			Name:  "buildah",
			Usage: app.T_("Build image using buildah"),
			Flags: []cli.Flag{
				&cli.StringFlag{
					Name:  "tag",
					Usage: app.T_("Tag for final image"),
					Value: "localhost/os:latest",
				},
				&cli.StringFlag{
					Name:  "base-image",
					Usage: app.T_("Override base image from config"),
				},
				&cli.StringFlag{
					Name:  "config",
					Usage: app.T_("Path to image.yml"),
				},
				&cli.StringFlag{
					Name:  "resources",
					Usage: app.T_("Path to resources directory"),
				},
				&cli.BoolFlag{
					Name:  "no-cache",
					Usage: app.T_("Disable layer caching"),
				},
				&cli.StringSliceFlag{
					Name:  "env",
					Usage: app.T_("Environment variables to pass (can be specified multiple times)"),
				},
			},
			Action: withGlobalWrapper(func(ctx context.Context, cmd *cli.Command, actions *Actions) error {
				opts := ImageBuildahOptions{
					Tag:           cmd.String("tag"),
					BaseImage:     cmd.String("base-image"),
					ConfigPath:    cmd.String("config"),
					ResourcesPath: cmd.String("resources"),
					NoCache:       cmd.Bool("no-cache"),
					EnvVars:       cmd.StringSlice("env"),
				}
				resp, err := actions.ImageBuildah(ctx, opts)
				if err != nil {
					return reply.CliResponse(ctx, newErrorResponse(err.Error()))
				}

				return reply.CliResponse(ctx, *resp)
			}),
		},
	}

	if appConfig.ConfigManager.GetConfig().IsAtomic {
		imageCmds = append(
			imageCmds,
			[]*cli.Command{
				{
					Name:  "apply",
					Usage: app.T_("Apply changes to the host"),
					Action: withRootCheckWrapper(func(ctx context.Context, cmd *cli.Command, actions *Actions) error {
						resp, err := actions.ImageApply(ctx)
						if err != nil {
							return reply.CliResponse(ctx, newErrorResponse(err.Error()))
						}

						return reply.CliResponse(ctx, *resp)
					}),
				},
				{
					Name:  "status",
					Usage: app.T_("Image status"),
					Action: withRootCheckWrapper(func(ctx context.Context, cmd *cli.Command, actions *Actions) error {
						resp, err := actions.ImageStatus(ctx)
						if err != nil {
							return reply.CliResponse(ctx, newErrorResponse(err.Error()))
						}

						return reply.CliResponse(ctx, *resp)
					}),
				},
				{
					Name:  "update",
					Usage: app.T_("Image update"),
					Action: withRootCheckWrapper(func(ctx context.Context, cmd *cli.Command, actions *Actions) error {
						resp, err := actions.ImageUpdate(ctx)
						if err != nil {
							return reply.CliResponse(ctx, newErrorResponse(err.Error()))
						}

						return reply.CliResponse(ctx, *resp)
					}),
				},
				{
					Name:  "history",
					Usage: app.T_("Image changes history"),
					Flags: []cli.Flag{
						&cli.StringFlag{
							Name:  "image",
							Usage: app.T_("Filter by image name"),
						},
						&cli.IntFlag{
							Name:  "limit",
							Usage: app.T_("Maximum number of records to return"),
							Value: 10,
						},
						&cli.IntFlag{
							Name:  "offset",
							Usage: app.T_("Starting position (offset) for the result set"),
							Value: 0,
						},
					},
					Action: withRootCheckWrapper(func(ctx context.Context, cmd *cli.Command, actions *Actions) error {
						resp, err := actions.ImageHistory(ctx, cmd.String("image"), cmd.Int("limit"), cmd.Int("offset"))
						if err != nil {
							return reply.CliResponse(ctx, newErrorResponse(err.Error()))
						}

						return reply.CliResponse(ctx, *resp)
					}),
				},
			}...,
		)
	}

	cmds := []*cli.Command{
		{
			Name:      "reinstall",
			Usage:     app.T_("Reinstall packages"),
			ArgsUsage: "packages",
			Flags: []cli.Flag{
				&cli.BoolFlag{
					Name:    "yes",
					Usage:   app.T_("Reinstall without confirmation"),
					Aliases: []string{"y"},
					Value:   false,
				},
				&cli.BoolFlag{
					Name:    "simulate",
					Usage:   app.T_("Simulate reinstallation"),
					Aliases: []string{"s"},
					Value:   false,
				},
			},
			Action: withRootCheckWrapper(func(ctx context.Context, cmd *cli.Command, actions *Actions) error {
				var resp *reply.APIResponse
				var err error
				if cmd.Bool("simulate") {
					resp, err = actions.CheckReinstall(ctx, cmd.Args().Slice())
				} else {
					resp, err = actions.Reinstall(ctx, cmd.Args().Slice(), cmd.Bool("yes"))
				}
				if err != nil {
					return reply.CliResponse(ctx, newErrorResponse(err.Error()))
				}

				return reply.CliResponse(ctx, *resp)
			}),
			ShellComplete: findPkgWithInstalled(true),
		},
		{
			Name:      "install",
			Usage:     app.T_("Package list for installation. The format package- package+ is supported."),
			ArgsUsage: "packages",
			Flags: []cli.Flag{
				&cli.BoolFlag{
					Name:    "yes",
					Usage:   app.T_("Install without confirmation"),
					Aliases: []string{"y"},
					Value:   false,
				},
				&cli.BoolFlag{
					Name:    "simulate",
					Usage:   app.T_("Simulate installation"),
					Aliases: []string{"s"},
					Value:   false,
				},
			},
			Action: withRootCheckWrapper(func(ctx context.Context, cmd *cli.Command, actions *Actions) error {
				var resp *reply.APIResponse
				var err error
				if cmd.Bool("simulate") {
					resp, err = actions.CheckInstall(ctx, cmd.Args().Slice())
				} else {
					resp, err = actions.Install(ctx, cmd.Args().Slice(), cmd.Bool("yes"))
				}
				if err != nil {
					return reply.CliResponse(ctx, newErrorResponse(err.Error()))
				}

				return reply.CliResponse(ctx, *resp)
			}),
			ShellComplete: findPkgWithInstalled(false),
		},
		{
			Name:      "remove",
			Aliases:   []string{"rm"},
			Usage:     app.T_("List of packages to remove"),
			ArgsUsage: "packages",
			Flags: []cli.Flag{
				&cli.BoolFlag{
					Name:    "yes",
					Usage:   app.T_("Remove without confirmation"),
					Aliases: []string{"y"},
					Value:   false,
				},
				&cli.BoolFlag{
					Name:    "purge",
					Usage:   app.T_("Attempt to remove all files"),
					Aliases: []string{"p"},
					Value:   false,
				},
				&cli.BoolFlag{
					Name:    "depends",
					Usage:   app.T_("Attempt to remove depends"),
					Aliases: []string{"d"},
					Value:   false,
				},
				&cli.BoolFlag{
					Name:    "simulate",
					Usage:   app.T_("Simulate removal"),
					Aliases: []string{"s"},
					Value:   false,
				},
			},
			Action: withRootCheckWrapper(func(ctx context.Context, cmd *cli.Command, actions *Actions) error {
				var resp *reply.APIResponse
				var err error
				if cmd.Bool("simulate") {
					resp, err = actions.CheckRemove(ctx, cmd.Args().Slice(), cmd.Bool("purge"), cmd.Bool("depends"))
				} else {
					resp, err = actions.Remove(ctx, cmd.Args().Slice(), cmd.Bool("purge"), cmd.Bool("depends"),
						cmd.Bool("yes"))
				}
				if err != nil {
					return reply.CliResponse(ctx, newErrorResponse(err.Error()))
				}

				return reply.CliResponse(ctx, *resp)
			}),
			ShellComplete: findPkgWithInstalled(true),
		},
		{
			Name:  "update",
			Usage: app.T_("Updating package database"),
			Flags: []cli.Flag{
				&cli.BoolFlag{
					Name:  "no-lock",
					Usage: app.T_("Skip file locking (use with caution)"),
					Value: false,
				},
			},
			Action: withRootCheckWrapper(func(ctx context.Context, cmd *cli.Command, actions *Actions) error {
				resp, err := actions.Update(ctx, cmd.Bool("no-lock"))
				if err != nil {
					return reply.CliResponse(ctx, newErrorResponse(err.Error()))
				}

				return reply.CliResponse(ctx, *resp)
			}),
		},
		{
			Name:  "upgrade",
			Usage: app.T_("General system upgrade"),
			Action: withRootCheckWrapper(func(ctx context.Context, cmd *cli.Command, actions *Actions) error {
				var resp *reply.APIResponse
				var err error
				if appConfig.ConfigManager.GetConfig().IsAtomic {
					resp, err = actions.ImageUpdate(ctx)
				} else {
					resp, err = actions.Upgrade(ctx)
				}

				if err != nil {
					return reply.CliResponse(ctx, newErrorResponse(err.Error()))
				}

				return reply.CliResponse(ctx, *resp)
			}),
		},
		{
			Name:      "info",
			Usage:     app.T_("Package information"),
			ArgsUsage: "package",
			Flags: []cli.Flag{
				&cli.BoolFlag{
					Name:  "full",
					Usage: app.T_("Full output of information"),
					Value: false,
				},
			},
			Action: withGlobalWrapper(func(ctx context.Context, cmd *cli.Command, actions *Actions) error {
				resp, err := actions.Info(ctx, cmd.Args().First(), cmd.Bool("full"))
				if err != nil {
					return reply.CliResponse(ctx, newErrorResponse(err.Error()))
				}

				return reply.CliResponse(ctx, *resp)
			}),
			ShellComplete: findPkgInfoOnlyFirstArg(),
		},
		{
			Name:      "search",
			Usage:     app.T_("Quick package search by name"),
			ArgsUsage: "package",
			Flags: []cli.Flag{
				&cli.BoolFlag{
					Name:    "installed",
					Usage:   app.T_("Only installed"),
					Aliases: []string{"i"},
					Value:   false,
				},
				&cli.BoolFlag{
					Name:  "full",
					Usage: app.T_("Full information output"),
					Value: false,
				},
			},
			Action: withGlobalWrapper(func(ctx context.Context, cmd *cli.Command, actions *Actions) error {
				resp, err := actions.Search(ctx, cmd.Args().First(), cmd.Bool("installed"), cmd.Bool("full"))
				if err != nil {
					return reply.CliResponse(ctx, newErrorResponse(err.Error()))
				}

				return reply.CliResponse(ctx, *resp)
			}),
		},
		{
			Name:  "list",
			Usage: app.T_("Building a query to get a list of packages"),
			Flags: []cli.Flag{
				&cli.StringFlag{
					Name:  "sort",
					Usage: app.T_("Sort packages by field, example fields: name, section"),
				},
				&cli.StringFlag{
					Name:  "order",
					Usage: app.T_("Sorting order: ASC or DESC"),
					Value: "ASC",
				},
				&cli.IntFlag{
					Name:  "limit",
					Usage: app.T_("Maximum number of records to return"),
					Value: 10,
				},
				&cli.IntFlag{
					Name:  "offset",
					Usage: app.T_("Starting position (offset) for the result set"),
					Value: 0,
				},
				&cli.StringSliceFlag{
					Name:  "filter",
					Usage: app.T_("Filter in the format key=value. The flag can be specified multiple times, for example: --filter name=zip --filter installed=true"),
				},
				&cli.BoolFlag{
					Name:  "force-update",
					Usage: app.T_("Force update all packages before query"),
					Value: false,
				},
				&cli.BoolFlag{
					Name:  "full",
					Usage: app.T_("Full information output"),
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
					return reply.CliResponse(ctx, newErrorResponse(err.Error()))
				}

				return reply.CliResponse(ctx, *resp)
			}),
		},
		{
			Name:     "image",
			Usage:    app.T_("Module for working with the image"),
			Aliases:  []string{"i"},
			Commands: imageCmds,
		},
		{
			Name:  "dbus-doc",
			Usage: app.T_("Show dbus online documentation"),
			Action: withGlobalWrapper(func(ctx context.Context, cmd *cli.Command, actions *Actions) error {
				reply.StopSpinner(appConfig)
				return actions.GenerateOnlineDoc(ctx)
			}),
		},
	}

	return &cli.Command{
		Name:            "system",
		Aliases:         []string{"s"},
		Usage:           app.T_("System package management"),
		HideHelpCommand: true,
		Commands:        cmds,
	}
}
