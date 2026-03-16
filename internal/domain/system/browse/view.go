package browse

import (
	"apm/internal/common/app"
	"apm/internal/common/helper"
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

var tabNames = []string{"All", "Installed", "Not installed"}

func (m *model) View() string {
	if m.width == 0 || m.height == 0 {
		return app.T_("Loading...")
	}

	var b strings.Builder
	b.WriteString(m.renderTabs())
	b.WriteByte('\n')
	b.WriteString(m.renderFilterBar())
	b.WriteByte('\n')
	b.WriteString(m.renderListHeader())
	b.WriteByte('\n')
	b.WriteString(m.renderList())
	b.WriteByte('\n')
	b.WriteString(m.renderDetail())
	b.WriteByte('\n')
	b.WriteString(m.renderStatusBar())
	return b.String()
}

func (m *model) accentStyle() lipgloss.Style {
	return lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(m.colors().Accent))
}

func (m *model) dimStyle() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(lipgloss.Color(m.colors().DialogHint))
}

func (m *model) colors() app.Colors {
	return m.appConfig.ConfigManager.GetConfig().Colors
}

func (m *model) renderTabs() string {
	accent := m.accentStyle()
	dim := m.dimStyle()

	var tabs []string
	for i, name := range tabNames {
		label := " " + app.T_(name) + " "
		if i == m.activeTab {
			tabs = append(tabs, accent.Render("["+label+"]"))
		} else {
			tabs = append(tabs, dim.Render(" "+label+" "))
		}
	}

	left := strings.Join(tabs, "")
	right := dim.Render(fmt.Sprintf(" %d ", m.totalCount))

	gap := m.width - lipgloss.Width(left) - lipgloss.Width(right)
	if gap < 0 {
		gap = 0
	}
	return left + strings.Repeat(" ", gap) + right
}

func (m *model) renderFilterBar() string {
	accent := m.accentStyle()
	dim := m.dimStyle()

	field := m.currentField()
	op := string(m.currentOp())

	style := func(focused bool) lipgloss.Style {
		if m.filterEditing && focused {
			return accent
		}
		return dim
	}

	fieldStr := style(m.filterFocus == focusField).Render("[" + field + " ▾]")
	opStr := style(m.filterFocus == focusOperator).Render("[" + op + " ▾]")

	var valueStr string
	if m.filterEditing && m.filterFocus == focusValue {
		valueStr = accent.Render(m.filter.value + "█")
	} else if m.filter.value != "" {
		valueStr = m.filter.value
	} else {
		valueStr = dim.Render(app.T_("press f to filter"))
	}

	label := app.T_("Search") + ": "
	if m.filterEditing {
		return accent.Render(label) + fieldStr + " " + opStr + " " + valueStr
	}
	return dim.Render(label) + fieldStr + " " + opStr + " " + valueStr
}

const listPrefix = 5

func (m *model) columnWidths() (nameW, verW, secW int) {
	available := m.width - listPrefix - 2
	if available < 30 {
		available = 30
	}
	nameW = available * 45 / 100
	verW = available * 30 / 100
	secW = available - nameW - verW
	if nameW < 10 {
		nameW = 10
	}
	if verW < 8 {
		verW = 8
	}
	if secW < 6 {
		secW = 6
	}
	return
}

func (m *model) renderListHeader() string {
	accent := m.accentStyle()
	nameW, verW, secW := m.columnWidths()

	return strings.Repeat(" ", listPrefix) +
		accent.Render(padOrTruncate(app.T_("Name"), nameW)) + " " +
		accent.Render(padOrTruncate(app.T_("Version"), verW)) + " " +
		accent.Render(padOrTruncate(app.T_("Section"), secW))
}

func (m *model) renderList() string {
	c := m.colors()
	dim := m.dimStyle()
	installed := lipgloss.NewStyle().Foreground(lipgloss.Color(c.DialogAction))
	cursor := m.accentStyle()

	nameW, verW, secW := m.columnWidths()
	lh := m.listHeight()

	if len(m.packages) == 0 {
		lines := make([]string, lh)
		lines[0] = dim.Render("  " + app.T_("No packages found"))
		return strings.Join(lines, "\n")
	}

	lines := make([]string, lh)
	for i := range lh {
		idx := m.offset + i
		if idx >= len(m.packages) {
			continue
		}

		pkg := m.packages[idx]
		sel := idx == m.cursor

		pre := "  "
		if sel {
			pre = cursor.Render("▌") + " "
		}

		badge := "○  "
		if pkg.Installed {
			badge = installed.Render("●") + "  "
		}

		row := padOrTruncate(pkg.Name, nameW) + " " +
			padOrTruncate(pkg.Version, verW) + " " +
			padOrTruncate(pkg.Section, secW)
		if sel {
			row = lipgloss.NewStyle().Bold(true).Render(row)
		}

		lines[i] = pre + badge + row
	}

	return strings.Join(lines, "\n")
}

func (m *model) renderDetail() string {
	c := m.colors()
	keyStyle := lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{
		Light: c.DialogLabelLight,
		Dark:  c.DialogLabelDark,
	})
	valStyle := lipgloss.NewStyle()
	accent := m.accentStyle()
	dim := m.dimStyle()
	instStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(c.DialogAction))

	dh := m.detailHeight()
	sep := dim.Render(strings.Repeat("─", m.width))

	if len(m.packages) == 0 || m.cursor >= len(m.packages) {
		lines := make([]string, dh)
		lines[0] = sep
		return strings.Join(lines, "\n")
	}

	pkg := m.packages[m.cursor]
	const kw = 17

	lines := []string{sep}

	lines = append(lines, fmtLine(app.T_("Name"), pkg.Name, kw, keyStyle, accent))

	if pkg.Installed {
		lines = append(lines, fmtLine(app.T_("Status"), app.T_("Installed"), kw, keyStyle, instStyle))
	} else {
		lines = append(lines, fmtLine(app.T_("Status"), app.T_("Not installed"), kw, keyStyle, dim))
	}

	lines = append(lines, fmtLine(app.T_("Version"), pkg.Version, kw, keyStyle, valStyle))
	if pkg.VersionInstalled != "" && pkg.VersionInstalled != pkg.Version {
		lines = append(lines, fmtLine(app.T_("Inst. version"), pkg.VersionInstalled, kw, keyStyle, valStyle))
	}

	lines = append(lines, fmtLine(app.T_("Section"), pkg.Section, kw, keyStyle, valStyle))

	if pkg.InstalledSize > 0 {
		lines = append(lines, fmtLine(app.T_("Size"), helper.AutoSize(pkg.InstalledSize), kw, keyStyle, valStyle))
	}
	if pkg.Architecture != "" {
		lines = append(lines, fmtLine(app.T_("Architecture"), pkg.Architecture, kw, keyStyle, valStyle))
	}
	if len(pkg.Depends) > 0 {
		lines = append(lines, fmtLine(app.T_("Dependencies"), truncate(strings.Join(pkg.Depends, ", "), m.width-kw-5), kw, keyStyle, valStyle))
	}
	if pkg.Summary != "" {
		lines = append(lines, fmtLine(app.T_("Summary"), truncate(pkg.Summary, m.width-kw-5), kw, keyStyle, valStyle))
	}

	// Подгоняем под фиксированную высоту
	for len(lines) < dh {
		lines = append(lines, "")
	}
	return strings.Join(lines[:dh], "\n")
}

func (m *model) renderStatusBar() string {
	hint := m.dimStyle().Faint(true)
	if m.filterEditing {
		return hint.Render(app.T_("Navigation: Tab/←/→ - switch field, ↑/↓ - select value, Enter - apply filter, Esc - back"))
	}
	return hint.Render(app.T_("Navigation: ↑/↓ or j/k - select, PgUp/PgDn - scroll, Ctrl+Home/End - top/bottom, Tab - filter tab, f - filter, Esc/q - quit"))
}

func padOrTruncate(s string, width int) string {
	w := lipgloss.Width(s)
	if w > width {
		if width > 3 {
			result := ""
			cur := 0
			for _, r := range s {
				rw := lipgloss.Width(string(r))
				if cur+rw > width-3 {
					break
				}
				result += string(r)
				cur += rw
			}
			return result + "..."
		}
		return s[:width]
	}
	return s + strings.Repeat(" ", width-w)
}

func truncate(s string, maxW int) string {
	if maxW <= 0 {
		return s
	}
	w := lipgloss.Width(s)
	if w <= maxW {
		return s
	}
	result := ""
	cur := 0
	for _, r := range s {
		rw := lipgloss.Width(string(r))
		if cur+rw > maxW-3 {
			break
		}
		result += string(r)
		cur += rw
	}
	return result + "..."
}

func fmtLine(key, value string, keyWidth int, keyStyle, valueStyle lipgloss.Style) string {
	kw := lipgloss.Width(key)
	pad := ""
	if kw < keyWidth {
		pad = strings.Repeat(" ", keyWidth-kw)
	}
	return keyStyle.Render("  "+key+pad) + valueStyle.Render(": "+value)
}
