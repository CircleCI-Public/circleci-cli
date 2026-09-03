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
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/MakeNowJust/heredoc"
	"github.com/google/uuid"
	"github.com/spf13/cobra"

	clierrors "github.com/CircleCI-Public/circleci-cli/clikit/errors"
	"github.com/CircleCI-Public/circleci-cli/clikit/iostream"
	"github.com/CircleCI-Public/circleci-cli/clikit/mdtable"
	"github.com/CircleCI-Public/circleci-cli/clikit/ui/components"
	"github.com/CircleCI-Public/circleci-cli/internal/apiclient"
	"github.com/CircleCI-Public/circleci-cli/internal/cmdutil"
)

// allExecutions is the --execution default: every parallel execution rather than
// one. A real execution index is never negative, so it cannot collide.
const allExecutions = -1

// Values for --chart, which decides how a parallel job's executions are plotted.
const (
	// chartAuto overlays the executions when they can be told apart — at most
	// maxCombinedSeries of them, and color available to distinguish them — and
	// falls back to a chart each otherwise.
	chartAuto = "auto"
	// chartCombined always overlays, however many executions there are.
	chartCombined = "combined"
	// chartSeparate always draws one chart per execution.
	chartSeparate = "separate"
)

// chartModes are the accepted --chart values, in the order the error message
// lists them.
var chartModes = []string{chartAuto, chartCombined, chartSeparate}

// maxCombinedSeries is how many executions "auto" will overlay on one chart.
// Past five, the palette stops offering reliably distinguishable colors (see
// theme.SeriesStyles) and a chart each is easier to read than a thicket.
const maxCombinedSeries = 5

// resourceUsageOutput is the typed output of "circleci job resource-usage get".
type resourceUsageOutput struct {
	ID            uuid.UUID                  `json:"id"`
	ResourceClass apiclient.JobResourceClass `json:"resource_class"`
	Executions    []executionUsageOutput     `json:"executions"`
}

// executionUsageOutput is one parallel execution's series, plus the summary
// statistics the rendered view puts in its table. The raw series are carried
// verbatim so a script can compute its own.
type executionUsageOutput struct {
	Index           int          `json:"execution"`
	IntervalMS      int          `json:"interval_ms"`
	Samples         int          `json:"samples"`
	DurationSeconds float64      `json:"duration_seconds"`
	CPU             *metricStats `json:"cpu,omitempty"`
	Memory          *metricStats `json:"memory,omitempty"`
	CPUCores        []float64    `json:"cpu_cores"`
	MemoryBytes     []int64      `json:"memory_bytes"`
	NetworkRxBytes  int64        `json:"network_rx_bytes"`
	NetworkTxBytes  int64        `json:"network_tx_bytes"`
}

// metricStats summarises one series. PeakPercentOfLimit is the headline number —
// how close the job came to exhausting the resource class — and is omitted when
// the API reported no limit to measure against.
type metricStats struct {
	Min                float64  `json:"min"`
	Mean               float64  `json:"mean"`
	Max                float64  `json:"max"`
	PeakPercentOfLimit *float64 `json:"peak_percent_of_limit,omitempty"`
}

func newResourceUsageGetCmd() *cobra.Command {
	var (
		execution int
		chartMode string
		jsonOut   bool
	)

	cmd := &cobra.Command{
		Use:   "get <job-id>",
		Short: "Chart a job's CPU and memory usage",
		Annotations: map[string]string{
			"help:arguments": heredoc.Docf(`
				%[1]s<job-id>%[1]s is the UUID of the job whose usage to fetch. Job UUIDs are
				shown in the output of %[1]scircleci workflow get%[1]s and %[1]scircleci job get%[1]s.
			`, "`"),
		},
		Long: heredoc.Doc(`
			Chart how much CPU and memory a job used against its resource class limits.
			Peak of limit sizes the executor: under 50% on both means a smaller class fits.
			Parallel executions are overlaid in color, or charted apart past 5 (--chart).

			JSON fields: id, resource_class.name/cpu_count/memory_limit_bytes, executions[].execution/interval_ms/samples/duration_seconds/cpu_cores/memory_bytes/network_rx_bytes/network_tx_bytes/cpu.min/mean/max/peak_percent_of_limit (and memory.*)
		`),
		Example: heredoc.Doc(`
			# Chart a job's CPU and memory usage
			$ circleci job resource-usage get 0dc4d8df-8f7e-41b0-a3ef-88066a5465c1

			# One chart per execution, however few there are
			$ circleci job resource-usage get 0dc4d8df-8f7e-41b0-a3ef-88066a5465c1 --chart separate

			# Only the third parallel execution
			$ circleci job resource-usage get 0dc4d8df-8f7e-41b0-a3ef-88066a5465c1 --execution 2

			# How close each execution came to its memory limit
			$ circleci job resource-usage get 0dc4d8df-8f7e-41b0-a3ef-88066a5465c1 --json \
			    | jq '.executions[].memory.peak_percent_of_limit'
		`),
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if cliErr := cmdutil.RequireArgs(args, "job-id"); cliErr != nil {
				return cliErr
			}
			jobID, err := uuid.Parse(args[0])
			if err != nil {
				return clierrors.New("args.invalid_job_id", "Invalid job ID",
					fmt.Sprintf("%q is not a valid job UUID.", args[0])).
					WithExitCode(clierrors.ExitBadArguments)
			}
			if err := validateChartMode(chartMode); err != nil {
				return err
			}
			ctx := cmd.Context()
			client, err := cmdutil.LoadClient(ctx)
			if err != nil {
				return err
			}
			return runResourceUsageGet(ctx, client, jobID, execution, chartMode, jsonOut)
		},
	}

	cmd.Flags().IntVar(&execution, "execution", allExecutions, "Parallel execution index to report on (default all)")
	cmd.Flags().StringVar(&chartMode, "chart", chartAuto, "Plot parallel executions together or apart: auto|combined|separate")
	cmdutil.AddJSONFlag(cmd, &jsonOut)
	cmdutil.AddJQFlag(cmd)

	return cmd
}

func validateChartMode(mode string) error {
	for _, m := range chartModes {
		if mode == m {
			return nil
		}
	}
	return clierrors.New("args.invalid_chart_mode", "Invalid --chart value",
		fmt.Sprintf("%q is not a chart mode.", mode)).
		WithSuggestions("Use one of: " + strings.Join(chartModes, ", ")).
		WithExitCode(clierrors.ExitBadArguments)
}

func runResourceUsageGet(ctx context.Context, client *apiclient.Client, jobID uuid.UUID, execution int, chartMode string, jsonOut bool) error {
	out, err := fetchResourceUsage(ctx, client, jobID, execution)
	if err != nil {
		return err
	}

	if jsonOut {
		return iostream.PrintJSON(ctx, out)
	}

	iostream.PrintMarkdown(ctx, resourceUsageMarkdown(out, chartMode, iostream.ColorEnabled(ctx)))
	return nil
}

// ResourceUsageGet renders a job's usage report exactly as "circleci job
// resource-usage get" does, covering every execution. It is exported so
// interactive callers (e.g. "circleci run get") can reuse the same output
// without duplicating the formatting code.
func ResourceUsageGet(ctx context.Context, client *apiclient.Client, jobID uuid.UUID, jsonOut bool) error {
	return runResourceUsageGet(ctx, client, jobID, allExecutions, chartAuto, jsonOut)
}

// ResourceUsageMarkdown assembles a job's usage report as markdown, for the
// interactive run-get flow's in-flow "resource usage" pager. It mirrors
// runResourceUsageGet's non-JSON path but returns the markdown rather than
// paging it, and always covers every execution — the option is job-level.
func ResourceUsageMarkdown(ctx context.Context, client *apiclient.Client, jobID uuid.UUID) (string, error) {
	out, err := fetchResourceUsage(ctx, client, jobID, allExecutions)
	if err != nil {
		return "", err
	}
	return resourceUsageMarkdown(out, chartAuto, iostream.ColorEnabled(ctx)), nil
}

// fetchResourceUsage loads a job's usage and folds in the summary statistics.
// execution selects a single parallel execution, or allExecutions for every one.
func fetchResourceUsage(ctx context.Context, client *apiclient.Client, jobID uuid.UUID, execution int) (*resourceUsageOutput, error) {
	usage, err := client.GetJobResourceUsage(ctx, jobID)
	if err != nil {
		return nil, cmdutil.APIErr(err, jobID.String(), "job.resource_usage_not_found",
			"No resource usage recorded for job %q.",
			"Usage is only recorded for jobs that ran an executor — an approval job, or one cancelled before it started, has none.",
			"Check the job exists with: circleci job get "+jobID.String())
	}
	cmdutil.TrackKnownID(ctx, cmdutil.KeyJobID, usage.ID)

	execs := usage.Executions
	if execution != allExecutions {
		found := false
		for _, e := range execs {
			if e.Index == execution {
				execs, found = []apiclient.JobResourceUsageExecution{e}, true
				break
			}
		}
		if !found {
			return nil, usageExecutionNotFound(usage, execution)
		}
	}

	out := &resourceUsageOutput{ID: usage.ID, ResourceClass: usage.ResourceClass}
	for _, e := range execs {
		out.Executions = append(out.Executions, executionUsage(e, usage.ResourceClass))
	}
	return out, nil
}

// usageExecutionNotFound reports an --execution index the job did not run,
// naming the range that is valid.
func usageExecutionNotFound(usage *apiclient.JobResourceUsage, execution int) error {
	return clierrors.New("job.execution_not_found", "Execution not found",
		fmt.Sprintf("Job %q recorded no usage for execution %d.", usage.ID, execution)).
		WithSuggestions("This job recorded " + usageExecutionCount(len(usage.Executions)) +
			"; omit --execution to report on all of them.").
		WithExitCode(clierrors.ExitNotFound)
}

func usageExecutionCount(n int) string {
	switch n {
	case 0:
		return "no executions"
	case 1:
		return "1 execution (index 0)"
	default:
		return fmt.Sprintf("%d executions (indexes 0-%d)", n, n-1)
	}
}

// executionUsage adapts one execution to the output shape, truncating both
// series to the number of samples they share so a chart column means the same
// instant on each.
func executionUsage(e apiclient.JobResourceUsageExecution, rc apiclient.JobResourceClass) executionUsageOutput {
	n := e.Samples()
	cpu, memory := e.CPUCores[:n], e.MemoryBytes[:n]

	out := executionUsageOutput{
		Index:           e.Index,
		IntervalMS:      e.IntervalMS,
		Samples:         n,
		DurationSeconds: e.Duration().Seconds(),
		CPUCores:        cpu,
		MemoryBytes:     memory,
		NetworkRxBytes:  e.NetworkRxBytes,
		NetworkTxBytes:  e.NetworkTxBytes,
	}
	out.CPU = summarise(cpu, rc.CPUCount)
	out.Memory = summarise(bytesToFloats(memory), float64(rc.MemoryLimitBytes))
	return out
}

// summarise computes a series' min, mean, max and peak as a percentage of limit.
// It returns nil for an empty series — an execution too short to sample has no
// statistics, as distinct from one that measured zero. A limit of zero or less
// (the API reporting none) leaves PeakPercentOfLimit unset rather than dividing
// by it.
func summarise(values []float64, limit float64) *metricStats {
	if len(values) == 0 {
		return nil
	}
	stats := &metricStats{Min: values[0], Max: values[0]}
	var sum float64
	for _, v := range values {
		sum += v
		stats.Min = min(stats.Min, v)
		stats.Max = max(stats.Max, v)
	}
	stats.Mean = sum / float64(len(values))
	if limit > 0 {
		pct := stats.Max / limit * 100
		stats.PeakPercentOfLimit = &pct
	}
	return stats
}

func bytesToFloats(vs []int64) []float64 {
	out := make([]float64, len(vs))
	for i, v := range vs {
		out[i] = float64(v)
	}
	return out
}

// --- rendering ---

// usageMetric describes one of the two things measured, so the combined and
// per-execution layouts can render both from the same code rather than
// duplicating it once for CPU and once for memory.
type usageMetric struct {
	// Title heads the metric's chart section.
	Title string
	// Limit is the resource class ceiling the chart is drawn against, and Ceiling
	// the same figure as a chart scale (zero when the API reported no limit, at
	// which point the chart scales to the data instead).
	Limit   string
	Ceiling float64
	// Format renders a measured value in the metric's own unit, for the table.
	Format func(float64) string
	// FormatTick renders a y-axis tick. It is Format without the padding that
	// keeps a table column aligned: on an axis, "4" reads better than "4.00".
	FormatTick func(float64) string
	// Series extracts the metric's samples from one execution.
	Series func(executionUsageOutput) []float64
	// Stats extracts the metric's precomputed summary from one execution.
	Stats func(executionUsageOutput) *metricStats
}

func usageMetrics(rc apiclient.JobResourceClass) []usageMetric {
	return []usageMetric{
		{
			Title:      "CPU (cores)",
			Limit:      cpuLimit(rc),
			Ceiling:    rc.CPUCount,
			Format:     formatCores,
			FormatTick: formatCoresTick,
			Series:     func(e executionUsageOutput) []float64 { return e.CPUCores },
			Stats:      func(e executionUsageOutput) *metricStats { return e.CPU },
		},
		{
			Title:      "Memory",
			Limit:      memoryLimit(rc),
			Ceiling:    float64(rc.MemoryLimitBytes),
			Format:     formatBytes,
			FormatTick: formatBytesTick,
			Series:     func(e executionUsageOutput) []float64 { return bytesToFloats(e.MemoryBytes) },
			Stats:      func(e executionUsageOutput) *metricStats { return e.Memory },
		},
	}
}

// resourceUsageMarkdown builds the usage report as markdown: the resource class
// and its limits, then the charts. A parallel job is laid out metric-major, with
// every execution overlaid on one chart per metric, or execution-major with a
// chart each — see combineExecutions for which. The charts sit in code fences so
// glamour reproduces their grid verbatim, and passes their colors through, rather
// than reflowing them as prose.
func resourceUsageMarkdown(u *resourceUsageOutput, chartMode string, color bool) string {
	var md strings.Builder
	md.WriteString("# Job Resource Usage\n")
	_, _ = fmt.Fprintf(&md, "- ID: `%s`\n", u.ID)
	_, _ = fmt.Fprintf(&md, "- Resource class: %s\n", resourceClassName(u.ResourceClass))
	if u.ResourceClass.CPUCount > 0 {
		_, _ = fmt.Fprintf(&md, "- CPU limit: %s\n", cpuLimit(u.ResourceClass))
	}
	if u.ResourceClass.MemoryLimitBytes > 0 {
		_, _ = fmt.Fprintf(&md, "- Memory limit: %s\n", memoryLimit(u.ResourceClass))
	}
	if len(u.Executions) > 1 {
		_, _ = fmt.Fprintf(&md, "- Parallel executions: %d\n", len(u.Executions))
	}
	// The sampling interval is a property of the job, not of one series, so it is
	// stated once here rather than repeated against every execution.
	interval, uniform := commonInterval(u.Executions)
	if uniform {
		_, _ = fmt.Fprintf(&md, "- Sample interval: %s\n", usageDuration(interval))
	}

	switch {
	case len(u.Executions) == 0:
		md.WriteString("\nNo executions recorded any usage.\n")
	case combineExecutions(chartMode, len(u.Executions), color):
		md.WriteString(combinedUsageMarkdown(u, color, uniform))
	default:
		for _, e := range u.Executions {
			if len(u.Executions) == 1 {
				md.WriteString("\n## Usage\n")
			} else {
				_, _ = fmt.Fprintf(&md, "\n## Execution %d\n", e.Index)
			}
			md.WriteString(executionUsageMarkdown(e, u.ResourceClass, color, uniform))
		}
	}

	return md.String()
}

// commonInterval is the sampling interval every execution shares. In practice
// they always do — the platform samples on one fixed clock — which is why the
// figure is reported once in the header rather than per execution. The bool
// guards against that stopping being true: when the executions disagree, or there
// are none, the per-execution views state their own interval instead so the
// report never prints one execution's figure as though it applied to all.
func commonInterval(execs []executionUsageOutput) (time.Duration, bool) {
	if len(execs) == 0 {
		return 0, false
	}
	ms := execs[0].IntervalMS
	for _, e := range execs[1:] {
		if e.IntervalMS != ms {
			return 0, false
		}
	}
	if ms <= 0 {
		return 0, false
	}
	return time.Duration(ms) * time.Millisecond, true
}

// combineExecutions decides whether to overlay a job's executions on one chart
// per metric. Only "auto" is a judgement call: it overlays when the lines can
// actually be told apart, which needs color and no more than maxCombinedSeries of
// them. Without color every line is drawn in the same ink, so separate charts say
// strictly more.
func combineExecutions(chartMode string, executions int, color bool) bool {
	if executions < 2 {
		return false
	}
	switch chartMode {
	case chartCombined:
		return true
	case chartSeparate:
		return false
	default:
		return color && executions <= maxCombinedSeries
	}
}

// combinedUsageMarkdown renders a parallel job metric-major: a sampling table,
// then one chart per metric with every execution overlaid on it. headerInterval
// reports that the header already stated the sampling interval, in which case the
// table does not carry a column repeating it.
func combinedUsageMarkdown(u *resourceUsageOutput, color, headerInterval bool) string {
	var md strings.Builder

	md.WriteString("\n## Sampling\n")
	headers := []string{"Execution", "Samples", "Duration"}
	if !headerInterval {
		headers = append(headers, "Interval")
	}
	headers = append(headers, "Network in", "Network out")

	sampling := mdtable.New(headers...)
	for _, e := range u.Executions {
		row := []string{
			strconv.Itoa(e.Index),
			strconv.Itoa(e.Samples),
			usageDuration(time.Duration(e.DurationSeconds * float64(time.Second))),
		}
		if !headerInterval {
			row = append(row, usageDuration(time.Duration(e.IntervalMS)*time.Millisecond))
		}
		row = append(row,
			formatBytes(float64(e.NetworkRxBytes)),
			formatBytes(float64(e.NetworkTxBytes)),
		)
		sampling.Row(row...)
	}
	md.WriteString(sampling.Render())

	for _, m := range usageMetrics(u.ResourceClass) {
		_, _ = fmt.Fprintf(&md, "\n## %s\n\n", combinedMetricTitle(m, u.Executions))

		table := mdtable.New("Execution", "Min", "Mean", "Max", "Peak of limit", "Shape")
		for _, e := range u.Executions {
			table.Row(append([]string{strconv.Itoa(e.Index)}, statsCells(m.Stats(e), m.Format, m.Series(e))...)...)
		}
		md.WriteString(table.Render())
		if block := usageChartBlock(usageSeries(m, u.Executions), m, longestDuration(u.Executions), color); block != "" {
			md.WriteString("\n" + block)
		}
	}

	return md.String()
}

// executionUsageMarkdown renders one execution: its sampling metadata, a summary
// table with a row per metric, and a chart per metric. headerInterval reports that
// the header already stated the sampling interval, in which case the sample count
// here does not repeat it.
func executionUsageMarkdown(e executionUsageOutput, rc apiclient.JobResourceClass, color, headerInterval bool) string {
	var md strings.Builder

	interval := time.Duration(e.IntervalMS) * time.Millisecond
	duration := time.Duration(e.DurationSeconds * float64(time.Second))
	if e.Samples == 0 {
		// Naming the interval is the whole explanation here, so this line states
		// it whether or not the header did.
		_, _ = fmt.Fprintf(&md, "\nNo samples recorded — usage is sampled every %s, and this execution did not run that long.\n",
			usageDuration(interval))
		return md.String()
	}

	if headerInterval {
		_, _ = fmt.Fprintf(&md, "- Samples: %d (%s)\n", e.Samples, usageDuration(duration))
	} else {
		_, _ = fmt.Fprintf(&md, "- Samples: %d, every %s (%s)\n", e.Samples, usageDuration(interval), usageDuration(duration))
	}
	_, _ = fmt.Fprintf(&md, "- Network: %s in, %s out\n\n",
		formatBytes(float64(e.NetworkRxBytes)), formatBytes(float64(e.NetworkTxBytes)))

	metrics := usageMetrics(rc)
	table := mdtable.New("Metric", "Min", "Mean", "Max", "Limit", "Peak of limit", "Shape")
	for _, m := range metrics {
		stats, values := m.Stats(e), m.Series(e)
		cells := statsCells(stats, m.Format, values)
		// The limit belongs between Max and Peak of limit, where the percentage
		// it is a percentage of reads left to right.
		row := append([]string{m.Title}, cells[:3]...)
		row = append(row, m.Limit)
		table.Row(append(row, cells[3:]...)...)
	}
	md.WriteString(table.Render())

	for _, m := range metrics {
		title := m.Title
		if stats := m.Stats(e); stats != nil {
			title = metricPeakTitle(m.Title, stats, m.Format, m.Limit)
		}
		_, _ = fmt.Fprintf(&md, "\n### %s\n\n", title)
		md.WriteString(usageChartBlock(usageSeries(m, []executionUsageOutput{e}), m, duration, color))
	}

	return md.String()
}

// statsCells renders a metric's min/mean/max, peak-of-limit and inline shape.
// A nil stats (no samples) renders as dashes rather than zeroes, which would read
// as "measured, and idle".
func statsCells(s *metricStats, format func(float64) string, values []float64) []string {
	if s == nil {
		return []string{"-", "-", "-", "-", "-"}
	}
	peak := "-"
	if s.PeakPercentOfLimit != nil {
		peak = fmt.Sprintf("%.0f%%", *s.PeakPercentOfLimit)
	}
	return []string{format(s.Min), format(s.Mean), format(s.Max), peak, usageShape(values)}
}

// combinedMetricTitle heads an overlaid chart with the highest peak any execution
// reached and how much of the limit that was — the summary the per-execution
// table below it then breaks down.
func combinedMetricTitle(m usageMetric, execs []executionUsageOutput) string {
	var worst *metricStats
	for _, e := range execs {
		s := m.Stats(e)
		if s != nil && (worst == nil || s.Max > worst.Max) {
			worst = s
		}
	}
	if worst == nil {
		return m.Title
	}
	return metricPeakTitle(m.Title, worst, m.Format, m.Limit) + ", worst execution"
}

// metricPeakTitle names the peak a metric reached and, when there is a limit to
// measure against, what fraction of it that was. The y axis already carries
// absolute values; the percentage is the number that answers whether the resource
// class was the right size.
func metricPeakTitle(name string, s *metricStats, format func(float64) string, limit string) string {
	title := fmt.Sprintf("%s — peak %s", name, format(s.Max))
	if s.PeakPercentOfLimit != nil {
		title += fmt.Sprintf(" of %s (%.0f%%)", limit, *s.PeakPercentOfLimit)
	}
	return title
}

// usageChartBlock wraps a rendered chart in a markdown code fence, so glamour
// reproduces its grid verbatim — and passes the per-series colors through —
// rather than reflowing it as prose. A chart with no samples yields nothing at
// all, so the caller does not have to guard for it.
func usageChartBlock(series []components.ChartSeries, m usageMetric, duration time.Duration, color bool) string {
	plot := usageChart(series, m, duration, color)
	if plot == "" {
		return ""
	}
	// The chart is text with no backticks of its own, so a three-backtick fence is
	// always enough; CodeFence keeps that decision in one place.
	fence := cmdutil.CodeFence(plot)
	return fmt.Sprintf("%stext\n%s\n%s\n", fence, plot, fence)
}

// longestDuration is the wall-clock span the widest series covers, which is what
// an overlaid chart's x axis runs to.
func longestDuration(execs []executionUsageOutput) time.Duration {
	var longest float64
	for _, e := range execs {
		longest = max(longest, e.DurationSeconds)
	}
	return time.Duration(longest * float64(time.Second))
}

// resourceClassName names the resource class, falling back when the API reported
// none so the report never shows a blank field.
func resourceClassName(rc apiclient.JobResourceClass) string {
	if rc.Name == "" {
		return "unknown"
	}
	return rc.Name
}

// cpuLimit names the resource class's core count. It drops the decimals a whole
// number does not need — a limit is a property of the executor, so "4 cores"
// reads better there than the "4.00" the measured columns are padded to.
func cpuLimit(rc apiclient.JobResourceClass) string {
	if rc.CPUCount <= 0 {
		return "-"
	}
	return strconv.FormatFloat(rc.CPUCount, 'f', -1, 64) + " cores"
}

func memoryLimit(rc apiclient.JobResourceClass) string {
	if rc.MemoryLimitBytes <= 0 {
		return "-"
	}
	return formatBytes(float64(rc.MemoryLimitBytes))
}

// formatCores renders a measured core count, always to two decimal places so a
// column of them lines up.
func formatCores(v float64) string {
	return fmt.Sprintf("%.2f", v)
}

// formatCoresTick is formatCores for a y-axis tick, where there is no column to
// line up with and a whole number of cores has no use for decimals: "4" rather
// than "4.00", but "1.5" kept as "1.5".
func formatCoresTick(v float64) string {
	return trimZeros(formatCores(v))
}

// formatBytesTick is formatBytes for a y-axis tick, dropping the decimals a whole
// number does not need: "8 GiB" rather than "8.00 GiB".
func formatBytesTick(v float64) string {
	num, unit, found := strings.Cut(formatBytes(v), " ")
	if !found {
		return num
	}
	return trimZeros(num) + " " + unit
}

// trimZeros drops a decimal fraction's trailing zeros, and the point along with
// them when nothing is left: "4.00" becomes "4" and "1.50" becomes "1.5". A value
// with no point is returned untouched, so it never eats a significant zero.
func trimZeros(s string) string {
	if !strings.Contains(s, ".") {
		return s
	}
	return strings.TrimSuffix(strings.TrimRight(s, "0"), ".")
}

// formatBytes renders a byte count in binary units (KiB, MiB, …) with three
// significant figures, which keeps a y axis of them the same width regardless of
// magnitude.
func formatBytes(v float64) string {
	const unit = 1024
	units := [...]string{"B", "KiB", "MiB", "GiB", "TiB", "PiB"}
	i := 0
	for v >= unit && i < len(units)-1 {
		v /= unit
		i++
	}
	switch {
	case i == 0:
		return fmt.Sprintf("%.0f B", v)
	case v < 10:
		return fmt.Sprintf("%.2f %s", v, units[i])
	case v < 100:
		return fmt.Sprintf("%.1f %s", v, units[i])
	default:
		return fmt.Sprintf("%.0f %s", v, units[i])
	}
}

// usageElapsed renders an x-axis tick: time elapsed since the first sample.
// Unlike usageDuration, zero means the start of the series rather than a span of
// unknown length.
func usageElapsed(d time.Duration) string {
	if d <= 0 {
		return "0s"
	}
	return usageDuration(d)
}

// usageDuration renders a sampling interval or elapsed span. Sub-second
// durations keep their own precision rather than rounding to seconds, so a
// 500ms interval does not print as "0s".
func usageDuration(d time.Duration) string {
	if d <= 0 {
		return "unknown"
	}
	if d < time.Second {
		return d.String()
	}
	d = d.Round(time.Second)
	h, m, s := int(d.Hours()), int(d.Minutes())%60, int(d.Seconds())%60
	switch {
	case h > 0:
		return fmt.Sprintf("%dh%dm%ds", h, m, s)
	case m > 0:
		return fmt.Sprintf("%dm%ds", m, s)
	default:
		return fmt.Sprintf("%ds", s)
	}
}
