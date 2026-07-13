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

package mdtable

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
	"gotest.tools/v3/assert"
	"gotest.tools/v3/assert/cmp"
)

// linesAligned asserts every line of a rendered table has the same display
// width — the property that keeps the column borders vertically aligned in a
// terminal. Display width is measured with ansi.StringWidth so wide runes
// count as their true cell count.
func linesAligned(t *testing.T, rendered string) {
	t.Helper()
	lines := strings.Split(strings.TrimRight(rendered, "\n"), "\n")
	assert.Assert(t, len(lines) > 0)
	want := ansi.StringWidth(lines[0])
	for i, line := range lines {
		assert.Check(t, cmp.Equal(ansi.StringWidth(line), want), "line %d: %q", i, line)
	}
}

func TestRenderPlain(t *testing.T) {
	tbl := New("Name", "Value")
	tbl.Row("FOO", "bar")
	tbl.Row("longer-name", "x")

	want := "" +
		"| Name        | Value |\n" +
		"| ----------- | ----- |\n" +
		"| FOO         | bar   |\n" +
		"| longer-name | x     |\n"
	assert.Equal(t, tbl.Render(), want)
	linesAligned(t, tbl.Render())
}

// TestRenderWideRunes is the regression guard: a cell holding an emoji (display
// width 2, but 1 rune and 3 bytes) must not push its row past the separator.
// Measuring with len() or padding with fmt's rune-counting %-*s both misalign
// such rows; only ansi.StringWidth-based sizing and padding keep them square.
func TestRenderWideRunes(t *testing.T) {
	tbl := New("Name", "Status", "Type")
	tbl.Row("test-macos", "✅ succeeded", "build")
	tbl.Row("ok-to-deploy", "✅ succeeded", "no-op")

	want := "" +
		"| Name         | Status       | Type  |\n" +
		"| ------------ | ------------ | ----- |\n" +
		"| test-macos   | ✅ succeeded | build |\n" +
		"| ok-to-deploy | ✅ succeeded | no-op |\n"
	assert.Equal(t, tbl.Render(), want)
	linesAligned(t, tbl.Render())
}

// TestRenderWideHeader covers a wide rune living in a header cell rather than a
// data cell, so the initial width seeded in New must also be display-measured.
func TestRenderWideHeader(t *testing.T) {
	tbl := New("✅", "Type")
	tbl.Row("a", "build")

	linesAligned(t, tbl.Render())
}
