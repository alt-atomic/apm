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
	"apm/internal/common/helper"
	"apm/internal/common/reply"
	"apm/lib"
	"context"
	"syscall"

	"github.com/urfave/cli/v3"
)

// newErrorResponse создаёт ответ с ошибкой и указанным сообщением.
func newErrorResponse(message string) reply.APIResponse {
	lib.Log.Error(message)

	return reply.APIResponse{
		Data:  map[string]interface{}{"message": message},
		Error: true,
	}
}

func wrapperWithOptions(requireRoot bool) func(func(context.Context, *cli.Command, *Actions) error) cli.ActionFunc {
	return func(actionFunc func(context.Context, *cli.Command, *Actions) error) cli.ActionFunc {
		return func(ctx context.Context, cmd *cli.Command) error {
			lib.Env.Format = cmd.String("format")
			ctx = context.WithValue(ctx, helper.TransactionKey, cmd.String("transaction"))

			if requireRoot && syscall.Geteuid() != 0 {
				return reply.CliResponse(ctx, newErrorResponse(lib.T_("Elevated rights are required to perform this action. Please use sudo or su")))
			}

			actions := NewActions()

			reply.CreateSpinner()
			return actionFunc(ctx, cmd, actions)
		}
	}
}

var withRootCheckWrapper = wrapperWithOptions(true)

func CommandList() *cli.Command {
	return &cli.Command{
		Name:    "kernel",
		Aliases: []string{"k"},
		Usage:   lib.T_("Kernel management"),
		Commands: []*cli.Command{
			{
				Name:  "list",
				Usage: lib.T_("List available kernels"),
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:  "flavour",
						Usage: lib.T_("Filter by kernel flavour (e.g., std-def, un-def)"),
					},
					&cli.BoolFlag{
						Name:    "installed",
						Usage:   lib.T_("Show only installed kernels"),
						Aliases: []string{"i"},
						Value:   false,
					},
					&cli.BoolFlag{
						Name:  "full",
						Usage: lib.T_("Show full information"),
						Value: false,
					},
				},
				Action: withRootCheckWrapper(func(ctx context.Context, cmd *cli.Command, actions *Actions) error {
					resp, err := actions.ListKernels(ctx, cmd.String("flavour"), cmd.Bool("installed"), cmd.Bool("full"))
					if err != nil {
						return reply.CliResponse(ctx, newErrorResponse(err.Error()))
					}
					return reply.CliResponse(ctx, *resp)
				}),
			},
			{
				Name:  "info",
				Usage: lib.T_("Show information about current"),
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
				Usage:     lib.T_("Install kernel with specified flavour"),
				ArgsUsage: "flavour",
				Flags: []cli.Flag{
					&cli.StringSliceFlag{
						Name:  "modules",
						Usage: lib.T_("Install additional kernel modules"),
					},
					&cli.BoolFlag{
						Name:  "headers",
						Usage: lib.T_("Install kernel headers"),
						Value: false,
					},
					&cli.BoolFlag{
						Name:  "dry-run",
						Usage: lib.T_("Show what would be installed without actually installing"),
						Value: false,
					},
				},
				Action: withRootCheckWrapper(func(ctx context.Context, cmd *cli.Command, actions *Actions) error {
					flavour := cmd.Args().First()
					if flavour == "" {
						return reply.CliResponse(ctx, newErrorResponse(lib.T_("Kernel flavour must be specified")))
					}

					resp, err := actions.InstallKernel(ctx, flavour, cmd.StringSlice("modules"), cmd.Bool("headers"), cmd.Bool("dry-run"))
					if err != nil {
						return reply.CliResponse(ctx, newErrorResponse(err.Error()))
					}
					return reply.CliResponse(ctx, *resp)
				}),
			},
			{
				Name:  "update",
				Usage: lib.T_("Update kernel to latest version"),
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:  "flavour",
						Usage: lib.T_("Update to specific flavour (default: current flavour)"),
					},
					&cli.StringSliceFlag{
						Name:  "modules",
						Usage: lib.T_("Install additional kernel modules"),
					},
					&cli.BoolFlag{
						Name:  "headers",
						Usage: lib.T_("Install kernel headers"),
						Value: false,
					},
					&cli.BoolFlag{
						Name:  "dry-run",
						Usage: lib.T_("Show what would be updated without actually updating"),
						Value: false,
					},
				},
				Action: withRootCheckWrapper(func(ctx context.Context, cmd *cli.Command, actions *Actions) error {
					resp, err := actions.UpdateKernel(ctx, cmd.String("flavour"), cmd.StringSlice("modules"), cmd.Bool("headers"), cmd.Bool("dry-run"))
					if err != nil {
						return reply.CliResponse(ctx, newErrorResponse(err.Error()))
					}
					return reply.CliResponse(ctx, *resp)
				}),
			},
			{
				Name:  "check-update",
				Usage: lib.T_("Check for kernel updates"),
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:  "flavour",
						Usage: lib.T_("Check updates for specific flavour"),
					},
				},
				Action: withRootCheckWrapper(func(ctx context.Context, cmd *cli.Command, actions *Actions) error {
					resp, err := actions.CheckKernelUpdate(ctx, cmd.String("flavour"))
					if err != nil {
						return reply.CliResponse(ctx, newErrorResponse(err.Error()))
					}
					return reply.CliResponse(ctx, *resp)
				}),
			},
			{
				Name:  "clean",
				Usage: lib.T_("Remove old kernel versions"),
				Flags: []cli.Flag{
					&cli.IntFlag{
						Name:  "keep",
						Usage: lib.T_("Number of kernel versions to keep"),
						Value: 2,
					},
					&cli.BoolFlag{
						Name:  "dry-run",
						Usage: lib.T_("Show what would be removed without actually removing"),
						Value: false,
					},
				},
				Action: withRootCheckWrapper(func(ctx context.Context, cmd *cli.Command, actions *Actions) error {
					resp, err := actions.CleanOldKernels(ctx, cmd.Int("keep"), cmd.Bool("dry-run"))
					if err != nil {
						return reply.CliResponse(ctx, newErrorResponse(err.Error()))
					}
					return reply.CliResponse(ctx, *resp)
				}),
			},
			{
				Name:  "modules",
				Usage: lib.T_("Kernel modules management"),
				Commands: []*cli.Command{
					{
						Name:  "list",
						Usage: lib.T_("List available modules for kernel"),
						Flags: []cli.Flag{
							&cli.StringFlag{
								Name:  "flavour",
								Usage: lib.T_("List modules for specific kernel flavour (default: current flavour)"),
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
						Usage:     lib.T_("Install kernel modules"),
						ArgsUsage: "module-name [module-name...]",
						Flags: []cli.Flag{
							&cli.StringFlag{
								Name:  "flavour",
								Usage: lib.T_("Install for specific kernel flavour"),
							},
							&cli.BoolFlag{
								Name:  "dry-run",
								Usage: lib.T_("Show what would be installed without actually installing"),
								Value: false,
							},
						},
						Action: withRootCheckWrapper(func(ctx context.Context, cmd *cli.Command, actions *Actions) error {
							modules := cmd.Args().Slice()
							if len(modules) == 0 {
								return reply.CliResponse(ctx, newErrorResponse(lib.T_("At least one module must be specified")))
							}

							resp, err := actions.InstallKernelModules(ctx, cmd.String("flavour"), modules, cmd.Bool("dry-run"))
							if err != nil {
								return reply.CliResponse(ctx, newErrorResponse(err.Error()))
							}
							return reply.CliResponse(ctx, *resp)
						}),
					},
					{
						Name:      "remove",
						Usage:     lib.T_("Remove kernel modules"),
						ArgsUsage: "module-name [module-name...]",
						Flags: []cli.Flag{
							&cli.StringFlag{
								Name:  "flavour",
								Usage: lib.T_("Remove from specific kernel flavour"),
							},
							&cli.BoolFlag{
								Name:  "dry-run",
								Usage: lib.T_("Show what would be removed without actually removing"),
								Value: false,
							},
						},
						Action: withRootCheckWrapper(func(ctx context.Context, cmd *cli.Command, actions *Actions) error {
							modules := cmd.Args().Slice()
							if len(modules) == 0 {
								return reply.CliResponse(ctx, newErrorResponse(lib.T_("At least one module must be specified")))
							}

							resp, err := actions.RemoveKernelModules(ctx, cmd.String("flavour"), modules, cmd.Bool("dry-run"))
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
