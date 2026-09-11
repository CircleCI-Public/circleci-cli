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
	"github.com/go-chi/render"
	"github.com/google/uuid"
	"gotest.tools/v3/assert"
	"gotest.tools/v3/assert/cmp"

	"github.com/CircleCI-Public/circleci-cli/clikit/iostream"
	"github.com/CircleCI-Public/circleci-cli/internal/apiclient"
	"github.com/CircleCI-Public/circleci-cli/internal/testing/httprecorder"
	"github.com/CircleCI-Public/circleci-cli/internal/testing/httprecorder/chirecorder"
)

var (
	testProjOrgID = uuid.MustParse("a0000001-0000-4000-8000-0000000000a1")
	testProjID    = uuid.MustParse("b0000001-0000-4000-8000-000000000001")
)

// projectListEntity builds a v3 project entity as the collection reports it:
// the org is denormalised into references, carrying its name as well as its id.
func projectListEntity(id uuid.UUID, name string) map[string]any {
	return map[string]any{
		"id":         id.String(),
		"attributes": map[string]any{"name": name},
		"references": map[string]any{
			"org": map[string]any{
				"id":         testProjOrgID.String(),
				"attributes": map[string]any{"name": "acme"},
			},
		},
	}
}

func TestClient_ListProjects(t *testing.T) {
	ctx := iostream.Testing(context.Background())

	rec := httprecorder.New()
	r := chi.NewMux()
	r.Use(chirecorder.Middleware(rec))
	r.Get("/api/v3/projects", func(w http.ResponseWriter, r *http.Request) {
		// Two pages, so the cursor loop is exercised rather than assumed.
		if r.URL.Query().Get("page[cursor]") == "" {
			render.JSON(w, r, map[string]any{
				"data": []any{projectListEntity(testProjID, "alpha")},
				"page": map[string]any{"next": "cursor-2", "prev": nil},
			})
			return
		}
		render.JSON(w, r, map[string]any{
			"data": []any{
				projectListEntity(uuid.MustParse("b0000002-0000-4000-8000-000000000002"), "beta"),
			},
			"page": map[string]any{"next": nil, "prev": nil},
		})
	})
	srv := httptest.NewServer(r)
	t.Cleanup(srv.Close)

	c := apiclient.New(apiclient.Config{BaseURL: srv.URL, Token: "the-token"})

	t.Run("follows the page cursor and reports the owning org", func(t *testing.T) {
		rec.Reset()

		got, err := c.ListProjects(ctx, apiclient.ProjectFilter{Following: true})
		assert.NilError(t, err)
		assert.Assert(t, cmp.Len(got, 2))
		assert.Check(t, cmp.DeepEqual(got[0], apiclient.ProjectSummary{
			ID:      testProjID,
			Name:    "alpha",
			OrgID:   testProjOrgID,
			OrgName: "acme",
		}))
		assert.Check(t, cmp.Equal(got[1].Name, "beta"))

		reqs := rec.AllRequests()
		assert.Assert(t, cmp.Len(reqs, 2))
		assert.Check(t, cmp.Equal(reqs[0].URL.Query().Get("filter[following]"), "true"))
		assert.Check(t, cmp.Equal(reqs[0].URL.Query().Get("page[limit]"), "50"))
		assert.Check(t, cmp.Equal(reqs[0].URL.Query().Get("page[cursor]"), ""))
		assert.Check(t, cmp.Equal(reqs[1].URL.Query().Get("page[cursor]"), "cursor-2"))
	})

	t.Run("each filter maps to its query parameter", func(t *testing.T) {
		rec.Reset()

		_, err := c.ListProjects(ctx, apiclient.ProjectFilter{
			OrgID: testProjOrgID.String(),
			Name:  "alpha",
		})
		assert.NilError(t, err)

		q := rec.AllRequests()[0].URL.Query()
		assert.Check(t, cmp.Equal(q.Get("filter[org_id]"), testProjOrgID.String()))
		assert.Check(t, cmp.Equal(q.Get("filter[name]"), "alpha"))
		// An unset filter is omitted, not sent empty: the API rejects
		// filter[following] with any value other than "true".
		assert.Check(t, !q.Has("filter[following]"))
		assert.Check(t, !q.Has("filter[slug]"))
	})

	t.Run("a slug is the only filter sent", func(t *testing.T) {
		rec.Reset()

		_, err := c.ListProjects(ctx, apiclient.ProjectFilter{Slug: "gh/acme/alpha"})
		assert.NilError(t, err)

		q := rec.AllRequests()[0].URL.Query()
		assert.Check(t, cmp.Equal(q.Get("filter[slug]"), "gh/acme/alpha"))
		assert.Check(t, !q.Has("filter[following]"))
		assert.Check(t, !q.Has("filter[org_id]"))
		assert.Check(t, !q.Has("filter[name]"))
	})
}
