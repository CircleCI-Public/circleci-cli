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

package runcompare

import (
	"context"
	"errors"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"

	clierrors "github.com/CircleCI-Public/circleci-cli/clikit/errors"
	"github.com/CircleCI-Public/circleci-cli/internal/apiclient"
	"github.com/CircleCI-Public/circleci-cli/internal/httpcl"
)

var (
	oldID     = uuid.MustParse("11111111-0000-4000-8000-000000000001")
	newID     = uuid.MustParse("22222222-0000-4000-8000-000000000002")
	projectID = uuid.MustParse("a0000000-0000-4000-8000-000000000001")
	t0        = time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC)
)

// stubAPI serves canned runs, workflows and charges keyed by run ID, and
// records the charge searches it was asked for.
type stubAPI struct {
	runs        map[uuid.UUID]*apiclient.RunV3
	workflows   map[uuid.UUID][]apiclient.WorkflowV3
	workflowErr map[uuid.UUID]error
	charges     map[string][]apiclient.Charge // filter → rows

	mu       sync.Mutex
	searches []apiclient.ChargeSearchParams
}

func (s *stubAPI) GetRunV3(_ context.Context, id uuid.UUID) (*apiclient.RunV3, error) {
	r, ok := s.runs[id]
	if !ok {
		return nil, &httpcl.HTTPError{StatusCode: http.StatusNotFound}
	}
	return r, nil
}

func (s *stubAPI) GetRunWorkflowsV3(_ context.Context, id uuid.UUID) ([]apiclient.WorkflowV3, error) {
	if err := s.workflowErr[id]; err != nil {
		return nil, err
	}
	return s.workflows[id], nil
}

func (s *stubAPI) SearchCharges(_ context.Context, p apiclient.ChargeSearchParams) ([]apiclient.Charge, error) {
	s.mu.Lock()
	s.searches = append(s.searches, p)
	s.mu.Unlock()
	return s.charges[p.Filter], nil
}

func endedRun(id uuid.UUID) *apiclient.RunV3 {
	return &apiclient.RunV3{ID: id, Phase: apiclient.PhaseEnded, ProjectID: projectID, CreatedAt: t0}
}

func workflow(created, ended time.Duration) apiclient.WorkflowV3 {
	wf := apiclient.WorkflowV3{CreatedAt: t0.Add(created)}
	if ended >= 0 {
		e := t0.Add(ended)
		wf.EndedAt = &e
	}
	return wf
}

func pct(v float64) *float64 { return &v }

func TestCompare(t *testing.T) {
	api := &stubAPI{
		runs: map[uuid.UUID]*apiclient.RunV3{oldID: endedRun(oldID), newID: endedRun(newID)},
		workflows: map[uuid.UUID][]apiclient.WorkflowV3{
			// Old: two workflows, the second ending last → 10m span.
			oldID: {workflow(0, 4*time.Minute), workflow(time.Second, 10*time.Minute)},
			// New: one workflow → 8m span.
			newID: {workflow(0, 8*time.Minute)},
		},
		charges: map[string][]apiclient.Charge{
			runFilter(oldID): {{Name: "build", Credits: 200}, {Name: "deploy", Credits: 100}},
			runFilter(newID): {{Name: "build", Credits: 330}},
		},
	}

	report, err := Compare(context.Background(), api, oldID, newID)
	assert.NilError(t, err)

	t.Run("summaries", func(t *testing.T) {
		assert.Check(t, is.DeepEqual(report.OldRun, RunSummary{ID: oldID, ProjectID: projectID, RunningSeconds: 600, Credits: 300}))
		assert.Check(t, is.DeepEqual(report.NewRun, RunSummary{ID: newID, ProjectID: projectID, RunningSeconds: 480, Credits: 330}))
	})

	t.Run("delta", func(t *testing.T) {
		assert.Check(t, is.DeepEqual(report.Delta, Delta{
			RunningSeconds: -120,
			RunningPct:     pct(-20),
			Credits:        30,
			CreditsPct:     pct(10),
		}))
	})

	t.Run("charge search window", func(t *testing.T) {
		assert.Assert(t, is.Len(api.searches, 2))
		for _, s := range api.searches {
			assert.Check(t, is.Equal(s.Analysis, apiclient.ChargeAnalysisWorkflow))
			assert.Check(t, is.DeepEqual(s.ProjectIDs, []uuid.UUID{projectID}))
			assert.Check(t, is.Equal(s.From, t0.Add(-5*time.Minute)), "filter %s", s.Filter)
		}
	})
}

func TestCompare_Refusals(t *testing.T) {
	running := endedRun(newID)
	running.Phase = apiclient.PhaseStarted

	tests := []struct {
		name     string
		oldID    uuid.UUID
		newRun   *apiclient.RunV3
		code     string
		message  string
		exitCode int
	}{
		{
			name:     "same run twice",
			oldID:    newID,
			newRun:   endedRun(newID),
			code:     "run.compare_same_run",
			message:  "nothing to compare",
			exitCode: clierrors.ExitBadArguments,
		},
		{
			name:     "run not ended",
			oldID:    oldID,
			newRun:   running,
			code:     "run.compare_not_ended",
			message:  "is started",
			exitCode: clierrors.ExitBadArguments,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			api := &stubAPI{runs: map[uuid.UUID]*apiclient.RunV3{oldID: endedRun(oldID), newID: tc.newRun}}

			_, err := Compare(context.Background(), api, tc.oldID, newID)
			cliErr, ok := errors.AsType[*clierrors.CLIError](err)
			assert.Assert(t, ok, "got %v", err)
			assert.Check(t, is.Equal(cliErr.Code, tc.code))
			assert.Check(t, is.Contains(cliErr.Message, tc.message))
			assert.Check(t, is.Equal(cliErr.ExitCode, tc.exitCode))
		})
	}
}

func TestCompare_FetchErrorNamesTheRun(t *testing.T) {
	api := &stubAPI{runs: map[uuid.UUID]*apiclient.RunV3{oldID: endedRun(oldID)}}

	_, err := Compare(context.Background(), api, oldID, newID)
	fetchErr, ok := errors.AsType[*FetchError](err)
	assert.Assert(t, ok, "got %v", err)
	assert.Check(t, is.Equal(fetchErr.RunID, newID))
	assert.Check(t, httpcl.HasStatusCode(err, http.StatusNotFound))
}

func TestCompare_RunWithoutWorkflows(t *testing.T) {
	api := &stubAPI{
		runs:        map[uuid.UUID]*apiclient.RunV3{oldID: endedRun(oldID), newID: endedRun(newID)},
		workflows:   map[uuid.UUID][]apiclient.WorkflowV3{newID: {workflow(0, time.Minute)}},
		workflowErr: map[uuid.UUID]error{oldID: &httpcl.HTTPError{StatusCode: http.StatusNotFound}},
	}

	report, err := Compare(context.Background(), api, oldID, newID)
	assert.NilError(t, err)
	assert.Check(t, is.Equal(report.OldRun.RunningSeconds, 0.0))
	assert.Check(t, is.Equal(report.NewRun.RunningSeconds, 60.0))
	assert.Check(t, is.Nil(report.Delta.RunningPct), "no percentage of a zero baseline")
	assert.Check(t, is.Nil(report.Delta.CreditsPct), "no percentage of a zero baseline")
}

func TestSpan(t *testing.T) {
	tests := []struct {
		name        string
		workflows   []apiclient.WorkflowV3
		first, last time.Time
	}{
		{name: "none"},
		{
			name:      "earliest created to latest ended",
			workflows: []apiclient.WorkflowV3{workflow(time.Minute, 3*time.Minute), workflow(0, 2*time.Minute)},
			first:     t0,
			last:      t0.Add(3 * time.Minute),
		},
		{
			name:      "a workflow without an end does not shorten the span",
			workflows: []apiclient.WorkflowV3{workflow(0, 5*time.Minute), workflow(time.Minute, -1)},
			first:     t0,
			last:      t0.Add(5 * time.Minute),
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			first, last := span(tc.workflows)
			assert.Check(t, is.Equal(first, tc.first))
			assert.Check(t, is.Equal(last, tc.last))
		})
	}
}

func TestPercent(t *testing.T) {
	tests := []struct {
		name         string
		change, base float64
		want         *float64
	}{
		{name: "zero base", change: 5, base: 0, want: nil},
		{name: "increase", change: 30, base: 300, want: pct(10)},
		{name: "decrease rounds to one decimal", change: -142, base: 662, want: pct(-21.5)},
		{name: "no change", change: 0, base: 10, want: pct(0)},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Check(t, is.DeepEqual(percent(tc.change, tc.base), tc.want))
		})
	}
}
