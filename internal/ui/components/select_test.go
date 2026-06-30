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
	"strings"
	"testing"

	"gotest.tools/v3/assert"
)

func selectOptions(n int) []string {
	opts := make([]string, n)
	for i := range opts {
		opts[i] = fmt.Sprintf("option-%02d", i)
	}
	return opts
}

// TestSelectModel_NoLimit verifies that with no height (or a height that fits
// the whole list) every option is shown and no scroll indicator appears.
func TestSelectModel_NoLimit(t *testing.T) {
	view := NewSelectModel("Pick", selectOptions(4)).View().Content
	for i := 0; i < 4; i++ {
		assert.Check(t, strings.Contains(view, fmt.Sprintf("option-%02d", i)), "option %d missing", i)
	}
	assert.Check(t, !strings.Contains(view, " of "), "unexpected scroll indicator: %q", view)
}

// TestSelectModel_ScrollsToKeepCursorVisible verifies the visible window slides
// to keep the cursor in view, hides the off-window options, and shows a position
// indicator. With height 7, two rows are reserved (prompt + hint), leaving 5
// option rows; a cursor at index 10 puts the window at indices 6–10.
func TestSelectModel_ScrollsToKeepCursorVisible(t *testing.T) {
	m := NewSelectModel("Pick", selectOptions(20)).WithHeight(7).WithCursor(10)
	view := m.View().Content

	for i := 6; i <= 10; i++ {
		assert.Check(t, strings.Contains(view, fmt.Sprintf("option-%02d", i)), "expected option %d in window", i)
	}
	assert.Check(t, !strings.Contains(view, "option-05"), "option below window should be hidden")
	assert.Check(t, !strings.Contains(view, "option-11"), "option above window should be hidden")
	assert.Check(t, strings.Contains(view, "(7–11 of 20)"), "missing/expected position indicator: %q", view)
}

// TestSelectModel_WindowClampedToEnd verifies the window never scrolls past the
// end: a cursor on the last option shows the final page, not a window running
// off the list.
func TestSelectModel_WindowClampedToEnd(t *testing.T) {
	m := NewSelectModel("Pick", selectOptions(20)).WithHeight(7).WithCursor(19)
	view := m.View().Content

	for i := 15; i <= 19; i++ {
		assert.Check(t, strings.Contains(view, fmt.Sprintf("option-%02d", i)), "expected option %d on final page", i)
	}
	assert.Check(t, !strings.Contains(view, "option-14"), "option before final page should be hidden")
	assert.Check(t, strings.Contains(view, "(16–20 of 20)"), "missing/expected position indicator: %q", view)
}
