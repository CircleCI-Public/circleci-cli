package cmd

// import (
// 	"fmt"
// 	"os"

// 	tea "github.com/charmbracelet/bubbletea"
// 	"github.com/charmbracelet/glamour"
// 	"github.com/charmbracelet/lipgloss"

// 	"github.com/charmbracelet/bubbles/key"
// 	"github.com/charmbracelet/bubbles/list"
// 	"github.com/charmbracelet/bubbles/viewport"

// 	"github.com/spf13/cobra"
// )

// /*
// 	Examples that were quite helpful

// 	For the scrollable viewport..
// 	https://github.com/charmbracelet/bubbletea/blob/master/examples/pager/main.go
// */

// // You generally won't need this unless you're processing stuff with
// // complicated ANSI escape sequences. Turn it on if you notice flickering.
// //
// // Also keep in mind that high performance rendering only works for programs
// // that use the full size of the terminal. We're enabling that below with
// // tea.EnterAltScreen().
// const useHighPerformanceRenderer = true

// var (
// 	appStyle = lipgloss.NewStyle().Padding(3)

// 	titleStyle = lipgloss.NewStyle().
// 			Foreground(lipgloss.Color("#FFFDF5")).
// 			Background(lipgloss.Color("#25A065")).
// 			Padding(0, 1)

// 	// statusMessageStyle = lipgloss.NewStyle().
// 	// 			Foreground(lipgloss.AdaptiveColor{Light: "#04B575", Dark: "#04B575"}).
// 	// 			Render
// )

// type CommandListItem struct {
// 	title       string
// 	description string
// 	cmd         *cobra.Command
// }

// func (i CommandListItem) Title() string       { return i.title }
// func (i CommandListItem) Description() string { return i.description }
// func (i CommandListItem) FilterValue() string { return i.title }

// type delegateKeyMap struct {
// 	choose key.Binding
// }

// type listKeyMap struct {
// 	toggleTitleBar   key.Binding
// 	toggleStatusBar  key.Binding
// 	togglePagination key.Binding
// 	toggleHelpMenu   key.Binding
// }

// type TeaModel struct {
// 	ready           bool
// 	rootCommand     *cobra.Command
// 	selectedCommand *CommandListItem
// 	list            list.Model
// 	helpViewport    viewport.Model
// 	termRenderer    *glamour.TermRenderer
// 	keys            *listKeyMap
// 	delegateKeys    *delegateKeyMap
// }

// func newItemDelegate(keys *delegateKeyMap) list.DefaultDelegate {
// 	d := list.NewDefaultDelegate()

// 	d.UpdateFunc = func(msg tea.Msg, m *list.Model) tea.Cmd {
// 		// i, ok := m.SelectedItem().(CommandListItem)

// 		// if !ok {
// 		// 	// TODO: what to do in error here?
// 		// 	return nil
// 		// }

// 		// switch msg := msg.(type) {
// 		// case tea.KeyMsg:
// 		// 	switch {
// 		// 	case key.Matches(msg, keys.choose):

// 		// 		return i.command()
// 		// 	}
// 		// }

// 		return nil
// 	}

// 	help := []key.Binding{keys.choose}

// 	d.ShortHelpFunc = func() []key.Binding {
// 		return help
// 	}

// 	d.FullHelpFunc = func() [][]key.Binding {
// 		return [][]key.Binding{help}
// 	}

// 	return d
// }

// // Additional short help entries. This satisfies the help.KeyMap interface and
// // is entirely optional.
// func (d delegateKeyMap) ShortHelp() []key.Binding {
// 	return []key.Binding{
// 		d.choose,
// 	}
// }

// // Additional full help entries. This satisfies the help.KeyMap interface and
// // is entirely optional.
// func (d delegateKeyMap) FullHelp() [][]key.Binding {
// 	return [][]key.Binding{
// 		{
// 			d.choose,
// 		},
// 	}
// }

// func newDelegateKeyMap() *delegateKeyMap {
// 	return &delegateKeyMap{
// 		choose: key.NewBinding(
// 			key.WithKeys("enter"),
// 			key.WithHelp("enter", "choose"),
// 		),
// 	}
// }

// func newListKeyMap() *listKeyMap {
// 	return &listKeyMap{
// 		toggleTitleBar: key.NewBinding(
// 			key.WithKeys("T"),
// 			key.WithHelp("T", "toggle title"),
// 		),
// 		toggleStatusBar: key.NewBinding(
// 			key.WithKeys("S"),
// 			key.WithHelp("S", "toggle status"),
// 		),
// 		togglePagination: key.NewBinding(
// 			key.WithKeys("P"),
// 			key.WithHelp("P", "toggle pagination"),
// 		),
// 		toggleHelpMenu: key.NewBinding(
// 			key.WithKeys("H"),
// 			key.WithHelp("H", "toggle help"),
// 		),
// 	}
// }

// func NewTeaModel(rootCmd *cobra.Command) (*TeaModel, error) {
// 	var (
// 		delegateKeys = newDelegateKeyMap()
// 		listKeys     = newListKeyMap()
// 	)

// 	var items []list.Item

// 	for _, cmd := range rootCmd.Commands() {
// 		items = append(items, CommandListItem{
// 			title:       cmd.Name(),
// 			description: cmd.Short,
// 			cmd:         cmd,
// 		})

// 	}

// 	// Setup list
// 	delegate := newItemDelegate(delegateKeys)
// 	commandList := list.New(items, delegate, 0, 0)
// 	commandList.Title = "CircleCI Commands"
// 	commandList.Styles.Title = titleStyle
// 	commandList.AdditionalFullHelpKeys = func() []key.Binding {
// 		return []key.Binding{
// 			listKeys.toggleTitleBar,
// 			listKeys.toggleStatusBar,
// 			listKeys.togglePagination,
// 			listKeys.toggleHelpMenu,
// 		}
// 	}

// 	// Setup help viewport
// 	helpViewport := viewport.New(78, 20) // these are just temp values..
// 	helpViewport.MouseWheelEnabled = true

// 	termRenderer, err := glamour.NewTermRenderer(glamour.WithStylePath("notty"))
// 	if err != nil {
// 		return nil, err
// 	}

// 	return &TeaModel{
// 		ready:           false,
// 		rootCommand:     rootCmd,
// 		selectedCommand: items[0].(*CommandListItem),
// 		list:            commandList,
// 		helpViewport:    helpViewport,
// 		termRenderer:    termRenderer,
// 		keys:            listKeys,
// 		delegateKeys:    delegateKeys,
// 	}, nil
// }

// func (m TeaModel) Init() tea.Cmd {
// 	return nil
// }

// func (m TeaModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
// 	var cmds []tea.Cmd

// 	// This will also call our delegate's update function.
// 	newListModel, listCmd := m.list.Update(msg)
// 	m.list = newListModel

// 	// Handle keyboard and mouse events in the viewport
// 	newHelpViewportModel, helpViewportCmd := m.helpViewport.Update(msg)
// 	m.helpViewport = newHelpViewportModel

// 	cmds = append(cmds, listCmd, helpViewportCmd)

// 	switch msg := msg.(type) {
// 	case tea.WindowSizeMsg:
// 		h, v := appStyle.GetFrameSize()

// 		newWidth := msg.Width - h
// 		newHeight := msg.Height - v

// 		// update list
// 		m.list.SetSize(newWidth, newHeight)

// 		// update help viewport
// 		if !m.ready {
// 			// Since this program is using the full size of the viewport we
// 			// need to wait until we've received the window dimensions before
// 			// we can initialize the viewport. The initial dimensions come in
// 			// quickly, though asynchronously, which is why we wait for them
// 			// here.
// 			m.helpViewport = viewport.New(newWidth, newHeight)
// 			m.helpViewport.HighPerformanceRendering = useHighPerformanceRenderer

// 			helpMsg, err := m.termRenderer.Render(m.selectedCommand.cmd.Long)
// 			if err != nil {
// 				return m, tea.Quit
// 			}

// 			m.helpViewport.SetContent(helpMsg)

// 			m.ready = true
// 		} else {
// 			m.helpViewport.Width = newWidth
// 			m.helpViewport.Height = newHeight
// 		}

// 		if useHighPerformanceRenderer {
// 			// Render (or re-render) the whole viewport. Necessary both to
// 			// initialize the viewport and when the window is resized.
// 			//
// 			// This is needed for high-performance rendering only.
// 			cmds = append(cmds, viewport.Sync(m.helpViewport))
// 		}

// 	case tea.KeyMsg:
// 		// Don't match any of the keys below if we're actively filtering.
// 		if m.list.FilterState() == list.Filtering {
// 			break
// 		}

// 		switch {
// 		case key.Matches(msg, m.list.KeyMap.CursorUp), key.Matches(msg, m.list.KeyMap.CursorDown):
// 			// set the selectedCommand on the model
// 			selectedCommand := m.list.SelectedItem().(CommandListItem)
// 			m.selectedCommand = m.list.SelectedItem().(CommandListItem)

// 			helpMsg, err := m.termRenderer.Render(selectedCommand.cmd.Long)
// 			if err != nil {
// 				return m, tea.Quit
// 			}

// 			m.helpViewport.SetContent(helpMsg)

// 			return m, tea.Batch(cmds...)
// 		case key.Matches(msg, m.delegateKeys.choose):
// 			// TODO: execute command when enter is pressed
// 			return m, nil

// 		case key.Matches(msg, m.keys.toggleTitleBar):
// 			v := !m.list.ShowTitle()
// 			m.list.SetShowTitle(v)
// 			m.list.SetShowFilter(v)
// 			m.list.SetFilteringEnabled(v)
// 			return m, nil

// 		case key.Matches(msg, m.keys.toggleStatusBar):
// 			m.list.SetShowStatusBar(!m.list.ShowStatusBar())
// 			return m, nil

// 		case key.Matches(msg, m.keys.togglePagination):
// 			m.list.SetShowPagination(!m.list.ShowPagination())
// 			return m, nil

// 			// case key.Matches(msg, m.keys.toggleHelpMenu):
// 			// 	m.list.SetShowHelp(!m.list.ShowHelp())
// 			// 	return m, nil

// 			// case key.Matches(msg, m.keys.insertItem):
// 			// 	m.delegateKeys.remove.SetEnabled(true)
// 			// 	newItem := m.itemGenerator.next()
// 			// 	insCmd := m.list.InsertItem(0, newItem)
// 			// 	statusCmd := m.list.NewStatusMessage(statusMessageStyle("Added " + newItem.Title()))
// 			// 	return m, tea.Batch(insCmd, statusCmd)
// 		}
// 	}

// 	return m, tea.Batch(cmds...)
// }

// func (m TeaModel) View() string {
// 	var content string

// 	if !m.ready {
// 		// things are starting up..
// 		content = "\n  Initializing..."
// 	} else {
// 		// a command has been selected, show the list + help viewport

// 		helpViewport := lipgloss.NewStyle().
// 			BorderStyle(lipgloss.RoundedBorder()).
// 			BorderForeground(lipgloss.Color("62")).
// 			PaddingRight(2).
// 			Render(m.helpViewport.View())

// 		content = lipgloss.JoinHorizontal(lipgloss.Top,
// 			m.list.View(),
// 			helpViewport,
// 		)
// 	}

// 	return appStyle.Render(content)
// }

// func RunTeaBrowser(cmd *cobra.Command, args []string) {
// 	model, err := NewTeaModel(cmd)
// 	if err != nil {
// 		fmt.Println("Could not initialize Bubble Tea model:", err)
// 		os.Exit(1)
// 	}

// 	p := tea.NewProgram(
// 		model,
// 		tea.WithAltScreen(), // use the full size of the terminal in its "alternate screen buffer"
// 		// tea.WithMouseCellMotion(), // turn on mouse support so we can track the mouse wheel
// 	)

// 	if err := p.Start(); err != nil {
// 		fmt.Println("Error running program:", err)
// 		os.Exit(1)
// 	}
// }
