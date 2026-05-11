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
	"apm/internal/common/app"

	"github.com/urfave/cli/v3"
)

// HelpCommand возвращает стандартную команду help.
func HelpCommand() *cli.Command {
	return &cli.Command{
		Name:      "help",
		Aliases:   []string{"h"},
		Usage:     app.T_("Show the list of commands or help for each command"),
		ArgsUsage: app.T_("[command]"),
		HideHelp:  true,
	}
}

// VersionCommand возвращает команду version с заданным action.
func VersionCommand(action cli.ActionFunc) *cli.Command {
	return &cli.Command{
		Name:      "version",
		Usage:     app.T_("Print version"),
		ArgsUsage: app.T_("[command]"),
		Action:    action,
	}
}

// NewDBusCommand строит CLI команду для запуска D-Bus сервиса.
func NewDBusCommand(name, usage string, action cli.ActionFunc) *cli.Command {
	return &cli.Command{
		Name:     name,
		Usage:    usage,
		Category: app.T_("Services"),
		Action:   action,
	}
}

// NewHTTPCommand строит CLI команду для запуска HTTP сервиса.
func NewHTTPCommand(name, usage, defaultListen string, action cli.ActionFunc) *cli.Command {
	return &cli.Command{
		Name:     name,
		Usage:    usage,
		Category: app.T_("Services"),
		Action:   action,
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    "listen",
				Aliases: []string{"l"},
				Usage:   app.T_("Listen address (host:port)"),
				Value:   defaultListen,
			},
			&cli.StringFlag{
				Name:    "api-token",
				Usage:   app.T_("API token in format <read|manage>:<token> (prefer APM_API_TOKEN env)"),
				Sources: cli.EnvVars("APM_API_TOKEN"),
			},
		},
	}
}
