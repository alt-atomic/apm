package main

import (
	"fmt"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
)

// Модель нашего приложения
type model struct {
	spinner spinner.Model
	done    bool // Признак, что работа закончена
}

func initialModel() model {
	s := spinner.New()
	s.Spinner = spinner.Line
	return model{
		spinner: s,
		done:    false,
	}
}

func (m model) Init() tea.Cmd {
	return m.spinner.Tick
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	// Сообщение о том, что пора завершать работу
	case finishMsg:
		m.done = true
		// Завершаем программу
		return m, tea.Quit

	// Автоматические «тики» спиннера
	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	}
	// По умолчанию ничего не делаем
	return m, nil
}

func (m model) View() string {
	return fmt.Sprintf("%s Выполняется...\n", m.spinner.View())
}

// Вспомогательное сообщение, сигнализирующее о завершении.
// Можно передавать в Update через tea.Msg
type finishMsg struct{}

func main() {
	// Создаём программу
	p := tea.NewProgram(initialModel())

	// Запускаем в отдельной горутине какую-то «долгую задачу»
	go func() {
		// Имитируем длительную операцию
		time.Sleep(3 * time.Second)
		// Посылаем программе сообщение, что всё готово
		p.Send(finishMsg{})
	}()

	// Запускаем Bubble Tea
	if err := p.Start(); err != nil {
		fmt.Println("Ошибка запуска программы:", err)
		return
	}
}
