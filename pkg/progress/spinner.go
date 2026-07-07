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

package progress

import (
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"golang.org/x/sys/unix"
)

const clearLine = "\r\033[K"

var spinnerFrames = []string{"|", "/", "-", "\\"}

// TaskUpdate is a neutral task update for the spinner.
type TaskUpdate struct {
	Name       string  // stable task id
	View       string  // display text
	IsProgress bool    // render a percent bar
	Percent    float64 // current percent (for IsProgress)
	DoneText   string  // completion text (for IsProgress)
	Done       bool    // task finished
}

type task struct {
	name       string
	view       string
	isProgress bool
	done       bool
	printed    bool
	percent    float64
	doneText   string
}

// Spinner is a terminal task/progress indicator.
type Spinner struct {
	mu           sync.Mutex
	tasks        []task
	frame        int
	colors       Colors
	activeLines  int
	stopCh       chan struct{}
	doneCh       chan struct{}
	filledStyle  lipgloss.Style
	emptyStyle   lipgloss.Style
	savedTermios *unix.Termios
}

// New creates a spinner with the palette, not started yet.
func New(colors Colors) *Spinner {
	return &Spinner{
		colors:      colors,
		stopCh:      make(chan struct{}),
		doneCh:      make(chan struct{}),
		filledStyle: lipgloss.NewStyle().Foreground(lipgloss.Color(colors.Filled)),
		emptyStyle:  lipgloss.NewStyle().Foreground(lipgloss.Color(colors.Empty)),
	}
}

// Start hides the cursor, disables echo and runs the redraw loop.
func (sp *Spinner) Start() {
	sp.disableEcho()
	fmt.Print("\033[?25l") // hide cursor
	go sp.run()
}

// Update adds or updates a task.
func (sp *Spinner) Update(u TaskUpdate) {
	sp.mu.Lock()
	defer sp.mu.Unlock()

	for i := range sp.tasks {
		if sp.tasks[i].name == u.Name {
			sp.tasks[i].done = u.Done
			sp.tasks[i].view = u.View
			sp.tasks[i].isProgress = u.IsProgress
			if u.IsProgress {
				sp.tasks[i].percent = u.Percent
				sp.tasks[i].doneText = u.DoneText
			}
			return
		}
	}

	sp.tasks = append(sp.tasks, task{
		name:       u.Name,
		view:       u.View,
		isProgress: u.IsProgress,
		done:       u.Done,
		percent:    u.Percent,
		doneText:   u.DoneText,
	})
}

// Stop halts animation, flushes completed tasks and restores the terminal.
func (sp *Spinner) Stop() {
	close(sp.stopCh)
	<-sp.doneCh

	sp.mu.Lock()
	al := sp.activeLines

	// move cursor to the top of the animated area
	if al > 1 {
		fmt.Printf("\033[%dA", al-1)
	}

	// print completed tasks not yet flushed by render
	maxWidth := termWidth() - 1
	linesUsed := 0
	for i := range sp.tasks {
		t := &sp.tasks[i]
		if t.done && !t.printed {
			t.printed = true
			fmt.Printf("%s%s\n", clearLine, truncLine("[✓] "+t.view, maxWidth))
			linesUsed++
		}
	}

	// clear remaining animation lines
	if extra := al - linesUsed; extra > 0 {
		for i := range extra {
			fmt.Print(clearLine)
			if i < extra-1 {
				fmt.Print("\n")
			}
		}
	}
	sp.mu.Unlock()

	fmt.Print(clearLine)
	fmt.Print("\033[?25h") // show cursor
	sp.restoreEcho()
}

func (sp *Spinner) run() {
	defer close(sp.doneCh)
	ticker := time.NewTicker(120 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-sp.stopCh:
			return
		case <-ticker.C:
			sp.render()
		}
	}
}

func (sp *Spinner) render() {
	sp.mu.Lock()

	prevActiveLines := sp.activeLines

	var completedLines []string
	for i := range sp.tasks {
		t := &sp.tasks[i]
		if t.done && !t.printed {
			t.printed = true
			if t.isProgress && len(t.doneText) > 0 {
				completedLines = append(completedLines, fmt.Sprintf(translate("Progress: %s completed"), t.doneText))
			} else {
				completedLines = append(completedLines, t.view)
			}
		}
	}

	n := 0
	for i := range sp.tasks {
		if !sp.tasks[i].done || !sp.tasks[i].printed {
			sp.tasks[n] = sp.tasks[i]
			n++
		}
	}
	sp.tasks = sp.tasks[:n]

	var actives []task
	for i := range sp.tasks {
		if !sp.tasks[i].done {
			actives = append(actives, sp.tasks[i])
		}
	}

	frame := spinnerFrames[sp.frame%len(spinnerFrames)]
	sp.frame++
	sp.activeLines = len(actives)

	filledStyle := sp.filledStyle
	emptyStyle := sp.emptyStyle

	sp.mu.Unlock()

	// keep lines within terminal width, wrap breaks redraw
	maxWidth := termWidth() - 1

	var buf strings.Builder

	if prevActiveLines > 1 {
		fmt.Fprintf(&buf, "\033[%dA", prevActiveLines-1)
	}

	for _, line := range completedLines {
		buf.WriteString(clearLine)
		buf.WriteString(truncLine("[✓] "+line, maxWidth))
		buf.WriteByte('\n')
	}

	// active tasks with spinner
	if len(actives) > 0 {
		for idx := range actives {
			line := "[" + frame + "] "
			if actives[idx].isProgress {
				line += renderProgressBar(actives[idx], filledStyle, emptyStyle)
			} else {
				line += actives[idx].view
			}
			buf.WriteString(clearLine)
			buf.WriteString(truncLine(line, maxWidth))
			if idx < len(actives)-1 {
				buf.WriteByte('\n')
			}
		}
	}

	if extra := prevActiveLines - len(actives); extra > 0 {
		for range extra {
			buf.WriteString("\n\033[K")
		}
		fmt.Fprintf(&buf, "\033[%dA", extra)
	}

	if len(actives) == 0 {
		buf.WriteString(clearLine)
	}

	_, _ = os.Stdout.WriteString(buf.String())
}

func renderProgressBar(t task, filledStyle, emptyStyle lipgloss.Style) string {
	const width = 30
	pct := t.percent
	if pct < 0 {
		pct = 0
	} else if pct > 100 {
		pct = 100
	}

	filled := int(pct / 100 * float64(width))
	bar := filledStyle.Render(strings.Repeat("█", filled)) + emptyStyle.Render(strings.Repeat("░", width-filled))
	return fmt.Sprintf("[%s] %.0f%% %s", bar, pct, t.view)
}

// termWidth returns terminal width in columns.
func termWidth() int {
	ws, err := unix.IoctlGetWinsize(int(os.Stdout.Fd()), unix.TIOCGWINSZ)
	if err != nil || ws.Col == 0 {
		return 80
	}
	return int(ws.Col)
}

// truncLine truncates a line to terminal width to avoid wrapping.
func truncLine(s string, width int) string {
	out := ansi.Truncate(s, width, "…")
	if out != s {
		out += "\x1b[m"
	}
	return out
}

func (sp *Spinner) disableEcho() {
	fd := int(os.Stdin.Fd())
	termios, err := unix.IoctlGetTermios(fd, unix.TCGETS)
	if err != nil {
		return
	}
	saved := *termios
	sp.savedTermios = &saved
	termios.Lflag &^= unix.ECHO
	_ = unix.IoctlSetTermios(fd, unix.TCSETS, termios)
}

func (sp *Spinner) restoreEcho() {
	if sp.savedTermios == nil {
		return
	}
	fd := int(os.Stdin.Fd())
	_ = unix.IoctlSetTermios(fd, unix.TCSETS, sp.savedTermios)
	sp.savedTermios = nil
}
