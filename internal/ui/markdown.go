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
	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"

	"github.com/CircleCI-Public/circleci-cli/internal/ui/components"
)

// MarkdownViewportModel is a full-screen pager for rendered markdown. It is a
// thin host around components.PagerModel (which provides the scrollable viewport,
// less-style "/" search and footer), adding the markdown-specific bindings: q and
// esc quit, and esc first dismisses an active search.
//
// Content is produced by a render callback so the markdown can be re-wrapped to
// the live terminal width whenever the window is resized.
type MarkdownViewportModel struct {
	pager components.PagerModel
}

// markdownPagerKeys is the footer key hint set for the markdown pager.
var markdownPagerKeys = []key.Binding{
	components.BindScroll,
	components.BindPage,
	components.BindTopBottom,
	components.BindSearch,
	components.BindQuit,
}

// MarkdownViewportFooterHeight is the number of rows reserved below the
// viewport for the footer (one blank separator row + the help line). Callers
// use it to decide whether content fits on one screen.
const MarkdownViewportFooterHeight = components.PagerFooterHeight

// NewMarkdownViewportModel returns a pager that displays the markdown produced
// by render. render is given the column width the content must fit into and is
// re-invoked on every resize so the markdown re-wraps.
func NewMarkdownViewportModel(render func(width int) string) MarkdownViewportModel {
	return MarkdownViewportModel{
		pager: components.NewPager().WithKeys(markdownPagerKeys...).WithReflow(render),
	}
}

func (m MarkdownViewportModel) Init() tea.Cmd { return nil }

func (m MarkdownViewportModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	// The lifecycle keys (q/esc) are the host's to bind; guard on Searching so they
	// are typed into the "/" prompt rather than acted on while it is open.
	if k, ok := msg.(tea.KeyPressMsg); ok && !m.pager.Searching() {
		switch {
		case key.Matches(k, components.BindQuit, components.KeyCtrlC):
			return m, tea.Quit
		case key.Matches(k, components.KeyEsc):
			// Esc clears an active search (dropping its highlights) and only quits
			// when there is no search to dismiss.
			if m.pager.SearchActive() {
				m.pager = m.pager.ClearSearch()
				return m, nil
			}
			return m, tea.Quit
		}
	}

	var cmd tea.Cmd
	m.pager, cmd = m.pager.Update(msg)
	return m, cmd
}

func (m MarkdownViewportModel) View() tea.View {
	return m.pager.View("")
}
