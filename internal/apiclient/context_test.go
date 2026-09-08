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
	"sync"
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

var (
	testCtxOrgID   = uuid.MustParse("a0000001-0000-4000-8000-0000000000a1")
	testCtxID      = uuid.MustParse("c0000001-0000-4000-8000-000000000001")
	testCtxGroupID = uuid.MustParse("d0000001-0000-4000-8000-000000000001")
	testCtxProjID  = uuid.MustParse("b0000001-0000-4000-8000-000000000001")
	testCtxRestrID = uuid.MustParse("e0000001-0000-4000-8000-000000000001")
)

// contextEntity builds a v3 context entity. Only the by-id read carries
// references, and only references.org — the list has none, and no context
// endpoint reports group access.
func contextEntity(id uuid.UUID, name string, withOrg bool) map[string]any {
	entity := map[string]any{
		"id": id.String(),
		"attributes": map[string]any{
			"name":       name,
			"created_at": "2020-01-01T12:00:00Z",
		},
	}
	if withOrg {
		entity["references"] = map[string]any{"org": map[string]any{"id": testCtxOrgID.String()}}
	}
	return entity
}

// restrictionEntity builds a v3 restriction entity. name is optional: an
// expression restriction has nothing to name.
func restrictionEntity(id uuid.UUID, restrictionType, matchPattern, name string) map[string]any {
	attrs := map[string]any{
		"restriction_type": restrictionType,
		"match_pattern":    matchPattern,
	}
	if name != "" {
		attrs["name"] = name
	}
	return map[string]any{
		"id":         id.String(),
		"attributes": attrs,
		"references": map[string]any{
			"context": map[string]any{"id": testCtxID.String()},
		},
	}
}

func TestClient_ListContexts(t *testing.T) {
	ctx := iostream.Testing(context.Background())

	rec := httprecorder.New()
	r := chi.NewMux()
	r.Use(chirecorder.Middleware(rec))
	r.Get("/api/v3/contexts", func(w http.ResponseWriter, r *http.Request) {
		// Two pages, so the cursor loop is exercised rather than assumed.
		if r.URL.Query().Get("page[cursor]") == "" {
			render.JSON(w, r, map[string]any{
				"data": []any{
					contextEntity(testCtxID, "build-secrets", false),
					contextEntity(uuid.MustParse("c0000002-0000-4000-8000-000000000002"), "deploy-keys", false),
				},
				"page": map[string]any{"next": "cursor-2", "prev": nil},
			})
			return
		}
		render.JSON(w, r, map[string]any{
			"data": []any{
				contextEntity(uuid.MustParse("c0000003-0000-4000-8000-000000000003"), "BUILD-extra", false),
			},
			"page": map[string]any{"next": nil, "prev": nil},
		})
	})
	srv := httptest.NewServer(r)
	t.Cleanup(srv.Close)

	c := apiclient.New(apiclient.Config{BaseURL: srv.URL, Token: "the-token"})

	t.Run("filters by org and follows the page cursor", func(t *testing.T) {
		got, err := c.ListContexts(ctx, testCtxOrgID, "")
		assert.NilError(t, err)
		assert.Check(t, cmp.Len(got, 3))
		assert.Check(t, cmp.Equal(got[0].Name, "build-secrets"))
		assert.Check(t, cmp.Equal(got[0].ID, testCtxID))
		assert.Check(t, cmp.Equal(got[2].Name, "BUILD-extra"))

		reqs := rec.AllRequests()
		assert.Assert(t, cmp.Len(reqs, 2))
		assert.Check(t, cmp.Equal(reqs[0].URL.Query().Get("filter[org_id]"), testCtxOrgID.String()))
		assert.Check(t, cmp.Equal(reqs[0].URL.Query().Get("page[limit]"), "100"))
		assert.Check(t, cmp.Equal(reqs[0].URL.Query().Get("page[cursor]"), ""))
		assert.Check(t, cmp.Equal(reqs[1].URL.Query().Get("page[cursor]"), "cursor-2"))
	})

	t.Run("name is sent as filter[name]", func(t *testing.T) {
		rec.Reset()
		// Matching is the API's job — it filters upstream, so it narrows
		// pagination as well as the page contents. All the client owes is the
		// parameter, and passing through whatever comes back.
		_, err := c.ListContexts(ctx, testCtxOrgID, "build")
		assert.NilError(t, err)

		req := rec.LastRequest()
		assert.Check(t, cmp.Equal(req.URL.Query().Get("filter[name]"), "build"))
	})

	t.Run("list items carry no org", func(t *testing.T) {
		got, err := c.ListContexts(ctx, testCtxOrgID, "")
		assert.NilError(t, err)
		assert.Assert(t, cmp.Len(got, 3))
		assert.Check(t, cmp.Equal(got[0].OrgID, uuid.Nil),
			"the list has no references; only the by-id read carries org")
	})

	t.Run("an empty name sends no name filter", func(t *testing.T) {
		rec.Reset()
		_, err := c.ListContexts(ctx, testCtxOrgID, "")
		assert.NilError(t, err)

		req := rec.LastRequest()
		_, present := req.URL.Query()["filter[name]"]
		assert.Check(t, !present, "filter[name] must be omitted, not sent empty")
	})
}

func TestClient_GetContextDetail(t *testing.T) {
	ctx := iostream.Testing(context.Background())

	rec := httprecorder.New()
	r := chi.NewMux()
	r.Use(chirecorder.Middleware(rec))
	r.Get("/api/v3/contexts/{id}", func(w http.ResponseWriter, r *http.Request) {
		render.JSON(w, r, map[string]any{
			"data": contextEntity(testCtxID, "build-secrets", true),
		})
	})
	r.Get("/api/v3/contexts/{id}/env-vars", func(w http.ResponseWriter, r *http.Request) {
		render.JSON(w, r, map[string]any{
			"data": []any{
				map[string]any{
					// No "id": v3 env vars are keyed by name within a context.
					"attributes": map[string]any{
						"name":            "DB_PASSWORD",
						"truncated_value": "abcd",
						"created_at":      "2020-01-01T12:00:00Z",
						"updated_at":      "2020-06-01T12:00:00Z",
					},
					"references": map[string]any{
						"context": map[string]any{"id": testCtxID.String()},
					},
				},
			},
			"page": map[string]any{"next": nil, "prev": nil},
		})
	})
	r.Get("/api/v3/context-restrictions", func(w http.ResponseWriter, r *http.Request) {
		// Deliberately project-then-group, so the ordering assertion below is
		// testing sortRestrictions rather than the server's arrival order.
		render.JSON(w, r, map[string]any{
			"data": []any{
				restrictionEntity(testCtxRestrID, "project", testCtxProjID.String(), "myrepo"),
				restrictionEntity(testCtxGroupID, "group", testCtxGroupID.String(), "All members"),
			},
			"page": map[string]any{"next": nil, "prev": nil},
		})
	})
	srv := httptest.NewServer(r)
	t.Cleanup(srv.Close)

	c := apiclient.New(apiclient.Config{BaseURL: srv.URL, Token: "the-token"})

	detail, err := c.GetContextDetail(ctx, testCtxID)
	assert.NilError(t, err)

	t.Run("carries the context and its org", func(t *testing.T) {
		assert.Check(t, cmp.Equal(detail.ID, testCtxID))
		assert.Check(t, cmp.Equal(detail.Name, "build-secrets"))
		assert.Check(t, cmp.Equal(detail.OrgID, testCtxOrgID))
	})

	t.Run("carries env vars including the truncated value", func(t *testing.T) {
		assert.Assert(t, cmp.Len(detail.EnvironmentVariables, 1))
		ev := detail.EnvironmentVariables[0]
		assert.Check(t, cmp.Equal(ev.Variable, "DB_PASSWORD"))
		assert.Check(t, cmp.Equal(ev.TruncatedValue, "abcd"))
		assert.Check(t, cmp.Equal(ev.ContextID, testCtxID))
	})

	t.Run("orders restrictions group-first regardless of arrival order", func(t *testing.T) {
		assert.Assert(t, cmp.Len(detail.Restrictions, 2))

		grp := detail.Restrictions[0]
		assert.Check(t, cmp.Equal(grp.RestrictionType, "group"))
		assert.Check(t, cmp.Equal(grp.Name, "All members"))
		assert.Check(t, cmp.Equal(grp.ID, testCtxGroupID))
		assert.Check(t, cmp.Equal(grp.MatchPattern, testCtxGroupID.String()))
		assert.Check(t, cmp.Equal(grp.ContextID, testCtxID))

		proj := detail.Restrictions[1]
		assert.Check(t, cmp.Equal(proj.RestrictionType, "project"))
		assert.Check(t, cmp.Equal(proj.MatchPattern, testCtxProjID.String()))
		assert.Check(t, cmp.Equal(proj.Name, "myrepo"), "name comes from references.project")
	})

	t.Run("scopes the restrictions request to the context", func(t *testing.T) {
		// The three reads run concurrently, so index into the recording by path
		// rather than by arrival order.
		var restrictionsReq *httprecorder.Request
		reqs := rec.AllRequests()
		assert.Assert(t, cmp.Len(reqs, 3))
		for i := range reqs {
			if reqs[i].URL.Path == "/api/v3/context-restrictions" {
				restrictionsReq = &reqs[i]
			}
		}
		assert.Assert(t, restrictionsReq != nil)
		assert.Check(t, cmp.Equal(restrictionsReq.URL.Query().Get("filter[context_id]"), testCtxID.String()))
	})
}

// TestClient_ListContextRestrictions covers all three restriction types from
// the one endpoint that serves them, and where each one's name comes from.
func TestClient_ListContextRestrictions(t *testing.T) {
	ctx := iostream.Testing(context.Background())

	rec := httprecorder.New()
	r := chi.NewMux()
	r.Use(chirecorder.Middleware(rec))
	r.Get("/api/v3/context-restrictions", func(w http.ResponseWriter, r *http.Request) {
		render.JSON(w, r, map[string]any{
			"data": []any{
				restrictionEntity(testCtxGroupID, "group", testCtxGroupID.String(), "All members"),
				restrictionEntity(testCtxRestrID, "project", testCtxProjID.String(), "myrepo"),
				restrictionEntity(
					uuid.MustParse("e0000002-0000-4000-8000-000000000002"),
					"expression", `pipeline.git.branch == "main"`, "",
				),
				// A project whose name the API could not resolve. Name
				// resolution is best-effort upstream — a failure there degrades
				// the listing to ids rather than failing it — so an unnamed
				// project restriction is a normal response, not a broken one.
				restrictionEntity(
					uuid.MustParse("e0000003-0000-4000-8000-000000000003"),
					"project", "b0000009-0000-4000-8000-000000000009", "",
				),
			},
			"page": map[string]any{"next": nil, "prev": nil},
		})
	})
	srv := httptest.NewServer(r)
	t.Cleanup(srv.Close)

	c := apiclient.New(apiclient.Config{BaseURL: srv.URL, Token: "the-token"})

	got, err := c.ListContextRestrictions(ctx, testCtxID)
	assert.NilError(t, err)
	assert.Assert(t, cmp.Len(got, 4))

	t.Run("scopes the request to the context", func(t *testing.T) {
		req := rec.LastRequest()
		assert.Check(t, cmp.Equal(req.URL.Query().Get("filter[context_id]"), testCtxID.String()))
	})

	t.Run("group restrictions come from here, keyed on the group id", func(t *testing.T) {
		assert.Check(t, cmp.Equal(got[0].RestrictionType, "group"))
		assert.Check(t, cmp.Equal(got[0].ID, testCtxGroupID))
		assert.Check(t, cmp.Equal(got[0].MatchPattern, testCtxGroupID.String()))
		assert.Check(t, cmp.Equal(got[0].Name, "All members"))
		assert.Check(t, cmp.Equal(got[0].ContextID, testCtxID))
	})

	t.Run("a project restriction names its project", func(t *testing.T) {
		assert.Check(t, cmp.Equal(got[1].RestrictionType, "project"))
		assert.Check(t, cmp.Equal(got[1].MatchPattern, testCtxProjID.String()))
		assert.Check(t, cmp.Equal(got[1].Name, "myrepo"))
	})

	t.Run("an expression restriction has no name", func(t *testing.T) {
		assert.Check(t, cmp.Equal(got[2].RestrictionType, "expression"))
		assert.Check(t, cmp.Equal(got[2].Name, ""))
	})

	t.Run("a project whose name did not resolve keeps its id", func(t *testing.T) {
		assert.Check(t, cmp.Equal(got[3].RestrictionType, "project"))
		assert.Check(t, cmp.Equal(got[3].Name, ""))
		assert.Check(t, cmp.Equal(got[3].MatchPattern, "b0000009-0000-4000-8000-000000000009"),
			"the id is the data; the name is decoration")
	})
}

// TestClient_GetContextDetail_Concurrent proves the three reads are in flight
// together rather than merely that all three happen.
//
// Each handler blocks until all three have arrived. Run in series the third
// request would never be made, the first two would never be released, and the
// gate would time out — so a regression to sequential calls fails this test
// rather than just making it slower.
func TestClient_GetContextDetail_Concurrent(t *testing.T) {
	ctx := iostream.Testing(context.Background())

	const wantInFlight = 3
	var (
		mu       sync.Mutex
		arrived  int
		allIn    = make(chan struct{})
		timedOut = make(chan struct{})
	)
	// gate blocks until every request has arrived, reporting false if that
	// never happens.
	gate := func() bool {
		mu.Lock()
		arrived++
		if arrived == wantInFlight {
			close(allIn)
		}
		mu.Unlock()

		select {
		case <-allIn:
			return true
		// Deliberately generous. The gate only has to distinguish "all three
		// arrived" from "they cannot, because they are serialised", and a long
		// timeout does not weaken that signal — it just delays the failure. A
		// tight one risks the opposite: a false failure on a loaded machine,
		// claiming a concurrency regression that is not there. A slow true
		// failure beats a fast false one.
		case <-time.After(15 * time.Second):
			close(timedOut)
			return false
		}
	}

	r := chi.NewMux()
	r.Get("/api/v3/contexts/{id}", func(w http.ResponseWriter, r *http.Request) {
		if !gate() {
			http.Error(w, "not concurrent", http.StatusInternalServerError)
			return
		}
		render.JSON(w, r, map[string]any{"data": contextEntity(testCtxID, "build-secrets", true)})
	})
	r.Get("/api/v3/contexts/{id}/env-vars", func(w http.ResponseWriter, r *http.Request) {
		if !gate() {
			http.Error(w, "not concurrent", http.StatusInternalServerError)
			return
		}
		render.JSON(w, r, map[string]any{"data": []any{}})
	})
	r.Get("/api/v3/context-restrictions", func(w http.ResponseWriter, r *http.Request) {
		if !gate() {
			http.Error(w, "not concurrent", http.StatusInternalServerError)
			return
		}
		render.JSON(w, r, map[string]any{"data": []any{}})
	})
	srv := httptest.NewServer(r)
	t.Cleanup(srv.Close)

	c := apiclient.New(apiclient.Config{BaseURL: srv.URL, Token: "the-token"})

	detail, err := c.GetContextDetail(ctx, testCtxID)
	assert.NilError(t, err)
	assert.Check(t, cmp.Equal(detail.Name, "build-secrets"))

	select {
	case <-timedOut:
		t.Error("requests were not concurrent: the gate timed out waiting for all three")
	default:
	}
}

// TestClient_GetContextDetail_FirstErrorWins covers a failure in one of the
// three concurrent reads: it surfaces, and nothing half-assembled is returned.
func TestClient_GetContextDetail_FirstErrorWins(t *testing.T) {
	ctx := iostream.Testing(context.Background())

	r := chi.NewMux()
	r.Get("/api/v3/contexts/{id}", func(w http.ResponseWriter, r *http.Request) {
		render.JSON(w, r, map[string]any{"data": contextEntity(testCtxID, "build-secrets", true)})
	})
	r.Get("/api/v3/contexts/{id}/env-vars", func(w http.ResponseWriter, r *http.Request) {
		render.JSON(w, r, map[string]any{"data": []any{}})
	})
	r.Get("/api/v3/context-restrictions", func(w http.ResponseWriter, r *http.Request) {
		render.Status(r, http.StatusForbidden)
		render.JSON(w, r, map[string]any{
			"error": map[string]any{"title": "Forbidden", "detail": "no permission"},
		})
	})
	srv := httptest.NewServer(r)
	t.Cleanup(srv.Close)

	c := apiclient.New(apiclient.Config{BaseURL: srv.URL, Token: "the-token"})

	detail, err := c.GetContextDetail(ctx, testCtxID)
	assert.Check(t, err != nil, "a failing read must fail the whole call")
	assert.Check(t, cmp.Nil(detail))
	assert.Check(t, httpcl.HasStatusCode(err, http.StatusForbidden))
}

func TestClient_ContextEnvVarWrites(t *testing.T) {
	ctx := iostream.Testing(context.Background())

	rec := httprecorder.New()
	r := chi.NewMux()
	r.Use(chirecorder.Middleware(rec))
	r.Post("/api/v3/contexts/{id}/env-vars/set", func(w http.ResponseWriter, r *http.Request) {
		// 200 for both create and overwrite — the endpoint does not distinguish.
		render.JSON(w, r, map[string]any{"data": map[string]any{}})
	})
	r.Delete("/api/v3/contexts/{id}/env-vars", func(w http.ResponseWriter, r *http.Request) {
		render.NoContent(w, r)
	})
	srv := httptest.NewServer(r)
	t.Cleanup(srv.Close)

	c := apiclient.New(apiclient.Config{BaseURL: srv.URL, Token: "the-token"})

	t.Run("set posts a flat body with the name in it", func(t *testing.T) {
		err := c.SetContextEnvVar(ctx, testCtxID, "MY_VAR", "s3cr3t")
		assert.NilError(t, err)

		req := rec.LastRequest()
		assert.Check(t, cmp.Equal(req.URL.Path, "/api/v3/contexts/"+testCtxID.String()+"/env-vars/set"))

		var body map[string]any
		decodeErr := req.Decode(&body)
		assert.NilError(t, decodeErr)
		assert.Check(t, cmp.Equal(body["name"], "MY_VAR"))
		assert.Check(t, cmp.Equal(body["value"], "s3cr3t"))
	})

	t.Run("delete names the variable in a filter, not the path", func(t *testing.T) {
		err := c.DeleteContextEnvVar(ctx, testCtxID, "MY_VAR")
		assert.NilError(t, err)

		req := rec.LastRequest()
		assert.Check(t, cmp.Equal(req.URL.Path, "/api/v3/contexts/"+testCtxID.String()+"/env-vars"))
		assert.Check(t, cmp.Equal(req.URL.Query().Get("filter[name]"), "MY_VAR"))
	})
}

func TestClient_ContextRestrictionWrites(t *testing.T) {
	ctx := iostream.Testing(context.Background())

	rec := httprecorder.New()
	r := chi.NewMux()
	r.Use(chirecorder.Middleware(rec))
	r.Post("/api/v3/context-restrictions", func(w http.ResponseWriter, r *http.Request) {
		render.Status(r, http.StatusCreated)
		render.JSON(w, r, map[string]any{
			"data": map[string]any{
				"id": testCtxRestrID.String(),
				"attributes": map[string]any{
					"restriction_type": "expression",
					"match_pattern":    `pipeline.git.branch == "main"`,
				},
				"references": map[string]any{
					"context": map[string]any{"id": testCtxID.String()},
				},
			},
		})
	})
	r.Delete("/api/v3/context-restrictions/{id}", func(w http.ResponseWriter, r *http.Request) {
		render.NoContent(w, r)
	})
	srv := httptest.NewServer(r)
	t.Cleanup(srv.Close)

	c := apiclient.New(apiclient.Config{BaseURL: srv.URL, Token: "the-token"})

	t.Run("create names the context in the body, not the path", func(t *testing.T) {
		got, err := c.CreateContextRestriction(ctx, testCtxID, "expression", `pipeline.git.branch == "main"`)
		assert.NilError(t, err)
		assert.Check(t, cmp.Equal(got.ID, testCtxRestrID))
		assert.Check(t, cmp.Equal(got.MatchPattern, `pipeline.git.branch == "main"`))
		assert.Check(t, cmp.Equal(got.Name, ""), "expression restrictions have nothing to name")

		req := rec.LastRequest()
		assert.Check(t, cmp.Equal(req.URL.Path, "/api/v3/context-restrictions"))

		var body struct {
			Data struct {
				Attributes struct {
					RestrictionType string `json:"restriction_type"`
					MatchPattern    string `json:"match_pattern"`
				} `json:"attributes"`
				References struct {
					Context struct {
						ID string `json:"id"`
					} `json:"context"`
				} `json:"references"`
			} `json:"data"`
		}
		decodeErr := req.Decode(&body)
		assert.NilError(t, decodeErr)
		assert.Check(t, cmp.Equal(body.Data.Attributes.RestrictionType, "expression"))
		assert.Check(t, cmp.Equal(body.Data.Attributes.MatchPattern, `pipeline.git.branch == "main"`))
		assert.Check(t, cmp.Equal(body.Data.References.Context.ID, testCtxID.String()))
	})

	t.Run("delete sends the restriction in the path and the context as a filter", func(t *testing.T) {
		err := c.DeleteContextRestriction(ctx, testCtxID, testCtxRestrID)
		assert.NilError(t, err)

		req := rec.LastRequest()
		assert.Check(t, cmp.Equal(req.URL.Path, "/api/v3/context-restrictions/"+testCtxRestrID.String()))
		assert.Check(t, cmp.Equal(req.URL.Query().Get("filter[context_id]"), testCtxID.String()))
	})
}

func TestClient_ListContextEnvVars_Empty(t *testing.T) {
	ctx := iostream.Testing(context.Background())

	r := chi.NewMux()
	r.Get("/api/v3/contexts/{id}/env-vars", func(w http.ResponseWriter, r *http.Request) {
		// A context with no variables answers with no page block at all, which
		// must terminate the cursor loop rather than spin on it.
		render.JSON(w, r, map[string]any{"data": []any{}})
	})
	srv := httptest.NewServer(r)
	t.Cleanup(srv.Close)

	c := apiclient.New(apiclient.Config{BaseURL: srv.URL, Token: "the-token"})

	got, err := c.ListContextEnvVars(ctx, testCtxID)
	assert.NilError(t, err)
	assert.Check(t, cmp.Len(got, 0))
}
