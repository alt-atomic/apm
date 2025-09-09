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

package distrobox

import (
	"apm/internal/common/app"
	"apm/internal/common/helper"
	"apm/internal/common/reply"
	"context"
	"syscall"

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

func wrapperWithOptions() func(func(context.Context, *cli.Command, *Actions) error) cli.ActionFunc {
	return func(actionFunc func(context.Context, *cli.Command, *Actions) error) cli.ActionFunc {
		return func(ctx context.Context, cmd *cli.Command) error {
			appConfig := app.GetAppConfig(ctx)
			appConfig.ConfigManager.SetFormat(cmd.String("format"))
			ctx = context.WithValue(ctx, helper.TransactionKey, cmd.String("transaction"))

			if syscall.Geteuid() == 0 {
				return reply.CliResponse(ctx, newErrorResponse(app.T_("Elevated rights are not allowed to perform this action. Please do not use sudo or su")))
			}

			actions := NewActions(appConfig)

			reply.CreateSpinner()
			return actionFunc(ctx, cmd, actions)
		}
	}
}

var withGlobalWrapper = wrapperWithOptions()

func CommandList(ctx context.Context) *cli.Command {
	appConfig := app.GetAppConfig(ctx)

	return &cli.Command{
		Name:    "distrobox",
		Aliases: []string{"d"},
		Hidden:  !appConfig.ConfigManager.GetConfig().ExistDistrobox,
		Usage:   app.T_("Managing packages and containers in distrobox"),
		Commands: []*cli.Command{
			{
				Name:  "update",
				Usage: app.T_("Update and synchronize the list of installed packages with the host"),
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:     "container",
						Usage:    app.T_("Container name. Required"),
						Aliases:  []string{"c"},
						Required: true,
					},
				},
				Action: withGlobalWrapper(func(ctx context.Context, cmd *cli.Command, actions *Actions) error {
					resp, err := actions.Update(ctx, cmd.String("container"))
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
					&cli.StringFlag{
						Name:     "container",
						Usage:    app.T_("Container name. Required"),
						Aliases:  []string{"c"},
						Required: true,
					},
				},
				Action: withGlobalWrapper(func(ctx context.Context, cmd *cli.Command, actions *Actions) error {
					resp, err := actions.Info(ctx, cmd.String("container"), cmd.Args().First())
					if err != nil {
						return reply.CliResponse(ctx, newErrorResponse(err.Error()))
					}

					return reply.CliResponse(ctx, *resp)
				}),
			},
			{
				Name:      "search",
				Usage:     app.T_("Quick package search by name"),
				ArgsUsage: "package",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:    "container",
						Usage:   app.T_("Container name. Optional flag"),
						Aliases: []string{"c"},
					},
				},
				Action: withGlobalWrapper(func(ctx context.Context, cmd *cli.Command, actions *Actions) error {
					resp, err := actions.Search(ctx, cmd.String("container"), cmd.Args().First())
					if err != nil {
						return reply.CliResponse(ctx, newErrorResponse(err.Error()))
					}

					return reply.CliResponse(ctx, *resp)
				}),
			},
			{
				Name:  "list",
				Usage: app.T_("Building query to retrieve package list"),
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:    "container",
						Usage:   app.T_("Container name. Optional flag"),
						Aliases: []string{"c"},
					},
					&cli.StringFlag{
						Name:  "sort",
						Usage: app.T_("Field for sorting, for example: name, version"),
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
						Usage: app.T_("Force update all packages before the request"),
						Value: false,
					},
				},
				Action: withGlobalWrapper(func(ctx context.Context, cmd *cli.Command, actions *Actions) error {
					params := ListParams{
						Container:   cmd.String("container"),
						Sort:        cmd.String("sort"),
						Order:       cmd.String("order"),
						Offset:      cmd.Int("offset"),
						Limit:       cmd.Int("limit"),
						Filters:     cmd.StringSlice("filter"),
						ForceUpdate: cmd.Bool("force-update"),
					}

					resp, err := actions.List(ctx, params)
					if err != nil {
						return reply.CliResponse(ctx, newErrorResponse(err.Error()))
					}

					return reply.CliResponse(ctx, *resp)
				}),
			},
			{
				Name:      "install",
				Usage:     app.T_("Install package"),
				ArgsUsage: "package",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:     "container",
						Usage:    app.T_("Container name. Required"),
						Aliases:  []string{"c"},
						Required: true,
					},
					&cli.BoolFlag{
						Name:  "export",
						Usage: app.T_("Export package"),
						Value: true,
					},
				},
				Action: withGlobalWrapper(func(ctx context.Context, cmd *cli.Command, actions *Actions) error {
					resp, err := actions.Install(ctx, cmd.String("container"), cmd.Args().First(), cmd.Bool("export"))
					if err != nil {
						return reply.CliResponse(ctx, newErrorResponse(err.Error()))
					}

					return reply.CliResponse(ctx, *resp)
				}),
			},
			{
				Name:      "remove",
				Usage:     app.T_("Remove package"),
				ArgsUsage: "package",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:     "container",
						Usage:    app.T_("Container name. Required"),
						Aliases:  []string{"c"},
						Required: true,
					},
					&cli.BoolFlag{
						Name:  "only-host",
						Usage: app.T_("Remove only from host, leave package in container"),
						Value: false,
					},
				},
				Action: withGlobalWrapper(func(ctx context.Context, cmd *cli.Command, actions *Actions) error {
					resp, err := actions.Remove(ctx, cmd.String("container"), cmd.Args().First(), cmd.Bool("only-host"))
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
					reply.StopSpinner()
					return actions.GenerateOnlineDoc(ctx)
				}),
			},
			{
				Name:    "container",
				Usage:   app.T_("Module for working with containers"),
				Aliases: []string{"c"},
				Commands: []*cli.Command{
					{
						Name:  "list",
						Usage: app.T_("List of containers"),
						Action: withGlobalWrapper(func(ctx context.Context, cmd *cli.Command, actions *Actions) error {
							resp, err := actions.ContainerList(ctx)
							if err != nil {
								return reply.CliResponse(ctx, newErrorResponse(err.Error()))
							}

							return reply.CliResponse(ctx, *resp)
						}),
					},
					{
						Name:  "create",
						Usage: app.T_("Add container"),
						Flags: []cli.Flag{
							&cli.StringFlag{
								Name:     "image",
								Usage:    app.T_("Container. Must be specified, options: alt, ubuntu, arch"),
								Required: true,
							},
							&cli.StringFlag{
								Name:     "name",
								Usage:    app.T_("Container name"),
								Required: false,
							},
						},
						Action: withGlobalWrapper(func(ctx context.Context, cmd *cli.Command, actions *Actions) error {
							imageVal := cmd.String("image")
							allowedImages := []string{"alt", "ubuntu", "arch"}
							valid := false
							for _, img := range allowedImages {
								if imageVal == img {
									valid = true
									break
								}
							}
							if !valid {
								return reply.CliResponse(ctx,
									newErrorResponse(app.T_("The value for image must be one of: alt, ubuntu, arch")))
							}

							var imageLink string
							switch imageVal {
							case "arch":
								imageLink = "archlinux:latest"
							case "ubuntu":
								imageLink = "ubuntu:latest"
							case "alt":
								imageLink = "registry.altlinux.org/sisyphus/base:latest"
							}

							name := "atomic-" + imageVal
							if cmd.String("name") != "" {
								name = cmd.String("name")
							}

							resp, err := actions.ContainerAdd(ctx, imageLink, name, "zsh mc nano", "")
							if err != nil {
								return reply.CliResponse(ctx, newErrorResponse(err.Error()))
							}

							return reply.CliResponse(ctx, *resp)
						}),
					},
					{
						Name:  "create-manual",
						Usage: app.T_("Manual container addition"),
						Flags: []cli.Flag{
							&cli.StringFlag{
								Name:     "image",
								Usage:    app.T_("Image link. Required"),
								Required: true,
							},
							&cli.StringFlag{
								Name:     "name",
								Usage:    app.T_("Container name. Required"),
								Required: true,
							},
							&cli.StringFlag{
								Name:  "additional-packages",
								Usage: app.T_("List of packages to install"),
								Value: "zsh",
							},
							&cli.StringFlag{
								Name:  "init-hooks",
								Usage: app.T_("Calling hook to execute commands"),
							},
						},
						Action: withGlobalWrapper(func(ctx context.Context, cmd *cli.Command, actions *Actions) error {
							imageVal := cmd.String("image")
							nameVal := cmd.String("name")
							addPkgVal := cmd.String("additional-packages")
							hookVal := cmd.String("init-hooks")

							resp, err := actions.ContainerAdd(ctx, imageVal, nameVal, addPkgVal, hookVal)
							if err != nil {
								return reply.CliResponse(ctx, newErrorResponse(err.Error()))
							}

							return reply.CliResponse(ctx, *resp)
						}),
					},
					{
						Name:    "remove",
						Usage:   app.T_("Remove container"),
						Aliases: []string{"rm"},
						Flags: []cli.Flag{
							&cli.StringFlag{
								Name:     "name",
								Usage:    app.T_("Container name. Required"),
								Required: true,
							},
						},
						Action: withGlobalWrapper(func(ctx context.Context, cmd *cli.Command, actions *Actions) error {
							resp, err := actions.ContainerRemove(ctx, cmd.String("name"))
							if err != nil {
								return reply.CliResponse(ctx, newErrorResponse(err.Error()))
							}

							return reply.CliResponse(ctx, *resp)
						}),
					},
				},
			},
		},
	}
}
