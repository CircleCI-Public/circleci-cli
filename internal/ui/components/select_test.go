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
	"fmt"
	"strings"
	"testing"
	"time"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/exp/teatest/v2"
	"gotest.tools/v3/assert"
	"gotest.tools/v3/assert/cmp"

	"github.com/CircleCI-Public/circleci-cli/internal/ui/components"
)

func selectOptions(n int) []string {
	opts := make([]string, n)
	for i := range opts {
		opts[i] = fmt.Sprintf("option-%02d", i)
	}
	return opts
}

// selectHarness wraps SelectModel so it can be driven as a standalone program in
// teatest: SelectModel itself never returns tea.Quit (the parent flow drives
// it), so the harness quits the program once a choice is confirmed or ctrl+c is
// pressed. The wrapped model stays reachable via m for post-run assertions.
type selectHarness struct {
	m components.SelectModel
}

func (h selectHarness) Init() tea.Cmd { return h.m.Init() }

func (h selectHarness) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if k, ok := msg.(tea.KeyPressMsg); ok && key.Matches(k, components.KeyCtrlC) {
		return h, tea.Quit
	}
	updated, cmd := h.m.Update(msg)
	h.m = updated.(components.SelectModel)
	if h.m.Done() {
		return h, tea.Quit
	}
	return h, cmd
}

func (h selectHarness) View() tea.View { return h.m.View() }

// pressKeys sends each key code to the program as a key-press message.
func pressKeys(tm *teatest.TestModel, codes ...rune) {
	for _, c := range codes {
		tm.Send(tea.KeyPressMsg{Code: c})
	}
}

// start runs a model through teatest at the given terminal size and waits for
// the first frame so any initial WindowSizeMsg has been applied.
func start(t *testing.T, m components.SelectModel, w, h int) *teatest.TestModel {
	t.Helper()
	tm := teatest.NewTestModel(t, selectHarness{m: m}, teatest.WithInitialTermSize(w, h))
	teatest.WaitFor(t, tm.Output(), func(b []byte) bool {
		return bytes.Contains(b, []byte("Pick"))
	}, teatest.WithDuration(time.Second))
	return tm
}

// finalView quits the program with ctrl+c (leaving any choice unconfirmed) and
// returns the final model's rendered list. ctrl+c is used rather than Enter so
// the snapshot still shows the option list rather than the chosen-row summary.
func finalView(t *testing.T, tm *teatest.TestModel) string {
	t.Helper()
	tm.Send(tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl})
	fm := tm.FinalModel(t, teatest.WithFinalTimeout(time.Second)).(selectHarness)
	assert.Check(t, !fm.m.Done(), "ctrl+c should leave the choice unconfirmed")
	return fm.m.View().Content
}

// runSelect drives a 20-option picker with a 5-row visible window (term height
// 7, minus the prompt and hint rows), feeds it the given key codes followed by
// Enter, and returns the index the user ended up confirming.
func runSelect(t *testing.T, codes ...rune) int {
	t.Helper()
	tm := start(t, components.NewSelectModel("Pick", selectOptions(20)), 80, 7)
	pressKeys(tm, codes...)
	pressKeys(tm, tea.KeyEnter)
	fm := tm.FinalModel(t, teatest.WithFinalTimeout(time.Second)).(selectHarness)
	assert.Check(t, fm.m.Done(), "expected the picker to have confirmed a choice")
	return fm.m.Selected()
}

// TestSelectModel_FitsWithoutScrolling verifies that when the list fits the
// terminal every option is shown and no scroll indicator appears.
func TestSelectModel_FitsWithoutScrolling(t *testing.T) {
	tm := start(t, components.NewSelectModel("Pick", selectOptions(4)), 80, 24)
	view := finalView(t, tm)

	for i := 0; i < 4; i++ {
		assert.Check(t, cmp.Contains(view, fmt.Sprintf("option-%02d", i)), "option %d missing", i)
	}
	assert.Check(t, !strings.Contains(view, " of "), "unexpected scroll indicator: %q", view)
}

// TestSelectModel_Note verifies WithNote renders its line(s) between the title
// and the options, and that a note reserves rows so the option window shrinks
// accordingly (leaving room for the note without overflowing the height).
func TestSelectModel_Note(t *testing.T) {
	m := components.NewSelectModel("Pick", selectOptions(4)).WithNote("config-fetch: no config found")
	view := finalView(t, start(t, m, 80, 24))

	assert.Check(t, cmp.Contains(view, "config-fetch: no config found"))
	// The note sits above the options.
	assert.Check(t, strings.Index(view, "config-fetch") < strings.Index(view, "option-00"),
		"note should render before the options: %q", view)

	// With height 7, reserving prompt + hint + a 1-line note leaves 4 option rows,
	// so a 20-option list scrolls and the window shows 4 (not 5) options.
	scroll := finalView(t, start(t, components.NewSelectModel("Pick", selectOptions(20)).WithNote("boom"), 80, 7))
	assert.Check(t, cmp.Contains(scroll, "(1–4 of 20)"), "note should shrink the window by one row: %q", scroll)
}

// TestSelectModel_ScrollsToKeepCursorVisible verifies the visible window slides
// to keep the cursor in view, hides the off-window options, and shows a position
// indicator. With height 7, two rows are reserved (prompt + hint), leaving 5
// option rows; a cursor at index 10 puts the window at indices 6–10.
func TestSelectModel_ScrollsToKeepCursorVisible(t *testing.T) {
	tm := start(t, components.NewSelectModel("Pick", selectOptions(20)).WithCursor(10), 80, 7)
	view := finalView(t, tm)

	for i := 6; i <= 10; i++ {
		assert.Check(t, cmp.Contains(view, fmt.Sprintf("option-%02d", i)), "expected option %d in window", i)
	}
	assert.Check(t, !strings.Contains(view, "option-05"), "option below window should be hidden")
	assert.Check(t, !strings.Contains(view, "option-11"), "option above window should be hidden")
	assert.Check(t, cmp.Contains(view, "(7–11 of 20)"))
}

// TestSelectModel_WindowClampedToEnd verifies the window never scrolls past the
// end: a cursor on the last option shows the final page, not a window running
// off the list.
func TestSelectModel_WindowClampedToEnd(t *testing.T) {
	tm := start(t, components.NewSelectModel("Pick", selectOptions(20)).WithCursor(19), 80, 7)
	view := finalView(t, tm)

	for i := 15; i <= 19; i++ {
		assert.Check(t, cmp.Contains(view, fmt.Sprintf("option-%02d", i)), "expected option %d on final page", i)
	}
	assert.Check(t, !strings.Contains(view, "option-14"), "option before final page should be hidden")
	assert.Check(t, cmp.Contains(view, "(16–20 of 20)"))
}

// TestSelectModel_NavigationKeys drives the picker through teatest's real
// Update loop and asserts where the cursor lands. With a 5-row window, PgUp/
// PgDown move by a page and clamp at the ends; Home/End (and the g/G vim
// aliases) jump to the first/last option.
func TestSelectModel_NavigationKeys(t *testing.T) {
	tests := []struct {
		name  string
		codes []rune
		want  int
	}{
		{"pgdown moves one page", []rune{tea.KeyPgDown}, 5},
		{"pgdown twice moves two pages", []rune{tea.KeyPgDown, tea.KeyPgDown}, 10},
		{"pgdown clamps to the last option", []rune{tea.KeyPgDown, tea.KeyPgDown, tea.KeyPgDown, tea.KeyPgDown, tea.KeyPgDown}, 19},
		{"pgup clamps to the first option", []rune{tea.KeyPgUp}, 0},
		{"end then pgup moves back one page", []rune{tea.KeyEnd, tea.KeyPgUp}, 14},
		{"home jumps to the first option", []rune{tea.KeyEnd, tea.KeyHome}, 0},
		{"end jumps to the last option", []rune{tea.KeyEnd}, 19},
		{"down moves one option", []rune{tea.KeyDown}, 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Check(t, cmp.Equal(runSelect(t, tt.codes...), tt.want))
		})
	}
}

// TestSelectModel_VimJumpKeys verifies the g/G vim aliases mirror Home/End.
func TestSelectModel_VimJumpKeys(t *testing.T) {
	assert.Check(t, cmp.Equal(runSelect(t, 'G'), 19), "G should jump to the last option")
	assert.Check(t, cmp.Equal(runSelect(t, 'G', 'g'), 0), "g should jump back to the first option")
}

// TestSelectModel_PagingScrollsWindow drives PgDown through teatest's real
// program loop, then snapshots the final model to prove paging slid the visible
// window — and its position indicator — and did not merely move the cursor.
// (Output() is a stream of terminal diffs, not a screen snapshot, so the
// assertion is on the final model's View instead.)
func TestSelectModel_PagingScrollsWindow(t *testing.T) {
	tm := start(t, components.NewSelectModel("Pick", selectOptions(20)), 80, 7)

	pressKeys(tm, tea.KeyPgDown)
	view := finalView(t, tm)

	// PgDown moved the cursor a full page (to index 5); the window slid just far
	// enough to keep it visible at the bottom edge, showing rows 2–6.
	assert.Check(t, cmp.Contains(view, "(2–6 of 20)"))
	assert.Check(t, cmp.Contains(view, "option-05"))
	assert.Check(t, !strings.Contains(view, "option-00"), "top of list should have scrolled out of view")
}
