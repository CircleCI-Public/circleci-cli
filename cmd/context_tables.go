package cmd

import (
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/evertras/bubble-table/table"
)

const (
	columnKeyProvider  = "provider"
	columnKeyName      = "name"
	columnKeyOrg       = "org"
	columnKeyCreatedAt = "created_at"
	columnKeyValue     = "value"
)

type ContextListRowData struct {
	provider  string
	name      string
	org       string
	createdAt time.Time
}

type ContextShowRowData struct {
	name  string
	value string
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

type ContextListModel struct {
	table table.Model
}

type ContextShowModel struct {
	table table.Model
}

func makeContextListRow(data ContextListRowData) table.Row {
	return table.NewRow(table.RowData{
		columnKeyProvider:  data.provider,
		columnKeyName:      data.name,
		columnKeyOrg:       data.org,
		columnKeyCreatedAt: data.createdAt,
	})
}

func NewContextListModel(data []ContextListRowData) ContextListModel {
	rows := make([]table.Row, len(data))
	for i, row := range data {
		rows[i] = makeContextListRow(row)
	}

	return ContextListModel{
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

func (m ContextListModel) View() string {
	view := lipgloss.JoinVertical(
		lipgloss.Left,
		m.table.View(),
	) + "\n"

	return lipgloss.NewStyle().MarginLeft(1).Render(view)
}

func makeContextShowRow(data ContextShowRowData) table.Row {
	return table.NewRow(table.RowData{
		columnKeyName:  data.name,
		columnKeyValue: data.value,
	})
}

func NewContextShowModel(data []ContextShowRowData) ContextShowModel {
	rows := make([]table.Row, len(data))
	for i, row := range data {
		rows[i] = makeContextShowRow(row)
	}

	return ContextShowModel{
		table: table.New([]table.Column{
			table.NewColumn(columnKeyName, "Environment Variable", 25),
			table.NewColumn(columnKeyValue, "Value", 50),
		}).WithRows(rows).
			Border(customBorder).
			WithBaseStyle(styleBase).
			SortByDesc(columnKeyCreatedAt),
	}
}

func (m ContextShowModel) View() string {
	view := lipgloss.JoinVertical(
		lipgloss.Left,
		m.table.View(),
	) + "\n"

	return lipgloss.NewStyle().MarginLeft(1).Render(view)
}
