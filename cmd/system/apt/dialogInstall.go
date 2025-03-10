package apt

import (
	"apm/cmd/common/helper"
	"fmt"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var choices = []string{"Установить", "Отмена"}

type model struct {
	pkg       []Package
	pckChange PackageChanges
	cursor    int
	choice    string
}

// NewDialogInstall запускает диалог отображения информации о пакете с выбором действия.
func NewDialogInstall(packageInfo []Package, packageChange PackageChanges) bool {
	m := model{pkg: packageInfo, pckChange: packageChange}
	p := tea.NewProgram(m)
	finalModel, err := p.Run()
	if err != nil {
		fmt.Println("Ошибка:", err)
		os.Exit(1)
	}

	if m, ok := finalModel.(model); ok {
		return m.choice == "Установить"
	}
	return false
}

func (m model) Init() tea.Cmd {
	return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q", "esc":
			return m, tea.Quit
		case "enter":
			m.choice = choices[m.cursor]
			return m, tea.Quit
		case "down", "j":
			m.cursor++
			if m.cursor >= len(choices) {
				m.cursor = 0
			}
		case "up", "k":
			m.cursor--
			if m.cursor < 0 {
				m.cursor = len(choices) - 1
			}
		}
	}
	return m, nil
}

func (m model) View() string {
	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#a2734c"))
	keyStyle := lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{
		Light: "#234f55",
		Dark:  "#82a0a3",
	}).PaddingLeft(1) // Левый отступ для ключей
	valueStyle := lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{
		Light: "#171717",
		Dark:  "#c4c8c6",
	})
	wrapStyle := lipgloss.NewStyle().Width(80)

	installStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#2bb389"))

	var sb strings.Builder
	const keyWidth = 18

	infoPackage := fmt.Sprintf("\nИнформация о %s:\n", helper.DeclOfNum(len(m.pkg), []string{"пакете", "пакетах", "пакетах"}))
	sb.WriteString(titleStyle.Render(infoPackage))
	for i, pkg := range m.pkg {
		if len(m.pkg) > 1 {
			sb.WriteString(titleStyle.Render(fmt.Sprintf("\nПакет %d:", i+1)))
		}
		installedText := "нет"
		if pkg.Installed {
			installedText = "да"
		}

		sb.WriteString("\n" + formatLine("Название", pkg.Name, keyWidth, keyStyle, valueStyle))
		sb.WriteString("\n" + formatLine("Категория", pkg.Section, keyWidth, keyStyle, valueStyle))
		sb.WriteString("\n" + formatLine("Мейнтейнер", pkg.Maintainer, keyWidth, keyStyle, valueStyle))
		sb.WriteString("\n" + formatLine("Версия", pkg.Version, keyWidth, keyStyle, valueStyle))
		wrappedDepends := wrapStyle.Render(formatDependencies(pkg.Depends))
		sb.WriteString("\n" + formatLine("Зависимости", wrappedDepends, keyWidth, keyStyle, valueStyle))
		sb.WriteString("\n" + formatLine("Размер", helper.AutoSize(pkg.InstalledSize), keyWidth, keyStyle, valueStyle))
		sb.WriteString("\n" + formatLine("Установлен", installedText, keyWidth, keyStyle, valueStyle))
	}

	sb.WriteString(titleStyle.Render("\n\nИзменения затронут:"))
	wrappedExtra := wrapStyle.Render(formatDependencies(m.pckChange.ExtraInstalled))
	wrappedUpgrade := wrapStyle.Render(formatDependencies(m.pckChange.UpgradedPackages))
	wrappedInstall := wrapStyle.Render(formatDependencies(m.pckChange.NewInstalledPackages))
	sb.WriteString("\n" + formatLine("Экстра пакеты", wrappedExtra, keyWidth, keyStyle, valueStyle))
	sb.WriteString("\n" + formatLine("Будут обновлены", wrappedUpgrade, keyWidth, keyStyle, valueStyle))
	sb.WriteString("\n" + formatLine("Будут установлены", wrappedInstall, keyWidth, keyStyle, valueStyle))

	packageUpgradedCount := fmt.Sprintf("%d %s", m.pckChange.UpgradedCount, helper.DeclOfNum(m.pckChange.UpgradedCount, []string{"пакет", "пакета", "пакетов"}))
	packageNewInstalledCount := fmt.Sprintf("%d %s", m.pckChange.NewInstalledCount, helper.DeclOfNum(m.pckChange.NewInstalledCount, []string{"пакет", "пакета", "пакетов"}))
	packageRemovedCount := fmt.Sprintf("%d %s", m.pckChange.RemovedCount, helper.DeclOfNum(m.pckChange.RemovedCount, []string{"пакет", "пакета", "пакетов"}))
	packageNotUpgradedCount := fmt.Sprintf("%d %s", m.pckChange.NotUpgradedCount, helper.DeclOfNum(m.pckChange.NotUpgradedCount, []string{"пакет", "пакета", "пакетов"}))

	sb.WriteString(titleStyle.Render("\n\nИтого:"))
	sb.WriteString("\n" + formatLine("Будет обновлено",
		packageUpgradedCount, keyWidth, keyStyle, valueStyle))
	sb.WriteString("\n" + formatLine("Установлено новых",
		packageNewInstalledCount, keyWidth, keyStyle, valueStyle))
	sb.WriteString("\n" + formatLine("Будет удалено",
		packageRemovedCount, keyWidth, keyStyle, valueStyle))
	sb.WriteString("\n" + formatLine("Не затронуты",
		packageNotUpgradedCount, keyWidth, keyStyle, valueStyle))

	sb.WriteString(titleStyle.Render("\n\nВыберите действие:\n"))
	for i, choice := range choices {
		prefix := "  "
		if i == m.cursor {
			prefix = "» "
		}
		if i == 1 {
			sb.WriteString("\n" + valueStyle.Render(prefix+choice))
		} else {
			sb.WriteString("\n" + installStyle.Render(prefix+choice))
		}
	}

	return sb.String()
}

func formatDependencies(depends []string) string {
	var filtered []string
	for _, dep := range depends {
		if strings.Contains(dep, "/") || strings.Contains(dep, ".so") {
			continue
		}
		filtered = append(filtered, dep)
	}
	if len(filtered) == 0 {
		return "нет"
	}
	if len(filtered) > 50 {
		return strings.Join(filtered[:20], ", ") + " и т.д."
	}
	return strings.Join(filtered, ", ")
}

func formatLine(key, value string, keyWidth int, keyStyle, valueStyle lipgloss.Style) string {
	keyLen := lipgloss.Width(key)
	padding := ""
	if keyLen < keyWidth {
		padding = strings.Repeat(" ", keyWidth-keyLen)
	}
	return keyStyle.Render(key+padding) + valueStyle.Render(fmt.Sprintf(": %s", value))
}
