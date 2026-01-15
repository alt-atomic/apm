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

package kernel

import (
	"apm/internal/common/app"
	"apm/internal/common/reply"
	"apm/internal/common/wrapper"
	"context"

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

var withRootCheckWrapper = wrapper.WithOptions(wrapper.RequireRoot, NewActions, newErrorResponse)

func CommandList(ctx context.Context) *cli.Command {
	appConfig := app.GetAppConfig(ctx)

	return &cli.Command{
		Name:    "kernel",
		Aliases: []string{"k"},
		Usage:   app.T_("Kernel Management. WARNING - experimental module"),
		Commands: []*cli.Command{
			{
				Name:  "list",
				Usage: app.T_("List available kernels"),
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:  "flavour",
						Usage: app.T_("Filter by kernel flavour (e.g., std-def, un-def)"),
					},
					&cli.BoolFlag{
						Name:    "installed",
						Usage:   app.T_("Show only installed kernels"),
						Aliases: []string{"i"},
						Value:   false,
					},
				},
				Action: withRootCheckWrapper(func(ctx context.Context, cmd *cli.Command, actions *Actions) error {
					resp, err := actions.ListKernels(ctx, cmd.String("flavour"), cmd.Bool("installed"))
					if err != nil {
						return reply.CliResponse(ctx, newErrorResponse(err.Error()))
					}
					return reply.CliResponse(ctx, *resp)
				}),
			},
			{
				Name:  "info",
				Usage: app.T_("Show information about current kernel"),
				Action: withRootCheckWrapper(func(ctx context.Context, cmd *cli.Command, actions *Actions) error {
					resp, err := actions.GetCurrentKernel(ctx)
					if err != nil {
						return reply.CliResponse(ctx, newErrorResponse(err.Error()))
					}
					return reply.CliResponse(ctx, *resp)
				}),
			},
			{
				Name:      "install",
				Usage:     app.T_("Install kernel with specified flavour"),
				ArgsUsage: "flavour",
				Flags: []cli.Flag{
					&cli.StringSliceFlag{
						Name:  "modules",
						Usage: app.T_("Install additional kernel modules"),
					},
					&cli.BoolFlag{
						Name:  "headers",
						Usage: app.T_("Install kernel headers"),
						Value: false,
					},
					&cli.BoolFlag{
						Name:    "simulate",
						Usage:   app.T_("Simulate installation"),
						Value:   false,
						Aliases: []string{"s"},
					},
				},
				Action: withRootCheckWrapper(func(ctx context.Context, cmd *cli.Command, actions *Actions) error {
					flavour := cmd.Args().First()
					if flavour == "" {
						return reply.CliResponse(ctx, newErrorResponse(app.T_("Kernel flavour must be specified")))
					}

					resp, err := actions.InstallKernel(ctx, flavour, cmd.StringSlice("modules"), cmd.Bool("headers"), cmd.Bool("simulate"))
					if err != nil {
						return reply.CliResponse(ctx, newErrorResponse(err.Error()))
					}
					return reply.CliResponse(ctx, *resp)
				}),
			},
			{
				Name:  "update",
				Usage: app.T_("Update kernel to latest version"),
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:  "flavour",
						Usage: app.T_("Update to specific flavour (default: current flavour)"),
					},
					&cli.StringSliceFlag{
						Name:  "modules",
						Usage: app.T_("Install additional kernel modules"),
					},
					&cli.BoolFlag{
						Name:  "headers",
						Usage: app.T_("Install kernel headers"),
						Value: false,
					},
					&cli.BoolFlag{
						Name:    "simulate",
						Usage:   app.T_("Simulate update"),
						Value:   false,
						Aliases: []string{"s"},
					},
				},
				Action: withRootCheckWrapper(func(ctx context.Context, cmd *cli.Command, actions *Actions) error {
					resp, err := actions.UpdateKernel(ctx, cmd.String("flavour"), cmd.StringSlice("modules"), cmd.Bool("headers"), cmd.Bool("simulate"))
					if err != nil {
						return reply.CliResponse(ctx, newErrorResponse(err.Error()))
					}
					return reply.CliResponse(ctx, *resp)
				}),
			},
			{
				Name:  "clean",
				Usage: app.T_("Remove old kernel versions"),
				Flags: []cli.Flag{
					&cli.BoolFlag{
						Name:  "no-backup",
						Usage: app.T_("Delete kernels even if it is in 'backup' state"),
						Value: false,
					},
					&cli.BoolFlag{
						Name:    "simulate",
						Usage:   app.T_("Show what would be removed without actually removing"),
						Value:   false,
						Aliases: []string{"s"},
					},
				},
				Action: withRootCheckWrapper(func(ctx context.Context, cmd *cli.Command, actions *Actions) error {
					resp, err := actions.CleanOldKernels(ctx, cmd.Bool("no-backup"), cmd.Bool("simulate"))
					if err != nil {
						return reply.CliResponse(ctx, newErrorResponse(err.Error()))
					}
					return reply.CliResponse(ctx, *resp)
				}),
			},
			{
				Name:  "modules",
				Usage: app.T_("Kernel modules management"),
				Commands: []*cli.Command{
					{
						Name:  "list",
						Usage: app.T_("List available modules for kernel"),
						Flags: []cli.Flag{
							&cli.StringFlag{
								Name:  "flavour",
								Usage: app.T_("List modules for specific kernel flavour (default: current flavour)"),
							},
						},
						Action: withRootCheckWrapper(func(ctx context.Context, cmd *cli.Command, actions *Actions) error {
							flavour := cmd.String("flavour")
							if flavour == "" {
								flavour = cmd.Args().First()
							}
							resp, err := actions.ListKernelModules(ctx, flavour)
							if err != nil {
								return reply.CliResponse(ctx, newErrorResponse(err.Error()))
							}
							return reply.CliResponse(ctx, *resp)
						}),
					},
					{
						Name:      "install",
						Usage:     app.T_("Install kernel modules"),
						ArgsUsage: "module-name [module-name...]",
						Flags: []cli.Flag{
							&cli.StringFlag{
								Name:  "flavour",
								Usage: app.T_("Install for specific kernel flavour"),
							},
							&cli.BoolFlag{
								Name:    "simulate",
								Usage:   app.T_("Show what would be installed without actually installing"),
								Aliases: []string{"s"},
								Value:   false,
							},
						},
						Action: withRootCheckWrapper(func(ctx context.Context, cmd *cli.Command, actions *Actions) error {
							modules := cmd.Args().Slice()
							if len(modules) == 0 {
								return reply.CliResponse(ctx, newErrorResponse(app.T_("At least one module must be specified")))
							}

							resp, err := actions.InstallKernelModules(ctx, cmd.String("flavour"), modules, cmd.Bool("simulate"))
							if err != nil {
								return reply.CliResponse(ctx, newErrorResponse(err.Error()))
							}
							return reply.CliResponse(ctx, *resp)
						}),
					},
					{
						Name:      "remove",
						Usage:     app.T_("Remove kernel modules"),
						ArgsUsage: "module-name [module-name...]",
						Flags: []cli.Flag{
							&cli.StringFlag{
								Name:  "flavour",
								Usage: app.T_("Remove from specific kernel flavour"),
							},
							&cli.BoolFlag{
								Name:    "simulate",
								Usage:   app.T_("Show what would be removed without actually removing"),
								Aliases: []string{"s"},
								Value:   false,
							},
						},
						Action: withRootCheckWrapper(func(ctx context.Context, cmd *cli.Command, actions *Actions) error {
							modules := cmd.Args().Slice()
							if len(modules) == 0 {
								return reply.CliResponse(ctx, newErrorResponse(app.T_("At least one module must be specified")))
							}

							resp, err := actions.RemoveKernelModules(ctx, cmd.String("flavour"), modules, cmd.Bool("simulate"))
							if err != nil {
								return reply.CliResponse(ctx, newErrorResponse(err.Error()))
							}
							return reply.CliResponse(ctx, *resp)
						}),
					},
				},
			},
			{
				Name:  "dbus-doc",
				Usage: app.T_("Show dbus online documentation"),
				Action: withRootCheckWrapper(func(ctx context.Context, cmd *cli.Command, actions *Actions) error {
					reply.StopSpinner(appConfig)
					return actions.GenerateOnlineDoc(ctx)
				}),
			},
		},
	}
}
