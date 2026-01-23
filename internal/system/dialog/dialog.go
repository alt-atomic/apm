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
	_package "apm/internal/common/apt/package"
	aptLib "apm/internal/common/binding/apt/lib"
	"apm/internal/common/helper"
	"apm/internal/common/reply"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type DialogAction int

const (
	ActionInstall DialogAction = iota
	ActionRemove
	ActionMultiInstall
	ActionUpgrade
)

var choices []string

type model struct {
	pkg        []_package.Package
	pckChange  aptLib.PackageChanges
	cursor     int
	choice     string
	vp         viewport.Model
	canceled   bool
	choiceType DialogAction
	appConfig  *app.Config
}

// NewDialog запускает диалог отображения информации о пакете с выбором действия.
func NewDialog(appConfig *app.Config, packageInfo []_package.Package, packageChange aptLib.PackageChanges, action DialogAction) (bool, error) {
	if appConfig.ConfigManager.GetConfig().Format != app.FormatText || !reply.IsTTY() {
		return true, nil
	}

	switch action {
	case ActionMultiInstall:
		choices = []string{app.T_("Edit"), app.T_("Abort")}
	case ActionInstall:
		choices = []string{app.T_("Install"), app.T_("Abort")}
	case ActionRemove:
		choices = []string{app.T_("Remove"), app.T_("Abort")}
	case ActionUpgrade:
		choices = []string{app.T_("Upgrade"), app.T_("Abort")}
	}

	m := model{
		pkg:        packageInfo,
		pckChange:  packageChange,
		vp:         viewport.New(80, 20),
		choiceType: action,
		appConfig:  appConfig,
	}
	p := tea.NewProgram(m,
		tea.WithOutput(os.Stdout),
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(),
		tea.WithoutSignalHandler())
	finalModel, err := p.Run()
	if err != nil {
		app.Log.Errorf(app.T_("Error starting TEA: %v"), err)
		return false, err
	}

	if m, ok := finalModel.(model); ok {
		if m.canceled || m.choice == "" {
			return false, errors.New(app.T_("Operation cancelled"))
		}

		return m.choice == app.T_("Install") || m.choice == app.T_("Remove") || m.choice == app.T_("Edit") || m.choice == app.T_("Upgrade"), nil
	}

	return false, errors.New(app.T_("Operation cancelled"))
}

func (m model) Init() tea.Cmd {
	m.vp.SetContent(m.buildContent())
	return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		// Обновляем размеры viewport, вычитая 5 строк для футера (меню)
		m.vp.Width = msg.Width
		m.vp.Height = msg.Height - 5
		m.vp.SetContent(m.buildContent())
		return m, nil

	case tea.KeyMsg:
		switch msg.Type {
		// Отмена диалога: Esc или Ctrl+C
		case tea.KeyCtrlC, tea.KeyEsc:
			m.canceled = true
			return m, tea.Quit

		// Завершение выбора
		case tea.KeyEnter:
			m.choice = choices[m.cursor]
			return m, tea.Quit

		// Навигация по меню с помощью стрелок
		case tea.KeyUp:
			m.cursor--
			if m.cursor < 0 {
				m.cursor = len(choices) - 1
			}
			return m, nil

		case tea.KeyDown:
			m.cursor++
			if m.cursor >= len(choices) {
				m.cursor = 0
			}
			return m, nil

		// Прокрутка viewport
		case tea.KeyPgUp, tea.KeyCtrlUp:
			m.vp.ScrollUp(5)
			return m, nil

		case tea.KeyPgDown, tea.KeyCtrlDown:
			m.vp.ScrollDown(5)
			return m, nil

		// Перемещение в самый верх
		case tea.KeyHome, tea.KeyCtrlHome:
			m.vp.GotoTop()
			return m, nil

		// Перемещение в самый низ
		case tea.KeyEnd, tea.KeyCtrlEnd:
			m.vp.GotoBottom()
			return m, nil

		// Обработка рун для "j" и "k"
		case tea.KeyRunes:
			switch msg.String() {
			case "j":
				m.cursor++
				if m.cursor >= len(choices) {
					m.cursor = 0
				}
				return m, nil
			case "k":
				m.cursor--
				if m.cursor < 0 {
					m.cursor = len(choices) - 1
				}
				return m, nil
			case "q":
				m.canceled = true
				return m, tea.Quit
			}

		default:
			var cmd tea.Cmd
			m.vp, cmd = m.vp.Update(msg)
			return m, cmd
		}

	case tea.MouseMsg:
		// Передаем события мыши в viewport для скролла
		var cmd tea.Cmd
		m.vp, cmd = m.vp.Update(msg)
		return m, cmd
	}
	return m, nil
}

// getDeleteStyle возвращает стиль для удаления.
func (m model) getDeleteStyle() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(lipgloss.Color(m.appConfig.ConfigManager.GetConfig().Colors.Delete))
}

// getInstallStyle возвращает стиль для установки.
func (m model) getInstallStyle() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(lipgloss.Color(m.appConfig.ConfigManager.GetConfig().Colors.Install))
}

// getShortcutStyle возвращает стиль для подсказок.
func (m model) getShortcutStyle() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(lipgloss.Color(m.appConfig.ConfigManager.GetConfig().Colors.Shortcut)).Faint(true)
}

func (m model) View() string {
	// Определяем стили для вывода
	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(m.appConfig.ConfigManager.GetConfig().Colors.Accent))
	valueStyle := lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{
		Light: m.appConfig.ConfigManager.GetConfig().Colors.ItemLight,
		Dark:  m.appConfig.ConfigManager.GetConfig().Colors.ItemDark,
	})

	contentView := m.vp.View()

	allLines := strings.Split(m.buildContent(), "\n")
	totalLines := len(allLines)
	if totalLines > m.vp.Height {
		contentView = m.addScrollIndicator(contentView, m.vp.YOffset, totalLines, m.vp.Height)
	}

	// Формируем строку с подсказками по клавишам
	keyboardShortcuts := m.getShortcutStyle().Render(app.T_("Navigation: ↑/↓ or j/k - select, Ctrl+↑/↓ or PgUp/PgDn - scroll, Ctrl+Home/End - top/bottom, Enter - choose, Esc/q - cancel"))

	// Формируем футер с выбором действия
	var footer strings.Builder
	footer.WriteString(titleStyle.Render(fmt.Sprintf("\n%s\n", app.T_("Select an action:"))))
	for i, choice := range choices {
		prefix := "  "
		if i == m.cursor {
			prefix = "» "
		}
		// Выбираем стиль в зависимости от типа диалога и выбранной кнопки
		var btnStyle lipgloss.Style
		if i == 0 {
			if m.choiceType == ActionRemove {
				btnStyle = m.getDeleteStyle()
			} else {
				btnStyle = m.getInstallStyle()
			}
		} else {
			btnStyle = valueStyle
		}
		footer.WriteString("\n" + btnStyle.Render(prefix+choice))
	}

	// Выводим сначала контент, затем подсказки и, наконец, меню выбора
	return contentView + "\n" + keyboardShortcuts + "\n" + footer.String()
}

// addScrollIndicator добавляет вертикальный индикатор прокрутки справа от контента.
func (m model) addScrollIndicator(contentView string, yOffset, totalLines, viewportHeight int) string {
	lines := strings.Split(contentView, "\n")
	scrollPercent := float64(yOffset) / float64(totalLines-viewportHeight)
	thumbIndex := int(scrollPercent * float64(viewportHeight))
	if thumbIndex >= viewportHeight {
		thumbIndex = viewportHeight - 1
	}

	indicatorStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(m.appConfig.ConfigManager.GetConfig().Colors.ScrollBar))
	for i := range lines {
		indicator := " "
		if i == thumbIndex {
			indicator = indicatorStyle.Render("█")
		}
		lines[i] = lines[i] + indicator
	}
	return strings.Join(lines, "\n")
}

// buildContent генерирует основное содержимое, которое помещается в viewport.
// Здесь выводится информация о пакетах и изменения, без интерактивного меню.
func (m model) buildContent() string {
	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(m.appConfig.ConfigManager.GetConfig().Colors.Accent))
	keyStyle := lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{
		Light: m.appConfig.ConfigManager.GetConfig().Colors.DialogKeyLight,
		Dark:  m.appConfig.ConfigManager.GetConfig().Colors.DialogKeyDark,
	}).PaddingLeft(1)
	valueStyle := lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{
		Light: m.appConfig.ConfigManager.GetConfig().Colors.ItemLight,
		Dark:  m.appConfig.ConfigManager.GetConfig().Colors.ItemDark,
	})

	var sb strings.Builder
	const keyWidth = 21

	// Сначала затронутые изменения
	sb.WriteString(titleStyle.Render(fmt.Sprintf("\n%s\n", app.T_("Affected changes:"))))
	extraStr := m.formatDependencies(m.pckChange.ExtraInstalled)
	upgradeStr := m.formatDependencies(m.pckChange.UpgradedPackages)
	installStr := m.formatDependencies(m.pckChange.NewInstalledPackages)
	removeStr := m.formatDependencies(m.pckChange.RemovedPackages)
	sb.WriteString("\n" + formatLine(app.T_("Extra packages"), extraStr, keyWidth, keyStyle, valueStyle))
	sb.WriteString("\n" + formatLine(app.T_("Will be updated"), upgradeStr, keyWidth, keyStyle, valueStyle))
	sb.WriteString("\n" + formatLine(app.T_("Will be installed"), installStr, keyWidth, keyStyle, valueStyle))
	sb.WriteString("\n" + formatLine(app.T_("Will be removed"), removeStr, keyWidth, keyStyle, valueStyle))

	// Затем итоги
	packageUpgradedCount := fmt.Sprintf(app.TN_("%d package", "%d packages", m.pckChange.UpgradedCount), m.pckChange.UpgradedCount)
	packageNewInstalledCount := fmt.Sprintf(app.TN_("%d package", "%d packages", m.pckChange.NewInstalledCount), m.pckChange.NewInstalledCount)
	packageRemovedCount := fmt.Sprintf(app.TN_("%d package", "%d packages", m.pckChange.RemovedCount), m.pckChange.RemovedCount)
	packageNotUpgradedCount := fmt.Sprintf(app.TN_("%d package", "%d packages", m.pckChange.NotUpgradedCount), m.pckChange.NotUpgradedCount)

	sb.WriteString(titleStyle.Render(fmt.Sprintf("\n\n%s\n", app.T_("Total:"))))
	sb.WriteString("\n" + formatLine(app.T_("Will be updated"), packageUpgradedCount, keyWidth, keyStyle, valueStyle))
	sb.WriteString("\n" + formatLine(app.T_("Will be installed"), packageNewInstalledCount, keyWidth, keyStyle, valueStyle))
	sb.WriteString("\n" + formatLine(app.T_("Will be removed"), packageRemovedCount, keyWidth, keyStyle, valueStyle))
	sb.WriteString("\n" + formatLine(app.T_("Not affected"), packageNotUpgradedCount, keyWidth, keyStyle, valueStyle))
	if m.choiceType == ActionUpgrade || m.choiceType == ActionInstall {
		sb.WriteString("\n" + formatLine(app.T_("Downloaded Size"), helper.AutoSize(int(m.pckChange.DownloadSize)), keyWidth, keyStyle, valueStyle))
		sb.WriteString("\n" + formatLine(app.T_("Installed Size"), helper.AutoSize(int(m.pckChange.InstallSize)), keyWidth, keyStyle, valueStyle))
	}

	// В конце - информация о пакетах
	if m.choiceType != ActionUpgrade {
		infoPackage := fmt.Sprintf("\n\n%s\n", app.TN_("Package information:", "Packages information:", len(m.pkg)))
		sb.WriteString(titleStyle.Render(infoPackage))
	}

	// Для больших списков показываем только названия пакетов
	if len(m.pkg) > 200 {
		for i, pkg := range m.pkg {
			if i == 0 && len(m.pkg) > 1 {
				sb.WriteString(titleStyle.Render(fmt.Sprintf("\n%s\n", app.T_("Package list:"))))
			}

			statusText := m.statusPackage(pkg)
			installedText := ""
			if pkg.Installed {
				installedText = " " + m.getInstallStyle().Render(app.T_("[Installed]"))
			}

			line := fmt.Sprintf("• %s%s - %s", pkg.Name, installedText, statusText)
			sb.WriteString("\n" + valueStyle.Render(line))
		}
	} else {
		// Обычный детальный вывод для списков ≤200 пакетов
		for i, pkg := range m.pkg {
			if len(m.pkg) > 1 {
				sb.WriteString(titleStyle.Render("\n"))
				sb.WriteString(titleStyle.Render(fmt.Sprintf(app.T_("\nPackage %d:"), i+1)))
			}
			installedText := m.getShortcutStyle().Render(app.T_("No"))
			if pkg.Installed {
				installedText = m.getInstallStyle().Render(app.T_("Yes"))
			}

			sb.WriteString("\n" + formatLine(app.T_("Name"), pkg.Name, keyWidth, keyStyle, valueStyle))
			sb.WriteString("\n" + formatLine(app.T_("Action"), m.statusPackage(pkg), keyWidth, keyStyle, valueStyle))
			sb.WriteString("\n" + formatLine(app.T_("Category"), pkg.Section, keyWidth, keyStyle, valueStyle))
			sb.WriteString("\n" + formatLine(app.T_("Maintainer"), pkg.Maintainer, keyWidth, keyStyle, valueStyle))
			sb.WriteString("\n" + formatLine(app.T_("Installed"), installedText, keyWidth, keyStyle, valueStyle))

			if pkg.Installed {
				// Выводим "Версия в облаке" обычным стилем
				sb.WriteString("\n" + formatLine(app.T_("Repository version"), pkg.Version, keyWidth, keyStyle, valueStyle))
				// Сравниваем версию в системе и облаке
				var systemVersionColored string
				if pkg.VersionInstalled == pkg.Version {
					systemVersionColored = m.getInstallStyle().Render(pkg.VersionInstalled)
				} else {
					systemVersionColored = m.getDeleteStyle().Render(pkg.VersionInstalled)
				}
				// Выводим "Версия в системе", уже с раскрашенным текстом
				sb.WriteString("\n" + formatLine(app.T_("System version"), systemVersionColored, keyWidth, keyStyle, valueStyle))
			} else {
				sb.WriteString("\n" + formatLine(app.T_("Repository version"), pkg.Version, keyWidth, keyStyle, valueStyle))
			}
			sb.WriteString("\n" + formatLine(app.T_("Size"), helper.AutoSize(pkg.InstalledSize), keyWidth, keyStyle, valueStyle))

			dependsStr := m.formatDependencies(pkg.Depends)
			sb.WriteString("\n" + formatLine(app.T_("Dependencies"), dependsStr, keyWidth, keyStyle, valueStyle))
		}
	}

	return sb.String()
}

func (m model) statusPackage(pkg _package.Package) string {
	// Создаём список возможных имён пакета для поиска в изменениях
	possibleNames := []string{pkg.Name}

	// Если архитектура i586, добавляем дополнительные варианты имён
	if pkg.Architecture == "i586" {
		possibleNames = append(possibleNames,
			"i586-"+pkg.Name,
			"i586-"+pkg.Name+".32bit",
		)
	}

	// Добавляем aliases если они есть
	possibleNames = append(possibleNames, pkg.Aliases...)

	// Проверяем все возможные имена во всех списках изменений
	for _, name := range possibleNames {
		if contains(m.pckChange.ExtraInstalled, name) || contains(m.pckChange.NewInstalledPackages, name) {
			return m.getInstallStyle().Render(app.T_("Will be installed"))
		}

		if contains(m.pckChange.UpgradedPackages, name) {
			return m.getInstallStyle().Render(app.T_("Will be updated"))
		}

		if contains(m.pckChange.RemovedPackages, name) {
			return m.getDeleteStyle().Render(app.T_("Will be removed"))
		}
	}

	return m.getShortcutStyle().Render(app.T_("No"))
}

func contains(slice []string, pkg string) bool {
	for _, v := range slice {
		if v == pkg {
			return true
		}
	}
	return false
}

func (m model) formatDependencies(depends []string) string {
	var filtered []string
	for _, dep := range depends {
		if strings.Contains(dep, "/") || strings.Contains(dep, ".so") {
			continue
		}
		filtered = append(filtered, dep)
	}
	if len(filtered) == 0 {
		return app.T_("No")
	}
	if len(filtered) > 500 {
		filtered = append(filtered[:500], app.T_("and others"))
	}
	maxLen := 0
	for _, dep := range filtered {
		if len(dep) > maxLen {
			maxLen = len(dep)
		}
	}
	colWidth := maxLen + 2
	var sb strings.Builder
	for i, dep := range filtered {
		sb.WriteString(fmt.Sprintf("%-*s", colWidth, dep))
		if (i+1)%3 == 0 || i == len(filtered)-1 {
			sb.WriteString("\n")
		}
	}

	// Убираем последний перевод строки
	resultStr := sb.String()
	resultStr = strings.TrimSuffix(resultStr, "\n")

	return resultStr
}

func formatLine(key, value string, keyWidth int, keyStyle, valueStyle lipgloss.Style) string {
	keyLen := lipgloss.Width(key)
	padding := ""
	if keyLen < keyWidth {
		padding = strings.Repeat(" ", keyWidth-keyLen)
	}
	formattedKey := keyStyle.Render(key + padding)
	lines := strings.Split(value, "\n")
	result := formattedKey + valueStyle.Render(": "+lines[0])
	indent := strings.Repeat(" ", keyWidth+3)
	for _, line := range lines[1:] {
		result += "\n" + indent + valueStyle.Render(line)
	}
	return result
}
