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

package fakes_test

import (
	"context"
	"net/http"
	"testing"

	"gotest.tools/v3/assert"
	"gotest.tools/v3/assert/cmp"

	"github.com/CircleCI-Public/circleci-cli/internal/testing/fakes"
)

// get issues a bare GET to the fake with the given bearer token ("" = none) and
// returns the status code. The path need not resolve to real data — auth runs
// as middleware ahead of the handler, so a rejected request never reaches it.
func get(t *testing.T, base, path, token string) int {
	t.Helper()
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, base+path, nil)
	assert.NilError(t, err)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := http.DefaultClient.Do(req)
	assert.NilError(t, err)
	assert.NilError(t, resp.Body.Close())
	return resp.StatusCode
}

// TestAuthEnforcement pins the fake's authentication contract: a non-exempt
// route needs an accepted bearer token, and everything else is a 401.
func TestAuthEnforcement(t *testing.T) {
	fake := fakes.NewCircleCI(t)

	t.Run("missing token is rejected", func(t *testing.T) {
		assert.Check(t, cmp.Equal(get(t, fake.URL(), "/api/v3/runs/whatever", ""), http.StatusUnauthorized))
	})

	t.Run("wrong token is rejected", func(t *testing.T) {
		assert.Check(t, cmp.Equal(get(t, fake.URL(), "/api/v3/runs/whatever", "nope"), http.StatusUnauthorized))
	})

	t.Run("default token is accepted", func(t *testing.T) {
		// Accepted by auth, so it reaches the handler — which 404s the unknown
		// id. The point is only that it is not a 401.
		assert.Check(t, cmp.Equal(get(t, fake.URL(), "/api/v3/runs/whatever", fakes.DefaultToken), http.StatusNotFound))
	})

	t.Run("exempt routes need no token", func(t *testing.T) {
		// The tool-releases feed is public; an unauthenticated request is served
		// (400 here for the missing required filter, not 401).
		assert.Check(t, cmp.Equal(get(t, fake.URL(), "/api/v3/tool/releases", ""), http.StatusBadRequest))
	})
}

// TestRequireTokens shows a test can pin the exact accepted token, rejecting the
// default in favour of its own.
func TestRequireTokens(t *testing.T) {
	fake := fakes.NewCircleCI(t)
	fake.RequireTokens("custom-token")

	assert.Check(t, cmp.Equal(get(t, fake.URL(), "/api/v3/runs/whatever", fakes.DefaultToken), http.StatusUnauthorized))
	assert.Check(t, cmp.Equal(get(t, fake.URL(), "/api/v3/runs/whatever", "custom-token"), http.StatusNotFound))
}
