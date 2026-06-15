// Copyright (c) 2026 Circle Internet Services, Inc.
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.
//
// SPDX-License-Identifier: MIT

package ui

import (
	"fmt"

	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/CircleCI-Public/circleci-cli/internal/ui/components"
	"github.com/CircleCI-Public/circleci-cli/internal/ui/theme"
)

// MarkdownViewportModel is a full-screen pager for rendered markdown. It puts
// the content in a scrollable viewport with a footer showing scroll position
// and key hints, modelled on the bubbletea glamour example.
//
// Content is produced by a render callback so the markdown can be re-wrapped to
// the live terminal width whenever the window is resized.
type MarkdownViewportModel struct {
	viewport viewport.Model
	// render returns the markdown rendered (and word-wrapped) to fit width
	// columns. It is called on the first window-size message and on every
	// resize thereafter.
	render func(width int) string
	ready  bool
}

// MarkdownViewportFooterHeight is the number of rows reserved below the
// viewport for the footer (one blank separator row + the help line). Callers
// use it to decide whether content fits on one screen.
const MarkdownViewportFooterHeight = 2

// NewMarkdownViewportModel returns a pager that displays the markdown produced
// by render. render is given the column width the content must fit into.
func NewMarkdownViewportModel(render func(width int) string) MarkdownViewportModel {
	return MarkdownViewportModel{render: render}
}

func (m MarkdownViewportModel) Init() tea.Cmd { return nil }

func (m MarkdownViewportModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		height := msg.Height - MarkdownViewportFooterHeight
		if height < 1 {
			height = 1
		}
		if !m.ready {
			m.viewport = viewport.New(
				viewport.WithWidth(msg.Width),
				viewport.WithHeight(height),
			)
			m.ready = true
		} else {
			m.viewport.SetWidth(msg.Width)
			m.viewport.SetHeight(height)
		}
		m.viewport.SetContent(m.render(msg.Width))
		return m, nil

	case tea.KeyPressMsg:
		switch msg.String() {
		case "q", components.KeyEsc, components.KeyCtrlC:
			return m, tea.Quit
		case "g", "home":
			m.viewport.GotoTop()
			return m, nil
		case "G", "end":
			m.viewport.GotoBottom()
			return m, nil
		}
	}

	var cmd tea.Cmd
	m.viewport, cmd = m.viewport.Update(msg)
	return m, cmd
}

func (m MarkdownViewportModel) View() tea.View {
	if !m.ready {
		return tea.NewView("")
	}
	body := lipgloss.JoinVertical(lipgloss.Left, m.viewport.View(), m.footer())
	v := tea.NewView(body)
	v.AltScreen = true
	return v
}

func (m MarkdownViewportModel) footer() string {
	hint := "↑/↓ scroll · f/b page · g/G top/bottom · q quit"
	pct := fmt.Sprintf("%3.0f%%", m.viewport.ScrollPercent()*100)
	return "\n" + theme.HelperStyle.Render(hint+"  "+pct)
}
