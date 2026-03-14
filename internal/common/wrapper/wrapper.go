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

package wrapper

import (
	"apm/internal/common/apmerr"
	"apm/internal/common/app"
	"apm/internal/common/helper"
	"apm/internal/common/reply"
	"context"
	"errors"
	"syscall"

	"github.com/urfave/cli/v3"
)

// RootCheckMode определяет режим проверки root-прав
type RootCheckMode int

const (
	// NoRootCheck - без проверки root
	NoRootCheck RootCheckMode = iota
	// RequireRoot - требует root права
	RequireRoot
	// ForbidRoot - запрещает root права
	ForbidRoot
)

// WithOptions создаёт универсальный wrapper для CLI команд с поддержкой generics.
// T - тип Actions для конкретного модуля.
func WithOptions[T any](
	rootCheck RootCheckMode,
	newActions func(*app.Config) *T,
	errorResponse func(error) reply.APIResponse,
) func(func(context.Context, *cli.Command, *T) error) cli.ActionFunc {
	return func(actionFunc func(context.Context, *cli.Command, *T) error) cli.ActionFunc {
		return func(ctx context.Context, cmd *cli.Command) error {
			appConfig := app.GetAppConfig(ctx)
			appConfig.ConfigManager.SetFormat(cmd.String("format"))
			if ft := cmd.String("format_type"); ft != "" {
				appConfig.ConfigManager.SetFormatType(ft)
			}
			if fields := cmd.StringSlice("output"); len(fields) > 0 {
				appConfig.ConfigManager.SetFields(fields)
			}
			ctx = context.WithValue(ctx, helper.TransactionKey, cmd.String("transaction"))

			if cmd.Bool("verbose") {
				appConfig.ConfigManager.EnableVerbose()
			}

			isRoot := syscall.Geteuid() == 0

			switch rootCheck {
			case NoRootCheck:
				// Без проверки, продолжаем выполнение
			case RequireRoot:
				if !isRoot {
					return reply.CliResponse(ctx, errorResponse(
						apmerr.New(apmerr.ErrorTypePermission, errors.New(app.T_("Elevated rights are required to perform this action. Please use sudo or su")))))
				}
			case ForbidRoot:
				if isRoot {
					return reply.CliResponse(ctx, errorResponse(
						apmerr.New(apmerr.ErrorTypePermission, errors.New(app.T_("Elevated rights are not allowed to perform this action. Please do not use sudo or su")))))
				}
			default:
				app.Log.Fatal("Unknown root check mode")
			}

			actions := newActions(appConfig)

			reply.CreateSpinner(appConfig)
			return actionFunc(ctx, cmd, actions)
		}
	}
}
