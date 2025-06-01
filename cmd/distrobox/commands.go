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
		Name:    "distrobox",
		Aliases: []string{"d"},
		Usage:   lib.T_("Managing packages and containers in distrobox"),
		Commands: []*cli.Command{
			{
				Name:  "update",
				Usage: lib.T_("Update and synchronize the list of installed packages with the host"),
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:     "container",
						Usage:    lib.T_("Container name. Required"),
						Aliases:  []string{"c"},
						Required: true,
					},
				},
				Action: withGlobalWrapper(func(ctx context.Context, cmd *cli.Command) error {
					resp, err := NewActions().Update(ctx, cmd.String("container"))
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
					&cli.StringFlag{
						Name:     "container",
						Usage:    lib.T_("Container name. Required"),
						Aliases:  []string{"c"},
						Required: true,
					},
				},
				Action: withGlobalWrapper(func(ctx context.Context, cmd *cli.Command) error {
					resp, err := NewActions().Info(ctx, cmd.String("container"), cmd.Args().First())
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
					&cli.StringFlag{
						Name:    "container",
						Usage:   lib.T_("Container name. Optional flag"),
						Aliases: []string{"c"},
					},
				},
				Action: withGlobalWrapper(func(ctx context.Context, cmd *cli.Command) error {
					resp, err := NewActions().Search(ctx, cmd.String("container"), cmd.Args().First())
					if err != nil {
						return reply.CliResponse(ctx, newErrorResponse(err.Error()))
					}

					return reply.CliResponse(ctx, *resp)
				}),
			},
			{
				Name:  "list",
				Usage: lib.T_("Building query to retrieve package list"),
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:    "container",
						Usage:   lib.T_("Container name. Optional flag"),
						Aliases: []string{"c"},
					},
					&cli.StringFlag{
						Name:  "sort",
						Usage: lib.T_("Field for sorting, for example: name, version"),
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
						Usage: lib.T_("Force update all packages before the request"),
						Value: false,
					},
				},
				Action: withGlobalWrapper(func(ctx context.Context, cmd *cli.Command) error {
					params := ListParams{
						Container:   cmd.String("container"),
						Sort:        cmd.String("sort"),
						Order:       cmd.String("order"),
						Offset:      cmd.Int("offset"),
						Limit:       cmd.Int("limit"),
						Filters:     cmd.StringSlice("filter"),
						ForceUpdate: cmd.Bool("force-update"),
					}

					resp, err := NewActions().List(ctx, params)
					if err != nil {
						return reply.CliResponse(ctx, newErrorResponse(err.Error()))
					}

					return reply.CliResponse(ctx, *resp)
				}),
			},
			{
				Name:      "install",
				Usage:     lib.T_("Install package"),
				ArgsUsage: "package",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:     "container",
						Usage:    lib.T_("Container name. Required"),
						Aliases:  []string{"c"},
						Required: true,
					},
					&cli.BoolFlag{
						Name:  "export",
						Usage: lib.T_("Export package"),
						Value: true,
					},
				},
				Action: withGlobalWrapper(func(ctx context.Context, cmd *cli.Command) error {
					resp, err := NewActions().Install(ctx, cmd.String("container"), cmd.Args().First(), cmd.Bool("export"))
					if err != nil {
						return reply.CliResponse(ctx, newErrorResponse(err.Error()))
					}

					return reply.CliResponse(ctx, *resp)
				}),
			},
			{
				Name:      "remove",
				Usage:     lib.T_("Remove package"),
				ArgsUsage: "package",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:     "container",
						Usage:    lib.T_("Container name. Required"),
						Aliases:  []string{"c"},
						Required: true,
					},
					&cli.BoolFlag{
						Name:  "only-host",
						Usage: lib.T_("Remove only from host, leave package in container"),
						Value: false,
					},
				},
				Action: withGlobalWrapper(func(ctx context.Context, cmd *cli.Command) error {
					resp, err := NewActions().Remove(ctx, cmd.String("container"), cmd.Args().First(), cmd.Bool("only-host"))
					if err != nil {
						return reply.CliResponse(ctx, newErrorResponse(err.Error()))
					}

					return reply.CliResponse(ctx, *resp)
				}),
			},
			{
				Name:    "container",
				Usage:   lib.T_("Module for working with containers"),
				Aliases: []string{"c"},
				Commands: []*cli.Command{
					{
						Name:  "list",
						Usage: lib.T_("List of containers"),
						Action: withGlobalWrapper(func(ctx context.Context, cmd *cli.Command) error {
							resp, err := NewActions().ContainerList(ctx)
							if err != nil {
								return reply.CliResponse(ctx, newErrorResponse(err.Error()))
							}

							return reply.CliResponse(ctx, *resp)
						}),
					},
					{
						Name:  "create",
						Usage: lib.T_("Add container"),
						Flags: []cli.Flag{
							&cli.StringFlag{
								Name:     "image",
								Usage:    lib.T_("Container. Must be specified, options: alt, ubuntu, arch"),
								Required: true,
							},
						},
						Action: withGlobalWrapper(func(ctx context.Context, cmd *cli.Command) error {
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
									newErrorResponse(lib.T_("The value for image must be one of: alt, ubuntu, arch")))
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

							resp, err := NewActions().ContainerAdd(ctx, imageLink, "atomic-"+imageVal, "zsh mc nano", "")
							if err != nil {
								return reply.CliResponse(ctx, newErrorResponse(err.Error()))
							}

							return reply.CliResponse(ctx, *resp)
						}),
					},
					{
						Name:  "create-manual",
						Usage: lib.T_("Manual container addition"),
						Flags: []cli.Flag{
							&cli.StringFlag{
								Name:     "image",
								Usage:    lib.T_("Image link. Required"),
								Required: true,
							},
							&cli.StringFlag{
								Name:     "name",
								Usage:    lib.T_("Container name. Required"),
								Required: true,
							},
							&cli.StringFlag{
								Name:  "additional-packages",
								Usage: lib.T_("List of packages to install"),
								Value: "zsh",
							},
							&cli.StringFlag{
								Name:  "init-hooks",
								Usage: lib.T_("Calling hook to execute commands"),
							},
						},
						Action: withGlobalWrapper(func(ctx context.Context, cmd *cli.Command) error {
							imageVal := cmd.String("image")
							nameVal := cmd.String("name")
							addPkgVal := cmd.String("additional-packages")
							hookVal := cmd.String("init-hooks")

							resp, err := NewActions().ContainerAdd(ctx, imageVal, nameVal, addPkgVal, hookVal)
							if err != nil {
								return reply.CliResponse(ctx, newErrorResponse(err.Error()))
							}

							return reply.CliResponse(ctx, *resp)
						}),
					},
					{
						Name:    "remove",
						Usage:   lib.T_("Remove container"),
						Aliases: []string{"rm"},
						Flags: []cli.Flag{
							&cli.StringFlag{
								Name:     "name",
								Usage:    lib.T_("Container name. Required"),
								Required: true,
							},
						},
						Action: withGlobalWrapper(func(ctx context.Context, cmd *cli.Command) error {
							resp, err := NewActions().ContainerRemove(ctx, cmd.String("name"))
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
