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
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"runtime"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"gotest.tools/v3/assert"
	"gotest.tools/v3/assert/cmp"

	"github.com/CircleCI-Public/circleci-cli/internal/apiclient"
	"github.com/CircleCI-Public/circleci-cli/internal/httpcl"
	"github.com/CircleCI-Public/circleci-cli/internal/iostream"
	"github.com/CircleCI-Public/circleci-cli/internal/testing/httprecorder"
	"github.com/CircleCI-Public/circleci-cli/internal/testing/httprecorder/chirecorder"
)

func TestClient_get(t *testing.T) {
	ctx := iostream.Testing(context.Background())

	rec := httprecorder.New()
	r := chi.NewMux()
	r.Use(chirecorder.Middleware(rec))
	r.Post("/hello", func(w http.ResponseWriter, r *http.Request) {
		render.JSON(w, r, map[string]any{"message": "hello"})
	})
	srv := httptest.NewServer(r)
	t.Cleanup(srv.Close)

	t.Run("Make a request", func(t *testing.T) {
		c := apiclient.New(srv.URL, "the-token", "1.2.3", nil)
		status, err := c.Do(ctx, http.MethodPost, "/hello",
			httpcl.Body(map[string]any{"hi": "there"}),
		)
		assert.NilError(t, err)
		assert.Check(t, cmp.Equal(status, http.StatusOK))
	})

	t.Run("Check request was recorded", func(t *testing.T) {
		assert.Check(t, cmp.DeepEqual(rec.LastRequest(), &httprecorder.Request{
			Method: http.MethodPost,
			URL:    url.URL{Path: "/hello"},
			Header: http.Header{
				"Accept":          {"application/json"},
				"Accept-Encoding": {"gzip"},
				"Authorization":   {"Bearer the-token"},
				"Content-Length":  {"14"},
				"Content-Type":    {"application/json; charset=utf-8"},
				"User-Agent":      {fmt.Sprintf("circleci-cli (%s/%s; 1.2.3)", runtime.GOOS, runtime.GOARCH)},
			},
			Body: new(`{"hi":"there"}`),
		}))
	})
}
