package components

import (
	"fmt"
	"os"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
)

type errMsg error

var CircleCISpinner = spinner.Spinner{
	Frames: []string{
		`
 οοο
ο ◉ 	Loading forever...press q to quit
 οοο	`,
		`
 οοο
ο ◉ ο	Loading forever...press q to quit
 οο 	`,
		`
 οοο
ο ◉ ο	Loading forever...press q to quit
 ο ο 	`,
		`
 οοο
ο ◉ ο	Loading forever...press q to quit
  οο	`,
		`
 οοο
  ◉ ο	Loading forever...press q to quit
 οοο	`,
		`
  οο
ο ◉ ο	Loading forever...press q to quit
 οοο	`,
		`
 ο ο
ο ◉ ο	Loading forever...press q to quit
 οοο	`,
		`
 οο
ο ◉ ο	Loading forever...press q to quit
 οοο	`,
	},
	FPS: time.Second / 4,
}

type model struct {
	spinner  spinner.Model
	quitting bool
	err      error
}

func initialModel() model {
	s := spinner.New()
	s.Spinner = CircleCISpinner
	// s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))
	return model{spinner: s}
}

func (m model) Init() tea.Cmd {
	return m.spinner.Tick
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.KeyMsg:
		switch msg.String() {
		case "q", "esc", "ctrl+c":
			m.quitting = true
			return m, tea.Quit
		default:
			return m, nil
		}

	case errMsg:
		m.err = msg
		return m, nil

	default:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	}

}

func (m model) View() string {
	if m.err != nil {
		return m.err.Error()
	}
	str := m.spinner.View()
	if m.quitting {
		return str + "\n"
	}
	return str
}

func Execute() {
	p := tea.NewProgram(initialModel())
	if err := p.Start(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
