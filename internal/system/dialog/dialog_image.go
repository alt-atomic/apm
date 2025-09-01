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
	"apm/internal/common/reply"
	"apm/lib"
	"errors"
	"fmt"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/lipgloss/list"
)

type PackageSelectionResult struct {
	InstallPackages []string
	RemovePackages  []string
	Canceled        bool
}

type selectionModel struct {
	installPackages []packageItem
	removePackages  []packageItem
	currentPanel    int
	cursor          int
	choices         []string
	choice          string
	canceled        bool
	width           int
	height          int
}

type packageItem struct {
	name     string
	selected bool
}

// NewPackageSelectionDialog запускает диалог выбора пакетов для установки/удаления
func NewPackageSelectionDialog(installPkgs, removePkgs []string) (*PackageSelectionResult, error) {
	if lib.Env.Format != "text" || !reply.IsTTY() {
		return &PackageSelectionResult{
			InstallPackages: installPkgs,
			RemovePackages:  removePkgs,
			Canceled:        false,
		}, nil
	}

	installItems := make([]packageItem, len(installPkgs))
	for i, pkg := range installPkgs {
		installItems[i] = packageItem{name: pkg, selected: true}
	}

	removeItems := make([]packageItem, len(removePkgs))
	for i, pkg := range removePkgs {
		removeItems[i] = packageItem{name: pkg, selected: true}
	}

	m := selectionModel{
		installPackages: installItems,
		removePackages:  removeItems,
		currentPanel:    0,
		cursor:          0,
		choices:         []string{lib.T_("Apply"), lib.T_("Abort")},
		width:           80,
		height:          24,
	}

	p := tea.NewProgram(m,
		tea.WithOutput(os.Stdout),
		tea.WithAltScreen(),
		tea.WithoutSignalHandler())

	finalModel, err := p.Run()
	if err != nil {
		lib.Log.Errorf(lib.T_("Error starting TEA: %v"), err)
		return nil, err
	}

	if m, ok := finalModel.(selectionModel); ok {
		if m.canceled || m.choice == lib.T_("Abort") {
			return &PackageSelectionResult{Canceled: true}, errors.New(lib.T_("Operation cancelled"))
		}

		// Собираем выбранные пакеты
		var selectedInstall, selectedRemove []string
		for _, item := range m.installPackages {
			if item.selected {
				selectedInstall = append(selectedInstall, item.name)
			}
		}
		for _, item := range m.removePackages {
			if item.selected {
				selectedRemove = append(selectedRemove, item.name)
			}
		}

		return &PackageSelectionResult{
			InstallPackages: selectedInstall,
			RemovePackages:  selectedRemove,
			Canceled:        false,
		}, nil
	}

	return &PackageSelectionResult{Canceled: true}, errors.New(lib.T_("Operation cancelled"))
}

func (m selectionModel) Init() tea.Cmd {
	return nil
}

func (m selectionModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlC, tea.KeyEsc:
			m.canceled = true
			return m, tea.Quit

		case tea.KeyEnter:
			if m.isInActionArea() {
				m.choice = m.choices[m.getActionCursor()]
			} else {
				m.choice = lib.T_("Apply")
			}
			return m, tea.Quit

		case tea.KeySpace:
			// Переключаем выбор пакета
			if !m.isInActionArea() {
				m.toggleCurrentPackage()
			}
			return m, nil

		case tea.KeyUp:
			m.moveCursorUp()
			return m, nil

		case tea.KeyDown:
			m.moveCursorDown()
			return m, nil

		case tea.KeyLeft:
			if m.currentPanel == 1 && len(m.installPackages) > 0 {
				m.currentPanel = 0
				m.cursor = 0
			}
			return m, nil

		case tea.KeyRight:
			if m.currentPanel == 0 && len(m.removePackages) > 0 {
				m.currentPanel = 1
				m.cursor = 0
			}
			return m, nil

		case tea.KeyTab:
			m.switchPanel()
			return m, nil

		case tea.KeyRunes:
			switch msg.String() {
			case "q":
				m.canceled = true
				return m, tea.Quit
			case "a":
				// Выбрать все в текущей панели
				m.selectAllInCurrentPanel(true)
				return m, nil
			case "n":
				// Снять выбор со всех в текущей панели
				m.selectAllInCurrentPanel(false)
				return m, nil
			}
		}
	}
	return m, nil
}

func (m selectionModel) View() string {
	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(lib.Env.Colors.Accent))
	installStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(lib.Env.Colors.Install))
	removeStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(lib.Env.Colors.Delete))
	shortcutStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(lib.Env.Colors.Shortcut)).Faint(true)

	var s strings.Builder

	// Заголовок
	s.WriteString(titleStyle.Render(lib.T_("Select packages to apply")) + "\n\n")

	separatorWidth := 1 // Ширина для диагонального разделителя
	panelWidth := (m.width - separatorWidth) / 2
	contentHeight := m.height - 8 // Резервируем место для заголовка, подсказок и кнопок

	// Создаем панели
	installPanel := m.buildPackagePanel(lib.T_("Install"), m.installPackages, 0, panelWidth, contentHeight, installStyle)
	removePanel := m.buildPackagePanel(lib.T_("Remove"), m.removePackages, 1, panelWidth, contentHeight, removeStyle)

	separator := m.buildDiagonalSeparator(contentHeight)

	panelsView := lipgloss.JoinHorizontal(lipgloss.Top, installPanel, separator, removePanel)
	centeredPanels := lipgloss.NewStyle().
		Width(m.width).
		Align(lipgloss.Center).
		Render(panelsView)
	s.WriteString(centeredPanels + "\n\n")

	// Подсказки по клавишам
	shortcuts := shortcutStyle.Render(lib.T_("Navigation: ↑/↓ - move, ←/→/Tab - switch panel, Space - toggle, a - select all, n - none, Enter - apply, Esc/q - cancel"))
	s.WriteString(shortcuts + "\n\n")

	// Кнопки действий
	s.WriteString(m.buildActionButtons())

	return s.String()
}

func (m selectionModel) buildPackagePanel(title string, packages []packageItem, panelIndex, width, height int, titleStyle lipgloss.Style) string {
	var s strings.Builder

	panelTitle := title + fmt.Sprintf(" (%d)", len(packages))

	headerStyle := titleStyle.Bold(true).Padding(0, 1)
	s.WriteString(headerStyle.Render(panelTitle) + "\n")

	if len(packages) == 0 {
		emptyMsg := lipgloss.NewStyle().
			Faint(true).
			Padding(1).
			Render(lib.T_("No packages"))
		s.WriteString(emptyMsg + "\n")
		return lipgloss.NewStyle().
			Width(width).
			Height(height).
			Align(lipgloss.Center).
			Render(s.String())
	}

	var packageNames []any
	for _, pkg := range packages {
		packageNames = append(packageNames, pkg.name)
	}

	packageList := list.New(packageNames...)

	itemStyle := lipgloss.NewStyle().
		PaddingLeft(1).
		PaddingRight(1).
		Bold(true)

	enumeratorStyle := lipgloss.NewStyle()

	if panelIndex == 0 {
		packageList = packageList.Enumerator(m.installPackageEnumerator)
	} else {
		packageList = packageList.Enumerator(m.removePackageEnumerator)
	}

	packageList = packageList.
		EnumeratorStyle(enumeratorStyle).
		ItemStyle(itemStyle)

	listContent := packageList.String()

	s.WriteString(listContent)

	return lipgloss.NewStyle().
		Width(width).
		Height(height).
		Align(lipgloss.Center).
		Render(s.String())
}

func (m selectionModel) buildActionButtons() string {
	var s strings.Builder

	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(lib.Env.Colors.Accent))
	installStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(lib.Env.Colors.Install))
	normalStyle := lipgloss.NewStyle()

	s.WriteString(titleStyle.Render(lib.T_("Action:")) + "\n")

	for i, choice := range m.choices {
		prefix := "  "
		if m.isInActionArea() && m.getActionCursor() == i {
			prefix = "► "
		}

		var style lipgloss.Style
		if i == 0 {
			style = installStyle
		} else {
			style = normalStyle
		}

		s.WriteString(style.Render(prefix+choice) + "\n")
	}

	return s.String()
}

func (m selectionModel) buildDiagonalSeparator(height int) string {
	// Создаем настоящий вертикальный разделитель используя lipgloss Border
	return lipgloss.NewStyle().
		Width(0).
		Height(height).
		BorderStyle(lipgloss.NormalBorder()).
		BorderLeft(true).
		BorderForeground(lipgloss.Color(lib.Env.Colors.Accent)).
		Render("")
}

// Кастомный enumerator для панели установки
func (m selectionModel) installPackageEnumerator(items list.Items, index int) string {
	if index < 0 || index >= len(m.installPackages) {
		return ""
	}

	pkg := m.installPackages[index]
	isCurrentIndex := m.currentPanel == 0 && m.cursor == index && !m.isInActionArea()

	// Используем нативные bullet символы из lipgloss
	var symbol string
	if pkg.selected {
		symbol = "●"
	} else {
		symbol = "○"
	}

	if isCurrentIndex {
		return fmt.Sprintf("▶  %s", symbol)
	} else {
		return fmt.Sprintf("   %s", symbol)
	}
}

// Кастомный enumerator для панели удаления
func (m selectionModel) removePackageEnumerator(items list.Items, index int) string {
	if index < 0 || index >= len(m.removePackages) {
		return ""
	}

	pkg := m.removePackages[index]
	isCurrentIndex := m.currentPanel == 1 && m.cursor == index && !m.isInActionArea()

	var symbol string
	if pkg.selected {
		symbol = "●"
	} else {
		symbol = "○"
	}

	if isCurrentIndex {
		return fmt.Sprintf("▶  %s", symbol)
	} else {
		return fmt.Sprintf("   %s", symbol)
	}
}

// Вспомогательные методы

func (m *selectionModel) moveCursorUp() {
	if m.isInActionArea() {
		actionCursor := m.getActionCursor()
		if actionCursor > 0 {
			m.cursor = m.getTotalPackages() + actionCursor - 1
		} else {
			// Переходим к последнему пакету
			if m.currentPanel == 0 && len(m.installPackages) > 0 {
				m.cursor = len(m.installPackages) - 1
			} else if m.currentPanel == 1 && len(m.removePackages) > 0 {
				m.cursor = len(m.removePackages) - 1
			}
		}
	} else {
		currentList := m.getCurrentPackageList()
		if len(currentList) > 0 {
			if m.cursor > 0 {
				m.cursor--
			} else {
				m.cursor = m.getTotalPackages()
			}
		}
	}
}

func (m *selectionModel) moveCursorDown() {
	if m.isInActionArea() {
		actionCursor := m.getActionCursor()
		if actionCursor < len(m.choices)-1 {
			m.cursor++
		} else {
			m.cursor = 0
		}
	} else {
		currentList := m.getCurrentPackageList()
		if len(currentList) > 0 {
			if m.cursor < len(currentList)-1 {
				m.cursor++
			} else {
				m.cursor = m.getTotalPackages()
			}
		}
	}
}

func (m *selectionModel) switchPanel() {
	if m.currentPanel == 0 && len(m.removePackages) > 0 {
		m.currentPanel = 1
	} else if m.currentPanel == 1 && len(m.installPackages) > 0 {
		m.currentPanel = 0
	}
	m.cursor = 0
}

func (m *selectionModel) toggleCurrentPackage() {
	if m.currentPanel == 0 && m.cursor < len(m.installPackages) {
		m.installPackages[m.cursor].selected = !m.installPackages[m.cursor].selected
	} else if m.currentPanel == 1 && m.cursor < len(m.removePackages) {
		m.removePackages[m.cursor].selected = !m.removePackages[m.cursor].selected
	}
}

func (m *selectionModel) selectAllInCurrentPanel(selected bool) {
	if m.currentPanel == 0 {
		for i := range m.installPackages {
			m.installPackages[i].selected = selected
		}
	} else {
		for i := range m.removePackages {
			m.removePackages[i].selected = selected
		}
	}
}

func (m selectionModel) getCurrentPackageList() []packageItem {
	if m.currentPanel == 0 {
		return m.installPackages
	}
	return m.removePackages
}

func (m selectionModel) getTotalPackages() int {
	return len(m.installPackages) + len(m.removePackages)
}

func (m selectionModel) isInActionArea() bool {
	return m.cursor >= m.getTotalPackages()
}

func (m selectionModel) getActionCursor() int {
	return m.cursor - m.getTotalPackages()
}

func (m selectionModel) getVisibleRange(packages []packageItem, maxVisible int) (int, int) {
	if len(packages) <= maxVisible {
		return 0, len(packages)
	}

	// Центрируем курсор в видимой области
	start := m.cursor - maxVisible/2
	if start < 0 {
		start = 0
	}

	end := start + maxVisible
	if end > len(packages) {
		end = len(packages)
		start = end - maxVisible
		if start < 0 {
			start = 0
		}
	}

	return start, end
}
