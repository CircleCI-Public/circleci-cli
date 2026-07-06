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

package components

import (
	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/CircleCI-Public/circleci-cli/internal/ui/theme"
)

// Tab-switching bindings: left/h/shift+tab select the previous tab, right/l/tab
// the next (both wrap). They are internal to the Tabs component so a host does
// not have to re-declare them.
var (
	tabKeyPrev = key.NewBinding(key.WithKeys("left", "h", "shift+tab"))
	tabKeyNext = key.NewBinding(key.WithKeys("right", "l", "tab"))
)

// Tab borders, styled after the lipgloss "layout" example: an active tab opens at
// the bottom (a blank Bottom edge) so it reads as continuous with the window body
// below it, while an inactive tab is a closed box whose bottom line forms part of
// the window's top border. The first/last tab's bottom outer corner is patched at
// render time so the tab row seams into the window's side borders (see View).
var (
	activeTabBorder = lipgloss.Border{
		Top: "─", Bottom: " ", Left: "│", Right: "│",
		TopLeft: "╭", TopRight: "╮", BottomLeft: "┘", BottomRight: "└",
	}
	inactiveTabBorder = lipgloss.Border{
		Top: "─", Bottom: "─", Left: "│", Right: "│",
		TopLeft: "╭", TopRight: "╮", BottomLeft: "┴", BottomRight: "┴",
	}
)

// Tabs is a horizontal tab bar seamed into a rounded-bordered window: the active
// tab's label reads as continuous with the body below it. It owns the active-tab
// index and the switch keys (←/→, tab/shift+tab); the host supplies the labels
// and, per frame, the active tab's body content (Tabs draws only the chrome, not
// the body — the host decides what each tab shows). Enter/esc and body navigation
// are left to the host.
type Tabs struct {
	labels []string
	active int
	color  bool
}

// NewTabs builds a tab bar over labels with the first tab active.
func NewTabs(labels []string, color bool) Tabs {
	return Tabs{labels: labels, color: color}
}

// Active returns the index of the active tab.
func (t Tabs) Active() int { return t.active }

// Count returns the number of tabs.
func (t Tabs) Count() int { return len(t.labels) }

// SetActive returns a copy with tab i active (wrapped into range).
func (t Tabs) SetActive(i int) Tabs {
	if n := len(t.labels); n > 0 {
		t.active = ((i % n) + n) % n
	}
	return t
}

// Next / Prev return a copy with the following / preceding tab active, wrapping
// at the ends.
func (t Tabs) Next() Tabs { return t.SetActive(t.active + 1) }
func (t Tabs) Prev() Tabs { return t.SetActive(t.active - 1) }

// Update switches the active tab on a tab-switch key (←/→, tab/shift+tab). The
// returned bool reports whether the key was consumed, so the host can forward
// everything else to the active tab's body.
func (t Tabs) Update(msg tea.Msg) (Tabs, bool) {
	k, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return t, false
	}
	switch {
	case key.Matches(k, tabKeyNext):
		return t.Next(), true
	case key.Matches(k, tabKeyPrev):
		return t.Prev(), true
	}
	return t, false
}

// View renders the tab row over a rounded-bordered window wrapping body: the tab
// row supplies the window's top edge, and the row is split to the window's width
// so the two seam together. The window is sized to the body's width.
func (t Tabs) View(body string) string {
	contentWidth := lipgloss.Width(body)
	// rowWidth is the outer window width. lipgloss .Width() sets the outer box
	// width (border + padding included), so the window is body + border(2) +
	// padding(2); the tab row is set to the same width so they line up. Round up so
	// the width splits evenly across the tabs (symmetric cells).
	rowWidth := contentWidth + 4
	if n := len(t.labels); n > 0 {
		if r := rowWidth % n; r != 0 {
			rowWidth += n - r
		}
	}

	window := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderTop(false). // the tab row supplies the top edge
		Padding(0, 1).
		Width(rowWidth)
	if t.color {
		window = window.BorderForeground(theme.ColorSecondary)
	}

	return lipgloss.JoinVertical(lipgloss.Left, t.renderRow(rowWidth), window.Render(body))
}

// renderRow draws the tab row at rowWidth, split evenly across the tabs. The
// active tab uses the open-bottom border so it reads as continuous with the
// window and its label is pink; the outer bottom corners of the first and last
// tab are patched so the row seams into the window's side borders below, while
// middle tabs keep their default corners.
func (t Tabs) renderRow(rowWidth int) string {
	n := len(t.labels)
	widths := splitWidth(rowWidth, n)

	cells := make([]string, n)
	for i, label := range t.labels {
		active := i == t.active
		border := inactiveTabBorder
		if active {
			border = activeTabBorder
		}
		switch i {
		case 0: // first tab: bottom-left seams into the window's left border
			if active {
				border.BottomLeft = "│"
			} else {
				border.BottomLeft = "├"
			}
		case n - 1: // last tab: bottom-right seams into the window's right border
			if active {
				border.BottomRight = "│"
			} else {
				border.BottomRight = "┤"
			}
		}

		st := lipgloss.NewStyle().Border(border, true).Padding(0, 1).
			Align(lipgloss.Center).Width(widths[i])
		switch {
		case t.color && active:
			st = st.BorderForeground(theme.ColorSecondary).Foreground(theme.ColorAccent).Bold(true)
		case t.color:
			st = st.BorderForeground(theme.ColorSecondary).Foreground(theme.ColorMuted)
		case active:
			st = st.Bold(true)
		}
		cells[i] = st.Render(label)
	}
	return lipgloss.JoinHorizontal(lipgloss.Bottom, cells...)
}

// splitWidth divides total into n parts as evenly as possible, handing the
// remainder to the leading parts, so the parts sum back to total.
func splitWidth(total, n int) []int {
	if n <= 0 {
		return nil
	}
	base := total / n
	out := make([]int, n)
	for i := range out {
		out[i] = base
	}
	for i := 0; i < total-base*n; i++ {
		out[i]++
	}
	return out
}
