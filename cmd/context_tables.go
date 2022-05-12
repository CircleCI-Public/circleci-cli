package cmd

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/evertras/bubble-table/table"
)

const (
	columnKeyProvider  = "provider"
	columnKeyName      = "name"
	columnKeyOrg       = "org"
	columnKeyCreatedAt = "created_at"
)

type RowData struct {
	provider  string
	name      string
	org       string
	createdAt time.Time
}

var (
	styleBase = lipgloss.NewStyle().
			BorderForeground(lipgloss.Color("#47A359")).
			Align(lipgloss.Right)

	customBorder = table.Border{
		Top:    "─",
		Left:   "│",
		Right:  "│",
		Bottom: "─",

		TopRight:    "╮",
		TopLeft:     "╭",
		BottomRight: "╯",
		BottomLeft:  "╰",

		TopJunction:    "┬",
		LeftJunction:   "├",
		RightJunction:  "┤",
		BottomJunction: "┴",
		InnerJunction:  "┼",

		InnerDivider: "│",
	}
)

type Model struct {
	table table.Model
}

func (m Model) Init() tea.Cmd {
	return nil
}

func makeRow(data RowData) table.Row {
	return table.NewRow(table.RowData{
		columnKeyProvider:  data.provider,
		columnKeyName:      data.name,
		columnKeyOrg:       data.org,
		columnKeyCreatedAt: data.createdAt,
	})
}

func NewContextListModel(data []RowData) Model {
	rows := make([]table.Row, len(data))

	for i, row := range data {
		rows[i] = makeRow(row)
	}

	return Model{
		table: table.New([]table.Column{
			table.NewColumn(columnKeyProvider, "Provider", 10),
			table.NewColumn(columnKeyName, "Name", 13),
			table.NewColumn(columnKeyOrg, "Organization", 15),
			table.NewColumn(columnKeyCreatedAt, "Created At", 50),
		}).WithRows(rows).
			Border(customBorder).
			WithBaseStyle(styleBase).
			SortByDesc(columnKeyCreatedAt),
	}
}

func (m Model) View() string {
	view := lipgloss.JoinVertical(
		lipgloss.Left,
		m.table.View(),
	) + "\n"

	return lipgloss.NewStyle().MarginLeft(1).Render(view)
}
