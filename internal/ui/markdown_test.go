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
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"gotest.tools/v3/assert"

	"github.com/CircleCI-Public/circleci-cli/internal/ui/theme"
)

func TestSearchMatches(t *testing.T) {
	tests := []struct {
		name    string
		content string
		query   string
		want    int
	}{
		{"literal substring", "alpha beta\nbeta gamma", "beta", 2},
		{"smart case insensitive", "Alpha\nalpha", "alpha", 2},
		{"smart case sensitive on uppercase", "Alpha\nalpha", "Alpha", 1},
		{"regex anchors per line", "cat\nscatter\ncat", "^cat$", 2},
		{"invalid regex falls back to literal", "a(b\nc", "a(b", 1},
		{"no match", "hello world", "zzz", 0},
		{"empty query", "hello", "", 0},
		{"matches ignore ansi styling", "\x1b[31mred\x1b[0m text", "red text", 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := searchMatches(tt.content, tt.query)
			assert.Equal(t, len(got), tt.want)
		})
	}
}

func TestSearchMatchColumnsSkipAnsi(t *testing.T) {
	// "text" sits after the colored "red " word; its columns must be measured
	// against the visible text, not the byte string with escape codes.
	got := searchMatches("\x1b[31mred\x1b[0m text", "text")
	assert.Equal(t, len(got), 1)
	assert.Equal(t, got[0], searchMatch{line: 0, colStart: 4, colEnd: 8})
}

// sized returns a ready pager populated with content, as if the terminal had
// reported its size.
func sized(content string) MarkdownViewportModel {
	m := NewMarkdownViewportModel(func(int) string { return content })
	model, _ := m.Update(tea.WindowSizeMsg{Width: 40, Height: 10})
	return model.(MarkdownViewportModel)
}

func typeKeys(m MarkdownViewportModel, s string) MarkdownViewportModel {
	for _, r := range s {
		model, _ := m.Update(tea.KeyPressMsg{Code: r, Text: string(r)})
		m = model.(MarkdownViewportModel)
	}
	return m
}

func press(m MarkdownViewportModel, code rune) MarkdownViewportModel {
	model, _ := m.Update(tea.KeyPressMsg{Code: code})
	return model.(MarkdownViewportModel)
}

func TestPagerSearchFlow(t *testing.T) {
	content := strings.Repeat("needle in a haystack\nfiller line\n", 20)
	m := sized(content)

	// "/" opens the prompt; characters accumulate in the input.
	m = press(m, '/')
	assert.Equal(t, m.searching, true)
	m = typeKeys(m, "needle")
	assert.Equal(t, m.input, "needle")
	assert.Assert(t, strings.Contains(m.View().Content, "/needle"))

	// Enter commits the search, highlights matches, and leaves search mode.
	m = press(m, tea.KeyEnter)
	assert.Equal(t, m.searching, false)
	assert.Equal(t, m.query, "needle")
	assert.Equal(t, len(m.matches), 20)
	assert.Equal(t, m.current, 0)
	assert.Equal(t, m.notFound, false)
	assert.Assert(t, strings.Contains(ansi.Strip(m.View().Content), "1/20"))

	// n advances the focused match and scrolls to one that's off screen; N goes back.
	m = press(m, 'n')
	assert.Equal(t, m.current, 1)
	for i := 0; i < 10; i++ {
		m = press(m, 'n')
	}
	assert.Assert(t, m.viewport.YOffset() > 0) // scrolled to reach a lower match
	assert.Equal(t, m.current, 11)
	m = press(m, 'N')
	assert.Equal(t, m.current, 10)

	// N from the first match wraps to the last.
	for m.current != 0 {
		m = press(m, 'N')
	}
	m = press(m, 'N')
	assert.Equal(t, m.current, 19)
}

func TestPagerSearchBackspaceAndCancel(t *testing.T) {
	m := sized(strings.Repeat("hello\n", 30))

	m = press(m, '/')
	m = typeKeys(m, "helx")
	m = press(m, tea.KeyBackspace)
	assert.Equal(t, m.input, "hel")

	// Esc cancels the prompt and commits nothing.
	m = press(m, tea.KeyEscape)
	assert.Equal(t, m.searching, false)
	assert.Equal(t, m.query, "")
}

func TestPagerSearchNotFound(t *testing.T) {
	m := sized(strings.Repeat("hello world\n", 30))

	m = press(m, '/')
	m = typeKeys(m, "absent")
	m = press(m, tea.KeyEnter)

	assert.Equal(t, m.notFound, true)
	assert.Equal(t, len(m.matches), 0)
	assert.Assert(t, strings.Contains(ansi.Strip(m.View().Content), "pattern not found"))
}

func TestPagerSearchEmptyCommitRepeatsLast(t *testing.T) {
	m := sized(strings.Repeat("repeat me\nother\n", 30))

	m = press(m, '/')
	m = typeKeys(m, "repeat")
	m = press(m, tea.KeyEnter)
	assert.Equal(t, len(m.matches), 30)

	// A bare "/" + Enter repeats the previous pattern, like less.
	m = press(m, '/')
	m = press(m, tea.KeyEnter)
	assert.Equal(t, m.query, "repeat")
	assert.Equal(t, len(m.matches), 30)
}

func TestPagerSearchSurvivesResize(t *testing.T) {
	m := sized(strings.Repeat("target\nfiller\n", 30))

	m = press(m, '/')
	m = typeKeys(m, "target")
	m = press(m, tea.KeyEnter)
	assert.Equal(t, len(m.matches), 30)

	// Resizing re-renders from scratch; the search must be re-applied so matches
	// stay highlighted against the freshly re-wrapped content.
	model, _ := m.Update(tea.WindowSizeMsg{Width: 60, Height: 20})
	m = model.(MarkdownViewportModel)
	assert.Equal(t, m.query, "target")
	assert.Equal(t, len(m.matches), 30)
}

func TestPagerEscClearsSearchThenQuits(t *testing.T) {
	m := sized(strings.Repeat("target\nfiller\n", 30))

	m = press(m, '/')
	m = typeKeys(m, "target")
	m = press(m, tea.KeyEnter)
	assert.Equal(t, len(m.matches), 30)

	// First Esc dismisses the search without quitting.
	model, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	m = model.(MarkdownViewportModel)
	assert.Equal(t, m.query, "")
	assert.Equal(t, len(m.matches), 0)
	assert.Assert(t, !isQuit(cmd))

	// With nothing left to clear, Esc quits.
	_, cmd = m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	assert.Assert(t, isQuit(cmd))
}

// isQuit reports whether cmd is bubbletea's quit command.
func isQuit(cmd tea.Cmd) bool {
	if cmd == nil {
		return false
	}
	_, ok := cmd().(tea.QuitMsg)
	return ok
}

func TestSearchMatchStylesDistinct(t *testing.T) {
	// The focused match must look different from the rest, or n/N gives no
	// visual feedback.
	assert.Assert(t, theme.SearchMatchStyle.GetBackground() != theme.SearchSelectedStyle.GetBackground())
}
