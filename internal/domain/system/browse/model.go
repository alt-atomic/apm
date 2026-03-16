package browse

import (
	"apm/internal/common/app"
	_package "apm/internal/common/apt/package"
	"apm/internal/common/filter"
	"context"
	"sort"
	"unicode/utf8"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
)

type filterFocus int

const (
	focusField filterFocus = iota
	focusOperator
	focusValue
)

const (
	tabInstalled    = 1
	tabNotInstalled = 2
	tabCount        = 3
)

type activeFilter struct {
	fieldIdx int
	opIdx    int
	value    string
}

type model struct {
	appConfig *app.Config
	dbService aptDatabaseService
	ctx       context.Context

	packages   []_package.Package
	totalCount int64

	cursor int
	offset int

	activeTab int

	filterEditing bool
	filterFocus   filterFocus
	filter        activeFilter

	filterFields []string
	filterOps    map[string][]filter.Op
	filterConfig *filter.Config

	width, height int
}

type packagesLoadedMsg struct {
	packages []_package.Package
	total    int64
}

func newModel(ctx context.Context, appConfig *app.Config, dbService aptDatabaseService, initialPkgs []_package.Package, total int64) model {
	cfg := _package.SystemFilterConfig

	fields := make([]string, 0, len(cfg.Fields))
	opsMap := make(map[string][]filter.Op, len(cfg.Fields))
	for name, fc := range cfg.Fields {
		fields = append(fields, name)
		if fc.AllowedOps != nil {
			opsMap[name] = fc.AllowedOps
		} else {
			opsMap[name] = filter.AllOps
		}
	}
	sort.Strings(fields)

	for i, f := range fields {
		if f == "name" && i != 0 {
			fields[0], fields[i] = fields[i], fields[0]
			break
		}
	}

	return model{
		appConfig:    appConfig,
		dbService:    dbService,
		ctx:          ctx,
		packages:     initialPkgs,
		totalCount:   total,
		filterFields: fields,
		filterOps:    opsMap,
		filterConfig: cfg,
	}
}

func (m *model) Init() tea.Cmd {
	return nil
}

func (m *model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil
	case packagesLoadedMsg:
		m.packages = msg.packages
		m.totalCount = msg.total
		m.cursor = 0
		m.offset = 0
		return m, nil
	case tea.MouseMsg:
		if msg.Action == tea.MouseActionPress {
			switch msg.Button {
			case tea.MouseButtonWheelUp:
				m.moveCursor(-3)
			case tea.MouseButtonWheelDown:
				m.moveCursor(3)
			default:
			}
		}
		return m, nil
	case tea.KeyMsg:
		if m.filterEditing {
			return m.handleFilterKey(msg)
		}
		return m.handleNormalKey(msg)
	default:
		return m, nil
	}
}

func (m *model) handleNormalKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, keys.Quit), key.Matches(msg, keys.Escape):
		return m, tea.Quit
	case key.Matches(msg, keys.Up):
		m.moveCursor(-1)
	case key.Matches(msg, keys.Down):
		m.moveCursor(1)
	case key.Matches(msg, keys.PageUp):
		m.moveCursor(-m.listHeight())
	case key.Matches(msg, keys.PageDown):
		m.moveCursor(m.listHeight())
	case key.Matches(msg, keys.Home):
		m.cursor = 0
		m.offset = 0
	case key.Matches(msg, keys.End):
		if len(m.packages) > 0 {
			m.cursor = len(m.packages) - 1
			m.adjustOffset()
		}
	case key.Matches(msg, keys.Tab):
		m.activeTab = (m.activeTab + 1) % tabCount
		return m, m.loadPackages()
	case key.Matches(msg, keys.ShiftTab):
		m.activeTab = (m.activeTab + tabCount - 1) % tabCount
		return m, m.loadPackages()
	case key.Matches(msg, keys.Filter):
		m.filterEditing = true
		m.filterFocus = focusValue
	default:
	}
	return m, nil
}

func (m *model) handleFilterKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, keys.Escape):
		m.filterEditing = false
		if m.filter.value == "" {
			return m, m.loadPackages()
		}
		return m, nil
	case key.Matches(msg, keys.Enter):
		m.filterEditing = false
		return m, m.loadPackages()
	case key.Matches(msg, keys.Tab), key.Matches(msg, fkeys.Right):
		m.filterFocus = (m.filterFocus + 1) % 3
	case key.Matches(msg, keys.ShiftTab), key.Matches(msg, fkeys.Left):
		m.filterFocus = (m.filterFocus + 2) % 3
	case key.Matches(msg, keys.Up):
		m.filterCyclePrev()
	case key.Matches(msg, keys.Down):
		m.filterCycleNext()
	default:
		if m.filterFocus == focusValue {
			changed := false
			s := msg.String()
			if key.Matches(msg, key.NewBinding(key.WithKeys("backspace"))) {
				if len(m.filter.value) > 0 {
					_, size := utf8.DecodeLastRuneInString(m.filter.value)
					m.filter.value = m.filter.value[:len(m.filter.value)-size]
					changed = true
				}
			} else if len(s) == 1 || utf8.RuneCountInString(s) == 1 {
				m.filter.value += s
				changed = true
			}
			if changed {
				return m, m.loadPackages()
			}
		}
	}
	return m, nil
}

func (m *model) filterCyclePrev() {
	switch m.filterFocus {
	case focusField:
		if m.filter.fieldIdx > 0 {
			m.filter.fieldIdx--
		} else {
			m.filter.fieldIdx = len(m.filterFields) - 1
		}
		m.filter.opIdx = 0
	case focusOperator:
		ops := m.currentOps()
		if m.filter.opIdx > 0 {
			m.filter.opIdx--
		} else {
			m.filter.opIdx = len(ops) - 1
		}
	default:
	}
}

func (m *model) filterCycleNext() {
	switch m.filterFocus {
	case focusField:
		m.filter.fieldIdx = (m.filter.fieldIdx + 1) % len(m.filterFields)
		m.filter.opIdx = 0
	case focusOperator:
		ops := m.currentOps()
		m.filter.opIdx = (m.filter.opIdx + 1) % len(ops)
	default:
	}
}

func (m *model) currentField() string {
	if m.filter.fieldIdx < len(m.filterFields) {
		return m.filterFields[m.filter.fieldIdx]
	}
	return "name"
}

func (m *model) currentOps() []filter.Op {
	ops := m.filterOps[m.currentField()]
	if len(ops) == 0 {
		return filter.AllOps
	}
	return ops
}

func (m *model) currentOp() filter.Op {
	ops := m.currentOps()
	if m.filter.opIdx < len(ops) {
		return ops[m.filter.opIdx]
	}
	return filter.OpLike
}

func (m *model) buildFilters() []filter.Filter {
	var filters []filter.Filter

	switch m.activeTab {
	case tabInstalled:
		filters = append(filters, filter.Filter{Field: "installed", Op: filter.OpEq, Value: "true"})
	case tabNotInstalled:
		filters = append(filters, filter.Filter{Field: "installed", Op: filter.OpEq, Value: "false"})
	default:
	}

	if m.filter.value != "" {
		filters = append(filters, filter.Filter{
			Field: m.currentField(),
			Op:    m.currentOp(),
			Value: m.filter.value,
		})
	}

	return filters
}

func (m *model) loadPackages() tea.Cmd {
	filters := m.buildFilters()
	ctx := m.ctx
	db := m.dbService
	return func() tea.Msg {
		pkgs, _ := db.QueryHostImagePackages(ctx, filters, "name", "ASC", 0, 0)
		count, _ := db.CountHostImagePackages(ctx, filters)
		return packagesLoadedMsg{packages: pkgs, total: count}
	}
}

func (m *model) moveCursor(delta int) {
	m.cursor += delta
	if m.cursor < 0 {
		m.cursor = 0
	}
	if len(m.packages) > 0 && m.cursor >= len(m.packages) {
		m.cursor = len(m.packages) - 1
	}
	m.adjustOffset()
}

func (m *model) adjustOffset() {
	lh := m.listHeight()
	if lh <= 0 {
		return
	}
	if m.cursor < m.offset {
		m.offset = m.cursor
	}
	if m.cursor >= m.offset+lh {
		m.offset = m.cursor - lh + 1
	}
}

func (m *model) listHeight() int {
	// tabs(1) + filter(1) + header(1) + statusbar(1) = 4 фиксированных строк
	overhead := 4
	detailH := m.detailHeight()
	lh := m.height - overhead - detailH
	if lh < 3 {
		lh = 3
	}
	return lh
}

func (m *model) detailHeight() int {
	h := (m.height * 38) / 100
	if h < 5 {
		h = 5
	}
	if h > 14 {
		h = 14
	}
	return h
}
