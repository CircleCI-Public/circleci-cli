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
	"time"

	"github.com/google/uuid"
)

// Event types, as they appear in each event's "type" field.
const (
	EventBranchPushed = "branch_pushed"
	EventRunStarted   = "run_started"
	EventJobFailed    = "job_failed"
	EventRunSummary   = "run_summary"
)

// Run outcomes reported in RunSummaryEvent.Outcome. The first three are the
// run's own result; the rest describe a preflight that stopped before the run
// ended.
const (
	OutcomeSucceeded   = "succeeded"
	OutcomeFailed      = "failed"
	OutcomeCanceled    = "canceled"
	OutcomeFailing     = "failing"
	OutcomeTimedOut    = "timed_out"
	OutcomeInterrupted = "interrupted"
)

// BranchPushedEvent reports the snapshot commit and the branch it was pushed
// to. The branch outlives the preflight only with --no-cleanup or --no-watch.
type BranchPushedEvent struct {
	Type   string    `json:"type"`
	ID     uuid.UUID `json:"preflight_id"`
	Branch string    `json:"branch"`
	Commit string    `json:"commit"`
	// Base is Commit's parent: where HEAD forked from BaseBranch, or HEAD
	// when the default branch could not be determined (BaseBranch empty).
	Base       string `json:"base"`
	BaseBranch string `json:"base_branch,omitempty"`
	// Changes is false when the working tree matched Base, in which case
	// Commit is Base itself.
	Changes bool `json:"changes"`
}

// RunStartedEvent reports the run the push triggered.
type RunStartedEvent struct {
	Type    string    `json:"type"`
	RunID   uuid.UUID `json:"run_id"`
	Project string    `json:"project"`
	Branch  string    `json:"branch"`
	Commit  string    `json:"commit"`
	URL     string    `json:"url"`
}

// JobFailedEvent reports one job that failed, as soon as it is seen.
type JobFailedEvent struct {
	Type string     `json:"type"`
	Job  JobFailure `json:"job"`
}

// JobFailure describes a failed job: where it ran, which steps failed, and
// which tests failed when the job stored test results.
type JobFailure struct {
	ID          uuid.UUID    `json:"id"`
	Name        string       `json:"name"`
	WorkflowID  uuid.UUID    `json:"workflow_id"`
	Workflow    string       `json:"workflow"`
	URL         string       `json:"url"`
	FailedSteps []FailedStep `json:"failed_steps"`
	// FailedTests holds at most MaxFailedTests entries; FailedTestCount is
	// the full number.
	FailedTests     []FailedTest `json:"failed_tests"`
	FailedTestCount int          `json:"failed_test_count"`
}

// FailedStep is one failed step of a failed job.
type FailedStep struct {
	Execution int    `json:"execution"`
	Num       int    `json:"num"`
	Name      string `json:"name"`
	ExitCode  *int   `json:"exit_code,omitempty"`
}

// FailedTest is one failed test from a job's stored test results.
type FailedTest struct {
	Classname string `json:"classname"`
	Name      string `json:"name"`
	Message   string `json:"message"`
}

// RunSummaryEvent is the last event of every watched preflight.
type RunSummaryEvent struct {
	Type    string    `json:"type"`
	RunID   uuid.UUID `json:"run_id"`
	Outcome string    `json:"outcome"`
	// Ended is false when the preflight stopped before the run did: at the
	// first failure, on timeout or on Ctrl+C.
	Ended      bool       `json:"ended"`
	DurationMS int64      `json:"duration_ms"`
	URL        string     `json:"url"`
	Errors     []RunError `json:"errors"`
	FailedJobs []JobRef   `json:"failed_jobs"`
	// RunCancelled is true when the preflight cancelled what was left of the
	// run on its way out.
	RunCancelled  bool   `json:"run_cancelled"`
	BranchDeleted bool   `json:"branch_deleted"`
	CleanupError  string `json:"cleanup_error,omitempty"`
}

// RunError is an error the run itself carries, such as a config that would
// not compile.
type RunError struct {
	Type    string `json:"type"`
	Message string `json:"message"`
}

// JobRef identifies a job in the run summary.
type JobRef struct {
	ID       uuid.UUID `json:"id"`
	Name     string    `json:"name"`
	Workflow string    `json:"workflow"`
	URL      string    `json:"url"`
}

// Progress is a snapshot of the run's workflows, reported each time one of
// them, or one of their jobs, changes status. It is for human display and is
// not part of the event stream.
type Progress struct {
	Elapsed   time.Duration
	Workflows []WorkflowProgress
}

// WorkflowProgress is one workflow's status in a Progress snapshot.
type WorkflowProgress struct {
	Name   string
	Status string
	Jobs   int
	Done   int
}

// Observer receives everything a preflight reports while it runs. The command
// layer implements one observer that writes the JSONL event stream and one that
// writes human-readable lines.
type Observer interface {
	BranchPushed(BranchPushedEvent) error
	WaitingForRun(branch string)
	RunStarted(RunStartedEvent) error
	Progress(Progress)
	JobFailed(JobFailedEvent) error
	RunSummary(RunSummaryEvent) error
}
