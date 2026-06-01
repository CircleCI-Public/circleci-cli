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

package httpnetrecorder_test

import (
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"gotest.tools/v3/assert"
	"gotest.tools/v3/assert/cmp"

	"github.com/CircleCI-Public/circleci-cli/internal/testing/httprecorder"
	"github.com/CircleCI-Public/circleci-cli/internal/testing/httprecorder/httpnetrecorder"
)

func TestMiddleware(t *testing.T) {
	rec := httprecorder.New()

	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, "a string body")
	})
	srv := httptest.NewServer(httpnetrecorder.Middleware(rec, h))
	t.Cleanup(srv.Close)

	t.Run("Make a request", func(t *testing.T) {
		res, err := http.Post(srv.URL+"/hello", "text/plain", strings.NewReader("sent string body"))
		assert.Assert(t, err)
		t.Cleanup(func() {
			assert.Check(t, res.Body.Close())
		})
		b, err := io.ReadAll(res.Body)
		assert.Check(t, err)
		assert.Check(t, cmp.Equal("a string body", string(b)))
	})

	t.Run("Check request was present", func(t *testing.T) {
		assert.Check(t, cmp.DeepEqual(
			[]httprecorder.Request{
				{
					Method: http.MethodPost,
					URL:    url.URL{Path: "/hello"},
					Header: http.Header{
						"Accept-Encoding": {"gzip"},
						"Content-Length":  {"16"},
						"Content-Type":    {"text/plain"},
						"User-Agent":      {"Go-http-client/1.1"},
					},
					Body: new("sent string body"),
				},
			},
			rec.AllRequests(),
		))
	})
}
