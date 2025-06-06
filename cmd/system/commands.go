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
	"apm/cmd/common/reply"
	"apm/lib"
	"context"
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
		Usage:   lib.T_("System package management"),
		Commands: []*cli.Command{
			{
				Name:      "install",
				Usage:     lib.T_("Package list for installation. The format package- package+ is supported."),
				ArgsUsage: "packages",
				Flags: []cli.Flag{
					&cli.BoolFlag{
						Name:    "apply",
						Usage:   lib.T_("Apply to image"),
						Aliases: []string{"a"},
						Value:   false,
						Hidden:  !lib.Env.IsAtomic,
					},
				},
				Action: withGlobalWrapper(func(ctx context.Context, cmd *cli.Command) error {
					resp, err := NewActions().Install(ctx, cmd.Args().Slice(), cmd.Bool("apply"))
					if err != nil {
						return reply.CliResponse(ctx, newErrorResponse(err.Error()))
					}

					return reply.CliResponse(ctx, *resp)
				}),
			},
			{
				Name:      "remove",
				Usage:     lib.T_("List of packages to remove"),
				ArgsUsage: "packages",
				Flags: []cli.Flag{
					&cli.BoolFlag{
						Name:    "apply",
						Usage:   lib.T_("Apply to image"),
						Aliases: []string{"a"},
						Value:   false,
						Hidden:  !lib.Env.IsAtomic,
					},
				},
				Action: withGlobalWrapper(func(ctx context.Context, cmd *cli.Command) error {
					resp, err := NewActions().Remove(ctx, cmd.Args().Slice(), cmd.Bool("apply"))
					if err != nil {
						return reply.CliResponse(ctx, newErrorResponse(err.Error()))
					}

					return reply.CliResponse(ctx, *resp)
				}),
			},
			{
				Name:  "update",
				Usage: lib.T_("Updating package database"),
				Action: withGlobalWrapper(func(ctx context.Context, cmd *cli.Command) error {
					resp, err := NewActions().Update(ctx)
					if err != nil {
						return reply.CliResponse(ctx, newErrorResponse(err.Error()))
					}

					return reply.CliResponse(ctx, *resp)
				}),
			},
			{
				Name:  "upgrade",
				Usage: lib.T_("General system upgrade"),
				Action: withGlobalWrapper(func(ctx context.Context, cmd *cli.Command) error {
					var resp *reply.APIResponse
					var err error
					if lib.Env.IsAtomic {
						resp, err = NewActions().ImageUpdate(ctx)
					} else {
						resp, err = NewActions().Upgrade(ctx)
					}

					if err != nil {
						return reply.CliResponse(ctx, newErrorResponse(err.Error()))
					}

					return reply.CliResponse(ctx, *resp)
				}),
			},
			{
				Name:      "info",
				Usage:     lib.T_("Package information"),
				ArgsUsage: "package",
				Flags: []cli.Flag{
					&cli.BoolFlag{
						Name:  "full",
						Usage: lib.T_("Full output of information"),
						Value: false,
					},
				},
				Action: withGlobalWrapper(func(ctx context.Context, cmd *cli.Command) error {
					resp, err := NewActions().Info(ctx, cmd.Args().First(), cmd.Bool("full"))
					if err != nil {
						return reply.CliResponse(ctx, newErrorResponse(err.Error()))
					}

					return reply.CliResponse(ctx, *resp)
				}),
			},
			{
				Name:      "search",
				Usage:     lib.T_("Quick package search by name"),
				ArgsUsage: "package",
				Flags: []cli.Flag{
					&cli.BoolFlag{
						Name:    "installed",
						Usage:   lib.T_("Only installed"),
						Aliases: []string{"i"},
						Value:   false,
					},
					&cli.BoolFlag{
						Name:  "full",
						Usage: lib.T_("Full information output"),
						Value: false,
					},
				},
				Action: withGlobalWrapper(func(ctx context.Context, cmd *cli.Command) error {
					resp, err := NewActions().Search(ctx, cmd.Args().First(), cmd.Bool("installed"), cmd.Bool("full"))
					if err != nil {
						return reply.CliResponse(ctx, newErrorResponse(err.Error()))
					}

					return reply.CliResponse(ctx, *resp)
				}),
			},
			{
				Name:  "list",
				Usage: "Построение запроса для получения списка пакетов",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:  "sort",
						Usage: lib.T_("Building query to fetch package list"),
					},
					&cli.StringFlag{
						Name:  "order",
						Usage: lib.T_("Sorting order: ASC or DESC"),
						Value: "ASC",
					},
					&cli.IntFlag{
						Name:  "limit",
						Usage: lib.T_("Maximum number of records to return"),
						Value: 10,
					},
					&cli.IntFlag{
						Name:  "offset",
						Usage: lib.T_("Starting position (offset) for the result set"),
						Value: 0,
					},
					&cli.StringSliceFlag{
						Name:  "filter",
						Usage: lib.T_("Filter in the format key=value. The flag can be specified multiple times, for example: --filter name=zip --filter installed=true"),
					},
					&cli.BoolFlag{
						Name:  "force-update",
						Usage: lib.T_("Force update all packages before query"),
						Value: false,
					},
					&cli.BoolFlag{
						Name:  "full",
						Usage: lib.T_("Full information output"),
						Value: false,
					},
				},
				Action: withGlobalWrapper(func(ctx context.Context, cmd *cli.Command) error {
					params := ListParams{
						Sort:        cmd.String("sort"),
						Order:       cmd.String("order"),
						Offset:      cmd.Int("offset"),
						Limit:       cmd.Int("limit"),
						Filters:     cmd.StringSlice("filter"),
						ForceUpdate: cmd.Bool("force-update"),
					}

					resp, err := NewActions().List(ctx, params, cmd.Bool("full"))
					if err != nil {
						return reply.CliResponse(ctx, newErrorResponse(err.Error()))
					}

					return reply.CliResponse(ctx, *resp)
				}),
			},
			{
				Name:    "image",
				Usage:   lib.T_("Module for working with the image"),
				Aliases: []string{"i"},
				Hidden:  !lib.Env.IsAtomic,
				Commands: []*cli.Command{
					{
						Name:  "apply",
						Usage: lib.T_("Apply changes to the host"),
						Action: withGlobalWrapper(func(ctx context.Context, cmd *cli.Command) error {
							resp, err := NewActions().ImageApply(ctx)
							if err != nil {
								return reply.CliResponse(ctx, newErrorResponse(err.Error()))
							}

							return reply.CliResponse(ctx, *resp)
						}),
					},
					{
						Name:  "status",
						Usage: lib.T_("Image status"),
						Action: withGlobalWrapper(func(ctx context.Context, cmd *cli.Command) error {
							resp, err := NewActions().ImageStatus(ctx)
							if err != nil {
								return reply.CliResponse(ctx, newErrorResponse(err.Error()))
							}

							return reply.CliResponse(ctx, *resp)
						}),
					},
					{
						Name:  "update",
						Usage: lib.T_("Image update"),
						Action: withGlobalWrapper(func(ctx context.Context, cmd *cli.Command) error {
							resp, err := NewActions().ImageUpdate(ctx)
							if err != nil {
								return reply.CliResponse(ctx, newErrorResponse(err.Error()))
							}

							return reply.CliResponse(ctx, *resp)
						}),
					},
					{
						Name:  "history",
						Usage: lib.T_("Image changes history"),
						Flags: []cli.Flag{
							&cli.StringFlag{
								Name:  "image",
								Usage: lib.T_("Filter by image name"),
							},
							&cli.IntFlag{
								Name:  "limit",
								Usage: lib.T_("Maximum number of records to return"),
								Value: 10,
							},
							&cli.IntFlag{
								Name:  "offset",
								Usage: lib.T_("Starting position (offset) for the result set"),
								Value: 0,
							},
						},
						Action: withGlobalWrapper(func(ctx context.Context, cmd *cli.Command) error {
							resp, err := NewActions().ImageHistory(ctx, cmd.String("image"), cmd.Int("limit"), cmd.Int("offset"))
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
