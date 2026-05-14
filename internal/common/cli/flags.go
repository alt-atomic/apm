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

// RootFlags возвращает набор глобальных флагов корневой команды.
func RootFlags() []cli.Flag {
	return []cli.Flag{
		&cli.StringFlag{
			Name:    "format",
			Usage:   app.T_("Output format: json, text"),
			Aliases: []string{"f"},
			Value:   "text",
		},
		&cli.StringFlag{
			Name:    "format-type",
			Usage:   app.T_("Display type: tree, plain"),
			Aliases: []string{"ft"},
		},
		&cli.StringSliceFlag{
			Name:    "output",
			Usage:   app.T_("Output only specified fields"),
			Aliases: []string{"o"},
		},
		&cli.StringFlag{
			Name:    "transaction",
			Usage:   app.T_("Internal property, adds the transaction to the output"),
			Aliases: []string{"t"},
		},
		&cli.BoolFlag{
			Name:    "verbose",
			Aliases: []string{"v"},
			Usage:   app.T_("Enable verbose logging to stdout"),
		},
	}
}
