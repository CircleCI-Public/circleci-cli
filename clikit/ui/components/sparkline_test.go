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
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
	"gotest.tools/v3/assert"
	"gotest.tools/v3/assert/cmp"

	"github.com/CircleCI-Public/circleci-cli/clikit/ui/components"
)

// brailleRunes reports how many of s are raised braille cells, i.e. how much of the
// row actually carries a line. A blank braille cell (U+2800) is not counted.
func brailleRunes(s string) int {
	n := 0
	for _, r := range s {
		if r > 0x2800 && r <= 0x28FF {
			n++
		}
	}
	return n
}

// TestSparklineWidth pins the width contract, which is a bound rather than an
// equality: a line segment spans the interval between two samples rather than a
// sample itself, and whether the final cell picks up a dot depends on the values.
func TestSparklineWidth(t *testing.T) {
	for _, values := range [][]float64{
		{2},
		{0, 1},
		{0, 1, 2, 3},
		{4, 0, 4, 0, 4, 0},
		{1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1},
	} {
		got := len([]rune(components.Sparkline(values, 4)))
		assert.Check(t, got >= 1 && got <= len(values),
			"%d cells for %d samples", got, len(values))
	}
}

func TestSparklineEmpty(t *testing.T) {
	assert.Equal(t, components.Sparkline(nil, 4), "")
	assert.Equal(t, components.Sparkline([]float64{}, 4), "")
}

// TestSparklineNoTrailingSpace matters because the row goes into a markdown table
// cell: the underlying canvas pads to its full width, and a trailing space inside
// the cell would shift the column.
func TestSparklineNoTrailingSpace(t *testing.T) {
	// A series that ends low leaves the final cells empty, which is when padding
	// would show.
	s := components.Sparkline([]float64{4, 3, 2, 1, 0, 0, 0}, 4)
	assert.Equal(t, s, strings.TrimRight(s, " "))
	assert.Check(t, !strings.Contains(s, "\n"), "sparkline is more than one row: %q", s)
}

// TestSparklineCeiling covers the two scaling modes. Against an explicit ceiling a
// series well under it stays low in the cell; scaled to its own peak the same
// series uses the full height.
func TestSparklineCeiling(t *testing.T) {
	values := []float64{0.1, 0.2, 0.3, 0.4}

	absolute := components.Sparkline(values, 100)
	relative := components.Sparkline(values, 0)

	// The bottom-row dots (⣀ and friends) sit low in the cell; a relatively scaled
	// series climbs off them.
	assert.Check(t, absolute != relative,
		"a distant ceiling drew the same row as relative scaling: %q", absolute)
	assert.Check(t, brailleRunes(absolute) > 0, "absolute row is blank: %q", absolute)
	assert.Check(t, brailleRunes(relative) > 0, "relative row is blank: %q", relative)
}

// TestSparklineFlatSeries is the reason this draws a line rather than block
// columns: scaled to its own peak a flat series is all-maximum, which columns would
// draw as a solid run of full blocks — easily misread as "pinned at the maximum"
// rather than "never changed".
func TestSparklineFlatSeries(t *testing.T) {
	flat := components.Sparkline([]float64{2, 2, 2, 2, 2}, 0)
	assert.Check(t, !strings.Contains(flat, "█"), "flat series drew blocks: %q", flat)
	assert.Check(t, brailleRunes(flat) > 0, "flat series drew nothing: %q", flat)
}

// TestSparklineIsPlainText guards the one property every caller depends on: the row
// is inert text, safe to drop into a markdown table cell or a golden file.
func TestSparklineIsPlainText(t *testing.T) {
	s := components.Sparkline([]float64{0, 4, 1, 3}, 4)
	assert.Check(t, cmp.Equal(s, ansi.Strip(s)), "sparkline carries escape sequences: %q", s)
}

func TestDownsamplePeaks(t *testing.T) {
	tests := []struct {
		name   string
		values []float64
		n      int
		want   []float64
	}{
		{
			name:   "already short enough is unchanged",
			values: []float64{1, 2, 3},
			n:      5,
			want:   []float64{1, 2, 3},
		},
		{
			name:   "exactly n is unchanged",
			values: []float64{1, 2, 3},
			n:      3,
			want:   []float64{1, 2, 3},
		},
		{
			name:   "n of zero is a no-op",
			values: []float64{1, 2, 3},
			n:      0,
			want:   []float64{1, 2, 3},
		},
		{
			name:   "a negative n is a no-op",
			values: []float64{1, 2, 3},
			n:      -1,
			want:   []float64{1, 2, 3},
		},
		{
			name:   "empty in, empty out",
			values: nil,
			n:      4,
			want:   nil,
		},
		{
			name: "buckets keep their peak, not their mean",
			// The spike in the second bucket survives; averaging would bury it.
			values: []float64{1, 2, 0, 9, 3, 4},
			n:      3,
			want:   []float64{2, 9, 4},
		},
		{
			name: "uneven buckets tile the whole series",
			// 7 into 3 leaves a remainder; the tail must still be covered rather than
			// dropped.
			values: []float64{1, 2, 3, 4, 5, 6, 7},
			n:      3,
			want:   []float64{2, 4, 7},
		},
		{
			name:   "down to a single bucket is the overall peak",
			values: []float64{1, 9, 2},
			n:      1,
			want:   []float64{9},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := components.DownsamplePeaks(tc.values, tc.n)
			assert.DeepEqual(t, got, tc.want)
			if tc.n > 0 {
				assert.Check(t, len(got) <= max(tc.n, len(tc.values)))
			}
		})
	}
}

// TestDownsamplePeaksNeverLosesThePeak is the property the bucketing exists for: a
// spike anywhere in the series must reach the output, wherever it falls.
func TestDownsamplePeaksNeverLosesThePeak(t *testing.T) {
	for spike := range 50 {
		values := make([]float64, 50)
		values[spike] = 99
		got := components.DownsamplePeaks(values, 8)
		assert.Check(t, slicesMax(got) == 99, "spike at index %d was lost: %v", spike, got)
	}
}

func slicesMax(values []float64) float64 {
	var peak float64
	for _, v := range values {
		peak = max(peak, v)
	}
	return peak
}

func TestTrimTrailingSpace(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{name: "empty", in: "", want: ""},
		{name: "nothing to trim", in: "a\nb", want: "a\nb"},
		{
			name: "trailing spaces come off every line",
			in:   "a  \nb\t \nc",
			// Only spaces are trimmed: a tab is content as far as this is concerned,
			// and the canvas pads with spaces.
			want: "a\nb\t\nc",
		},
		{
			name: "leading and interior spaces are kept",
			in:   "  a b  ",
			want: "  a b",
		},
		{
			name: "trailing blank lines come off the end",
			in:   "a\n   \n\n",
			want: "a",
		},
		{
			name: "interior blank lines are kept",
			// A gap between rows is part of the drawing; only the tail is noise.
			in:   "a\n\nb\n  ",
			want: "a\n\nb",
		},
		{name: "all blank collapses to nothing", in: "   \n \n", want: ""},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, components.TrimTrailingSpace(tc.in), tc.want)
		})
	}
}
