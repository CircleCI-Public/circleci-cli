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
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"gotest.tools/v3/assert"
)

func newTestPicker(t *testing.T) ThemePickerModel {
	t.Helper()
	labels := []string{"auto (default)", "dark", "light"}
	themes := []string{"auto", "dark", "light"}
	// The render callback echoes which theme/width it was asked for so the test
	// can assert the preview tracks the highlighted option.
	render := func(theme string, width int) string {
		return fmt.Sprintf("rendered:%s@%d", theme, width)
	}
	return NewThemePickerModel("Select a theme", labels, themes, render, false, true).WithCursor(0)
}

// updatePicker applies msg and then runs any returned command to completion,
// feeding each resulting message back in. The preview is rendered off the Update
// loop via a command, so tests must drain it to observe the rendered content.
// SelectModel emits no commands, so the chain is at most one render command deep.
func updatePicker(m ThemePickerModel, msg tea.Msg) ThemePickerModel {
	model, cmd := m.Update(msg)
	m = model.(ThemePickerModel)
	for cmd != nil {
		next := cmd()
		if next == nil {
			break
		}
		model, cmd = m.Update(next)
		m = model.(ThemePickerModel)
	}
	return m
}

// Before the first window size arrives the view is empty and the preview is not
// rendered.
func TestThemePickerNotReady(t *testing.T) {
	m := newTestPicker(t)
	assert.Equal(t, m.View().Content, "")
}

// A window size renders the preview for the initially-highlighted theme, and
// the body shows both panes.
func TestThemePickerRendersPreview(t *testing.T) {
	m := newTestPicker(t)
	m = updatePicker(m, tea.WindowSizeMsg{Width: 100, Height: 24})

	view := m.View().Content
	assert.Assert(t, strings.Contains(view, "Select a theme"), "selector pane missing")
	assert.Assert(t, strings.Contains(view, "rendered:auto@"), "preview for first theme missing")
}

// Before the async render lands (cmd not yet run), the preview shows the
// loading placeholder rather than blank space.
func TestThemePickerShowsLoadingPlaceholder(t *testing.T) {
	m := newTestPicker(t)
	// Apply the size but do NOT drain the render command, so the model is still
	// in its loading state.
	model, cmd := m.Update(tea.WindowSizeMsg{Width: 100, Height: 24})
	m = model.(ThemePickerModel)
	assert.Assert(t, cmd != nil, "a cache miss should return a render command")

	view := m.View().Content
	assert.Assert(t, strings.Contains(view, "Loading"), "loading placeholder missing")
	assert.Assert(t, !strings.Contains(view, "rendered:auto@"), "preview should not be rendered yet")
}

// With animation enabled the spinner ticks (Init schedules it); with animation
// disabled (CIRCLE_SPINNER_DISABLED / non-TTY) it stays still so scripted
// sessions don't see continuous repaints.
func TestThemePickerSpinnerAnimationGating(t *testing.T) {
	labels := []string{"auto"}
	themes := []string{"auto"}
	render := func(string, int) string { return "x" }

	animated := NewThemePickerModel("t", labels, themes, render, false, true)
	assert.Assert(t, animated.Init() != nil, "animated picker should start ticking")

	static := NewThemePickerModel("t", labels, themes, render, false, false)
	assert.Assert(t, static.Init() == nil, "static picker should not tick")

	// With animation disabled the preview renders synchronously — no async
	// command, no loading placeholder, just the content.
	model, cmd := static.Update(tea.WindowSizeMsg{Width: 100, Height: 24})
	static = model.(ThemePickerModel)
	assert.Assert(t, cmd == nil, "static picker should render synchronously, not via a command")
	assert.Assert(t, strings.Contains(static.View().Content, "x"), "preview content should be rendered")
	assert.Assert(t, !strings.Contains(static.View().Content, "Loading"), "no placeholder when synchronous")
}

// Moving the cursor re-renders the preview for the newly-highlighted theme.
func TestThemePickerPreviewFollowsCursor(t *testing.T) {
	m := newTestPicker(t)
	m = updatePicker(m, tea.WindowSizeMsg{Width: 100, Height: 24})
	m = updatePicker(m, tea.KeyPressMsg{Code: tea.KeyDown})

	assert.Equal(t, m.Selected(), 1)
	assert.Assert(t, strings.Contains(m.View().Content, "rendered:dark@"), "preview did not follow cursor")
}

// Enter confirms the highlighted theme without cancelling.
func TestThemePickerEnterSelects(t *testing.T) {
	m := newTestPicker(t)
	m = updatePicker(m, tea.WindowSizeMsg{Width: 100, Height: 24})
	m = updatePicker(m, tea.KeyPressMsg{Code: tea.KeyDown})

	model, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	m = model.(ThemePickerModel)
	assert.Assert(t, cmd != nil, "enter should quit")
	assert.Equal(t, m.Cancelled(), false)
	assert.Equal(t, m.Selected(), 1)
}

// Revisiting a theme is served from cache: render runs once per theme, not on
// every pass over it. This is what keeps navigation from blocking on glamour.
func TestThemePickerCachesRenders(t *testing.T) {
	labels := []string{"auto", "dark", "light"}
	themes := []string{"auto", "dark", "light"}
	calls := map[string]int{}
	render := func(theme string, _ int) string {
		calls[theme]++
		return "rendered:" + theme
	}
	m := NewThemePickerModel("Select a theme", labels, themes, render, false, true).WithCursor(0)
	m = updatePicker(m, tea.WindowSizeMsg{Width: 100, Height: 24})

	// Walk down to the bottom and back up to the top, crossing each theme twice.
	for range 2 {
		m = updatePicker(m, tea.KeyPressMsg{Code: tea.KeyDown})
	}
	for range 2 {
		m = updatePicker(m, tea.KeyPressMsg{Code: tea.KeyUp})
	}

	for theme, n := range calls {
		assert.Equal(t, n, 1, "theme %q rendered %d times, want 1", theme, n)
	}
	assert.Equal(t, len(calls), 3, "every theme should have been rendered once")
}

// Esc cancels.
func TestThemePickerEscCancels(t *testing.T) {
	m := newTestPicker(t)
	m = updatePicker(m, tea.WindowSizeMsg{Width: 100, Height: 24})

	model, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	m = model.(ThemePickerModel)
	assert.Assert(t, cmd != nil, "esc should quit")
	assert.Equal(t, m.Cancelled(), true)
}
