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
	"math"
	"slices"
	"strings"
	"time"

	"charm.land/lipgloss/v2"
	"github.com/NimbleMarkets/ntcharts/v2/canvas"
	"github.com/NimbleMarkets/ntcharts/v2/canvas/runes"
	"github.com/NimbleMarkets/ntcharts/v2/linechart/timeserieslinechart"

	"github.com/CircleCI-Public/circleci-cli/clikit/ui/theme"
)

// LineChart layout defaults, applied to the zero value of the matching field.
const (
	// LineChartDefaultWidth is the total chart width when Width is unset, y-axis
	// gutter included. It fits an 80-column terminal.
	LineChartDefaultWidth = 72

	// LineChartDefaultHeight is the total chart height when Height is unset,
	// including the two rows the x axis and its labels take. See [LineChart.YStep]
	// for why the remainder wants to be a power of two.
	LineChartDefaultHeight = 10

	// LineChartDefaultYStep labels every other plot row, which over the eight rows
	// LineChartDefaultHeight leaves puts the ticks on quarters of the ceiling.
	LineChartDefaultYStep = 2

	// LineChartDefaultXTicks is the number of x-axis marks when XTicks is unset.
	// Four leaves a "1m30s"-sized label clear space either side on a default-width
	// chart.
	LineChartDefaultXTicks = 4

	// legendGlyph is the swatch beside each series in the legend: a full braille
	// cell, so it is drawn from the same ink as the line it stands for.
	legendGlyph = "⣿"

	// xAxisStep only has to be non-zero. It is what makes the underlying chart
	// reserve the two bottom rows for the x axis and its labels; the labels
	// themselves are suppressed and the row is laid out by drawXAxis.
	xAxisStep = 1
)

// chartEpoch is the wall-clock origin sample timestamps are built from. The
// underlying chart addresses points by absolute time, but a LineChart's samples
// are evenly spaced and labelled by position, so the instant chosen is irrelevant
// — it only has to be fixed, so the same samples always render to the same bytes.
var chartEpoch = time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)

// ChartSeries is one line on a [LineChart].
type ChartSeries struct {
	// Name labels the series in the legend. A chart with a single series, or one
	// whose series are all unnamed, renders no legend.
	Name string
	// Values are the samples, oldest first, taken at even intervals.
	Values []float64
}

// LineChart is a multi-series line chart drawn with braille dots, for embedding in
// output rather than driving interactively: it renders to a plain string.
//
// It wraps NimbleMarkets/ntcharts and adds what that library leaves to the caller
// — axis tick marks, x labels that stay on round values and always name both ends
// of the series, a legend, and per-series colors from [theme.SeriesStyles].
//
// Series are laid out on a common x domain: the longest spans the full width and a
// shorter one simply stops early, so overlaid series that ran for different lengths
// of time stay comparable and one finishing sooner shows as such rather than being
// stretched to match.
type LineChart struct {
	Series []ChartSeries

	// Ceiling is the top of the y axis — typically a known limit, so that a line
	// near the top reads as approaching it. Values above it are clamped. Zero or
	// less means "the largest value across every series", which plots relative
	// shape instead.
	Ceiling float64

	// Width and Height are the total size in cells, including the y-axis gutter and
	// the two rows the x axis and its labels take. Zero means the
	// LineChartDefault… value.
	Width, Height int

	// YStep is the interval in rows between y-axis ticks. Zero means
	// [LineChartDefaultYStep].
	//
	// A tick's value is derived from the y range divided by the number of plot rows,
	// so a plot height that YStep does not divide cleanly yields ticks at unround
	// values like "2.29". Height less two — the rows the x axis occupies — wants to
	// be a multiple of YStep, and ideally a power of two.
	YStep int

	// XTicks is how many marks to place along the x axis, spread evenly with the
	// outermost two under the first and last samples. Zero means
	// [LineChartDefaultXTicks]. Marks whose labels will not fit are dropped, so this
	// is an upper bound rather than a promise.
	XTicks int

	// FormatY renders a y-axis tick value. Nil formats with two decimal places.
	FormatY func(float64) string

	// FormatX renders an x-axis tick, given how far along the axis it sits as a
	// fraction from 0 (the first sample) to 1 (the last). Taking a fraction rather
	// than a timestamp keeps this component free of any notion of what the x axis
	// measures — elapsed time, request count, anything. Nil labels the axis with the
	// fraction as a percentage.
	FormatX func(frac float64) string

	// Color enables the per-series colors that tell overlaid lines apart. Pass
	// iostream.ColorEnabled(ctx). With it off every line is drawn in the same ink,
	// so a caller overlaying more than one series should consider rendering a chart
	// per series instead.
	Color bool
}

// Render draws the chart as a newline-separated block with no trailing newline:
// the plot, the labelled axes, and a legend row when more than one series is
// named. A chart with no samples renders as the empty string, so the caller can
// substitute its own "no data" wording.
//
// The result is plain text plus, when Color is set, the per-series escape
// sequences. It carries no cursor movement, so it is equally correct piped to a
// file, embedded in a markdown code fence, or printed to a terminal.
func (c LineChart) Render() string {
	longest := 0
	for _, s := range c.Series {
		longest = max(longest, len(s.Values))
	}
	if longest == 0 {
		return ""
	}

	width, height := c.Width, c.Height
	if width <= 0 {
		width = LineChartDefaultWidth
	}
	if height <= 0 {
		height = LineChartDefaultHeight
	}
	yStep := c.YStep
	if yStep <= 0 {
		yStep = LineChartDefaultYStep
	}

	// The samples are evenly spaced, so the span they are laid out over is
	// arbitrary; one second per sample keeps the numbers small. FormatX labels the
	// axis by position, so nothing downstream sees this unit.
	span := time.Duration(longest-1) * time.Second
	if span <= 0 {
		// A single sample spans no time at all, which leaves the x axis with nothing
		// to scale by.
		span = time.Second
	}

	chart := timeserieslinechart.New(width, height,
		timeserieslinechart.WithYRange(0, c.ceiling()),
		timeserieslinechart.WithTimeRange(chartEpoch, chartEpoch.Add(span)),
		timeserieslinechart.WithXYSteps(xAxisStep, yStep),
		// The axes and their labels stay unstyled: the per-series colors are what
		// carry meaning, and coloring the frame as well only competes with them.
		timeserieslinechart.WithAxesStyles(lipgloss.NewStyle(), lipgloss.NewStyle()),
		timeserieslinechart.WithYLabelFormatter(func(_ int, v float64) string { return c.formatY(v) }),
		// x labels are suppressed here and laid out by drawXAxis; see there for why.
		timeserieslinechart.WithXLabelFormatter(func(int, float64) string { return "" }),
	)
	for i, s := range c.Series {
		chart.SetDataSetStyle(s.Name, c.seriesStyle(i))
		for j, v := range s.Values {
			chart.PushDataSet(s.Name, timeserieslinechart.TimePoint{
				Time:  chartEpoch.Add(time.Duration(j) * time.Second),
				Value: v,
			})
		}
	}
	chart.DrawBrailleAll()
	c.drawXAxis(&chart)
	drawYAxisTicks(&chart)

	plot := TrimTrailingSpace(chart.View())
	if legend := c.legend(chart.Origin().X + 1); legend != "" {
		plot += "\n" + legend
	}
	return plot
}

// ceiling resolves the top of the y axis: the caller's Ceiling when set, otherwise
// the largest sample present, so a chart with no known limit still shows relative
// shape rather than dividing by zero.
func (c LineChart) ceiling() float64 {
	if c.Ceiling > 0 {
		return c.Ceiling
	}
	var peak float64
	for _, s := range c.Series {
		for _, v := range s.Values {
			peak = max(peak, v)
		}
	}
	if peak <= 0 {
		return 1
	}
	return peak
}

func (c LineChart) formatY(v float64) string {
	if c.FormatY == nil {
		return defaultFormatY(v)
	}
	return c.FormatY(v)
}

func (c LineChart) formatX(frac float64) string {
	if c.FormatX == nil {
		return defaultFormatX(frac)
	}
	return c.FormatX(frac)
}

// seriesStyle is the color for series i, or no color at all when the caller has
// none.
func (c LineChart) seriesStyle(i int) lipgloss.Style {
	if !c.Color {
		return lipgloss.NewStyle()
	}
	return theme.SeriesStyle(i)
}

// legend names each series beside a swatch in its own color, indented to line up
// with the plot. The underlying library draws no legend, and an overlaid chart is
// unreadable without one. A single series needs none: it has nothing to be
// distinguished from.
func (c LineChart) legend(indent int) string {
	if len(c.Series) < 2 {
		return ""
	}
	named := false
	for _, s := range c.Series {
		if s.Name != "" {
			named = true
			break
		}
	}
	if !named {
		return ""
	}

	parts := make([]string, len(c.Series))
	for i, s := range c.Series {
		parts[i] = c.seriesStyle(i).Render(legendGlyph) + " " + s.Name
	}
	return strings.Repeat(" ", indent) + strings.Join(parts, "  ")
}

// drawXAxis draws tick marks and labels onto the chart's x axis. The underlying
// library leaves the rule bare and steps its own labels by column, which puts them
// on unround values and drops whichever would overrun the canvas — usually the
// last, so the end of the series goes unnamed. Here the ticks are spread across the
// plot with the outermost two pinned to the first and last samples, and each is
// marked on the rule so a label can be tied to a column.
//
// This writes into the chart's canvas rather than patching its rendered output.
// Rendered rows carry the per-series ANSI escapes, so indexing one by rune lands in
// the wrong column — or inside an escape sequence — as soon as a data line reaches
// the bottom row of the plot. Canvas cells are addressed exactly. Writing a tick
// over the junction glyph the library leaves where a line meets the axis is
// deliberate too: the mark says more than the junction, and the two side by side
// render as "┬┴", which reads as corruption.
func (c LineChart) drawXAxis(chart *timeserieslinechart.Model) {
	axis, plotW := chart.Origin(), chart.GraphWidth()
	if plotW < 2 || axis.Y+1 >= chart.Height() {
		return
	}

	ticks := c.XTicks
	if ticks <= 0 {
		ticks = LineChartDefaultXTicks
	}
	cols, labels := c.xTicks(min(ticks, plotW), plotW)
	labelRow, marked := layoutAxisLabels(labels, cols, plotW)

	for _, col := range marked {
		// The rule starts one cell right of the corner where the axes meet.
		chart.Canvas.SetRune(canvas.Point{X: axis.X + 1 + col, Y: axis.Y}, '┬')
	}
	chart.Canvas.SetString(canvas.Point{X: axis.X + 1, Y: axis.Y + 1}, labelRow)
}

// xTicks resolves the tick columns and their labels, spread evenly across the plot
// with the outermost two under the first and last samples.
func (c LineChart) xTicks(n, plotW int) (cols []int, labels []string) {
	cols = make([]int, 0, n)
	labels = make([]string, 0, n)
	for i := range n {
		frac := 0.0
		if n > 1 {
			frac = float64(i) / float64(n-1)
		}
		cols = append(cols, int(math.Round(frac*float64(plotW-1))))
		labels = append(labels, c.formatX(frac))
	}
	return cols, labels
}

// drawYAxisTicks marks the y axis at every row carrying a tick label. The
// underlying library draws the axis as a plain vertical rule, which leaves each
// label describing a row with nothing tying it to one.
//
// Which rows are labelled is read off the canvas — a row is labelled if the cell to
// the left of the axis is not empty — rather than recomputed. Label placement
// belongs to the library, which suppresses a label that would repeat the value
// above it, so recomputing here would eventually disagree with what is drawn.
func drawYAxisTicks(chart *timeserieslinechart.Model) {
	axis := chart.Origin()
	if axis.X < 1 {
		return // no gutter, so no labels to mark
	}
	for y := max(axis.Y-chart.GraphHeight(), 0); y < axis.Y; y++ {
		// The axis row itself is skipped: its label sits beside the corner where the
		// two axes meet, and a corner is already a mark.
		if chart.Canvas.Cell(canvas.Point{X: axis.X - 1, Y: y}).Rune != runes.Null {
			chart.Canvas.SetRune(canvas.Point{X: axis.X, Y: y}, '┤')
		}
	}
}

// layoutAxisLabels places tick labels on one row plotW wide: the first flush left
// from its mark, the last flush right to its mark, and the rest centred on theirs.
// A label that would touch one already placed is dropped along with its tick mark —
// the two ends are the ones worth keeping, and a row of collided half-labels is
// worse than a sparse one. The returned columns are the ticks whose labels
// survived, so the rule only marks what it can name.
func layoutAxisLabels(labels []string, cols []int, plotW int) (row string, marked []int) {
	out := []rune(strings.Repeat(" ", plotW))
	taken := make([]bool, plotW)

	put := func(i, start int) {
		label := []rune(labels[i])
		if len(label) == 0 || len(label) > plotW {
			return
		}
		start = min(max(start, 0), plotW-len(label))
		// Require a clear cell either side so neighbouring labels never run together
		// into one unreadable string.
		for j := max(start-1, 0); j < min(start+len(label)+1, plotW); j++ {
			if taken[j] {
				return
			}
		}
		copy(out[start:], label)
		for j := start; j < start+len(label); j++ {
			taken[j] = true
		}
		marked = append(marked, cols[i])
	}

	// The ends go down first, so a crowded interior label can never displace them.
	last := len(labels) - 1
	put(0, cols[0])
	if last > 0 {
		put(last, cols[last]-len([]rune(labels[last]))+1)
	}
	for i := 1; i < last; i++ {
		put(i, cols[i]-len([]rune(labels[i]))/2)
	}

	slices.Sort(marked)
	return strings.TrimRight(string(out), " "), marked
}

// defaultFormatY is the y-axis tick formatter used when a chart supplies none.
func defaultFormatY(v float64) string { return fmt.Sprintf("%.2f", v) }

// defaultFormatX is the x-axis tick formatter used when a chart supplies none: the
// position along the axis as a percentage, which says something useful without
// assuming what the axis measures.
func defaultFormatX(frac float64) string { return fmt.Sprintf("%.0f%%", frac*100) }
