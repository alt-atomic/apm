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
	_package "apm/internal/common/apt/package"
	"apm/internal/common/helper"
	"apm/internal/common/reply"
	"apm/internal/common/wrapper"
	"apm/internal/domain/system/appstream"
	"context"
	"fmt"
	"strings"

	"github.com/urfave/cli/v3"
)

// newErrorResponseFromError создаёт ответ с ошибкой, извлекая тип из apmerr.APMError.
func newErrorResponseFromError(err error) reply.APIResponse {
	app.Log.Error(err.Error())
	return reply.ErrorResponseFromError(err)
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

		exclude := make(map[string]struct{}, len(args))
		for i := 0; i < len(args)-1; i++ {
			exclude[strings.TrimRight(strings.TrimSpace(args[i]), "+-")] = struct{}{}
		}

		like := currentToken + "%"
		svc := NewActions(appConfig).serviceAptDatabase
		if svc == nil {
			return
		}

		pkgs, _ := svc.SearchPackagesMultiLimit(ctx, like, 200, installed)

		for _, p := range pkgs {
			if _, seen := exclude[p.Name]; seen {
				continue
			}
			fmt.Println(p.Name)
		}
	}
}

// findPkgInfoOnlyFirstArg выполняет поиск информации о пакете только для первого аргумента.
func findPkgInfoOnlyFirstArg() func(ctx context.Context, cmd *cli.Command) {
	return func(ctx context.Context, cmd *cli.Command) {
		if cmd.NArg() >= 2 {
			return
		}
		findPkgWithInstalled(false)(ctx, cmd)
	}
}

var withGlobalWrapper = wrapper.WithOptions(wrapper.NoRootCheck, NewActions, newErrorResponseFromError)
var withRootCheckWrapper = wrapper.WithOptions(wrapper.RequireRoot, NewActions, newErrorResponseFromError)

// applyAptOptions парсит -o флаги и применяет к actions
func applyAptOptions(cmd *cli.Command, actions *Actions) {
	opts := cmd.StringSlice("option")
	if len(opts) == 0 {
		return
	}
	overrides := make(map[string]string, len(opts))
	for _, opt := range opts {
		if k, v, ok := strings.Cut(opt, "="); ok {
			overrides[strings.TrimSpace(k)] = strings.TrimSpace(v)
		}
	}
	_, _ = actions.SetAptConfigOverrides(overrides)
}

// aptOptionFlag общий флаг для всех команд работы с пакетами
var aptOptionFlag = func() cli.Flag {
	return &cli.StringSliceFlag{
		Name:  "option",
		Usage: app.T_("Override APT config option, e.g. Dir::Cache::Archives=/tmp"),
	}
}

func upgradeCommand(appConfig *app.Config) *cli.Command {
	if appConfig.ConfigManager.GetConfig().IsAtomic {
		return &cli.Command{
			Name:  "upgrade",
			Usage: app.T_("Upgrade system image"),
			Flags: []cli.Flag{
				&cli.BoolFlag{
					Name:  "no-cache",
					Usage: app.T_("Disable APT package cache for image build"),
					Value: false,
				},
			},
			Action: withRootCheckWrapper(func(ctx context.Context, cmd *cli.Command, actions *Actions) error {
				resp, err := actions.ImageUpdate(ctx, !cmd.Bool("no-cache"))
				if err != nil {
					return reply.CliResponse(ctx, newErrorResponseFromError(err))
				}
				return reply.CliResponse(ctx, reply.OK(resp))
			}),
		}
	}

	return &cli.Command{
		Name:  "upgrade",
		Usage: app.T_("General system upgrade"),
		Flags: []cli.Flag{
			&cli.BoolFlag{
				Name:    "simulate",
				Usage:   app.T_("Simulate upgrade"),
				Aliases: []string{"s"},
				Value:   false,
			},
			&cli.BoolFlag{
				Name:    "download-only",
				Usage:   app.T_("Download packages without installation"),
				Aliases: []string{"d"},
				Value:   false,
			},
			aptOptionFlag(),
		},
		Action: withRootCheckWrapper(func(ctx context.Context, cmd *cli.Command, actions *Actions) error {
			applyAptOptions(cmd, actions)
			if cmd.Bool("simulate") {
				resp, err := actions.CheckUpgrade(ctx)
				if err != nil {
					return reply.CliResponse(ctx, newErrorResponseFromError(err))
				}
				return reply.CliResponse(ctx, reply.OK(resp))
			}
			resp, err := actions.Upgrade(ctx, cmd.Bool("download-only"))
			if err != nil {
				return reply.CliResponse(ctx, newErrorResponseFromError(err))
			}
			return reply.CliResponse(ctx, reply.OK(resp))
		}),
	}
}

func CommandList(ctx context.Context) *cli.Command {
	appConfig := app.GetAppConfig(ctx)

	imageCmds := []*cli.Command{
		{
			Name:  "build",
			Usage: app.T_("Build image"),
			Action: withGlobalWrapper(func(ctx context.Context, cmd *cli.Command, actions *Actions) error {
				resp, err := actions.ImageBuild(ctx)
				if err != nil {
					return reply.CliResponse(ctx, newErrorResponseFromError(err))
				}

				return reply.CliResponse(ctx, reply.OK(resp))
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
					Flags: []cli.Flag{
						&cli.BoolFlag{
							Name:  "pull",
							Usage: app.T_("Always pull the base image from the registry"),
							Value: false,
						},
						&cli.BoolFlag{
							Name:  "no-cache",
							Usage: app.T_("Disable APT package cache for image build"),
							Value: false,
						},
					},
					Action: withRootCheckWrapper(func(ctx context.Context, cmd *cli.Command, actions *Actions) error {
						resp, err := actions.ImageApply(ctx, cmd.Bool("pull"), !cmd.Bool("no-cache"))
						if err != nil {
							return reply.CliResponse(ctx, newErrorResponseFromError(err))
						}

						return reply.CliResponse(ctx, reply.OK(resp))
					}),
				},
				{
					Name:  "status",
					Usage: app.T_("Image status"),
					Action: withRootCheckWrapper(func(ctx context.Context, cmd *cli.Command, actions *Actions) error {
						resp, err := actions.ImageStatus(ctx)
						if err != nil {
							return reply.CliResponse(ctx, newErrorResponseFromError(err))
						}

						return reply.CliResponse(ctx, reply.OK(resp))
					}),
				},
				{
					Name:  "update",
					Usage: app.T_("Upgrade system image"),
					Flags: []cli.Flag{
						&cli.BoolFlag{
							Name:  "no-cache",
							Usage: app.T_("Disable APT package cache for image build"),
							Value: false,
						},
					},
					Action: withRootCheckWrapper(func(ctx context.Context, cmd *cli.Command, actions *Actions) error {
						resp, err := actions.ImageUpdate(ctx, !cmd.Bool("no-cache"))
						if err != nil {
							return reply.CliResponse(ctx, newErrorResponseFromError(err))
						}

						return reply.CliResponse(ctx, reply.OK(resp))
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
							return reply.CliResponse(ctx, newErrorResponseFromError(err))
						}

						return reply.CliResponse(ctx, reply.OK(resp))
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
				aptOptionFlag(),
			},
			Action: withRootCheckWrapper(func(ctx context.Context, cmd *cli.Command, actions *Actions) error {
				applyAptOptions(cmd, actions)
				if cmd.Bool("simulate") {
					resp, err := actions.CheckReinstall(ctx, cmd.Args().Slice())
					if err != nil {
						return reply.CliResponse(ctx, newErrorResponseFromError(err))
					}
					return reply.CliResponse(ctx, reply.OK(resp))
				}
				resp, err := actions.Reinstall(ctx, cmd.Args().Slice(), cmd.Bool("yes"))
				if err != nil {
					return reply.CliResponse(ctx, newErrorResponseFromError(err))
				}
				return reply.CliResponse(ctx, reply.OK(resp))
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
				&cli.BoolFlag{
					Name:    "download-only",
					Usage:   app.T_("Download packages without installation"),
					Aliases: []string{"d"},
					Value:   false,
				},
				aptOptionFlag(),
			},
			Action: withRootCheckWrapper(func(ctx context.Context, cmd *cli.Command, actions *Actions) error {
				applyAptOptions(cmd, actions)
				if cmd.Bool("simulate") {
					resp, err := actions.CheckInstall(ctx, cmd.Args().Slice())
					if err != nil {
						return reply.CliResponse(ctx, newErrorResponseFromError(err))
					}
					return reply.CliResponse(ctx, reply.OK(resp))
				}
				resp, err := actions.Install(ctx, cmd.Args().Slice(), cmd.Bool("yes"), cmd.Bool("download-only"))
				if err != nil {
					return reply.CliResponse(ctx, newErrorResponseFromError(err))
				}
				return reply.CliResponse(ctx, reply.OK(resp))
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
				aptOptionFlag(),
			},
			Action: withRootCheckWrapper(func(ctx context.Context, cmd *cli.Command, actions *Actions) error {
				applyAptOptions(cmd, actions)
				if cmd.Bool("simulate") {
					resp, err := actions.CheckRemove(ctx, cmd.Args().Slice(), false, cmd.Bool("depends"))
					if err != nil {
						return reply.CliResponse(ctx, newErrorResponseFromError(err))
					}
					return reply.CliResponse(ctx, reply.OK(resp))
				}
				resp, err := actions.Remove(ctx, cmd.Args().Slice(), false, cmd.Bool("depends"),
					cmd.Bool("yes"))
				if err != nil {
					return reply.CliResponse(ctx, newErrorResponseFromError(err))
				}
				return reply.CliResponse(ctx, reply.OK(resp))
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
				&cli.BoolFlag{
					Name:  "only-db",
					Usage: app.T_("Only update installed status in DB without refreshing repositories"),
				},
				aptOptionFlag(),
			},
			Action: withRootCheckWrapper(func(ctx context.Context, cmd *cli.Command, actions *Actions) error {
				applyAptOptions(cmd, actions)
				resp, err := actions.Update(ctx, cmd.Bool("no-lock"), cmd.Bool("only-db"))
				if err != nil {
					return reply.CliResponse(ctx, newErrorResponseFromError(err))
				}

				return reply.CliResponse(ctx, reply.OK(resp))
			}),
		},
		upgradeCommand(appConfig),
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
				resp, err := actions.Info(ctx, cmd.Args().First())
				if err != nil {
					return reply.CliResponse(ctx, newErrorResponseFromError(err))
				}
				return reply.CliResponse(ctx, reply.OK(map[string]interface{}{
					"message":     resp.Message,
					"packageInfo": actions.FormatPackageOutput(resp.PackageInfo, cmd.Bool("full")),
				}))
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
				resp, err := actions.Search(ctx, cmd.Args().First(), cmd.Bool("installed"))
				if err != nil {
					return reply.CliResponse(ctx, newErrorResponseFromError(err))
				}
				return reply.CliResponse(ctx, reply.OK(map[string]interface{}{
					"message":  resp.Message,
					"packages": actions.FormatPackageOutput(resp.Packages, cmd.Bool("full")),
				}))
			}),
		},
		{
			Name:  "sections",
			Usage: app.T_("Show all available package sections"),
			Action: withGlobalWrapper(func(ctx context.Context, cmd *cli.Command, actions *Actions) error {
				resp, err := actions.Sections(ctx)
				if err != nil {
					return reply.CliResponse(ctx, newErrorResponseFromError(err))
				}
				return reply.CliResponse(ctx, reply.OK(resp))
			}),
		},
		{
			Name:  "list",
			Usage: app.T_("Building a query to get a list of packages"),
			Description: helper.FilterDescription(
				"--filter name=zip --filter name[eq]=zip --filter size[gt]=1000 --filter section[eq]=games|education",
				app.T_("Application fields are available with the \"app.\" prefix: app.name, app.categories, app.type, etc."),
			),
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
					Usage: app.T_("Filter in the format key[op]=value or key=value"),
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
				filters, err := _package.SystemFilterConfig.Parse(cmd.StringSlice("filter"))
				if err != nil {
					return reply.CliResponse(ctx, newErrorResponseFromError(err))
				}

				params := ListParams{
					Sort:        cmd.String("sort"),
					Order:       cmd.String("order"),
					Offset:      cmd.Int("offset"),
					Limit:       cmd.Int("limit"),
					Filters:     filters,
					ForceUpdate: cmd.Bool("force-update"),
				}

				resp, err := actions.List(ctx, params)
				if err != nil {
					return reply.CliResponse(ctx, newErrorResponseFromError(err))
				}
				return reply.CliResponse(ctx, reply.OK(map[string]interface{}{
					"message":    resp.Message,
					"packages":   actions.FormatPackageOutput(resp.Packages, cmd.Bool("full")),
					"totalCount": resp.TotalCount,
				}))
			}),
		},
		{
			Name:     "application",
			Usage:    app.T_("Module for application information"),
			Category: app.T_("Applications"),
			Commands: appstream.CommandList(ctx),
		},
		{
			Name:     "image",
			Usage:    app.T_("Module for working with the image"),
			Aliases:  []string{"i"},
			Category: app.T_("Image"),
			Commands: imageCmds,
		},
		{
			Name:     "dbus-doc",
			Usage:    app.T_("Show dbus online documentation"),
			Category: app.T_("Documentation"),
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
