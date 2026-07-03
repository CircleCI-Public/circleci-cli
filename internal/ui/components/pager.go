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
	"fmt"
	"image/color"

	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/CircleCI-Public/circleci-cli/internal/ui/theme"
)

// PagerFooterHeight is the number of rows PagerModel reserves below the viewport
// for its footer (one blank separator row + the help/status line). Callers use it
// to decide whether content fits on one screen without paging.
const PagerFooterHeight = 2

// PagerModel is a scrollable full-screen pager with less-style search built in.
// It wraps a viewport, owns the search engine, and renders a footer showing the
// scroll position, key hints and search state. It handles scrolling (↑/↓, page
// keys via the viewport), jump-to-top/bottom (g/G, home/end) and the whole "/"
// search interaction (typing, up/down to recall recent patterns, n/N navigation,
// match highlighting, scroll-to-match).
//
// It deliberately does NOT handle the lifecycle keys (q, esc, ctrl+c): those mean
// different things to different hosts — quit here, go back to a picker there — so
// the embedding model owns them. The typical host loop is:
//
//	if !pager.Searching() {
//	    switch key {
//	    case "ctrl+c", "q": quit
//	    case "esc":
//	        if pager.SearchActive() { pager = pager.ClearSearch(); return }
//	        quit / go back
//	    }
//	}
//	pager, cmd = pager.Update(msg)
//
// Guarding on Searching() first is important: while the "/" prompt is open those
// keys are text to be typed (or the prompt's own cancel/commit), not lifecycle
// actions, so they must fall through to Update.
//
// Build with NewPager; the zero value is not usable. Content is supplied with
// SetContent / SetContentFollowingTail, or recomputed per width by a WithReflow
// callback on resize.
type PagerModel struct {
	vp     viewport.Model
	search searchState

	// content is the current raw (pre-highlight) content. It is retained so search
	// highlights can be re-applied, and so a resize before the first content is set
	// still renders once a size is known.
	content string
	// reflow, when set, recomputes content for a given width on resize (e.g.
	// re-wrapping markdown). When nil, content is left as last set and the viewport
	// soft-wraps it to the new width.
	reflow func(width int) string
	// hint is the key-hint text shown at the right of the footer.
	hint string

	// border, when set (bordered), frames the viewport. It is applied as the
	// viewport's Style, so the viewport reserves the frame from its own dimensions
	// and the content is re-wrapped to the reduced width. Unset renders flush.
	border   lipgloss.Style
	bordered bool

	ready  bool
	width  int
	height int
}

// NewPager returns an empty pager. Chain WithHint / WithReflow / WithContent to
// configure it before use.
func NewPager() PagerModel {
	return PagerModel{search: newSearchState()}
}

// WithHint sets the footer key-hint text (e.g. "↑/↓ scroll · / search · q quit").
func (m PagerModel) WithHint(hint string) PagerModel {
	m.hint = hint
	return m
}

// WithBorder frames the viewport in a rounded border of the given color, offset
// from the text by a right pad. Use it to lift an overlay (e.g. the help view)
// off the content behind it. The border is drawn inside the pager's width, so
// the content re-wraps to the reduced interior — callers pass the full terminal
// size as usual. Unset (the default) renders flush, with no border.
func (m PagerModel) WithBorder(c color.Color) PagerModel {
	m.border = lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(c).
		PaddingRight(2)
	m.bordered = true
	return m
}

// WithReflow installs a callback that recomputes the content for a given width,
// invoked on every resize. Use it for content that must be re-wrapped to the
// terminal width (rendered markdown). Content that should merely soft-wrap needs
// no reflow; set it with SetContent instead.
func (m PagerModel) WithReflow(reflow func(width int) string) PagerModel {
	m.reflow = reflow
	return m
}

// WithContent seeds the initial content. It is rendered once a size is known.
func (m PagerModel) WithContent(raw string) PagerModel {
	m.content = raw
	return m
}

// Ready reports whether a terminal size has been seen, so the pager can render.
func (m PagerModel) Ready() bool { return m.ready }

// Searching reports whether the "/" input prompt is currently active.
func (m PagerModel) Searching() bool { return m.search.searching }

// SearchActive reports whether a committed search is present to dismiss (a query
// with matches, or a not-found notice). Hosts use it to decide whether Esc should
// clear the search or fall through to their own quit/back action.
func (m PagerModel) SearchActive() bool { return m.search.active() }

// ScrollPercent is the viewport's scroll position in the range [0, 1].
func (m PagerModel) ScrollPercent() float64 { return m.vp.ScrollPercent() }

// SetSize applies a terminal size, sizing the viewport (reserving the footer),
// re-wrapping via the reflow callback when set, and re-applying search
// highlights. It is equivalent to feeding the pager a tea.WindowSizeMsg.
func (m PagerModel) SetSize(width, height int) PagerModel {
	m.resize(width, height)
	return m
}

// SetContent replaces the paged content, preserving the scroll position, and
// re-applies any active search highlight.
func (m PagerModel) SetContent(raw string) PagerModel {
	m.content = raw
	m.applyContent()
	return m
}

// SetContentFollowingTail is like SetContent but keeps the view pinned to the
// bottom when it was already there, so streamed output that a reader is watching
// live keeps scrolling while a reader who has scrolled up to read is left alone.
func (m PagerModel) SetContentFollowingTail(raw string) PagerModel {
	atBottom := m.ready && m.vp.AtBottom()
	m.content = raw
	m.applyContent()
	if atBottom && m.ready {
		m.vp.GotoBottom()
	}
	return m
}

// ResetSearch clears the committed search (query, matches, prompt) and re-applies
// plain content, while preserving the in-memory recall history. Use it when
// opening fresh content that should start unsearched but still offer previous
// patterns for recall at the "/" prompt.
func (m PagerModel) ResetSearch() PagerModel {
	m.search = m.search.reset()
	m.applyContent()
	return m
}

// ClearSearch dismisses the committed search, dropping its highlights, and leaves
// the viewport scrolled where it is.
func (m PagerModel) ClearSearch() PagerModel {
	m.search.clear(m.content, &m.vp)
	return m
}

// GotoTop scrolls to the top of the content.
func (m PagerModel) GotoTop() PagerModel {
	if m.ready {
		m.vp.GotoTop()
	}
	return m
}

// GotoBottom scrolls to the bottom of the content.
func (m PagerModel) GotoBottom() PagerModel {
	if m.ready {
		m.vp.GotoBottom()
	}
	return m
}

func (m PagerModel) Init() tea.Cmd { return nil }

// Update handles scrolling, jump keys and the whole "/" search interaction. It
// leaves q/esc/ctrl+c untouched (except while the search prompt is open, where
// esc/enter cancel/commit the pattern) so the host can bind them. See PagerModel.
func (m PagerModel) Update(msg tea.Msg) (PagerModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.resize(msg.Width, msg.Height)
		return m, nil
	case tea.KeyPressMsg:
		if m.search.searching {
			if committed := m.search.inputKey(msg); committed {
				m.search.run(m.content, &m.vp)
			}
			return m, nil
		}
		switch msg.String() {
		case KeySlash:
			m.search.begin()
			return m, nil
		case KeyN:
			m.search.next(m.content, &m.vp)
			return m, nil
		case KeyShiftN:
			m.search.prev(m.content, &m.vp)
			return m, nil
		case KeyG, KeyHome:
			m.vp.GotoTop()
			return m, nil
		case KeyShiftG, KeyEnd:
			m.vp.GotoBottom()
			return m, nil
		}
	}
	var cmd tea.Cmd
	m.vp, cmd = m.vp.Update(msg)
	return m, cmd
}

// View renders the viewport above the footer, on the alternate screen. status is
// an optional, already-styled segment shown at the left of the footer before the
// search state (e.g. a "streaming…" indicator); pass "" for none. It returns an
// empty view until a terminal size is known.
func (m PagerModel) View(status string) tea.View {
	if !m.ready {
		return tea.NewView("")
	}
	body := lipgloss.JoinVertical(lipgloss.Left, m.vp.View(), m.footer(status))
	v := tea.NewView(body)
	v.AltScreen = true
	return v
}

// footer renders the bottom line: the live "/" prompt while typing, otherwise the
// caller's status segment, the committed-search state, and the key hints with the
// scroll percentage.
func (m PagerModel) footer(status string) string {
	if m.search.searching {
		return "\n" + m.search.promptText()
	}
	pct := fmt.Sprintf("%3.0f%%", m.vp.ScrollPercent()*100)
	return "\n" + status + m.search.statusText() + theme.HelperStyle.Render(m.hint+"  "+pct)
}

// resize sizes the viewport to the terminal (reserving the footer), re-wraps the
// content via reflow when set, and re-applies search highlights.
func (m *PagerModel) resize(width, height int) {
	m.width, m.height = width, height
	vpHeight := height - PagerFooterHeight
	if vpHeight < 1 {
		vpHeight = 1
	}
	if !m.ready {
		m.vp = viewport.New(viewport.WithWidth(width), viewport.WithHeight(vpHeight))
		m.ready = true
	} else {
		m.vp.SetWidth(width)
		m.vp.SetHeight(vpHeight)
	}
	// A border is the viewport's own Style: it reserves the frame from the width
	// and height set above, so the interior — what content must wrap to — is
	// narrower. Reflow to that interior width, not the full terminal width.
	contentWidth := width
	if m.bordered {
		m.vp.Style = m.border
		if fw := m.border.GetHorizontalFrameSize(); fw < width {
			contentWidth = width - fw
		}
	}
	if m.reflow != nil {
		m.content = m.reflow(contentWidth)
	}
	m.search.reapply(m.content, &m.vp)
}

// applyContent re-renders the viewport from content with search highlights, once
// a size is known. Before the first resize it is a no-op; the content is retained
// and rendered by the first resize.
func (m *PagerModel) applyContent() {
	if !m.ready {
		return
	}
	m.search.reapply(m.content, &m.vp)
}
