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

package components_test

import (
	"bytes"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/charmbracelet/x/exp/teatest/v2"
	"gotest.tools/v3/assert"
	"gotest.tools/v3/assert/cmp"

	"github.com/CircleCI-Public/circleci-cli/internal/ui/components"
)

// pagerHint is used for every test pager so startPager can wait for the first
// frame by matching a stable substring, and so search assertions have known
// surrounding text.
const pagerHint = "/ search · q quit"

// pagerHarness drives a PagerModel as a standalone program in teatest. The pager
// deliberately leaves the lifecycle keys (q/esc/ctrl+c) to its host, so the
// harness binds them exactly as the real hosts do: q/ctrl+c quit, and esc first
// dismisses an active search. The wrapped model stays reachable via m for
// post-run assertions.
type pagerHarness struct {
	m components.PagerModel
}

func (h pagerHarness) Init() tea.Cmd { return h.m.Init() }

func (h pagerHarness) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if k, ok := msg.(tea.KeyPressMsg); ok && !h.m.Searching() {
		switch k.String() {
		case components.KeyCtrlC, components.KeyQ:
			return h, tea.Quit
		case components.KeyEsc:
			if h.m.SearchActive() {
				h.m = h.m.ClearSearch()
				return h, nil
			}
			return h, tea.Quit
		}
	}
	var cmd tea.Cmd
	h.m, cmd = h.m.Update(msg)
	return h, cmd
}

func (h pagerHarness) View() tea.View { return h.m.View("") }

// startPager runs a pager over static content at the given size, using a reflow
// callback (as the markdown host does) so the first WindowSizeMsg loads the
// content, and waits for the first frame.
func startPager(t *testing.T, content string, w, h int) *teatest.TestModel {
	t.Helper()
	m := components.NewPager().WithHint(pagerHint).WithReflow(func(int) string { return content })
	tm := teatest.NewTestModel(t, pagerHarness{m: m}, teatest.WithInitialTermSize(w, h))
	teatest.WaitFor(t, tm.Output(), func(b []byte) bool {
		return bytes.Contains(b, []byte("q quit"))
	}, teatest.WithDuration(time.Second))
	return tm
}

// pagerType sends each rune of s as a key press, mirroring a user typing at the
// "/" search prompt.
func pagerType(tm *teatest.TestModel, s string) {
	for _, r := range s {
		tm.Send(tea.KeyPressMsg{Code: r, Text: string(r)})
	}
}

// pagerSnapshot quits the pager (q) and returns its final, ANSI-stripped frame,
// where the footer renders the current search state. q is safe only outside the
// search prompt (inside it, q is typed into the pattern).
func pagerSnapshot(t *testing.T, tm *teatest.TestModel) string {
	t.Helper()
	tm.Send(tea.KeyPressMsg{Code: 'q', Text: "q"})
	fm := tm.FinalModel(t, teatest.WithFinalTimeout(time.Second)).(pagerHarness)
	return ansi.Strip(fm.m.View("").Content)
}

// pagerSearch opens the prompt, types query, and commits it with Enter.
func pagerSearch(tm *teatest.TestModel, query string) {
	tm.Send(tea.KeyPressMsg{Code: '/', Text: "/"})
	pagerType(tm, query)
	tm.Send(tea.KeyPressMsg{Code: tea.KeyEnter})
}

// TestPagerModel_SearchMatchCount exercises the search matcher through the footer
// counter: a committed pattern focuses its first hit (so it reads "1/<total>"),
// and a missed pattern shows the not-found notice. The cases cover literal
// matching, smart case, per-line regex anchors, the invalid-regex literal
// fallback, and ANSI-insensitive matching.
func TestPagerModel_SearchMatchCount(t *testing.T) {
	tests := []struct {
		name    string
		content string
		query   string
		want    string
	}{
		{"literal substring", "alpha beta\nbeta gamma", "beta", "1/2"},
		{"smart case insensitive", "Alpha\nalpha", "alpha", "1/2"},
		{"smart case sensitive on uppercase", "Alpha\nalpha", "Alpha", "1/1"},
		{"regex anchors per line", "cat\nscatter\ncat", "^cat$", "1/2"},
		{"invalid regex falls back to literal", "a(b\nc", "a(b", "1/1"},
		{"no match shows not found", "hello world", "zzz", "pattern not found: zzz"},
		{"matches ignore ansi styling", "\x1b[31mred\x1b[0m text", "red text", "1/1"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tm := startPager(t, tt.content, 80, 24)
			pagerSearch(tm, tt.query)
			assert.Check(t, cmp.Contains(pagerSnapshot(t, tm), tt.want))
		})
	}
}

// TestPagerModel_SearchNextPrevWrap confirms n advances the focused match and N
// from the first wraps to the last, less-style.
func TestPagerModel_SearchNextPrevWrap(t *testing.T) {
	content := strings.Repeat("needle in a haystack\nfiller line\n", 20)

	t.Run("n advances", func(t *testing.T) {
		tm := startPager(t, content, 80, 24)
		pagerSearch(tm, "needle")
		tm.Send(tea.KeyPressMsg{Code: 'n', Text: "n"})
		assert.Check(t, cmp.Contains(pagerSnapshot(t, tm), "2/20"))
	})

	t.Run("N from the first wraps to the last", func(t *testing.T) {
		tm := startPager(t, content, 80, 24)
		pagerSearch(tm, "needle")
		tm.Send(tea.KeyPressMsg{Code: 'N', Text: "N"})
		assert.Check(t, cmp.Contains(pagerSnapshot(t, tm), "20/20"))
	})
}

// TestPagerModel_EscClearsSearchThenQuits confirms the first esc after a search
// dismisses it (dropping the counter) without quitting, and a later q ends the
// program with the search gone.
func TestPagerModel_EscClearsSearchThenQuits(t *testing.T) {
	tm := startPager(t, strings.Repeat("target\nfiller\n", 30), 80, 24)
	pagerSearch(tm, "target")

	tm.Send(tea.KeyPressMsg{Code: tea.KeyEscape})
	view := pagerSnapshot(t, tm)
	assert.Check(t, !strings.Contains(view, "/target"), "search should have been cleared")
	assert.Check(t, !strings.Contains(view, "1/30"))
}

// TestPagerModel_SearchCancelCommitsNothing confirms esc at the prompt abandons
// the pattern rather than committing it.
func TestPagerModel_SearchCancelCommitsNothing(t *testing.T) {
	tm := startPager(t, strings.Repeat("hello\n", 30), 80, 24)
	tm.Send(tea.KeyPressMsg{Code: '/', Text: "/"})
	pagerType(tm, "hello")
	tm.Send(tea.KeyPressMsg{Code: tea.KeyEscape})

	view := pagerSnapshot(t, tm)
	assert.Check(t, !strings.Contains(view, "/hello"), "cancelled search must not commit")
	assert.Check(t, !strings.Contains(view, "pattern not found"))
}

// TestPagerModel_SearchSurvivesResize confirms a resize re-applies the search
// against the re-wrapped content so the counter is preserved.
func TestPagerModel_SearchSurvivesResize(t *testing.T) {
	tm := startPager(t, strings.Repeat("target\nfiller\n", 30), 80, 24)
	pagerSearch(tm, "target")

	tm.Send(tea.WindowSizeMsg{Width: 60, Height: 20})
	view := pagerSnapshot(t, tm)
	assert.Check(t, cmp.Contains(view, "/target"))
	assert.Check(t, cmp.Contains(view, "1/30"))
}

// TestPagerModel_TailFollow verifies SetContentFollowingTail pins the view to the
// bottom when it was already there (streaming output the reader is watching), but
// leaves the offset alone once the reader has scrolled up.
func TestPagerModel_TailFollow(t *testing.T) {
	content := strings.Repeat("line\n", 100)

	// A fresh, empty viewport counts as "at the bottom", so the first content set
	// sticks to the tail.
	m := components.NewPager().WithHint(pagerHint).SetSize(80, 10)
	m = m.SetContentFollowingTail(content)
	assert.Check(t, cmp.Equal(m.ScrollPercent(), 1.0), "streaming should follow the tail")

	// After scrolling up to read, further output must not yank the view down.
	m = m.GotoTop()
	m = m.SetContentFollowingTail(content + strings.Repeat("more\n", 50))
	assert.Check(t, cmp.Equal(m.ScrollPercent(), 0.0), "a reader who scrolled up should be left alone")
}

// TestPagerModel_SetContentPreservesPosition verifies SetContent (the non-
// streaming setter) leaves the scroll position where it is.
func TestPagerModel_SetContentPreservesPosition(t *testing.T) {
	m := components.NewPager().WithHint(pagerHint).SetSize(80, 10)
	m = m.SetContent(strings.Repeat("line\n", 100)).GotoBottom()
	assert.Check(t, cmp.Equal(m.ScrollPercent(), 1.0))

	m = m.SetContent(strings.Repeat("other\n", 100))
	assert.Check(t, cmp.Equal(m.ScrollPercent(), 1.0), "SetContent should not move the viewport")
}

// TestPagerModel_ResetSearchDropsHighlights confirms ResetSearch clears a
// committed search so freshly opened content starts unsearched.
func TestPagerModel_ResetSearchDropsHighlights(t *testing.T) {
	tm := startPager(t, strings.Repeat("target\nfiller\n", 30), 80, 24)
	pagerSearch(tm, "target")

	// Reset via a resize path is awkward through teatest, so assert on the model
	// directly: after ResetSearch the search is no longer active.
	fm := func() components.PagerModel {
		tm.Send(tea.KeyPressMsg{Code: 'q', Text: "q"})
		return tm.FinalModel(t, teatest.WithFinalTimeout(time.Second)).(pagerHarness).m
	}()
	assert.Check(t, fm.SearchActive(), "precondition: search is active before reset")
	assert.Check(t, !fm.ResetSearch().SearchActive(), "ResetSearch should drop the committed search")
}

// pagerFinalModel quits the pager (q) and returns the underlying PagerModel for
// post-run assertions.
func pagerFinalModel(t *testing.T, tm *teatest.TestModel) components.PagerModel {
	t.Helper()
	tm.Send(tea.KeyPressMsg{Code: 'q', Text: "q"})
	return tm.FinalModel(t, teatest.WithFinalTimeout(time.Second)).(pagerHarness).m
}

// TestPagerModel_SearchHistoryRecall confirms that after committing searches, the
// "/" prompt recalls recent patterns with up/down (most-recent first), like less,
// and that committing a recalled pattern searches for it.
func TestPagerModel_SearchHistoryRecall(t *testing.T) {
	content := strings.Repeat("alpha\nbeta\ngamma\n", 10)
	tm := startPager(t, content, 80, 24)

	// Build a history: alpha (oldest), then beta, then gamma (newest).
	pagerSearch(tm, "alpha")
	pagerSearch(tm, "beta")
	pagerSearch(tm, "gamma")

	// Open a fresh prompt and page back: the first up recalls the newest (gamma),
	// the second recalls beta.
	tm.Send(tea.KeyPressMsg{Code: '/', Text: "/"})
	tm.Send(tea.KeyPressMsg{Code: tea.KeyUp})
	tm.Send(tea.KeyPressMsg{Code: tea.KeyUp})
	// Commit the recalled pattern (beta) and confirm it drove the search.
	tm.Send(tea.KeyPressMsg{Code: tea.KeyEnter})

	view := pagerSnapshot(t, tm)
	assert.Check(t, cmp.Contains(view, "/beta"), "up should recall a recent pattern: %q", view)
	assert.Check(t, cmp.Contains(view, "1/10"))
}

// TestPagerModel_SearchHistoryDownRestoresDraft confirms paging back up then down
// past the newest entry restores the in-progress draft the user had typed. The
// draft "dra" matches nothing, so committing it surfaces the not-found notice —
// proving the draft (not the recalled "alpha") was what got committed.
func TestPagerModel_SearchHistoryDownRestoresDraft(t *testing.T) {
	tm := startPager(t, strings.Repeat("alpha\nbeta\n", 10), 80, 24)
	pagerSearch(tm, "alpha")

	tm.Send(tea.KeyPressMsg{Code: '/', Text: "/"})
	pagerType(tm, "dra")
	tm.Send(tea.KeyPressMsg{Code: tea.KeyUp})    // recall "alpha", stashing "dra"
	tm.Send(tea.KeyPressMsg{Code: tea.KeyDown})  // back to the live draft
	tm.Send(tea.KeyPressMsg{Code: tea.KeyEnter}) // commit whatever is in the prompt

	view := pagerSnapshot(t, tm)
	assert.Check(t, cmp.Contains(view, "pattern not found: dra"),
		"down should restore the typed draft, so committing searches for it: %q", view)
}

// TestPagerModel_SearchHistorySurvivesReset confirms recall history persists
// across ResetSearch (opening fresh content), so a session's earlier patterns
// remain available even after the committed search is cleared.
func TestPagerModel_SearchHistorySurvivesReset(t *testing.T) {
	tm := startPager(t, strings.Repeat("alpha\nbeta\n", 10), 80, 24)
	pagerSearch(tm, "alpha")

	m := pagerFinalModel(t, tm)
	// Reset (as run get does when opening another step), then recall: the earlier
	// pattern should still be reachable via up.
	m = m.ResetSearch()
	assert.Check(t, !m.SearchActive(), "reset should clear the committed search")

	m, _ = m.Update(tea.KeyPressMsg{Code: '/', Text: "/"})
	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyUp})
	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	assert.Check(t, cmp.Contains(ansi.Strip(m.View("").Content), "/alpha"),
		"history should survive reset and be recalled with up")
}

// TestPagerModel_NotReadyRendersEmpty confirms a pager that has not yet seen a
// terminal size renders nothing rather than panicking.
func TestPagerModel_NotReadyRendersEmpty(t *testing.T) {
	m := components.NewPager().WithHint(pagerHint).WithContent("hello")
	assert.Check(t, !m.Ready())
	assert.Check(t, cmp.Equal(m.View("").Content, ""))
}
