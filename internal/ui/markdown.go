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
	"regexp"
	"strings"

	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

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

	// base is the rendered (and colored) markdown for the current width, before
	// any search highlighting is applied. Search rebuilds the viewport content
	// from this so highlights can be re-applied or cleared without re-rendering.
	base string

	// Search state, modelled on less. searching is true while the user is typing
	// a pattern at the "/" prompt; input holds the in-progress pattern. query is
	// the last committed pattern (used to repeat the search and drive n/N). matches
	// are every hit in document order; current indexes the focused one (-1 when
	// there are none). notFound records that the last committed pattern matched
	// nothing, for the footer.
	searching bool
	input     string
	query     string
	matches   []searchMatch
	current   int
	notFound  bool
}

// searchMatch locates a single hit: the logical line it sits on (index into the
// base content split by "\n") and the visible column range it spans. Columns are
// terminal cell positions, which is what lipgloss.StyleRanges expects.
type searchMatch struct {
	line     int
	colStart int
	colEnd   int
}

// MarkdownViewportFooterHeight is the number of rows reserved below the
// viewport for the footer (one blank separator row + the help line). Callers
// use it to decide whether content fits on one screen.
const MarkdownViewportFooterHeight = 2

// NewMarkdownViewportModel returns a pager that displays the markdown produced
// by render. render is given the column width the content must fit into.
func NewMarkdownViewportModel(render func(width int) string) MarkdownViewportModel {
	return MarkdownViewportModel{render: render, current: -1}
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
		// Re-render to the new width, then recompute matches against the
		// re-wrapped content so highlights and positions stay correct.
		m.base = m.render(msg.Width)
		m.recomputeMatches()
		m.refreshContent()
		return m, nil

	case tea.KeyPressMsg:
		if m.searching {
			return m.updateSearchInput(msg)
		}
		switch msg.String() {
		case components.KeyQ, components.KeyCtrlC:
			return m, tea.Quit
		case components.KeyEsc:
			// Esc clears an active search (dropping its highlights) and only
			// quits when there is no search to dismiss.
			if m.query != "" || m.notFound {
				m.clearSearch()
				return m, nil
			}
			return m, tea.Quit
		case components.KeyG, components.KeyHome:
			m.viewport.GotoTop()
			return m, nil
		case components.KeyShiftG, components.KeyEnd:
			m.viewport.GotoBottom()
			return m, nil
		case components.KeySlash:
			m.searching = true
			m.input = ""
			m.notFound = false
			return m, nil
		case components.KeyN:
			m.focusMatch(m.current + 1)
			return m, nil
		case components.KeyShiftN:
			m.focusMatch(m.current - 1)
			return m, nil
		}
	}

	var cmd tea.Cmd
	m.viewport, cmd = m.viewport.Update(msg)
	return m, cmd
}

// updateSearchInput handles key presses while the "/" search prompt is active:
// editing the pattern, committing it with Enter, or cancelling with Esc.
func (m MarkdownViewportModel) updateSearchInput(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case components.KeyEnter:
		m.searching = false
		// An empty pattern repeats the previous search, matching less.
		if m.input != "" {
			m.query = m.input
		}
		m.input = ""
		m.runSearch()
		return m, nil
	case components.KeyEsc, components.KeyCtrlC:
		m.searching = false
		m.input = ""
		return m, nil
	case components.KeyBackspace:
		if r := []rune(m.input); len(r) > 0 {
			m.input = string(r[:len(r)-1])
		}
		return m, nil
	}
	// Append printable characters. msg.Text is empty for non-printable keys.
	m.input += msg.Text
	return m, nil
}

// runSearch (re)evaluates the committed query, focuses the first match at or
// after the current scroll position (wrapping to the top otherwise), and redraws
// with highlights. Used when a pattern is freshly committed.
func (m *MarkdownViewportModel) runSearch() {
	m.recomputeMatches()
	m.notFound = m.query != "" && len(m.matches) == 0
	if len(m.matches) == 0 {
		m.current = -1
		m.refreshContent()
		return
	}
	m.current = m.firstMatchFrom(m.viewport.YOffset())
	m.refreshContent()
	m.scrollToCurrent()
}

// clearSearch dismisses the current search: it drops the query, matches, and
// highlights, leaving the pager scrolled where it is.
func (m *MarkdownViewportModel) clearSearch() {
	m.query = ""
	m.matches = nil
	m.current = -1
	m.notFound = false
	m.refreshContent()
}

// recomputeMatches finds every match of the committed query in the current base
// content. It does not touch the focused match or the viewport.
func (m *MarkdownViewportModel) recomputeMatches() {
	m.matches = searchMatches(m.base, m.query)
	if m.current >= len(m.matches) {
		m.current = len(m.matches) - 1
	}
}

// focusMatch moves the focus to match i (wrapping around the ends, less-style),
// redraws so the new selection is styled, and scrolls it into view.
func (m *MarkdownViewportModel) focusMatch(i int) {
	if len(m.matches) == 0 {
		return
	}
	m.current = (i%len(m.matches) + len(m.matches)) % len(m.matches)
	m.refreshContent()
	m.scrollToCurrent()
}

// firstMatchFrom returns the index of the first match on or after line yoff,
// wrapping to the first match when none are below.
func (m MarkdownViewportModel) firstMatchFrom(yoff int) int {
	for i, mt := range m.matches {
		if mt.line >= yoff {
			return i
		}
	}
	return 0
}

// scrollToCurrent brings the focused match into view, leaving the offset alone
// when it is already on screen so navigation between visible matches doesn't
// jump the page around.
func (m *MarkdownViewportModel) scrollToCurrent() {
	if m.current < 0 || m.current >= len(m.matches) {
		return
	}
	line := m.matches[m.current].line
	if line < m.viewport.YOffset() || line >= m.viewport.YOffset()+m.viewport.Height() {
		m.viewport.SetYOffset(line)
	}
}

// refreshContent rebuilds the viewport content from base, styling every match
// (the focused one distinctly) so highlights survive scrolling and resizes.
func (m *MarkdownViewportModel) refreshContent() {
	m.viewport.SetContent(styleMatches(m.base, m.matches, m.current))
}

// styleMatches returns content with each match highlighted. The match at index
// current uses the selected style; the rest use the plain match style. It styles
// the original (colored) lines in place via lipgloss.StyleRanges, so surrounding
// markdown styling is preserved.
func styleMatches(content string, matches []searchMatch, current int) string {
	if len(matches) == 0 {
		return content
	}
	byLine := make(map[int][]lipgloss.Range, len(matches))
	for i, mt := range matches {
		style := theme.SearchMatchStyle
		if i == current {
			style = theme.SearchSelectedStyle
		}
		byLine[mt.line] = append(byLine[mt.line], lipgloss.NewRange(mt.colStart, mt.colEnd, style))
	}
	lines := strings.Split(content, "\n")
	for i, ranges := range byLine {
		if i >= 0 && i < len(lines) {
			lines[i] = lipgloss.StyleRanges(lines[i], ranges...)
		}
	}
	return strings.Join(lines, "\n")
}

// searchMatches finds every match of query in content. Each line is searched on
// its ANSI-stripped text so colors don't perturb positions, and match columns
// are returned as terminal cell offsets (what lipgloss.StyleRanges expects). The
// query is treated as a regular expression (like less), falling back to a literal
// search when it is not valid regex. Matching is case-insensitive unless the
// pattern contains an uppercase letter (smart case).
func searchMatches(content, query string) []searchMatch {
	if query == "" {
		return nil
	}
	re := compileSearch(query)
	if re == nil {
		return nil
	}
	var out []searchMatch
	for i, line := range strings.Split(content, "\n") {
		plain := ansi.Strip(line)
		for _, loc := range re.FindAllStringIndex(plain, -1) {
			colStart := ansi.StringWidth(plain[:loc[0]])
			colEnd := ansi.StringWidth(plain[:loc[1]])
			if colEnd > colStart {
				out = append(out, searchMatch{line: i, colStart: colStart, colEnd: colEnd})
			}
		}
	}
	return out
}

func compileSearch(query string) *regexp.Regexp {
	// (?i) gives smart-case: case-insensitive unless the pattern has an uppercase
	// letter. Each line is matched separately, so ^ and $ already anchor per line.
	prefix := ""
	if !strings.ContainsAny(query, "ABCDEFGHIJKLMNOPQRSTUVWXYZ") {
		prefix = "(?i)"
	}
	if re, err := regexp.Compile(prefix + query); err == nil {
		return re
	}
	// Not valid regex — fall back to a literal substring search.
	re, err := regexp.Compile(prefix + regexp.QuoteMeta(query))
	if err != nil {
		return nil
	}
	return re
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
	// While typing a pattern, show the live "/" prompt in place of the hints.
	if m.searching {
		return "\n" + theme.HelperStyle.Render("/"+m.input) + theme.AccentStyle.Render("▌")
	}

	hint := "↑/↓ scroll · f/b page · g/G top/bottom · / search · q quit"
	pct := fmt.Sprintf("%3.0f%%", m.viewport.ScrollPercent()*100)

	var status string
	switch {
	case m.notFound:
		status = theme.WarningStyle.Render("pattern not found: "+m.query) + "  "
	case len(m.matches) > 0:
		status = theme.AccentStyle.Render("/"+m.query) +
			theme.HelperStyle.Render(fmt.Sprintf(" %d/%d · n/N next/prev  ", m.current+1, len(m.matches)))
	}

	return "\n" + status + theme.HelperStyle.Render(hint+"  "+pct)
}
