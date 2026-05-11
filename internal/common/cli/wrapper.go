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

package cli

import (
	"apm/internal/common/apmerr"
	"apm/internal/common/app"
	"apm/internal/common/helper"
	"apm/internal/common/reply"
	"context"
	"errors"
	"fmt"
	"syscall"

	urfave "github.com/urfave/cli/v3"
)

// RootCheckMode определяет режим проверки root-прав.
type RootCheckMode int

const (
	NoRootCheck RootCheckMode = iota
	RequireRoot
	ForbidRoot
)

// CheckRoot проверяет, что текущие права совместимы с заданным режимом.
func CheckRoot(mode RootCheckMode) error {
	return evaluateRootCheck(syscall.Geteuid() == 0, mode)
}

func evaluateRootCheck(isRoot bool, mode RootCheckMode) error {
	switch mode {
	case NoRootCheck:
		return nil
	case RequireRoot:
		if !isRoot {
			return errors.New(app.T_("Elevated rights are required to perform this action. Please use sudo or su"))
		}
	case ForbidRoot:
		if isRoot {
			return errors.New(app.T_("Elevated rights are not allowed to perform this action. Please do not use sudo or su"))
		}
	default:
		return fmt.Errorf("unknown root check mode: %d", mode)
	}
	return nil
}

// WithOptions создаёт универсальный wrapper для CLI команд с поддержкой generics.
// T — тип Actions для конкретного модуля.
func WithOptions[T any](
	appConfig *app.Config,
	reporter *reply.Reporter,
	rootCheck RootCheckMode,
	newActions func(*app.Config, *reply.Reporter) *T,
	errorResponse func(error) reply.APIResponse,
) func(func(context.Context, *urfave.Command, *T) error) urfave.ActionFunc {
	return func(actionFunc func(context.Context, *urfave.Command, *T) error) urfave.ActionFunc {
		return func(ctx context.Context, cmd *urfave.Command) error {
			appConfig.ConfigManager.SetFormat(cmd.String("format"))
			if ft := cmd.String("format-type"); ft != "" {
				appConfig.ConfigManager.SetFormatType(ft)
			}
			if fields := cmd.StringSlice("output"); len(fields) > 0 {
				appConfig.ConfigManager.SetFields(fields)
			}
			ctx = context.WithValue(ctx, helper.TransactionKey, cmd.String("transaction"))

			if cmd.Bool("verbose") {
				appConfig.ConfigManager.EnableVerbose()
			}

			if err := CheckRoot(rootCheck); err != nil {
				return reporter.CliResponse(ctx, errorResponse(
					apmerr.New(apmerr.ErrorTypePermission, err)))
			}

			actions := newActions(appConfig, reporter)

			reply.CreateSpinner(appConfig)
			return actionFunc(ctx, cmd, actions)
		}
	}
}
