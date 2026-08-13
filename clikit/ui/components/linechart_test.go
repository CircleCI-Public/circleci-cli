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
	"fmt"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
	"gotest.tools/v3/assert"
	"gotest.tools/v3/assert/cmp"

	"github.com/CircleCI-Public/circleci-cli/clikit/ui/components"
)

// rampChart is a single rising-then-falling series against a ceiling of 4, with
// integer y ticks and second-based x labels — enough shape to exercise both axes.
func rampChart() components.LineChart {
	return components.LineChart{
		Series:  []components.ChartSeries{{Values: []float64{0, 1, 2, 3, 4, 3, 2, 1, 0}}},
		Ceiling: 4,
		FormatY: func(v float64) string { return fmt.Sprintf("%.0f", v) },
		FormatX: func(frac float64) string { return fmt.Sprintf("%.0fs", frac*120) },
	}
}

func TestLineChartRender(t *testing.T) {
	lines := strings.Split(rampChart().Render(), "\n")

	// The plot rows, the x-axis rule, and its label row. No legend: a single series
	// has nothing to be distinguished from.
	assert.Check(t, cmp.Equal(len(lines), components.LineChartDefaultHeight))

	axis := lines[len(lines)-2]
	labels := lines[len(lines)-1]

	// Both ends of the series are named. The right-hand one is what the underlying
	// library drops when it would overrun the canvas, which is the whole reason the
	// axis is laid out here.
	assert.Check(t, strings.Contains(labels, "0s"), "labels: %q", labels)
	assert.Check(t, strings.Contains(labels, "120s"), "labels: %q", labels)
	assert.Check(t, strings.HasSuffix(labels, "120s"), "last label is not flush right: %q", labels)

	// Every label that was placed has a mark on the rule tying it to a column.
	assert.Check(t, cmp.Equal(strings.Count(axis, "┬"), strings.Count(labels, "s")),
		"marks and labels disagree:\n%s\n%s", axis, labels)
}

// TestLineChartYAxisTicks is the y-axis counterpart: a row carrying a tick label
// must be marked on the axis, and a row without one must not be.
func TestLineChartYAxisTicks(t *testing.T) {
	lines := strings.Split(rampChart().Render(), "\n")

	var labelled, plain int
	for _, line := range lines[:len(lines)-2] {
		gutter, _, found := strings.Cut(line, "┤")
		if found {
			// A marked row is a labelled row: the gutter holds the tick value.
			assert.Check(t, strings.TrimSpace(gutter) != "", "marked row has no label: %q", line)
			labelled++
			continue
		}
		gutter, _, found = strings.Cut(line, "│")
		assert.Check(t, found, "plot row has no axis glyph: %q", line)
		assert.Check(t, strings.TrimSpace(gutter) == "", "unmarked row carries a label: %q", line)
		plain++
	}
	// Eight plot rows at the default step: four labelled, four not.
	assert.Check(t, cmp.Equal(labelled, 4))
	assert.Check(t, cmp.Equal(plain, 4))
}

// TestLineChartAlignment is the property that matters more than any single
// rendering: the axis glyph lands in the same column on every row, so the tick
// labels line up with the rows they describe.
func TestLineChartAlignment(t *testing.T) {
	c := rampChart()
	c.FormatY = func(v float64) string { return fmt.Sprintf("%.1f GiB", v) }
	lines := strings.Split(c.Render(), "\n")

	const gutter = len("4.0 GiB")
	for i, line := range lines[:len(lines)-1] {
		runes := []rune(line)
		assert.Check(t, cmp.Equal(ansi.StringWidth(string(runes[:gutter])), gutter), "line %d: %q", i, line)
		axis := string(runes[gutter])
		assert.Check(t, axis == "┤" || axis == "│" || axis == "└", "line %d: unexpected axis glyph %q in %q", i, axis, line)
	}
	assert.Check(t, ansi.StringWidth(lines[len(lines)-2]) <= components.LineChartDefaultWidth)
}

// TestLineChartNoTrailingSpace guards the whole rendering against the padding the
// underlying canvas applies to every row, which would otherwise put invisible
// trailing whitespace into every golden file capturing a chart.
func TestLineChartNoTrailingSpace(t *testing.T) {
	c := rampChart()
	c.Series = append(c.Series, components.ChartSeries{Name: "second", Values: []float64{4, 4, 4}})
	c.Series[0].Name = "first"
	for i, line := range strings.Split(c.Render(), "\n") {
		assert.Check(t, line == strings.TrimRight(line, " "), "line %d has trailing space: %q", i, line)
	}
}

func TestLineChartLegend(t *testing.T) {
	t.Run("named series are listed", func(t *testing.T) {
		c := rampChart()
		c.Series = []components.ChartSeries{
			{Name: "exec 0", Values: []float64{1, 2}},
			{Name: "exec 1", Values: []float64{3, 4}},
		}
		lines := strings.Split(c.Render(), "\n")
		last := lines[len(lines)-1]
		assert.Check(t, strings.Contains(last, "exec 0"), "legend: %q", last)
		assert.Check(t, strings.Contains(last, "exec 1"), "legend: %q", last)
	})

	t.Run("a single series has no legend", func(t *testing.T) {
		lines := strings.Split(rampChart().Render(), "\n")
		assert.Check(t, !strings.Contains(lines[len(lines)-1], "⣿"))
	})

	t.Run("unnamed series have no legend", func(t *testing.T) {
		c := rampChart()
		c.Series = []components.ChartSeries{{Values: []float64{1, 2}}, {Values: []float64{3, 4}}}
		lines := strings.Split(c.Render(), "\n")
		assert.Check(t, !strings.Contains(lines[len(lines)-1], "⣿"))
	})
}

// TestLineChartColor checks that the per-series colors are emitted only when asked
// for. They have to survive being written to a pipe — the charts are embedded in
// markdown and rendered later — so this must not depend on a terminal.
func TestLineChartColor(t *testing.T) {
	c := rampChart()
	c.Series = []components.ChartSeries{
		{Name: "a", Values: []float64{1, 2}},
		{Name: "b", Values: []float64{3, 4}},
	}

	plain := c.Render()
	assert.Check(t, cmp.Equal(plain, ansi.Strip(plain)), "uncolored chart carries escapes")

	c.Color = true
	colored := c.Render()
	assert.Check(t, colored != ansi.Strip(colored), "colored chart carries no escapes")
	// Color must not change the layout, only the ink.
	assert.Check(t, cmp.Equal(ansi.Strip(colored), plain))
}

func TestLineChartEmpty(t *testing.T) {
	assert.Equal(t, components.LineChart{Ceiling: 4}.Render(), "")
	// A series with no samples counts as no series at all.
	assert.Equal(t, components.LineChart{
		Series:  []components.ChartSeries{{Name: "a"}},
		Ceiling: 4,
	}.Render(), "")
}

// TestLineChartSingleSample covers the degenerate span: one sample covers no time,
// which would otherwise leave the x axis with nothing to scale by.
func TestLineChartSingleSample(t *testing.T) {
	c := rampChart()
	c.Series = []components.ChartSeries{{Values: []float64{2}}}
	assert.Check(t, c.Render() != "")
}

// TestLineChartDefaultFormatters covers the nil-formatter paths: a chart with no
// FormatY or FormatX still labels both axes, rather than rendering a bare frame.
func TestLineChartDefaultFormatters(t *testing.T) {
	c := components.LineChart{
		Series:  []components.ChartSeries{{Values: []float64{0, 1, 2, 3, 4}}},
		Ceiling: 4,
	}
	lines := strings.Split(c.Render(), "\n")

	// Two decimal places by default, so the ceiling reads as "4.00".
	assert.Check(t, strings.HasPrefix(lines[0], "4.00"), "y axis: %q", lines[0])
	// And the x axis falls back to the position along it as a percentage, which says
	// something useful without assuming what the axis measures.
	labels := lines[len(lines)-1]
	assert.Check(t, strings.Contains(labels, "0%"), "x axis: %q", labels)
	assert.Check(t, strings.HasSuffix(labels, "100%"), "x axis: %q", labels)
}

// TestLineChartDefaultSize checks the zero value renders at the documented size
// rather than collapsing.
func TestLineChartDefaultSize(t *testing.T) {
	c := components.LineChart{Series: []components.ChartSeries{{Values: []float64{1, 2, 3}}}}
	lines := strings.Split(c.Render(), "\n")
	assert.Check(t, cmp.Equal(len(lines), components.LineChartDefaultHeight))
	for i, line := range lines {
		assert.Check(t, ansi.StringWidth(line) <= components.LineChartDefaultWidth, "line %d: %q", i, line)
	}
}
