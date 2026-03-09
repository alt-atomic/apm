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

package dialog

import (
	"apm/internal/common/app"
	"apm/internal/common/sandbox"
	"errors"
	"fmt"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type selectorModel struct {
	containers []sandbox.ContainerInfo
	cursor     int
	selected   string
	canceled   bool
	quitting   bool
	colors     app.Colors
}

func newSelectorModel(containers []sandbox.ContainerInfo, colors app.Colors) selectorModel {
	return selectorModel{
		containers: containers,
		colors:     colors,
	}
}

func (m selectorModel) Init() tea.Cmd {
	return nil
}

func (m selectorModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlC, tea.KeyEsc:
			m.canceled = true
			m.quitting = true
			return m, tea.Quit
		case tea.KeyEnter:
			if m.cursor == len(m.containers) {
				m.canceled = true
			} else {
				m.selected = m.containers[m.cursor].ContainerName
			}
			m.quitting = true
			return m, tea.Quit
		case tea.KeyUp:
			m.cursor--
			if m.cursor < 0 {
				m.cursor = len(m.containers)
			}
		case tea.KeyDown:
			m.cursor++
			if m.cursor > len(m.containers) {
				m.cursor = 0
			}
		case tea.KeyRunes:
			switch msg.String() {
			case "j":
				m.cursor++
				if m.cursor > len(m.containers) {
					m.cursor = 0
				}
			case "k":
				m.cursor--
				if m.cursor < 0 {
					m.cursor = len(m.containers)
				}
			case "q":
				m.canceled = true
				m.quitting = true
				return m, tea.Quit
			}
		default:
		}
	}
	return m, nil
}

func (m selectorModel) View() string {
	if m.quitting {
		return ""
	}

	titleStyle := lipgloss.NewStyle().Bold(true).
		Foreground(lipgloss.Color(m.colors.Accent))
	activeStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(m.colors.DialogAction))
	itemStyle := lipgloss.NewStyle().
		Foreground(lipgloss.AdaptiveColor{
			Light: m.colors.TextLight,
			Dark:  m.colors.TextDark,
		})
	hintStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(m.colors.DialogHint)).Faint(true)

	var sb strings.Builder
	sb.WriteString(titleStyle.Render(app.T_("Select container:")))
	sb.WriteString("\n")

	for i, c := range m.containers {
		osSuffix := ""
		if c.OS != "" {
			osSuffix = hintStyle.Render(fmt.Sprintf(" (%s)", c.OS))
		}

		if i == m.cursor {
			sb.WriteString(activeStyle.Render("  › "+c.ContainerName) + osSuffix + "\n")
		} else {
			sb.WriteString(itemStyle.Render("    "+c.ContainerName) + osSuffix + "\n")
		}
	}

	cancelStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(m.colors.DialogDanger))
	cancelLabel := app.T_("Cancel")
	if m.cursor == len(m.containers) {
		sb.WriteString(cancelStyle.Render("  › "+cancelLabel) + "\n")
	} else {
		sb.WriteString(hintStyle.Render("    "+cancelLabel) + "\n")
	}

	sb.WriteString(hintStyle.Render(app.T_("Navigation: ↑/↓ or j/k - select, Enter - confirm, Esc/q - cancel")))

	return sb.String()
}

// SelectContainer показывает компактный inline-селектор контейнера
func SelectContainer(containers []sandbox.ContainerInfo, colors app.Colors) (string, error) {
	if len(containers) == 0 {
		return "", errors.New(app.T_("No containers found"))
	}

	if len(containers) == 1 {
		return containers[0].ContainerName, nil
	}

	m := newSelectorModel(containers, colors)
	p := tea.NewProgram(m,
		tea.WithOutput(os.Stdout),
		tea.WithoutSignalHandler())

	finalModel, err := p.Run()
	if err != nil {
		return "", fmt.Errorf(app.T_("Error starting selector: %v"), err)
	}

	if result, ok := finalModel.(selectorModel); ok {
		if result.canceled || result.selected == "" {
			return "", errors.New(app.T_("Operation cancelled"))
		}
		return result.selected, nil
	}

	return "", errors.New(app.T_("Operation cancelled"))
}
