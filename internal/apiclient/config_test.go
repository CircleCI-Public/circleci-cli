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
	"gotest.tools/v3/assert"
	"gotest.tools/v3/assert/cmp"

	"github.com/CircleCI-Public/circleci-cli/clikit/iostream"
	"github.com/CircleCI-Public/circleci-cli/internal/apiclient"
	"github.com/CircleCI-Public/circleci-cli/internal/testing/httprecorder"
	"github.com/CircleCI-Public/circleci-cli/internal/testing/httprecorder/chirecorder"
)

const testCompileOrgID = "f22b6566-597d-46d5-ba74-99ef5bb3d85c"

// compileServer serves POST /api/v3/configs/compile, responding with status and
// body (a non-200 status responds with a plain message instead of body).
func compileServer(t *testing.T, rec *httprecorder.RequestRecorder, status int, body map[string]any) *httptest.Server {
	t.Helper()

	r := chi.NewMux()
	r.Use(chirecorder.Middleware(rec))
	r.Post("/api/v3/configs/compile", func(w http.ResponseWriter, r *http.Request) {
		if status != http.StatusOK {
			render.Status(r, status)
			render.JSON(w, r, map[string]any{"message": http.StatusText(status)})
			return
		}
		render.JSON(w, r, body)
	})
	srv := httptest.NewServer(r)
	t.Cleanup(srv.Close)
	return srv
}

func TestClient_Compile(t *testing.T) {
	ctx := iostream.Testing(context.Background())

	t.Run("a compiled config is valid and carries the expanded YAML", func(t *testing.T) {
		rec := httprecorder.New()
		srv := compileServer(t, rec, http.StatusOK, map[string]any{
			"data": map[string]any{
				"attributes": map[string]any{
					"phase":           "ended",
					"outcome":         "succeeded",
					"compiled_config": "version: 2.1\njobs: {}",
				},
			},
		})

		c := apiclient.New(apiclient.Config{BaseURL: srv.URL, Token: "the-token"})
		res, err := c.Compile(ctx, apiclient.CompileInput{
			ConfigYAML:  "version: 2.1",
			OrgID:       testCompileOrgID,
			PreviewNext: true,
		})
		assert.NilError(t, err)
		assert.Check(t, cmp.Equal(res.Valid, true))
		assert.Check(t, cmp.Equal(res.CompiledYAML, "version: 2.1\njobs: {}"))
		assert.Check(t, cmp.Len(res.Errors, 0))

		var body map[string]any
		assert.NilError(t, rec.LastRequest().Decode(&body))
		assert.Check(t, cmp.DeepEqual(body, map[string]any{
			"data": map[string]any{
				"attributes": map[string]any{
					"config":              "version: 2.1",
					"should_preview_next": true,
				},
				"references": map[string]any{
					"org": map[string]any{"id": testCompileOrgID},
				},
			},
		}))
	})

	t.Run("outcome failed is an invalid config, not an error", func(t *testing.T) {
		rec := httprecorder.New()
		srv := compileServer(t, rec, http.StatusOK, map[string]any{
			"data": map[string]any{
				"attributes": map[string]any{
					"phase":   "ended",
					"outcome": "failed",
				},
			},
			"meta": map[string]any{
				"messages": []map[string]any{
					{"title": "jobs.build.docker: required key [image] not found"},
				},
			},
		})

		c := apiclient.New(apiclient.Config{BaseURL: srv.URL, Token: "the-token"})
		res, err := c.Compile(ctx, apiclient.CompileInput{ConfigYAML: "version: 2.1"})
		assert.NilError(t, err)
		assert.Check(t, cmp.Equal(res.Valid, false))
		assert.Check(t, cmp.Equal(res.CompiledYAML, ""))
		assert.Check(t, cmp.DeepEqual(res.Errors, []string{
			"jobs.build.docker: required key [image] not found",
		}))
	})

	t.Run("no org means no references member", func(t *testing.T) {
		rec := httprecorder.New()
		srv := compileServer(t, rec, http.StatusOK, map[string]any{
			"data": map[string]any{"attributes": map[string]any{"outcome": "succeeded"}},
		})

		c := apiclient.New(apiclient.Config{BaseURL: srv.URL, Token: "the-token"})
		_, err := c.Compile(ctx, apiclient.CompileInput{ConfigYAML: "version: 2.1"})
		assert.NilError(t, err)

		var body map[string]any
		assert.NilError(t, rec.LastRequest().Decode(&body))
		data, _ := body["data"].(map[string]any)
		assert.Assert(t, data != nil)
		_, hasRefs := data["references"]
		assert.Check(t, !hasRefs, "an absent org must not send an empty references object")
	})
}

// TestClient_Compile_RequestFailures pins that a request the endpoint refuses is
// an error, not a compile result: there is no second endpoint to try, so every
// such status has to reach the caller. A 404 in particular must not be mistaken
// for "invalid config" — it means the host does not serve the route.
func TestClient_Compile_RequestFailures(t *testing.T) {
	ctx := iostream.Testing(context.Background())

	for _, status := range []int{
		http.StatusBadRequest,
		http.StatusUnauthorized,
		http.StatusForbidden,
		http.StatusNotFound,
	} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			rec := httprecorder.New()
			srv := compileServer(t, rec, status, nil)

			c := apiclient.New(apiclient.Config{BaseURL: srv.URL, Token: "the-token"})
			_, err := c.Compile(ctx, apiclient.CompileInput{ConfigYAML: "version: 2.1"})
			assert.Check(t, err != nil)
			assert.Check(t, cmp.Len(rec.AllRequests(), 1))
		})
	}
}
