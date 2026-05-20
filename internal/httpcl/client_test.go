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
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/render"
	"gotest.tools/v3/assert"
	"gotest.tools/v3/assert/cmp"

	"github.com/CircleCI-Public/circleci-cli/internal/httpcl"
	"github.com/CircleCI-Public/circleci-cli/internal/iostream"
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
	r.Get("/status/{status}", func(w http.ResponseWriter, r *http.Request) {
		status, err := strconv.Atoi(chi.URLParam(r, "status"))
		if err != nil {
			render.Status(r, http.StatusBadRequest)
			return
		}

		render.Status(r, status)
		render.JSON(w, r, map[string]any{
			"message": fmt.Sprintf("status %d", status),
		})
	})

	srv := httptest.NewServer(r)
	t.Cleanup(srv.Close)

	c := httpcl.New(httpcl.Config{
		BaseURL: srv.URL,
	})

	ctx := iostream.Testing(context.Background())

	t.Run("parameters", func(t *testing.T) {
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

	t.Run("errors", func(t *testing.T) {
		tests := []struct {
			status      int
			expectError string
			expectBody  string
		}{
			{
				status:      http.StatusBadRequest,
				expectError: "GET /status/400: 400 Bad Request",
				expectBody:  `{"message":"status 400"}` + "\n",
			},
			{
				status:      http.StatusNotFound,
				expectError: "GET /status/404: 404 Not Found",
				expectBody:  `{"message":"status 404"}` + "\n",
			},
			{
				status:      http.StatusInternalServerError,
				expectError: "GET /status/500: 500 Internal Server Error",
				expectBody:  `{"message":"status 500"}` + "\n",
			},
			{
				status:      http.StatusBadGateway,
				expectError: "GET /status/502: 502 Bad Gateway",
				expectBody:  `{"message":"status 502"}` + "\n",
			},
		}

		for _, tt := range tests {
			t.Run(fmt.Sprintf("status %d", tt.status), func(t *testing.T) {
				var body map[string]any
				status, err := c.Call(ctx, httpcl.NewRequest(http.MethodGet, "/status/%d",
					httpcl.RouteParams(tt.status),
					httpcl.JSONDecoder(&body),
				))
				assert.Check(t, cmp.Error(err, tt.expectError))
				assert.Check(t, cmp.Equal(status, tt.status))
				assert.Check(t, cmp.Nil(body))
				assert.Check(t, httpcl.HasStatusCode(err, tt.status))
				httpError, ok := errors.AsType[*httpcl.HTTPError](err)
				assert.Check(t, ok)
				assert.Check(t, cmp.Equal(httpError.StatusCode, tt.status))
				assert.Check(t, cmp.Equal(string(httpError.Body), tt.expectBody))
			})
		}
	})

}
