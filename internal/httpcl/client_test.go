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

package httpcl_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/render"
	"gotest.tools/v3/assert"
	"gotest.tools/v3/assert/cmp"

	"github.com/CircleCI-Public/circleci-cli-v2/internal/httpcl"
	"github.com/CircleCI-Public/circleci-cli-v2/internal/iostream"
)

func TestClient_Call(t *testing.T) {
	r := chi.NewMux()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Get("/hello/{id}/{child-id}", func(w http.ResponseWriter, r *http.Request) {
		render.JSON(w, r, map[string]any{
			"message":  "hello",
			"id":       chi.URLParam(r, "id"),
			"child-id": chi.URLParam(r, "child-id"),
			"query":    r.URL.Query().Encode(),
			"header":   r.Header.Get("X-Header"),
		})
	})

	srv := httptest.NewServer(r)
	t.Cleanup(srv.Close)

	c := httpcl.New(httpcl.Config{
		BaseURL:        srv.URL,
		DisableRetries: true,
	})

	ctx := iostream.Testing(context.Background())

	t.Run("check", func(t *testing.T) {
		var body map[string]any
		status, err := c.Call(ctx, httpcl.NewRequest(http.MethodGet, "/hello/%s/%d",
			httpcl.RouteParams("abc", 123),
			httpcl.QueryParam("q1", "first"),
			httpcl.QueryParam("q1", "second"),
			httpcl.QueryParam("q2", "other"),
			httpcl.Header("X-Header", "the-value"),
			httpcl.JSONDecoder(&body),
		))
		assert.NilError(t, err)
		assert.Check(t, cmp.Equal(status, http.StatusOK))
		assert.Check(t, cmp.DeepEqual(body, map[string]any{
			"id":       "abc",
			"header":   "the-value",
			"child-id": "123",
			"message":  "hello",
			"query":    "q1=first&q1=second&q2=other",
		}))
	})
}
