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

package reply

import (
	"apm/internal/common/app"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/charmbracelet/lipgloss"
	"golang.org/x/sys/unix"
)

const clearLine = "\r\033[K"

var spinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

var (
	mu           sync.Mutex
	activeSp     *simpleSpinner
	savedTermios *unix.Termios
)

type task struct {
	eventType        string
	name             string
	viewName         string
	state            string
	printed          bool
	progressPercent  float64
	progressDoneText string
}

type simpleSpinner struct {
	mu          sync.Mutex
	tasks       []task
	frame       int
	colors      app.Colors
	activeLines int
	stopCh      chan struct{}
	doneCh      chan struct{}
}

func disableEcho() {
	fd := int(os.Stdin.Fd())
	termios, err := unix.IoctlGetTermios(fd, unix.TCGETS)
	if err != nil {
		return
	}
	saved := *termios
	savedTermios = &saved
	termios.Lflag &^= unix.ECHO
	_ = unix.IoctlSetTermios(fd, unix.TCSETS, termios)
}

func restoreEcho() {
	if savedTermios == nil {
		return
	}
	fd := int(os.Stdin.Fd())
	_ = unix.IoctlSetTermios(fd, unix.TCSETS, savedTermios)
	savedTermios = nil
}

// CreateSpinner создание и запуск спиннера.
func CreateSpinner(appConfig *app.Config) {
	if appConfig.ConfigManager.GetConfig().Format != app.FormatText || !IsTTY() {
		return
	}

	mu.Lock()
	defer mu.Unlock()

	if activeSp != nil {
		return
	}

	disableEcho()
	fmt.Print("\033[?25l") // скрыть курсор

	sp := &simpleSpinner{
		colors: appConfig.ConfigManager.GetColors(),
		stopCh: make(chan struct{}),
		doneCh: make(chan struct{}),
	}
	activeSp = sp

	go sp.run()
}

// StopSpinner остановка с сохранением завершённых задач.
func StopSpinner(appConfig *app.Config) {
	stopSpinner(appConfig, true)
}

// StopSpinnerForDialog остановка с полной очисткой перед диалогом.
func StopSpinnerForDialog(appConfig *app.Config) {
	stopSpinner(appConfig, false)
}

func stopSpinner(appConfig *app.Config, keepTasks bool) {
	if appConfig.ConfigManager.GetConfig().Format != app.FormatText || !IsTTY() {
		return
	}

	mu.Lock()
	defer mu.Unlock()

	if activeSp == nil {
		return
	}

	close(activeSp.stopCh)
	<-activeSp.doneCh

	activeSp.mu.Lock()
	al := activeSp.activeLines

	// Возвращаем курсор к началу анимируемой области
	if al > 1 {
		fmt.Printf("\033[%dA", al-1)
	}

	linesUsed := 0
	if keepTasks {
		for i := range activeSp.tasks {
			t := &activeSp.tasks[i]
			if t.state == StateAfter && !t.printed {
				t.printed = true
				fmt.Printf("%s[✓] %s\n", clearLine, t.viewName)
				linesUsed++
			}
		}
	}

	// Очищаем оставшиеся строки анимации
	if extra := al - linesUsed; extra > 0 {
		for i := 0; i < extra; i++ {
			fmt.Print(clearLine)
			if i < extra-1 {
				fmt.Print("\n")
			}
		}
	}
	activeSp.mu.Unlock()

	fmt.Print(clearLine)
	fmt.Print("\033[?25h") // показать курсор
	activeSp = nil
	restoreEcho()
}

// UpdateTask обновление задачи/прогресса.
func UpdateTask(appConfig *app.Config, eventType string, taskName string, viewName string, state string, progressValue float64, progressDone string) {
	if appConfig.ConfigManager.GetConfig().Format != app.FormatText || !IsTTY() {
		return
	}

	mu.Lock()
	sp := activeSp
	if sp == nil {
		mu.Unlock()
		return
	}
	sp.mu.Lock()
	mu.Unlock()
	defer sp.mu.Unlock()

	for i, t := range sp.tasks {
		if t.name == taskName {
			sp.tasks[i].state = state
			sp.tasks[i].viewName = viewName
			sp.tasks[i].eventType = eventType
			if eventType == EventTypeProgress {
				sp.tasks[i].progressPercent = progressValue
				sp.tasks[i].progressDoneText = progressDone
			}
			return
		}
	}

	sp.tasks = append(sp.tasks, task{
		eventType:        eventType,
		name:             taskName,
		viewName:         viewName,
		state:            state,
		progressPercent:  progressValue,
		progressDoneText: progressDone,
	})
}

func (sp *simpleSpinner) run() {
	defer close(sp.doneCh)
	ticker := time.NewTicker(80 * time.Millisecond)
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

func (sp *simpleSpinner) render() {
	sp.mu.Lock()
	defer sp.mu.Unlock()

	// Возвращаем курсор к началу анимируемой области
	if sp.activeLines > 1 {
		fmt.Printf("\033[%dA", sp.activeLines-1)
	}

	// Печатаем завершённые задачи как постоянные строки
	for i := range sp.tasks {
		t := &sp.tasks[i]
		if t.state == StateAfter && !t.printed {
			t.printed = true
			if t.eventType == EventTypeProgress && len(t.progressDoneText) > 0 {
				fmt.Printf("%s[✓] %s\n", clearLine, fmt.Sprintf(app.T_("Progress: %s completed"), t.progressDoneText))
			} else {
				fmt.Printf("%s[✓] %s\n", clearLine, t.viewName)
			}
		}
	}

	n := 0
	for i := range sp.tasks {
		if !(sp.tasks[i].state == StateAfter && sp.tasks[i].printed) {
			sp.tasks[n] = sp.tasks[i]
			n++
		}
	}
	sp.tasks = sp.tasks[:n]

	var actives []*task
	for i := range sp.tasks {
		if sp.tasks[i].state != StateAfter {
			actives = append(actives, &sp.tasks[i])
		}
	}

	if len(actives) > 0 {
		frame := spinnerFrames[sp.frame%len(spinnerFrames)]
		sp.frame++

		for idx, active := range actives {
			if active.eventType == EventTypeProgress {
				fmt.Printf("%s[%s] %s", clearLine, frame, renderProgressBar(*active, sp.colors))
			} else {
				fmt.Printf("%s[%s] %s", clearLine, frame, active.viewName)
			}
			if idx < len(actives)-1 {
				fmt.Print("\n")
			}
		}
	}

	// Очищаем лишние строки от предыдущего рендера
	if extra := sp.activeLines - len(actives); extra > 0 {
		for i := 0; i < extra; i++ {
			fmt.Print("\n\033[K")
		}
		fmt.Printf("\033[%dA", extra)
	}

	sp.activeLines = len(actives)

	if len(actives) == 0 {
		fmt.Print(clearLine)
	}
}

func renderProgressBar(t task, colors app.Colors) string {
	const width = 30
	pct := t.progressPercent
	if pct < 0 {
		pct = 0
	} else if pct > 100 {
		pct = 100
	}

	filled := int(pct / 100 * float64(width))
	filledStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(colors.ProgressEnd))
	emptyStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(colors.ProgressStart))
	bar := filledStyle.Render(strings.Repeat("█", filled)) + emptyStyle.Render(strings.Repeat("░", width-filled))
	return fmt.Sprintf("[%s] %.0f%% %s", bar, pct, t.viewName)
}
