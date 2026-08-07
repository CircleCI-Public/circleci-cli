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

package apiclient_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"

	"github.com/CircleCI-Public/circleci-cli/internal/apiclient"
	"github.com/CircleCI-Public/circleci-cli/internal/iostream"
)

// TestGetWorkflowJobsV3_QueuedJobPhase pins the correction applied to the jobs
// list endpoint's optimistic phase. The body below is a real response, trimmed to
// three jobs: every job in the workflow reads "started" the moment the workflow
// dispatches it, and the ones still waiting for an executor simply omit
// started_at — while GET /jobs/{id} for those same jobs returns phase "queued"
// with no parallel_executions.
//
// Reported verbatim, a queued job is indistinguishable from a running one: the
// interactive picker draws the blue running ●, the summary tables say
// "🔵 running", --json says "phase":"started" — and drilling in finds no steps.
// So a "started" job with no start time is reported as queued.
func TestGetWorkflowJobsV3_QueuedJobPhase(t *testing.T) {
	ctx := iostream.Testing(context.Background())
	workflowID := uuid.MustParse("a2d4f75d-f6f9-46dc-8e7b-1ea85c40d0ed")

	const body = `{"data":[
		{"id":"4405c050-a8b8-474f-aa70-79efcf25bc4a",
		 "attributes":{"number":47378,"name":"docs","type":"build","phase":"started"}},
		{"id":"1a8727c9-ccf2-40f5-814c-7c1cd56c5677",
		 "attributes":{"number":47379,"name":"test-macos","type":"build","phase":"started","started_at":"2026-08-07T12:09:59.650Z"}},
		{"id":"bc7361ba-9aa1-44da-822d-beb9e9dca672",
		 "attributes":{"number":47383,"name":"check","type":"build","phase":"ended","outcome":"succeeded","started_at":"2026-08-07T12:09:59.346Z","ended_at":"2026-08-07T12:12:01.000Z"}}
	]}`

	r := chi.NewMux()
	r.Get("/api/v3/jobs", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(body))
	})
	srv := httptest.NewServer(r)
	t.Cleanup(srv.Close)

	c := apiclient.New(apiclient.Config{BaseURL: srv.URL, Token: "t", Version: "1.2.3"})
	jobs, err := c.GetWorkflowJobsV3(ctx, workflowID)
	assert.NilError(t, err)
	assert.Assert(t, is.Len(jobs, 3))

	// "started" with no start time is really queued — the state that used to read
	// as running and then had no steps to open.
	assert.Check(t, is.Equal(jobs[0].Name, "docs"))
	assert.Check(t, is.Equal(jobs[0].Phase, apiclient.PhaseQueued))
	assert.Check(t, is.Nil(jobs[0].StartedAt))
	assert.Check(t, is.Equal(jobs[0].Status(), "⌛ queued"))
	assert.Check(t, is.Equal(apiclient.PhaseOutcomeSymbol(jobs[0].Phase, jobs[0].Outcome, jobs[0].CurrentOutcome), "○"))

	// A job with a start time is left alone: it really is running.
	assert.Check(t, is.Equal(jobs[1].Name, "test-macos"))
	assert.Check(t, is.Equal(jobs[1].Phase, apiclient.PhaseStarted))
	assert.Check(t, is.Equal(apiclient.PhaseOutcomeSymbol(jobs[1].Phase, jobs[1].Outcome, jobs[1].CurrentOutcome), "●"))

	// So is a finished one — the correction only ever applies to "started".
	assert.Check(t, is.Equal(jobs[2].Name, "check"))
	assert.Check(t, is.Equal(jobs[2].Phase, apiclient.PhaseEnded))
	assert.Check(t, is.Equal(jobs[2].Status(), "✅ succeeded"))
}
