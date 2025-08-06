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

package helper

import (
	"apm/lib"
	"fmt"
	"github.com/charmbracelet/lipgloss"

	"github.com/urfave/cli/v3"
)

// SetupHelpTemplates overrides the templates for the root command, commands, and subcommands
func SetupHelpTemplates() {
	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#a2734c"))

	// Overriding the "root" template (the equivalent of AppHelpTemplate)
	cli.RootCommandHelpTemplate = fmt.Sprintf(`%s
   {{template "helpNameTemplate" .}}

%s
   {{if .UsageText}}{{wrap .UsageText 3}}{{else}}{{.FullName}} [command [command options]]{{end}}{{if .Version}}{{if not .HideVersion}}

%s
   {{.Version}}{{end}}{{end}}{{if .Description}}

%s
   {{template "descriptionTemplate" .}}{{end}}{{if .VisibleCommands}}

%s{{template "visibleCommandCategoryTemplate" .}}{{end}}{{if .VisibleFlagCategories}}

%s{{template "visibleFlagCategoryTemplate" .}}{{else if .VisibleFlags}}

%s{{template "visibleFlagTemplate" .}}{{end}}
`,
		titleStyle.Render(lib.T_("Module:")),
		titleStyle.Render(lib.T_("Usage:")),
		titleStyle.Render(lib.T_("Version:")),
		titleStyle.Render(lib.T_("Description:")),
		titleStyle.Render(lib.T_("Commands:")),
		titleStyle.Render(lib.T_("Options:")),
		titleStyle.Render(lib.T_("Options:")),
	)

	// Overriding the template for "command" help
	cli.CommandHelpTemplate = fmt.Sprintf(`%s
   {{template "helpNameTemplate" .}}

%s
   {{template "usageTemplate" .}}{{if .Category}}

%s
   {{.Category}}{{end}}{{if .Description}}

%s
   {{template "descriptionTemplate" .}}{{end}}{{if .VisibleFlagCategories}}

%s{{template "visibleFlagCategoryTemplate" .}}{{else if .VisibleFlags}}

%s{{template "visibleFlagTemplate" .}}{{end}}{{if .VisiblePersistentFlags}}

%s{{template "visiblePersistentFlagTemplate" .}}{{end}}
`,
		titleStyle.Render(lib.T_("Module:")),
		titleStyle.Render(lib.T_("Usage:")),
		titleStyle.Render(lib.T_("Category:")),
		titleStyle.Render(lib.T_("Description:")),
		titleStyle.Render(lib.T_("Options:")),
		titleStyle.Render(lib.T_("Options:")),
		titleStyle.Render(lib.T_("Global options:")),
	)

	// Overriding the template for "subcommand" help (if nested commands are used)
	cli.SubcommandHelpTemplate = fmt.Sprintf(`%s
   {{template "helpNameTemplate" .}}

%s
   {{if .UsageText}}{{wrap .UsageText 3}}{{else}}{{.FullName}} [command [command options]]{{end}}{{if .Category}}

%s
   {{.Category}}{{end}}{{if .Description}}

%s
   {{template "descriptionTemplate" .}}{{end}}{{if .VisibleCommands}}

%s{{template "visibleCommandTemplate" .}}{{end}}{{if .VisibleFlagCategories}}

%s{{template "visibleFlagCategoryTemplate" .}}{{else if .VisibleFlags}}

%s{{template "visibleFlagTemplate" .}}{{end}}
`,
		titleStyle.Render(lib.T_("Module:")),
		titleStyle.Render(lib.T_("Usage:")),
		titleStyle.Render(lib.T_("Category:")),
		titleStyle.Render(lib.T_("Description:")),
		titleStyle.Render(lib.T_("Commands:")),
		titleStyle.Render(lib.T_("Options:")),
		titleStyle.Render(lib.T_("Options:")),
	)
}
