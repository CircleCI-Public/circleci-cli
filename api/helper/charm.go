package helper

import (
	"fmt"
	"net/http"
	"os"

	"github.com/CircleCI-Public/circleci-cli/api/graphql"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type errMsg error

type model struct {
	spinner    spinner.Model
	restClient *http.Client
	restReq    *http.Request
	RestRes    *http.Response
	gqlClient  *graphql.Client
	gqlReq     *graphql.Request
	gqlRes     interface{}
	quitting   bool
	Err        error
}

type respMsg struct {
	err error
}

func initializeModel() model {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))
	return model{spinner: s}
}

func InitializeGqlModel(cl *graphql.Client, req *graphql.Request, res interface{}) model {
	m := initializeModel()
	m.gqlClient = cl
	m.gqlReq = req
	m.gqlRes = &res
	return m
}

func InitializeRestModel(cl *http.Client, req *http.Request) model {
	m := initializeModel()
	m.restClient = cl
	m.restReq = req
	return m
}

func InitializeTeaProgram(m model) {
	p := tea.NewProgram(m)

	if p.Start() != nil {
		fmt.Println("Error: Could not start Tea Program")
		os.Exit(1)
	}
}

func (m model) Init() tea.Cmd {
	checkServer := func() tea.Msg {
		if m.gqlClient != nil {
			m.Err = m.gqlClient.Run(m.gqlReq, &m.gqlRes)
		} else {
			// fmt.Println(m.restReq)
			m.RestRes, m.Err = m.restClient.Do(m.restReq)
			fmt.Println(1, m.RestRes)
		}

		return respMsg{err: m.Err}
	}
	return tea.Batch(
		m.spinner.Tick,
		checkServer,
	)
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

	case respMsg:
		m.quitting = true
		return m, tea.Quit

	case errMsg:
		m.Err = msg
		return m, nil

	default:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	}

}

func (m model) View() string {
	if m.Err != nil {
		return m.Err.Error()
	}
	str := fmt.Sprintf("%s Fetching data… (press q to quit)", m.spinner.View())
	if m.quitting {
		return ""
	}
	return str
}
