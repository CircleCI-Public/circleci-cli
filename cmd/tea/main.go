package tea

import (
	"fmt"
	"os"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/spf13/cobra"
)

var (
	appStyle = lipgloss.NewStyle().Padding(1, 2)

	titleStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFFDF5")).
			Background(lipgloss.Color("#25A065")).
			Padding(0, 1)

	statusMessageStyle = lipgloss.
				NewStyle().
				Foreground(lipgloss.AdaptiveColor{
			Light: "#04B575",
			Dark:  "#04B575",
		}).
		Render
)

type model struct {
	ready          bool
	currentCmd     *cobra.Command
	highlightedCmd *cobra.Command
	list           list.Model
	viewport       viewport.Model
	keys           *listKeyMap
	delegateKeys   *delegateKeyMap
}

func newModel(rootCmd *cobra.Command) model {
	var (
		listKeys     = newListKeyMap()
		delegateKeys = newDelegateKeyMap()
	)

	items := generateChildCommandList(rootCmd)

	// Setup list
	delegate := newItemDelegate(delegateKeys)
	commandList := list.New(items, delegate, 0, 0)
	commandList.Title = "CircleCI CLI"
	commandList.Styles.Title = titleStyle
	commandList.AdditionalFullHelpKeys = func() []key.Binding {
		return []key.Binding{
			listKeys.toggleSpinner,
			listKeys.toggleTitleBar,
			listKeys.toggleStatusBar,
			listKeys.togglePagination,
			listKeys.toggleHelpMenu,
		}
	}

	// Setup viewport
	v := viewport.New(0, 0) // these are just temp values..
	v.MouseWheelEnabled = true

	firstCmd := items[0].(commandListItem).cmd

	return model{
		ready:          false,
		currentCmd:     rootCmd,
		highlightedCmd: firstCmd,
		list:           commandList,
		viewport:       v,
		keys:           listKeys,
		delegateKeys:   delegateKeys,
	}
}

func (m model) Init() tea.Cmd {
	return nil
}

// func (m model) getCurrentCommand() commandListItem {
// 	// TODO: this throws an error when you try filtering
// 	// interface conversion: list.Item is nil, not tea.commandListItem

// 	return m.list.SelectedItem().(commandListItem)
// }

// func (m model) getCurrentCobraCommand() *cobra.Command {
// 	return m.getCurrentCommand().cmd
// }

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	// This will also call our delegate's update function.
	newListModel, cmd := m.list.Update(msg)
	m.list = newListModel
	cmds = append(cmds, cmd)

	newViewportModel, cmd := m.viewport.Update(msg)
	m.viewport = newViewportModel
	cmds = append(cmds, cmd)

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		// update list sizing
		h, v := appStyle.GetFrameSize()
		newWidth := msg.Width - h
		newHeight := msg.Height - v

		m.list.SetSize(newWidth, newHeight)

		// update viewport sizing
		if !m.ready {
			// Since this program is using the full size of the viewport we
			// need to wait until we've received the window dimensions before
			// we can initialize the viewport. The initial dimensions come in
			// quickly, though asynchronously, which is why we wait for them
			// here.
			m.viewport = viewport.New(newWidth, newHeight)
			// m.viewport.YPosition = headerHeight
			// m.viewport.HighPerformanceRendering = useHighPerformanceRenderer
			m.ready = true
			m.viewport.SetContent(m.currentCmd.UsageString())

			// This is only necessary for high performance rendering, which in
			// most cases you won't need.
			//
			// Render the viewport one line below the header.
			// m.viewport.YPosition = headerHeight + 1
		} else {
			m.viewport.Width = newWidth
			m.viewport.Height = newHeight
		}

		// if useHighPerformanceRenderer {
		// 	// Render (or re-render) the whole viewport. Necessary both to
		// 	// initialize the viewport and when the window is resized.
		// 	//
		// 	// This is needed for high-performance rendering only.
		// 	cmds = append(cmds, viewport.Sync(m.viewport))
		// }

	case tea.KeyMsg:
		// Don't match any of the keys below if we're actively filtering.
		if m.list.FilterState() == list.Filtering {
			break
		}

		listKeys := m.list.KeyMap

		switch {
		case
			key.Matches(msg, listKeys.CursorDown),
			key.Matches(msg, listKeys.CursorUp),
			key.Matches(msg, listKeys.NextPage),
			key.Matches(msg, listKeys.PrevPage):

			m.highlightedCmd = m.list.SelectedItem().(commandListItem).cmd
			m.viewport.SetContent(m.highlightedCmd.UsageString())

			return m, nil

		case key.Matches(msg, m.delegateKeys.choose):
			// if the current command has sub-commands traverse the command tree "down" a level
			chosenCmd := m.list.SelectedItem().(commandListItem).cmd
			chosenCmdChildCmds := chosenCmd.Commands()

			if len(chosenCmdChildCmds) > 0 {
				m.list.SetItems(generateChildCommandList(chosenCmd))

				m.currentCmd = chosenCmd
				m.highlightedCmd = chosenCmdChildCmds[0]

				m.viewport.SetContent(m.highlightedCmd.UsageString())
			} else {
				// TODO:
				// chosen command is a "node" comand (ie. it has no child commands) and should
				// instead be executed.. execute it here
			}
			return m, nil

		case key.Matches(msg, m.delegateKeys.goBack):
			// if the current command has a parent command, traverse the tree "up" a level

			if m.currentCmd.HasParent() {
				parentCmd := m.currentCmd.Parent()

				m.list.SetItems(generateChildCommandList(parentCmd))

				m.currentCmd = parentCmd
				m.highlightedCmd = parentCmd.Commands()[0]

				m.viewport.SetContent(m.highlightedCmd.UsageString())
			}
			return m, nil

		case key.Matches(msg, m.keys.toggleSpinner):
			cmd := m.list.ToggleSpinner()
			return m, cmd

		case key.Matches(msg, m.keys.toggleTitleBar):
			v := !m.list.ShowTitle()
			m.list.SetShowTitle(v)
			m.list.SetShowFilter(v)
			m.list.SetFilteringEnabled(v)
			return m, nil

		case key.Matches(msg, m.keys.toggleStatusBar):
			m.list.SetShowStatusBar(!m.list.ShowStatusBar())
			return m, nil

		case key.Matches(msg, m.keys.togglePagination):
			m.list.SetShowPagination(!m.list.ShowPagination())
			return m, nil

		case key.Matches(msg, m.keys.toggleHelpMenu):
			m.list.SetShowHelp(!m.list.ShowHelp())
			return m, nil
		}
	}

	return m, tea.Batch(cmds...)
}

func (m model) View() string {
	return appStyle.Render(lipgloss.JoinHorizontal(lipgloss.Center,
		m.list.View(),
		m.viewport.View(),
	))
}

func RunTeaBrowser(cmd *cobra.Command, args []string) {
	p := tea.NewProgram(
		newModel(cmd),
		tea.WithAltScreen(),       // use the full size of the terminal in its "alternate screen buffer"
		tea.WithMouseCellMotion(), // turn on mouse support so we can track the mouse wheel
	)

	if err := p.Start(); err != nil {
		fmt.Println("Error running program:", err)
		os.Exit(1)
	}
}
