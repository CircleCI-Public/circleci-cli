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
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/google/uuid"
	"gotest.tools/v3/assert"
	"gotest.tools/v3/assert/cmp"

	"github.com/CircleCI-Public/circleci-cli/clikit/iostream"
	"github.com/CircleCI-Public/circleci-cli/internal/apiclient"
	"github.com/CircleCI-Public/circleci-cli/internal/httpcl"
	"github.com/CircleCI-Public/circleci-cli/internal/testing/httprecorder"
	"github.com/CircleCI-Public/circleci-cli/internal/testing/httprecorder/chirecorder"
)

func TestClient_SearchTestFlakes(t *testing.T) {
	ctx := iostream.Testing(context.Background())

	projectID := uuid.MustParse("b0000000-0000-4000-8000-000000000001")
	// Each of these projects makes the server misbehave in one way.
	repeatingProjectID := uuid.MustParse("b0000000-0000-4000-8000-000000000002") // always next "c1"
	endlessProjectID := uuid.MustParse("b0000000-0000-4000-8000-000000000003")   // always a fresh next
	jobID := uuid.MustParse("c0000000-0000-4000-8000-000000000001")
	workflowID := uuid.MustParse("c0000000-0000-4000-8000-000000000002")
	runID := uuid.MustParse("c0000000-0000-4000-8000-000000000003")

	flake := func(name string) map[string]any {
		return map[string]any{
			"attributes": map[string]any{
				"name":           name,
				"classname":      "internal/run",
				"file":           "internal/run/watch_test.go",
				"test_atom":      "",
				"duration_ms":    70,
				"message":        "boom",
				"execution":      3,
				"rerun_attempt":  0,
				"job_name":       "test",
				"workflow_name":  "ci",
				"job_started_at": "2026-11-03T22:19:35.573Z",
				"job_ended_at":   "2026-11-03T22:25:00Z",
			},
			"references": map[string]any{
				"job":      map[string]any{"id": jobID},
				"workflow": map[string]any{"id": workflowID},
				"run":      map[string]any{"id": runID},
				"project":  map[string]any{"id": projectID},
			},
		}
	}

	rec := httprecorder.New()
	r := chi.NewMux()
	r.Use(chirecorder.Middleware(rec))
	r.Post("/api/v3/test-flakes/search", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Scope struct {
				ProjectID string `json:"project_id"`
			} `json:"scope"`
			Page struct {
				Cursor string `json:"cursor"`
			} `json:"page"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			render.Status(r, http.StatusBadRequest)
			render.JSON(w, r, map[string]any{"error": map[string]any{"title": "Bad request."}})
			return
		}
		switch body.Scope.ProjectID {
		case projectID.String():
		case repeatingProjectID.String():
			render.JSON(w, r, map[string]any{"data": []any{flake("TestA")}, "page": map[string]any{"next": "c1", "prev": nil}})
			return
		case endlessProjectID.String():
			render.JSON(w, r, map[string]any{"data": []any{flake("TestA")}, "page": map[string]any{"next": body.Page.Cursor + "x", "prev": nil}})
			return
		default:
			render.Status(r, http.StatusNotFound)
			render.JSON(w, r, map[string]any{"error": map[string]any{"title": "Project not found."}})
			return
		}
		// Two pages: the first hands back a cursor, the second ends with a null next.
		if body.Page.Cursor == "" {
			render.JSON(w, r, map[string]any{"data": []any{flake("TestA")}, "page": map[string]any{"next": "p2", "prev": nil}})
			return
		}
		render.JSON(w, r, map[string]any{"data": []any{flake("TestB")}, "page": map[string]any{"next": nil, "prev": "p1"}})
	})
	srv := httptest.NewServer(r)
	t.Cleanup(srv.Close)

	c := apiclient.New(apiclient.Config{BaseURL: srv.URL, Token: "the-token"})

	t.Run("follows the cursor and decodes attributes and references", func(t *testing.T) {
		rec.Reset()

		search, err := c.SearchTestFlakes(ctx, projectID, 50)
		assert.NilError(t, err)
		assert.Check(t, cmp.Equal(search.Stopped, apiclient.TestFlakesComplete))
		got := search.Flakes
		assert.Assert(t, cmp.Len(got, 2))
		assert.Check(t, cmp.Equal(got[0].Name, "TestA"))
		assert.Check(t, cmp.Equal(got[1].Name, "TestB"))
		assert.Check(t, cmp.DeepEqual(got[0], apiclient.TestFlake{
			Name:         "TestA",
			Classname:    "internal/run",
			File:         "internal/run/watch_test.go",
			DurationMS:   70,
			Message:      new("boom"),
			Execution:    3,
			RerunAttempt: new(0),
			JobName:      "test",
			WorkflowName: "ci",
			JobStartedAt: time.Date(2026, 11, 3, 22, 19, 35, 573_000_000, time.UTC),
			JobEndedAt:   time.Date(2026, 11, 3, 22, 25, 0, 0, time.UTC),
			JobID:        jobID,
			WorkflowID:   workflowID,
			RunID:        runID,
		}))

		reqs := rec.AllRequests()
		assert.Assert(t, cmp.Len(reqs, 2))
		assert.Check(t, cmp.Equal(*reqs[0].Body,
			`{"scope":{"project_id":"b0000000-0000-4000-8000-000000000001"},"page":{"limit":20,"cursor":""}}`))
		assert.Check(t, cmp.Contains(*reqs[1].Body, `"cursor":"p2"`))
	})

	t.Run("a repeated cursor stops paging after the repeat", func(t *testing.T) {
		rec.Reset()

		search, err := c.SearchTestFlakes(ctx, repeatingProjectID, 50)
		assert.NilError(t, err)
		assert.Check(t, cmp.Equal(search.Stopped, apiclient.TestFlakesRepeatedCursor))
		assert.Check(t, cmp.Len(search.Flakes, 2))

		reqs := rec.AllRequests()
		assert.Assert(t, cmp.Len(reqs, 2))
		assert.Check(t, cmp.Contains(*reqs[0].Body, `"cursor":""`))
		assert.Check(t, cmp.Contains(*reqs[1].Body, `"cursor":"c1"`))
	})

	t.Run("stops at maxPages when more pages remain", func(t *testing.T) {
		rec.Reset()

		search, err := c.SearchTestFlakes(ctx, endlessProjectID, 3)
		assert.NilError(t, err)
		assert.Check(t, cmp.Equal(search.Stopped, apiclient.TestFlakesPageCap))
		assert.Check(t, cmp.Len(search.Flakes, 3))
		assert.Check(t, cmp.Len(rec.AllRequests(), 3))
	})

	t.Run("a last page at exactly maxPages is complete", func(t *testing.T) {
		search, err := c.SearchTestFlakes(ctx, projectID, 2)
		assert.NilError(t, err)
		assert.Check(t, cmp.Equal(search.Stopped, apiclient.TestFlakesComplete))
		assert.Check(t, cmp.Len(search.Flakes, 2))
	})

	t.Run("unknown project surfaces the 404", func(t *testing.T) {
		_, err := c.SearchTestFlakes(ctx, uuid.MustParse("b0000000-0000-4000-8000-0000000000ff"), 50)
		assert.Check(t, httpcl.HasStatusCode(err, http.StatusNotFound), "got: %v", err)
	})
}
