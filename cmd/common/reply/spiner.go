package reply

import (
	"apm/lib"
	"fmt"
	"log"
	"os"
	"sync"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
)

var (
	p        *tea.Program
	doneChan chan struct{}
	mu       sync.Mutex
)

type model struct {
	spinner spinner.Model
}

// CreateSpinner — запуск спиннера в отдельной горутине.
func CreateSpinner() {
	if lib.Env.Format != "text" {
		return
	}

	mu.Lock()
	defer mu.Unlock()

	if p != nil {
		return
	}
	doneChan = make(chan struct{})

	p = tea.NewProgram(
		newSpinner(),
		tea.WithOutput(os.Stdout),
		//tea.WithAltScreen(),
	)

	go func() {
		if err := p.Start(); err != nil {
			log.Println("Ошибка запуска спиннера:", err)
		}
		// Как только p.Start() вернулась (Quit завершён) — закрываем канал
		close(doneChan)
	}()
}

// StopSpinner — останавливает спиннер (ждёт, пока Bubble Tea полностью выйдет).
func StopSpinner() {
	if lib.Env.Format != "text" {
		return
	}

	mu.Lock()
	defer mu.Unlock()

	if p != nil {
		// Посылаем сигнал на завершение
		p.Quit()
		// Ждём, пока p.Start() действительно вернётся и канал doneChan закроется
		<-doneChan
		p = nil
		// Стираем строку (если хотим убрать "Выполняется...")
		fmt.Print("\r\033[K")
	}
}

// newSpinner инициализирует нашу модель со спиннером
func newSpinner() model {
	s := spinner.New()
	s.Spinner = spinner.Points
	return model{spinner: s}
}

func (m model) Init() tea.Cmd {
	return m.spinner.Tick
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg.(type) {
	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	}
	return m, nil
}

// View — просто рисуем одну строку с "Выполняется..."
// Перевод каретки в начало строки и очистка, чтобы перерисовывать одну строку.
func (m model) View() string {
	return fmt.Sprintf("\r\033[K%s \033[33mВыполнение...\033[0m", m.spinner.View())
}
