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

package job

import (
	"strconv"
	"time"

	"github.com/CircleCI-Public/circleci-cli/clikit/ui/components"
)

// usageShapeWidth caps the summary table's sparkline so a long series does not
// widen the table past the terminal.
const usageShapeWidth = 20

// usageSeries builds the chart series for one metric across the given executions,
// each named for the legend.
func usageSeries(m usageMetric, execs []executionUsageOutput) []components.ChartSeries {
	series := make([]components.ChartSeries, 0, len(execs))
	for _, e := range execs {
		series = append(series, components.ChartSeries{
			Name:   "exec " + strconv.Itoa(e.Index),
			Values: m.Series(e),
		})
	}
	return series
}

// usageChart renders one metric's series as a line chart. The y axis is the
// resource class limit, so a line near the top means the job was saturating its
// executor, and every execution of the job is drawn to the same scale and can be
// compared by eye. The x axis is labelled with time elapsed since the first sample.
//
// Colors are applied only when the caller has them, and survive the code fence the
// chart is embedded in — glamour passes escape sequences inside one through
// untouched.
func usageChart(series []components.ChartSeries, m usageMetric, duration time.Duration, color bool) string {
	return components.LineChart{
		Series:  series,
		Ceiling: m.Ceiling,
		FormatY: m.FormatTick,
		FormatX: func(frac float64) string {
			return usageElapsed(time.Duration(frac * float64(duration)))
		},
		Color: color,
	}.Render()
}

// usageShape is the summary table's inline sparkline. It is scaled to the series'
// own peak, not the resource class limit: showing usage against the limit is the
// chart's job, and this cell exists precisely to show the shape a chart squashed
// against a distant limit cannot.
func usageShape(values []float64) string {
	if len(values) == 0 {
		return "-"
	}
	return "`" + components.Sparkline(components.DownsamplePeaks(values, usageShapeWidth), 0) + "`"
}
