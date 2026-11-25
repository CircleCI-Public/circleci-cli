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

package run

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/CircleCI-Public/circleci-cli/clikit/iostream"
	"github.com/CircleCI-Public/circleci-cli/clikit/mdtable"
	"github.com/CircleCI-Public/circleci-cli/internal/apiclient"
	"github.com/CircleCI-Public/circleci-cli/internal/cmdutil"
	"github.com/CircleCI-Public/circleci-cli/internal/ui"
)

// TimingReport prints every job in the run with its duration, slowest first,
// followed by the run's wall clock and the commands that drill into the slowest
// job. It is the output path for --timing on "run get" and never enters the TUI:
// the performance twin of FailureReport.
//
// It reads the same two endpoints "run get" does (the run's workflows and each
// workflow's jobs), so it costs no request per job. Output is written to stdout
// so it can be piped directly into an agent's context window.
func TimingReport(ctx context.Context, client *apiclient.Client, r *apiclient.RunV3) error {
	out, err := fetchRunOutput(ctx, client, r)
	if err != nil {
		return err
	}
	iostream.Print(ctx, timingReport(out))
	return nil
}

// timingRow is one job of the timing table, with the workflow it belongs to.
type timingRow struct {
	job      jobOutput
	workflow string
}

// duration is the job's run time; the bool is false until the job has ended.
func (t timingRow) duration() (time.Duration, bool) {
	if t.job.StartedAt == nil || t.job.StoppedAt == nil {
		return 0, false
	}
	return max(t.job.StoppedAt.Sub(*t.job.StartedAt), 0), true
}

// rank orders the rows: ended jobs first, then running, then not started.
func (t timingRow) rank() int {
	switch {
	case t.job.StoppedAt != nil:
		return 0
	case t.job.StartedAt != nil:
		return 1
	default:
		return 2
	}
}

// timingReport renders the timing report for a run: a one-line header, one
// table row per job sorted slowest first, and the next steps. A run with no
// jobs says so and nothing else.
func timingReport(r runGetOutput) string {
	var rows []timingRow
	for _, wf := range r.Workflows {
		for _, j := range wf.Jobs {
			rows = append(rows, timingRow{job: j, workflow: wf.Name})
		}
	}

	var out strings.Builder
	if len(rows) == 0 {
		out.WriteString("## job timing\n\nNo jobs found for this run.\n")
		return out.String()
	}

	// Ended jobs by duration, longest first; running jobs by start, earliest
	// first (the longest running so far); jobs not started keep their listing
	// order. Stable, so equal durations keep the workflow order.
	sort.SliceStable(rows, func(i, j int) bool {
		if ri, rj := rows[i].rank(), rows[j].rank(); ri != rj {
			return ri < rj
		}
		switch rows[i].rank() {
		case 0:
			di, _ := rows[i].duration()
			dj, _ := rows[j].duration()
			return di > dj
		case 1:
			return rows[i].job.StartedAt.Before(*rows[j].job.StartedAt)
		default:
			return false
		}
	})

	var (
		lastEnd time.Time
		jobTime time.Duration
		ended   int
	)
	for _, row := range rows {
		if d, ok := row.duration(); ok {
			jobTime += d
			ended++
			if row.job.StoppedAt.After(lastEnd) {
				lastEnd = *row.job.StoppedAt
			}
		}
	}

	fmt.Fprintf(&out, "## job timing: %d %s", len(rows), plural(len(rows), "job", "jobs"))
	if !lastEnd.IsZero() {
		fmt.Fprintf(&out, ", wall clock %s", ui.FormatElapsed(max(lastEnd.Sub(r.CreatedAt), 0)))
	}
	fmt.Fprintf(&out, ", run created %s\n\n", cmdutil.DisplayTimeSeconds(r.CreatedAt))

	table := mdtable.New("Job", "Workflow", "Status", "Duration", "Started", "ID")
	for _, row := range rows {
		j := row.job
		// A started job with no end is still running: "~" rather than a blank,
		// as the step rows do. One not started has neither column.
		dur, started := "", ""
		if d, ok := row.duration(); ok {
			dur = ui.FormatElapsed(d)
		} else if j.StartedAt != nil {
			dur = "~"
		}
		if j.StartedAt != nil {
			started = "+" + ui.FormatElapsed(max(j.StartedAt.Sub(r.CreatedAt), 0))
		}
		table.Row(j.Name, row.workflow, apiclient.PhaseOutcomeText(j.Phase, j.Outcome, j.CurrentOutcome),
			dur, started, j.ID.String())
	}
	out.WriteString(table.Render())

	out.WriteString("\n## next steps\n\n")
	if ended > 0 {
		fmt.Fprintf(&out, "- Wall clock is run creation to the last job end; the %d finished %s add up to %s.\n",
			ended, plural(ended, "job", "jobs"), ui.FormatElapsed(jobTime))
		slowest := rows[0].job
		fmt.Fprintf(&out, "- Step timings for the slowest job %q: circleci job get %s\n", slowest.Name, slowest.ID)
		fmt.Fprintf(&out, "- CPU and memory of the slowest job against its resource class: circleci job resource-usage get %s\n", slowest.ID)
	} else {
		fmt.Fprintf(&out, "- No job has finished yet; follow the run: circleci run watch %s\n", r.ID)
	}
	return out.String()
}

// plural picks the singular or plural word for n.
func plural(n int, singular, plural string) string {
	if n == 1 {
		return singular
	}
	return plural
}
