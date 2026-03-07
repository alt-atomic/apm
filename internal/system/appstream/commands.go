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

package appstream

import (
	"apm/internal/common/app"
	"apm/internal/common/helper"
	"apm/internal/common/reply"
	"apm/internal/common/swcat"
	"apm/internal/common/wrapper"
	"context"
	"errors"

	"github.com/urfave/cli/v3"
)

func newErrorResponseFromError(err error) reply.APIResponse {
	app.Log.Error(err.Error())
	return reply.ErrorResponseFromError(err)
}

var withRootCheckWrapper = wrapper.WithOptions(wrapper.RequireRoot, NewActions, newErrorResponseFromError)
var withGlobalWrapper = wrapper.WithOptions(wrapper.NoRootCheck, NewActions, newErrorResponseFromError)

// CommandList возвращает список CLI-подкоманд для AppStream модуля.
func CommandList(_ context.Context) []*cli.Command {
	return []*cli.Command{
		{
			Name:  "update",
			Usage: app.T_("Update applications data from XML catalogs"),
			Action: withRootCheckWrapper(func(ctx context.Context, cmd *cli.Command, actions *Actions) error {
				resp, err := actions.Update(ctx)
				if err != nil {
					return reply.CliResponse(ctx, newErrorResponseFromError(err))
				}
				return reply.CliResponse(ctx, reply.OK(resp))
			}),
		},
		{
			Name:      "info",
			Usage:     app.T_("Show applications data for a package"),
			ArgsUsage: "<package_name>",
			Action: withGlobalWrapper(func(ctx context.Context, cmd *cli.Command, actions *Actions) error {
				if cmd.Args().Len() == 0 {
					return reply.CliResponse(ctx, newErrorResponseFromError(errors.New(app.T_("Package name is required"))))
				}
				resp, err := actions.Info(ctx, cmd.Args().First())
				if err != nil {
					return reply.CliResponse(ctx, newErrorResponseFromError(err))
				}
				return reply.CliResponse(ctx, reply.OK(resp))
			}),
		},
		{
			Name:  "categories",
			Usage: app.T_("Show all available application categories"),
			Action: withGlobalWrapper(func(ctx context.Context, cmd *cli.Command, actions *Actions) error {
				resp, err := actions.Categories(ctx)
				if err != nil {
					return reply.CliResponse(ctx, newErrorResponseFromError(err))
				}
				return reply.CliResponse(ctx, reply.OK(resp))
			}),
		},
		{
			Name:        "list",
			Usage:       app.T_("Building a query to get a list of components"),
			Description: helper.FilterDescription("--filter pkgname=steam --filter components.type[eq]=desktop-application"),
			Flags: []cli.Flag{
				&cli.StringFlag{
					Name:  "sort",
					Usage: app.T_("Sort by field, example fields: pkgname"),
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
			},
			Action: withGlobalWrapper(func(ctx context.Context, cmd *cli.Command, actions *Actions) error {
				filters, err := swcat.FilterConfig.Parse(cmd.StringSlice("filter"))
				if err != nil {
					return reply.CliResponse(ctx, newErrorResponseFromError(err))
				}

				params := ListParams{
					Sort:    cmd.String("sort"),
					Order:   cmd.String("order"),
					Offset:  cmd.Int("offset"),
					Limit:   cmd.Int("limit"),
					Filters: filters,
				}

				resp, err := actions.List(ctx, params)
				if err != nil {
					return reply.CliResponse(ctx, newErrorResponseFromError(err))
				}
				return reply.CliResponse(ctx, reply.OK(resp))
			}),
		},
	}
}
