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

package apiclient

import (
	"context"
	"net/http"
	"time"

	"github.com/google/uuid"

	"github.com/CircleCI-Public/circleci-cli/internal/httpcl"
)

// testFlakesPageSize is the largest page the test-flakes endpoint accepts; a
// larger page[limit] is rejected with 400 rather than clamped.
const testFlakesPageSize = 20

// TestFlake is one flake occurrence: a single failed execution of a test that
// later passed within the same pipeline. A test that flaked three times
// appears as three TestFlakes.
type TestFlake struct {
	Name         string    `json:"name"`
	Classname    string    `json:"classname"`
	File         string    `json:"file"`
	DurationMS   int64     `json:"duration_ms"`
	Message      *string   `json:"message"` // the failure output; can run to tens of KB
	Execution    int       `json:"execution"`
	RerunAttempt *int      `json:"rerun_attempt"`
	JobName      string    `json:"job_name"`
	WorkflowName string    `json:"workflow_name"`
	JobStartedAt time.Time `json:"job_started_at"`
	JobEndedAt   time.Time `json:"job_ended_at"`

	JobID      uuid.UUID `json:"-"`
	WorkflowID uuid.UUID `json:"-"`
	RunID      uuid.UUID `json:"-"`
}

type testFlakeWire struct {
	Attributes TestFlake `json:"attributes"`
	References struct {
		Job      v3Ref `json:"job"`
		Workflow v3Ref `json:"workflow"`
		Run      v3Ref `json:"run"`
	} `json:"references"`
}

type v3Ref struct {
	ID uuid.UUID `json:"id"`
}

type testFlakesSearchRequest struct {
	Scope struct {
		ProjectID uuid.UUID `json:"project_id"`
	} `json:"scope"`
	Page struct {
		Limit  int    `json:"limit"`
		Cursor string `json:"cursor"`
	} `json:"page"`
}

// TestFlakeStop says why SearchTestFlakes stopped paging.
type TestFlakeStop int

const (
	// TestFlakesComplete means every page was fetched.
	TestFlakesComplete TestFlakeStop = iota
	// TestFlakesPageCap means maxPages pages were fetched and more remain.
	TestFlakesPageCap
	// TestFlakesRepeatedCursor means the server returned the cursor just sent, so
	// following it would fetch the same page forever.
	TestFlakesRepeatedCursor
)

// TestFlakeSearch is the outcome of walking the test-flakes pages: the
// occurrences fetched, and whether the walk reached the last page.
type TestFlakeSearch struct {
	Flakes  []TestFlake
	Stopped TestFlakeStop
}

// SearchTestFlakes returns the flake occurrences the v3 test-flakes endpoint
// holds for a project, following page cursors until the last page or until
// maxPages pages have been fetched. The endpoint is experimental and orders
// results by file, classname and name, not by how often a test flaked, so a
// search stopped early is missing whole tests, not just older occurrences.
func (c *Client) SearchTestFlakes(ctx context.Context, projectID uuid.UUID, maxPages int) (TestFlakeSearch, error) {
	var flakes []TestFlake
	cursor := ""
	for page := 1; ; page++ {
		var body testFlakesSearchRequest
		body.Scope.ProjectID = projectID
		body.Page.Limit = testFlakesPageSize
		body.Page.Cursor = cursor

		var resp v3List[testFlakeWire]
		_, err := c.main.Call(ctx, httpcl.NewRequest(http.MethodPost, "/api/v3/test-flakes/search",
			httpcl.Body(body),
			httpcl.JSONDecoder(&resp),
		))
		if err != nil {
			return TestFlakeSearch{}, err
		}
		for _, w := range resp.Data {
			f := w.Attributes
			f.JobID = w.References.Job.ID
			f.WorkflowID = w.References.Workflow.ID
			f.RunID = w.References.Run.ID
			flakes = append(flakes, f)
		}

		next := ""
		if resp.Page.Next != nil {
			next = *resp.Page.Next
		}
		switch {
		case next == "" || len(resp.Data) == 0:
			return TestFlakeSearch{Flakes: flakes}, nil
		case next == cursor:
			return TestFlakeSearch{Flakes: flakes, Stopped: TestFlakesRepeatedCursor}, nil
		case page >= maxPages:
			return TestFlakeSearch{Flakes: flakes, Stopped: TestFlakesPageCap}, nil
		}
		cursor = next
	}
}
