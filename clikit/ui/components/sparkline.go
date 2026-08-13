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
	"strings"

	"github.com/NimbleMarkets/ntcharts/v2/sparkline"
)

// Sparkline renders values as a single row of braille — a chart small enough for a
// table cell or a status line, with no axes and no labels. A line rather than block
// columns because a flat series scaled to its own peak is all-maximum, which columns
// draw as a solid run of full blocks: easily misread as "pinned at the maximum"
// rather than "never changed".
//
// The row is at most one cell per sample, and often one fewer: a line segment
// spans the interval between two samples rather than a sample itself, and whether
// the final cell picks up a dot depends on the values. Callers needing an exact
// width should pad the result rather than assume one.
//
// ceiling is the value that reaches full height; zero or less scales to the
// largest sample present, which is usually what a sparkline sitting beside an
// absolute chart wants. Pass values through [DownsamplePeaks] first to bound the
// width: the underlying ring buffer keeps only the *last* n samples, silently
// dropping the start of a longer series. Empty values yield an empty string.
func Sparkline(values []float64, ceiling float64) string {
	if len(values) == 0 {
		return ""
	}
	opts := []sparkline.Option{}
	if ceiling > 0 {
		opts = append(opts, sparkline.WithMaxValue(ceiling))
	}
	s := sparkline.New(len(values), 1, opts...)
	s.PushAll(values)
	s.DrawBraille()
	return TrimTrailingSpace(s.View())
}

// DownsamplePeaks reduces values to at most n buckets, each holding the maximum of
// the samples that fell into it. The maximum — not the mean — is deliberate: these
// charts exist to show peaks, and averaging is exactly what hides the spike that
// sent someone looking. An n of zero or less, or a series already short enough,
// returns values unchanged.
func DownsamplePeaks(values []float64, n int) []float64 {
	if n <= 0 || len(values) <= n {
		return values
	}
	out := make([]float64, n)
	for i := range out {
		// Bucket i covers [i*len/n, (i+1)*len/n), computed from the ends rather than
		// by stepping, so the buckets tile the series exactly with no remainder left
		// over at the tail.
		lo := i * len(values) / n
		hi := (i + 1) * len(values) / n
		if hi <= lo {
			hi = lo + 1
		}
		peak := values[lo]
		for _, v := range values[lo+1 : hi] {
			if v > peak {
				peak = v
			}
		}
		out[i] = peak
	}
	return out
}

// TrimTrailingSpace strips trailing spaces from every line, and blank lines from
// the end. The chart canvas pads each of its rows out to the full width, which
// would otherwise bury invisible trailing whitespace in every line of output and
// in every golden file that captures one.
func TrimTrailingSpace(s string) string {
	lines := strings.Split(s, "\n")
	for i, line := range lines {
		lines[i] = strings.TrimRight(line, " ")
	}
	for len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	return strings.Join(lines, "\n")
}
