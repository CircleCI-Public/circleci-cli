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

package ui_test

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

	"github.com/CircleCI-Public/circleci-cli/internal/ui"
	"github.com/CircleCI-Public/circleci-cli/internal/ui/theme"
)

// startPager runs a markdown pager over content at the given terminal size and
// waits for the first frame (so the initial WindowSizeMsg has rendered).
func startPager(t *testing.T, content string, w, h int) *teatest.TestModel {
	t.Helper()
	m := ui.NewMarkdownViewportModel(func(int) string { return content })
	tm := teatest.NewTestModel(t, m, teatest.WithInitialTermSize(w, h))
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

// pagerSnapshot quits the pager (q) and returns its final, ANSI-stripped frame.
// The pager renders its current search state in the footer, so the snapshot is
// where committed-search assertions are made. q is safe only outside the search
// prompt (inside it, q is typed into the pattern).
func pagerSnapshot(t *testing.T, tm *teatest.TestModel) string {
	t.Helper()
	tm.Send(tea.KeyPressMsg{Code: 'q', Text: "q"})
	fm := tm.FinalModel(t, teatest.WithFinalTimeout(time.Second)).(ui.MarkdownViewportModel)
	return ansi.Strip(fm.View().Content)
}

// search opens the prompt, types query, and commits it with Enter.
func search(tm *teatest.TestModel, query string) {
	tm.Send(tea.KeyPressMsg{Code: '/', Text: "/"})
	pagerType(tm, query)
	tm.Send(tea.KeyPressMsg{Code: tea.KeyEnter})
}

// TestPagerSearchMatchCount exercises the search matcher's behavior through the
// pager footer: the "n/m" counter's denominator reflects how many hits a
// committed pattern found (the focus starts on the first, so it reads "1/m"),
// and a missed pattern shows the not-found notice. The cases cover literal
// matching, smart case, per-line regex anchors, the invalid-regex literal
// fallback, and ANSI-insensitive matching.
func TestPagerSearchMatchCount(t *testing.T) {
	tests := []struct {
		name    string
		content string
		query   string
		want    string // substring expected in the footer ("1/<total>" or the notice)
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
			search(tm, tt.query)
			assert.Check(t, cmp.Contains(pagerSnapshot(t, tm), tt.want))
		})
	}
}

// TestPagerSearchCommits confirms typing a pattern and pressing Enter commits
// it: the footer shows the committed pattern and focuses the first of its
// matches.
func TestPagerSearchCommits(t *testing.T) {
	tm := startPager(t, strings.Repeat("needle in a haystack\nfiller line\n", 20), 80, 24)
	search(tm, "needle")

	view := pagerSnapshot(t, tm)
	assert.Check(t, cmp.Contains(view, "/needle"))
	assert.Check(t, cmp.Contains(view, "1/20"))
}

// TestPagerSearchNext advances the focused match with n.
func TestPagerSearchNext(t *testing.T) {
	tm := startPager(t, strings.Repeat("needle in a haystack\nfiller line\n", 20), 80, 24)
	search(tm, "needle")

	assert.Assert(t, t.Run("n advances to the next match", func(t *testing.T) {
		tm.Send(tea.KeyPressMsg{Code: 'n', Text: "n"})
		assert.Check(t, cmp.Contains(pagerSnapshot(t, tm), "2/20"))
	}))
}

// TestPagerSearchPrevWraps confirms N from the first match wraps to the last,
// less-style.
func TestPagerSearchPrevWraps(t *testing.T) {
	tm := startPager(t, strings.Repeat("needle in a haystack\nfiller line\n", 20), 80, 24)
	search(tm, "needle")

	assert.Assert(t, t.Run("N from the first match wraps to the last", func(t *testing.T) {
		tm.Send(tea.KeyPressMsg{Code: 'N', Text: "N"})
		assert.Check(t, cmp.Contains(pagerSnapshot(t, tm), "20/20"))
	}))
}

// TestPagerSearchScrollsToMatch confirms advancing to a match below the fold
// scrolls the viewport (the footer's scroll percentage leaves 0%).
func TestPagerSearchScrollsToMatch(t *testing.T) {
	tm := startPager(t, strings.Repeat("needle in a haystack\nfiller line\n", 20), 80, 10)
	search(tm, "needle")

	assert.Assert(t, t.Run("advancing past the fold scrolls the viewport", func(t *testing.T) {
		for range 11 {
			tm.Send(tea.KeyPressMsg{Code: 'n', Text: "n"})
		}

		view := pagerSnapshot(t, tm)
		assert.Check(t, cmp.Contains(view, "12/20"))
		assert.Check(t, !strings.Contains(view, "  0%"), "viewport should have scrolled off the top: %q", view)
	}))
}

// TestPagerSearchBackspace confirms backspace edits the in-progress pattern
// before it is committed.
func TestPagerSearchBackspace(t *testing.T) {
	tm := startPager(t, strings.Repeat("hello\n", 30), 80, 24)
	tm.Send(tea.KeyPressMsg{Code: '/', Text: "/"})
	pagerType(tm, "helx")
	tm.Send(tea.KeyPressMsg{Code: tea.KeyBackspace})
	tm.Send(tea.KeyPressMsg{Code: tea.KeyEnter})

	view := pagerSnapshot(t, tm)
	assert.Check(t, cmp.Contains(view, "/hel "), "the x should have been deleted before commit")
	assert.Check(t, !strings.Contains(view, "helx"))
}

// TestPagerSearchCancelCommitsNothing confirms Esc at the prompt abandons the
// pattern: nothing is committed and no highlights appear.
func TestPagerSearchCancelCommitsNothing(t *testing.T) {
	tm := startPager(t, strings.Repeat("hello\n", 30), 80, 24)
	tm.Send(tea.KeyPressMsg{Code: '/', Text: "/"})
	pagerType(tm, "hello")
	tm.Send(tea.KeyPressMsg{Code: tea.KeyEscape})

	view := pagerSnapshot(t, tm)
	assert.Check(t, !strings.Contains(view, "/hello"), "cancelled search must not commit")
	assert.Check(t, !strings.Contains(view, "pattern not found"))
}

// TestPagerSearchEmptyCommitRepeatsLast confirms a bare "/" + Enter repeats the
// previous pattern, like less.
func TestPagerSearchEmptyCommitRepeatsLast(t *testing.T) {
	tm := startPager(t, strings.Repeat("repeat me\nother\n", 30), 80, 24)
	search(tm, "repeat")

	assert.Assert(t, t.Run("a bare slash-enter repeats the previous pattern", func(t *testing.T) {
		tm.Send(tea.KeyPressMsg{Code: '/', Text: "/"})
		tm.Send(tea.KeyPressMsg{Code: tea.KeyEnter})

		view := pagerSnapshot(t, tm)
		assert.Check(t, cmp.Contains(view, "/repeat"))
		assert.Check(t, cmp.Contains(view, "1/30"))
	}))
}

// TestPagerSearchSurvivesResize confirms a resize re-renders from scratch yet
// keeps the search applied, so matches stay highlighted against the re-wrapped
// content.
func TestPagerSearchSurvivesResize(t *testing.T) {
	tm := startPager(t, strings.Repeat("target\nfiller\n", 30), 80, 24)
	search(tm, "target")

	assert.Assert(t, t.Run("the search survives a resize", func(t *testing.T) {
		tm.Send(tea.WindowSizeMsg{Width: 60, Height: 20})

		view := pagerSnapshot(t, tm)
		assert.Check(t, cmp.Contains(view, "/target"))
		assert.Check(t, cmp.Contains(view, "1/30"))
	}))
}

// TestPagerEscClearsSearch confirms the first Esc after a search dismisses it
// (dropping the highlights and counter) without quitting the pager — the
// subsequent q is what ends the program, and its final frame shows the search
// gone.
func TestPagerEscClearsSearch(t *testing.T) {
	tm := startPager(t, strings.Repeat("target\nfiller\n", 30), 80, 24)
	search(tm, "target")

	assert.Assert(t, t.Run("esc clears the search without quitting the pager", func(t *testing.T) {
		tm.Send(tea.KeyPressMsg{Code: tea.KeyEscape})

		// The pager stays open: the q inside pagerSnapshot is what quits, and its
		// final frame shows the search gone.
		view := pagerSnapshot(t, tm)
		assert.Check(t, !strings.Contains(view, "/target"), "search should have been cleared")
		assert.Check(t, !strings.Contains(view, "1/30"))
	}))
}

// TestPagerEscQuitsWhenNoSearch confirms Esc with no active search quits the
// pager.
func TestPagerEscQuitsWhenNoSearch(t *testing.T) {
	tm := startPager(t, strings.Repeat("target\nfiller\n", 30), 80, 24)
	tm.Send(tea.KeyPressMsg{Code: tea.KeyEscape})
	tm.WaitFinished(t, teatest.WithFinalTimeout(time.Second))
}

// TestSearchMatchStylesDistinct guards that the focused match looks different
// from the rest, or n/N gives no visual feedback.
func TestSearchMatchStylesDistinct(t *testing.T) {
	assert.Check(t, theme.SearchMatchStyle.GetBackground() != theme.SearchSelectedStyle.GetBackground())
}
