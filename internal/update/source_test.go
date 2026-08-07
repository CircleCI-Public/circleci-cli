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

package update

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"gotest.tools/v3/assert"
	"gotest.tools/v3/assert/cmp"

	"github.com/CircleCI-Public/circleci-cli/internal/apiclient"
)

// releaseServer starts a fake /api/v3/tool/releases server. handler is invoked
// for each request and records the query it saw into gotFilter.
func releaseServer(t *testing.T, handler http.HandlerFunc) (*apiclient.Client, *string) {
	t.Helper()
	var lastFilter string
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v3/tool/releases", func(w http.ResponseWriter, r *http.Request) {
		lastFilter = r.URL.Query().Get("filter[tool]")
		handler(w, r)
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	client := apiclient.New(apiclient.Config{BaseURL: srv.URL, Token: "tok", Version: "1.2.3"})
	return client, &lastFilter
}

func TestProxySource_Success(t *testing.T) {
	client, gotFilter := releaseServer(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[{"id":"b0f8c1e2-4d3a-5f6b-8c7d-9e0f1a2b3c4d","attributes":{"tool":"circleci-cli","version":"1.3.0","published_at":"2026-08-05T09:12:00.000Z"}}]}`))
	})

	rel, err := NewProxySource(client).Latest(testCtx())
	assert.NilError(t, err)
	assert.Assert(t, rel != nil)
	assert.Check(t, cmp.Equal(rel.Version, "1.3.0"))
	assert.Check(t, cmp.Equal(rel.PublishedAt.IsZero(), false))
	assert.Check(t, cmp.Equal(*gotFilter, "circleci-cli"))
}

func TestProxySource_EmptyData_IsFailure(t *testing.T) {
	client, _ := releaseServer(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[]}`))
	})

	// An empty collection must be treated as a fetch failure, not "no update".
	rel, err := NewProxySource(client).Latest(testCtx())
	assert.Assert(t, err != nil)
	assert.Check(t, cmp.Nil(rel))
}

func TestProxySource_503_IsError(t *testing.T) {
	client, _ := releaseServer(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	})

	rel, err := NewProxySource(client).Latest(testCtx())
	assert.Assert(t, err != nil) // transient → surfaced so no state is written
	assert.Check(t, cmp.Nil(rel))
}

func TestProxySource_400_IsSilent(t *testing.T) {
	client, _ := releaseServer(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
	})

	rel, err := NewProxySource(client).Latest(testCtx())
	assert.NilError(t, err) // our bug, never surfaced
	assert.Check(t, cmp.Nil(rel))
}

func TestProxySource_401_IsSilent(t *testing.T) {
	client, _ := releaseServer(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	})

	rel, err := NewProxySource(client).Latest(testCtx())
	assert.NilError(t, err) // never look like an auth problem
	assert.Check(t, cmp.Nil(rel))
}
