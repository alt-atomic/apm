package apt

import (
	"apm/cmd/common/helper"
	"apm/cmd/common/reply"
	"apm/lib"
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
)

var choices = []string{"Установить", "Отмена"}

type model struct {
	pkg        []Package
	pckChange  PackageChanges
	cursor     int
	choice     string
	vp         viewport.Model
	canceled   bool
	choiceType DialogAction
}

// NewDialog запускает диалог отображения информации о пакете с выбором действия.
func NewDialog(packageInfo []Package, packageChange PackageChanges, action DialogAction) (bool, error) {
	if lib.Env.Format != "text" && reply.IsTTY() {
		return true, nil
	}

	switch action {
	case ActionInstall:
		choices = []string{"Установить", "Отмена"}
	case ActionRemove:
		choices = []string{"Удалить", "Отмена"}
	}

	m := model{
		pkg:        packageInfo,
		pckChange:  packageChange,
		vp:         viewport.New(80, 20),
		choiceType: action,
	}
	p := tea.NewProgram(m,
		tea.WithOutput(os.Stdout),
		tea.WithAltScreen(),
		tea.WithoutSignalHandler())
	finalModel, err := p.Run()
	if err != nil {
		lib.Log.Errorf("error start tea: %v", err)
		return false, err
	}

	if m, ok := finalModel.(model); ok {
		if m.canceled || m.choice == "" {
			return false, fmt.Errorf("диалог был отменён")
		}
		return m.choice == "Установить" || m.choice == "Удалить", nil
	}

	return false, fmt.Errorf("диалог был отменён")
}

func (m model) Init() tea.Cmd {
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
		case tea.KeyPgUp:
			m.vp.LineUp(5)
			return m, nil

		case tea.KeyPgDown:
			m.vp.LineDown(5)
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
	}
	return m, nil
}

var (
	deleteStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("#a81c1f"))
	installStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#2bb389"))
)

func (m model) View() string {
	// Определяем стили для вывода
	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#a2734c"))
	valueStyle := lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{
		Light: "#171717",
		Dark:  "#c4c8c6",
	})
	// Стиль для подсказок по клавишам (невзрачный серый)
	shortcutStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#888888")).Faint(true)

	contentView := m.vp.View()

	allLines := strings.Split(m.buildContent(), "\n")
	totalLines := len(allLines)
	if totalLines > m.vp.Height {
		contentView = addScrollIndicator(contentView, m.vp.YOffset, totalLines, m.vp.Height)
	}

	// Формируем строку с подсказками по клавишам
	keyboardShortcuts := shortcutStyle.Render("Навигация: ↑/↓, j/k - выбор, PgUp/PgDn - прокрутка, ctrl+Home/End - начало/конец, Enter - выбрать, Esc/q - отмена")

	// Формируем футер с выбором действия
	var footer strings.Builder
	footer.WriteString(titleStyle.Render("\nВыберите действие:\n"))
	for i, choice := range choices {
		prefix := "  "
		if i == m.cursor {
			prefix = "» "
		}
		// Выбираем стиль в зависимости от типа диалога и выбранной кнопки
		var btnStyle lipgloss.Style
		if i == 0 {
			if m.choiceType == ActionRemove {
				btnStyle = deleteStyle
			} else {
				btnStyle = installStyle
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
func addScrollIndicator(contentView string, yOffset, totalLines, viewportHeight int) string {
	lines := strings.Split(contentView, "\n")
	scrollPercent := float64(yOffset) / float64(totalLines-viewportHeight)
	thumbIndex := int(scrollPercent * float64(viewportHeight))
	if thumbIndex >= viewportHeight {
		thumbIndex = viewportHeight - 1
	}

	indicatorStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#ff0000"))
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
	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#a2734c"))
	keyStyle := lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{
		Light: "#234f55",
		Dark:  "#82a0a3",
	}).PaddingLeft(1)
	valueStyle := lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{
		Light: "#171717",
		Dark:  "#c4c8c6",
	})

	var sb strings.Builder
	const keyWidth = 18

	infoPackage := fmt.Sprintf("\nИнформация о %s:\n", helper.DeclOfNum(len(m.pkg), []string{"пакете", "пакетах", "пакетах"}))
	sb.WriteString(titleStyle.Render(infoPackage))
	for i, pkg := range m.pkg {
		if len(m.pkg) > 1 {
			sb.WriteString(titleStyle.Render("\n"))
			sb.WriteString(titleStyle.Render(fmt.Sprintf("\nПакет %d:", i+1)))
		}
		installedText := deleteStyle.Render("нет")
		if pkg.Installed {
			installedText = installStyle.Render("да")
		}

		sb.WriteString("\n" + formatLine("Название", pkg.Name, keyWidth, keyStyle, valueStyle))
		sb.WriteString("\n" + formatLine("Категория", pkg.Section, keyWidth, keyStyle, valueStyle))
		sb.WriteString("\n" + formatLine("Мейнтейнер", pkg.Maintainer, keyWidth, keyStyle, valueStyle))
		sb.WriteString("\n" + formatLine("Установлен", installedText, keyWidth, keyStyle, valueStyle))

		if pkg.Installed {
			// Выводим "Версия в облаке" обычным стилем
			sb.WriteString("\n" + formatLine("Версия в облаке", pkg.Version, keyWidth, keyStyle, valueStyle))
			// Сравниваем версию в системе и облаке
			var systemVersionColored string
			if pkg.VersionInstalled == pkg.Version {
				systemVersionColored = installStyle.Render(pkg.VersionInstalled)
			} else {
				systemVersionColored = deleteStyle.Render(pkg.VersionInstalled)
			}
			// Выводим "Версия в системе", уже с раскрашенным текстом
			sb.WriteString("\n" + formatLine("Версия в системе", systemVersionColored, keyWidth, keyStyle, valueStyle))
		} else {
			sb.WriteString("\n" + formatLine("Версия в облаке", pkg.Version, keyWidth, keyStyle, valueStyle))
		}
		sb.WriteString("\n" + formatLine("Размер", helper.AutoSize(pkg.InstalledSize), keyWidth, keyStyle, valueStyle))

		dependsStr := formatDependencies(pkg.Depends)
		sb.WriteString("\n" + formatLine("Зависимости", dependsStr, keyWidth, keyStyle, valueStyle))
	}

	sb.WriteString(titleStyle.Render("\n\nИзменения затронут:"))
	extraStr := formatDependencies(m.pckChange.ExtraInstalled)
	upgradeStr := formatDependencies(m.pckChange.UpgradedPackages)
	installStr := formatDependencies(m.pckChange.NewInstalledPackages)
	removeStr := formatDependencies(m.pckChange.RemovedPackages)
	sb.WriteString("\n" + formatLine("Экстра пакеты", extraStr, keyWidth, keyStyle, valueStyle))
	sb.WriteString("\n" + formatLine("Будут обновлены", upgradeStr, keyWidth, keyStyle, valueStyle))
	sb.WriteString("\n" + formatLine("Будут установлены", installStr, keyWidth, keyStyle, valueStyle))
	sb.WriteString("\n" + formatLine("Будут удалены", removeStr, keyWidth, keyStyle, valueStyle))

	packageUpgradedCount := fmt.Sprintf("%d %s", m.pckChange.UpgradedCount, helper.DeclOfNum(m.pckChange.UpgradedCount, []string{"пакет", "пакета", "пакетов"}))
	packageNewInstalledCount := fmt.Sprintf("%d %s", m.pckChange.NewInstalledCount, helper.DeclOfNum(m.pckChange.NewInstalledCount, []string{"пакет", "пакета", "пакетов"}))
	packageRemovedCount := fmt.Sprintf("%d %s", m.pckChange.RemovedCount, helper.DeclOfNum(m.pckChange.RemovedCount, []string{"пакет", "пакета", "пакетов"}))
	packageNotUpgradedCount := fmt.Sprintf("%d %s", m.pckChange.NotUpgradedCount, helper.DeclOfNum(m.pckChange.NotUpgradedCount, []string{"пакет", "пакета", "пакетов"}))
	sb.WriteString(titleStyle.Render("\n\nИтого:"))
	sb.WriteString("\n" + formatLine("Будет обновлено", packageUpgradedCount, keyWidth, keyStyle, valueStyle))
	sb.WriteString("\n" + formatLine("Будет установлено", packageNewInstalledCount, keyWidth, keyStyle, valueStyle))
	sb.WriteString("\n" + formatLine("Будет удалено", packageRemovedCount, keyWidth, keyStyle, valueStyle))
	sb.WriteString("\n" + formatLine("Не затронуты", packageNotUpgradedCount, keyWidth, keyStyle, valueStyle))
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
	if len(filtered) > 500 {
		filtered = append(filtered[:500], "и другие")
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
	return sb.String()
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
