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
	"regexp"
	"strings"

	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/CircleCI-Public/circleci-cli/internal/ui/theme"
)

// searchState is the less-style incremental-search engine that backs PagerModel.
// It owns the search state — the in-progress "/" prompt, the committed pattern,
// the set of matches and which one is focused — and drives the viewport it is
// handed: applying highlights, moving the focus with n/N, and scrolling the
// focused match into view. It is content-agnostic; the pager passes the current
// (rendered) content into each operation.
//
// The zero value is not ready; build one with newSearchState so the focus starts
// at -1 (no match).
type searchState struct {
	// searching is true while the user is typing a pattern at the "/" prompt;
	// input holds the in-progress pattern. query is the last committed pattern
	// (used to repeat the search and drive n/N). matches are every hit in
	// document order; current indexes the focused one (-1 when there are none).
	// notFound records that the last committed pattern matched nothing, for the
	// footer.
	searching bool
	input     string
	query     string
	matches   []searchMatch
	current   int
	notFound  bool

	// history is the list of committed patterns, oldest first, recalled at the
	// prompt with up/down like less. histIdx is the recall cursor while the prompt
	// is open: it indexes history, and equals len(history) when sitting on the
	// live (un-recalled) input. draft stashes that live input while the user pages
	// back through history, so paging forward past the newest entry restores it.
	// History is in-memory only and is deliberately preserved across reset so it
	// survives moving between steps in a single session.
	history []string
	histIdx int
	draft   string
}

// searchMatch locates a single hit: the logical line it sits on (index into the
// content split by "\n") and the visible column range it spans. Columns are
// terminal cell positions, which is what lipgloss.StyleRanges expects.
type searchMatch struct {
	line     int
	colStart int
	colEnd   int
}

func newSearchState() searchState { return searchState{current: -1} }

// reset returns a fresh search state that keeps the recall history but drops the
// committed query, matches and prompt. The pager uses it when opening new content
// that should start unsearched while still offering previous patterns for recall.
func (s searchState) reset() searchState {
	return searchState{current: -1, history: s.history}
}

// active reports whether there is a committed search to dismiss — a query with
// matches, or a not-found notice.
func (s searchState) active() bool { return s.query != "" || s.notFound }

// begin opens a fresh "/" prompt, clearing any in-progress input and not-found
// notice (the committed query is kept so it can be repeated with a bare Enter).
// The recall cursor is parked on the live input, past the newest history entry.
func (s *searchState) begin() {
	s.searching = true
	s.input = ""
	s.notFound = false
	s.histIdx = len(s.history)
	s.draft = ""
}

// inputKey processes a key press while the "/" prompt is active: editing the
// pattern, cancelling with Esc, or committing with Enter. It returns true when
// Enter committed a pattern, so the caller should follow with run.
func (s *searchState) inputKey(msg tea.KeyPressMsg) (committed bool) {
	switch msg.String() {
	case KeyEnter:
		s.searching = false
		// An empty pattern repeats the previous search, matching less.
		if s.input != "" {
			s.query = s.input
			s.pushHistory(s.input)
		}
		s.input = ""
		return true
	case KeyEsc, KeyCtrlC:
		s.searching = false
		s.input = ""
		return false
	case KeyBackspace:
		if r := []rune(s.input); len(r) > 0 {
			s.input = string(r[:len(r)-1])
		}
		return false
	case KeyUp:
		s.recallOlder()
		return false
	case KeyDown:
		s.recallNewer()
		return false
	}
	// Append printable characters. msg.Text is empty for non-printable keys.
	s.input += msg.Text
	return false
}

// pushHistory records a committed pattern as the most recent entry. An existing
// identical entry is first removed so a repeated or recalled pattern moves to the
// newest slot (like less) rather than duplicating.
func (s *searchState) pushHistory(pattern string) {
	for i, p := range s.history {
		if p == pattern {
			s.history = append(s.history[:i], s.history[i+1:]...)
			break
		}
	}
	s.history = append(s.history, pattern)
}

// recallOlder replaces the prompt input with the previous history entry (up).
// The first step back stashes the live draft so recallNewer can restore it.
func (s *searchState) recallOlder() {
	if s.histIdx == 0 {
		return // already at the oldest entry
	}
	if s.histIdx == len(s.history) {
		s.draft = s.input
	}
	s.histIdx--
	s.input = s.history[s.histIdx]
}

// recallNewer moves toward more recent history entries (down), restoring the
// stashed live draft once it pages past the newest entry.
func (s *searchState) recallNewer() {
	if s.histIdx >= len(s.history) {
		return // already on the live input
	}
	s.histIdx++
	if s.histIdx == len(s.history) {
		s.input = s.draft
		return
	}
	s.input = s.history[s.histIdx]
}

// run (re)evaluates the committed query against content, focuses the first match
// at or after the viewport's current scroll position (wrapping to the top
// otherwise), applies highlights and scrolls it into view. Used right after a
// pattern is committed via inputKey.
func (s *searchState) run(content string, vp *viewport.Model) {
	s.matches = searchMatches(content, s.query)
	s.notFound = s.query != "" && len(s.matches) == 0
	if len(s.matches) == 0 {
		s.current = -1
		vp.SetContent(s.highlight(content))
		return
	}
	s.current = s.firstMatchFrom(vp.YOffset())
	vp.SetContent(s.highlight(content))
	s.scrollToCurrent(vp)
}

// reapply recomputes matches for the committed query against content and
// re-applies highlights to the viewport without moving the focus or scrolling.
// Used when the content changed underneath an active search — a resize that
// re-wraps it, or fresh streamed output.
func (s *searchState) reapply(content string, vp *viewport.Model) {
	s.matches = searchMatches(content, s.query)
	if s.current >= len(s.matches) {
		s.current = len(s.matches) - 1
	}
	vp.SetContent(s.highlight(content))
}

// clear dismisses the current search: it drops the query, matches and
// highlights, leaving the viewport scrolled where it is.
func (s *searchState) clear(content string, vp *viewport.Model) {
	s.query = ""
	s.matches = nil
	s.current = -1
	s.notFound = false
	vp.SetContent(s.highlight(content))
}

// next focuses the following match (wrapping past the end, less-style),
// re-applies highlights and scrolls it into view.
func (s *searchState) next(content string, vp *viewport.Model) {
	s.focusMatch(s.current+1, content, vp)
}

// prev focuses the preceding match (wrapping past the start), re-applies
// highlights and scrolls it into view.
func (s *searchState) prev(content string, vp *viewport.Model) {
	s.focusMatch(s.current-1, content, vp)
}

// focusMatch moves the focus to match i (wrapping around the ends), redraws so
// the new selection is styled, and scrolls it into view.
func (s *searchState) focusMatch(i int, content string, vp *viewport.Model) {
	if len(s.matches) == 0 {
		return
	}
	s.current = (i%len(s.matches) + len(s.matches)) % len(s.matches)
	vp.SetContent(s.highlight(content))
	s.scrollToCurrent(vp)
}

// firstMatchFrom returns the index of the first match on or after line yoff,
// wrapping to the first match when none are below.
func (s searchState) firstMatchFrom(yoff int) int {
	for i, mt := range s.matches {
		if mt.line >= yoff {
			return i
		}
	}
	return 0
}

// scrollToCurrent brings the focused match into view, leaving the offset alone
// when it is already on screen so navigation between visible matches doesn't
// jump the page around.
func (s searchState) scrollToCurrent(vp *viewport.Model) {
	if s.current < 0 || s.current >= len(s.matches) {
		return
	}
	line := s.matches[s.current].line
	if line < vp.YOffset() || line >= vp.YOffset()+vp.Height() {
		vp.SetYOffset(line)
	}
}

// highlight returns content with each match highlighted: the focused match in
// the selected style, the rest in the plain match style. It styles the original
// (colored) lines in place via lipgloss.StyleRanges, so surrounding styling is
// preserved. Content with no matches is returned unchanged.
func (s searchState) highlight(content string) string {
	if len(s.matches) == 0 {
		return content
	}
	byLine := make(map[int][]lipgloss.Range, len(s.matches))
	for i, mt := range s.matches {
		style := theme.SearchMatchStyle
		if i == s.current {
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

// promptText renders the live "/input▌" prompt shown in the footer while the
// user is typing a pattern. It is meaningful only while searching is true.
func (s searchState) promptText() string {
	return theme.HelperStyle.Render("/"+s.input) + theme.AccentStyle.Render("▌")
}

// statusText renders the footer segment describing the committed search: the
// "pattern not found" warning, or the focused pattern with its "n/m · n/N
// next/prev" counter. It is empty when no search has been committed. The
// returned segment carries its own trailing spacing so it can be concatenated
// directly before the key hints.
func (s searchState) statusText() string {
	switch {
	case s.notFound:
		return theme.WarningStyle.Render("pattern not found: "+s.query) + "  "
	case len(s.matches) > 0:
		return theme.AccentStyle.Render("/"+s.query) +
			theme.HelperStyle.Render(fmt.Sprintf(" %d/%d · n/N next/prev  ", s.current+1, len(s.matches)))
	}
	return ""
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
