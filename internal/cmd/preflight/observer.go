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

package preflight

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/CircleCI-Public/circleci-cli/clikit/iostream"
	"github.com/CircleCI-Public/circleci-cli/internal/preflight"
	"github.com/CircleCI-Public/circleci-cli/internal/ui"
)

// jsonObserver writes the preflight's events to stdout, one JSON object per
// line, and nothing else: progress is for humans and is dropped.
type jsonObserver struct {
	ctx context.Context
}

func (o *jsonObserver) BranchPushed(e preflight.BranchPushedEvent) error {
	return iostream.PrintJSON(o.ctx, e)
}

func (o *jsonObserver) WaitingForRun(string) {}

func (o *jsonObserver) RunStarted(e preflight.RunStartedEvent) error {
	return iostream.PrintJSON(o.ctx, e)
}

func (o *jsonObserver) Progress(preflight.Progress) {}

func (o *jsonObserver) JobFailed(e preflight.JobFailedEvent) error {
	return iostream.PrintJSON(o.ctx, e)
}

func (o *jsonObserver) RunSummary(e preflight.RunSummaryEvent) error {
	return iostream.PrintJSON(o.ctx, e)
}

// textObserver reports the preflight as lines on stderr. It never redraws, so
// the output reads the same in a terminal, a pipe or a CI log.
type textObserver struct {
	ctx    context.Context
	branch string
	id     string
}

func (o *textObserver) BranchPushed(e preflight.BranchPushedEvent) error {
	o.branch, o.id = e.Branch, e.ID.String()
	base := "HEAD"
	if e.BaseBranch != "" {
		base = e.BaseBranch
	}
	if !e.Changes {
		iostream.ErrPrintf(o.ctx, "No changes from %s; pushed %s to %s\n", base, shortSHA(e.Commit), e.Branch)
		return nil
	}
	iostream.ErrPrintf(o.ctx, "Pushed working tree as %s (changes from %s) to %s\n", shortSHA(e.Commit), base, e.Branch)
	return nil
}

func (o *textObserver) WaitingForRun(branch string) {
	iostream.ErrPrintf(o.ctx, "Waiting for CircleCI to start a run on %s...\n", branch)
}

func (o *textObserver) RunStarted(e preflight.RunStartedEvent) error {
	iostream.ErrPrintf(o.ctx, "Run %s started: %s\n\n", e.RunID, e.URL)
	return nil
}

func (o *textObserver) Progress(p preflight.Progress) {
	parts := make([]string, 0, len(p.Workflows))
	for _, wf := range p.Workflows {
		parts = append(parts, fmt.Sprintf("%s=%s (%d/%d jobs)", wf.Name, wf.Status, wf.Done, wf.Jobs))
	}
	if len(parts) == 0 {
		parts = append(parts, "waiting for workflows")
	}
	iostream.ErrPrintf(o.ctx, "[%s]  %s\n", ui.FormatElapsed(p.Elapsed), strings.Join(parts, "  "))
}

func (o *textObserver) JobFailed(e preflight.JobFailedEvent) error {
	j := e.Job
	iostream.ErrPrintf(o.ctx, "%s Job %q failed in workflow %q: %s\n", iostream.SymbolFail(o.ctx), j.Name, j.Workflow, j.URL)
	for _, s := range j.FailedSteps {
		if s.ExitCode != nil {
			iostream.ErrPrintf(o.ctx, "    step %d %q exited %d\n", s.Num, s.Name, *s.ExitCode)
		} else {
			iostream.ErrPrintf(o.ctx, "    step %d %q failed\n", s.Num, s.Name)
		}
	}
	for _, t := range j.FailedTests {
		name := t.Name
		if t.Classname != "" {
			name = t.Classname + " " + t.Name
		}
		iostream.ErrPrintf(o.ctx, "    test %s: %s\n", name, firstLine(t.Message))
	}
	if more := j.FailedTestCount - len(j.FailedTests); more > 0 {
		iostream.ErrPrintf(o.ctx, "    … and %d more failed test(s)\n", more)
	}
	return nil
}

func (o *textObserver) RunSummary(e preflight.RunSummaryEvent) error {
	elapsed := ui.FormatElapsed(msDuration(e.DurationMS))
	iostream.ErrPrintf(o.ctx, "\n")
	switch e.Outcome {
	case preflight.OutcomeSucceeded:
		iostream.ErrPrintf(o.ctx, "%s Run %s succeeded (%s)\n", iostream.SymbolOK(o.ctx), e.RunID, elapsed)
	case preflight.OutcomeFailing:
		iostream.ErrPrintf(o.ctx, "%s Run %s has a failed job — stopping (%s)\n", iostream.SymbolFail(o.ctx), e.RunID, elapsed)
	case preflight.OutcomeTimedOut, preflight.OutcomeInterrupted:
		iostream.ErrPrintf(o.ctx, "Stopped watching run %s (%s)\n", e.RunID, elapsed)
	case preflight.OutcomeCanceled:
		iostream.ErrPrintf(o.ctx, "Run %s was cancelled (%s)\n", e.RunID, elapsed)
	default:
		iostream.ErrPrintf(o.ctx, "%s Run %s failed (%s)\n", iostream.SymbolFail(o.ctx), e.RunID, elapsed)
	}
	if e.RunCancelled {
		iostream.ErrPrintf(o.ctx, "Cancelled the rest of the run\n")
	}
	switch {
	case e.BranchDeleted:
		iostream.ErrPrintf(o.ctx, "Deleted branch %s\n", o.branch)
	case e.CleanupError != "":
		iostream.ErrPrintf(o.ctx, "%s Could not delete branch %s: %s\n  Retry with: circleci preflight cleanup --id %s\n",
			iostream.SymbolWarn(o.ctx), o.branch, e.CleanupError, o.id)
	default:
		o.kept()
	}
	return nil
}

// kept tells the user the branch outlives a --no-watch preflight.
func (o *textObserver) kept() {
	iostream.ErrPrintf(o.ctx, "Kept branch %s; delete it with: circleci preflight cleanup --id %s\n", o.branch, o.id)
}

func msDuration(ms int64) time.Duration {
	return time.Duration(ms) * time.Millisecond
}

func shortSHA(sha string) string {
	if len(sha) > 7 {
		return sha[:7]
	}
	return sha
}

func firstLine(s string) string {
	line, _, _ := strings.Cut(strings.TrimSpace(s), "\n")
	return line
}
