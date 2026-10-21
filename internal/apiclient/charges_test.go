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
	"github.com/google/uuid"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"

	"github.com/CircleCI-Public/circleci-cli/clikit/iostream"
	"github.com/CircleCI-Public/circleci-cli/internal/apiclient"
)

func TestSearchCharges(t *testing.T) {
	ctx := iostream.Testing(context.Background())
	projectID := uuid.MustParse("a0000000-0000-4000-8000-000000000001")
	from := time.Date(2026, 9, 1, 11, 55, 0, 0, time.UTC)
	to := time.Date(2026, 9, 2, 12, 30, 0, 0, time.UTC)

	// Two pages: the client must send the cursor it was handed and stop when
	// the second page carries no next cursor.
	pages := map[string]string{
		"": `{"data":[
			{"attributes":{"name":"build","credits":120,"runs":1,"credits_per_run":120},
			 "references":{"project":{"id":"a0000000-0000-4000-8000-000000000001"}}}
		],"page":{"next":"page-2"}}`,
		"page-2": `{"data":[
			{"attributes":{"name":"deploy","credits":30,"runs":1,"credits_per_run":30},
			 "references":{"project":{"id":"a0000000-0000-4000-8000-000000000001"}}}
		],"page":{}}`,
	}

	var bodies []map[string]any
	r := chi.NewMux()
	r.Post("/api/v3/analysis/charges", func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		bodies = append(bodies, body)
		cursor, _ := body["page"].(map[string]any)["cursor"].(string)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(pages[cursor]))
	})
	srv := httptest.NewServer(r)
	t.Cleanup(srv.Close)

	c := apiclient.New(apiclient.Config{BaseURL: srv.URL, Token: "t", Version: "1.2.3"})
	charges, err := c.SearchCharges(ctx, apiclient.ChargeSearchParams{
		Analysis:   apiclient.ChargeAnalysisWorkflow,
		ProjectIDs: []uuid.UUID{projectID},
		From:       from,
		To:         to,
		Filter:     `pipeline.id == "5034460f-c7c4-4c43-9457-de07e2029e7b"`,
	})
	assert.NilError(t, err)

	t.Run("request body", func(t *testing.T) {
		assert.Assert(t, is.Len(bodies, 2))
		assert.Check(t, is.DeepEqual(bodies[0], map[string]any{
			"analysis": "charge.workflow",
			"scope": map[string]any{
				"project_ids": []any{"a0000000-0000-4000-8000-000000000001"},
				"from":        "2026-09-01T11:55:00Z",
				"to":          "2026-09-02T12:30:00Z",
			},
			"filter": `pipeline.id == "5034460f-c7c4-4c43-9457-de07e2029e7b"`,
			"page":   map[string]any{"limit": float64(20)},
		}))
		assert.Check(t, is.DeepEqual(bodies[1]["page"], map[string]any{"cursor": "page-2", "limit": float64(20)}))
	})

	t.Run("rows from every page", func(t *testing.T) {
		assert.Check(t, is.DeepEqual(charges, []apiclient.Charge{
			{Name: "build", Credits: 120, Runs: 1, CreditsPerRun: 120, ProjectID: projectID},
			{Name: "deploy", Credits: 30, Runs: 1, CreditsPerRun: 30, ProjectID: projectID},
		}))
	})
}

func TestSearchCharges_NoRows(t *testing.T) {
	ctx := iostream.Testing(context.Background())

	r := chi.NewMux()
	r.Post("/api/v3/analysis/charges", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[]}`))
	})
	srv := httptest.NewServer(r)
	t.Cleanup(srv.Close)

	c := apiclient.New(apiclient.Config{BaseURL: srv.URL, Token: "t", Version: "1.2.3"})
	charges, err := c.SearchCharges(ctx, apiclient.ChargeSearchParams{
		Analysis:   apiclient.ChargeAnalysisWorkflow,
		ProjectIDs: []uuid.UUID{uuid.New()},
	})
	assert.NilError(t, err)
	assert.Check(t, is.Len(charges, 0))
}
