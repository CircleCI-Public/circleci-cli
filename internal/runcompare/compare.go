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

// Package runcompare compares two runs of a project — typically the same
// pipeline before and after a config change — by total running time and total
// credits.
package runcompare

import (
	"context"
	"fmt"
	"math"
	"net/http"
	"time"

	"github.com/google/uuid"
	"golang.org/x/sync/errgroup"

	clierrors "github.com/CircleCI-Public/circleci-cli/clikit/errors"
	"github.com/CircleCI-Public/circleci-cli/internal/apiclient"
	"github.com/CircleCI-Public/circleci-cli/internal/httpcl"
)

// API is the subset of *apiclient.Client that Compare uses.
type API interface {
	GetRunV3(ctx context.Context, id uuid.UUID) (*apiclient.RunV3, error)
	GetRunWorkflowsV3(ctx context.Context, runID uuid.UUID) ([]apiclient.WorkflowV3, error)
	SearchCharges(ctx context.Context, params apiclient.ChargeSearchParams) ([]apiclient.Charge, error)
}

// Report is the comparison of an old run against a new one.
type Report struct {
	OldRun RunSummary `json:"old_run"`
	NewRun RunSummary `json:"new_run"`
	Delta  Delta      `json:"delta"`
}

// RunSummary is one run's totals.
type RunSummary struct {
	ID        uuid.UUID `json:"id"`
	ProjectID uuid.UUID `json:"project_id"`
	// RunningSeconds is the run's wall clock: from the earliest workflow's
	// creation to the latest workflow's end.
	RunningSeconds float64 `json:"running_seconds"`
	// Credits is the sum of every charge recorded against the run.
	Credits int64 `json:"credits"`
}

// Delta is new minus old. The percentages are relative to the old value and
// nil when the old value is zero.
type Delta struct {
	RunningSeconds float64  `json:"running_seconds"`
	RunningPct     *float64 `json:"running_pct"`
	Credits        int64    `json:"credits"`
	CreditsPct     *float64 `json:"credits_pct"`
}

// FetchError reports which run an API call failed for, so the caller can name
// it in the error it shows.
type FetchError struct {
	RunID uuid.UUID
	Err   error
}

func (e *FetchError) Error() string { return fmt.Sprintf("run %s: %v", e.RunID, e.Err) }
func (e *FetchError) Unwrap() error { return e.Err }

// Charges are recorded asynchronously after each job, so the search window
// reaches past the run's end; the lead covers the endpoint rounding its bounds
// down to five minutes.
const (
	chargesLead = 5 * time.Minute
	chargesLag  = 24 * time.Hour
)

// Validate rejects a pair of run IDs that cannot be compared, without calling
// the API. Compare runs it too; callers run it first to fail before any work.
func Validate(oldID, newID uuid.UUID) error {
	if oldID == newID {
		return clierrors.New("run.compare_same_run", "Same run twice",
			fmt.Sprintf("Both arguments are run %s; there is nothing to compare.", oldID)).
			WithSuggestions("Pass the run from the old config first, then the run from the new config").
			WithExitCode(clierrors.ExitBadArguments)
	}
	return nil
}

// Compare summarises both runs and the difference between them. Both runs
// must have ended: a run still going has neither a final running time nor its
// final credits.
func Compare(ctx context.Context, api API, oldID, newID uuid.UUID) (*Report, error) {
	if err := Validate(oldID, newID); err != nil {
		return nil, err
	}

	var oldRun, newRun RunSummary
	g, gctx := errgroup.WithContext(ctx)
	g.Go(func() (err error) { oldRun, err = summarise(gctx, api, oldID); return err })
	g.Go(func() (err error) { newRun, err = summarise(gctx, api, newID); return err })
	if err := g.Wait(); err != nil {
		return nil, err
	}

	return &Report{
		OldRun: oldRun,
		NewRun: newRun,
		Delta: Delta{
			RunningSeconds: newRun.RunningSeconds - oldRun.RunningSeconds,
			RunningPct:     percent(newRun.RunningSeconds-oldRun.RunningSeconds, oldRun.RunningSeconds),
			Credits:        newRun.Credits - oldRun.Credits,
			CreditsPct:     percent(float64(newRun.Credits-oldRun.Credits), float64(oldRun.Credits)),
		},
	}, nil
}

func summarise(ctx context.Context, api API, id uuid.UUID) (RunSummary, error) {
	run, err := api.GetRunV3(ctx, id)
	if err != nil {
		return RunSummary{}, &FetchError{RunID: id, Err: err}
	}
	if run.Phase != apiclient.PhaseEnded {
		return RunSummary{}, clierrors.New("run.compare_not_ended", "Run has not ended",
			fmt.Sprintf("Run %s is %s; its running time and credits are not final yet.", id, run.Phase)).
			WithSuggestions("Wait for it to finish: circleci run watch " + id.String()).
			WithExitCode(clierrors.ExitBadArguments)
	}

	workflows, err := api.GetRunWorkflowsV3(ctx, id)
	// A run whose config failed to compile never creates workflows, and the
	// list endpoint answers 404 for it; that run simply ran for no time.
	if err != nil && !httpcl.HasStatusCode(err, http.StatusNotFound) {
		return RunSummary{}, &FetchError{RunID: id, Err: err}
	}
	first, last := span(workflows)

	end := last
	if end.IsZero() {
		end = run.CreatedAt
	}
	charges, err := api.SearchCharges(ctx, apiclient.ChargeSearchParams{
		Analysis:   apiclient.ChargeAnalysisWorkflow,
		ProjectIDs: []uuid.UUID{run.ProjectID},
		From:       run.CreatedAt.Add(-chargesLead),
		To:         end.Add(chargesLag),
		Filter:     runFilter(id),
	})
	if err != nil {
		return RunSummary{}, &FetchError{RunID: id, Err: err}
	}

	var running float64
	if !first.IsZero() && !last.IsZero() {
		running = last.Sub(first).Seconds()
	}
	return RunSummary{
		ID:             id,
		ProjectID:      run.ProjectID,
		RunningSeconds: running,
		Credits:        sumCredits(charges),
	}, nil
}

// runFilter scopes a charges search to one run. In the charge vocabulary a v3
// run is a pipeline: its ID is the charge's pipeline.id.
func runFilter(id uuid.UUID) string {
	return fmt.Sprintf("pipeline.id == %q", id.String())
}

// span returns the earliest workflow creation and the latest workflow end.
// Either is zero when no workflow supplies it.
func span(workflows []apiclient.WorkflowV3) (first, last time.Time) {
	for _, wf := range workflows {
		if first.IsZero() || wf.CreatedAt.Before(first) {
			first = wf.CreatedAt
		}
		if wf.EndedAt != nil && wf.EndedAt.After(last) {
			last = *wf.EndedAt
		}
	}
	return first, last
}

func sumCredits(charges []apiclient.Charge) int64 {
	var total int64
	for _, c := range charges {
		total += c.Credits
	}
	return total
}

// percent returns change as a percentage of base, to one decimal place, or nil
// when base is zero and the percentage is undefined.
func percent(change, base float64) *float64 {
	if base == 0 {
		return nil
	}
	p := math.Round(change/base*1000) / 10
	return &p
}
