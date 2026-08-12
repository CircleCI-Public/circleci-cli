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

package githubapp_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"gotest.tools/v3/assert"
	"gotest.tools/v3/assert/cmp"

	"github.com/CircleCI-Public/circleci-cli/clikit/iostream"
	"github.com/CircleCI-Public/circleci-cli/internal/apiclient"
	"github.com/CircleCI-Public/circleci-cli/internal/githubapp"
)

// testCtx returns a context wired with test IO streams, which the API client's
// debug logging requires.
func testCtx() context.Context {
	return iostream.Testing(context.Background())
}

// pagedReposServer serves the GitHub App repositories endpoint, returning at
// most two items per page regardless of the requested limit, so the pagination
// loop in ResolveRepoID is exercised across multiple pages.
func pagedReposServer(t *testing.T, fullNames []string) *apiclient.Client {
	t.Helper()

	type repo struct {
		ID       int64  `json:"id"`
		FullName string `json:"repo_full_name"`
	}
	all := make([]repo, len(fullNames))
	for i, fn := range fullNames {
		all[i] = repo{ID: int64(i + 1), FullName: fn}
	}

	const pageSize = 2
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		page, _ := strconv.Atoi(r.URL.Query().Get("page"))
		if page < 1 {
			page = 1
		}
		start := (page - 1) * pageSize
		end := start + pageSize
		if start > len(all) {
			start = len(all)
		}
		if end > len(all) {
			end = len(all)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"items":       all[start:end],
			"total_count": len(all),
		})
	}))
	t.Cleanup(srv.Close)

	return apiclient.New(apiclient.Config{BaseURL: srv.URL, Token: "test-token"})
}

// TestResolveRepoID_PageCap pins that exhausting the page cap is reported as its
// own condition rather than as "no match".
//
// Conflating them makes onboard tell the user the GitHub App cannot access a
// repository it may well have access to, and the advice that follows — grant
// access, then re-run — can never help, because the next run searches the same
// bounded prefix.
func TestResolveRepoID_PageCap(t *testing.T) {
	// A server that always has more pages: total_count never gets satisfied, so
	// the loop can only end by hitting its cap.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"items": []map[string]any{
				{"id": 1, "repo_full_name": "myorg/filler-a"},
				{"id": 2, "repo_full_name": "myorg/filler-b"},
			},
			"total_count": 1_000_000,
		})
	}))
	t.Cleanup(srv.Close)
	client := apiclient.New(apiclient.Config{BaseURL: srv.URL, Token: "test-token"})

	id, err := githubapp.ResolveRepoID(testCtx(), client, "org-1", "myorg/needle")

	assert.Check(t, cmp.Equal(id, ""))
	assert.Check(t, errors.Is(err, githubapp.ErrTooManyRepositories),
		"hitting the page cap must be distinguishable from an absent repository, got: %v", err)
}

func TestResolveRepoID(t *testing.T) {
	repos := []string{"myorg/a", "myorg/b", "myorg/c", "myorg/Target"}

	t.Run("matches a repo on a later page", func(t *testing.T) {
		client := pagedReposServer(t, repos)
		id, err := githubapp.ResolveRepoID(testCtx(), client, "org-uuid", "myorg/c")
		assert.NilError(t, err)
		assert.Equal(t, id, "3")
	})

	t.Run("matches case-insensitively", func(t *testing.T) {
		client := pagedReposServer(t, repos)
		id, err := githubapp.ResolveRepoID(testCtx(), client, "org-uuid", "MYORG/TARGET")
		assert.NilError(t, err)
		assert.Equal(t, id, "4")
	})

	t.Run("returns empty when not found", func(t *testing.T) {
		client := pagedReposServer(t, repos)
		id, err := githubapp.ResolveRepoID(testCtx(), client, "org-uuid", "myorg/missing")
		assert.NilError(t, err)
		assert.Equal(t, id, "")
	})

	t.Run("returns empty for a blank name without calling the API", func(t *testing.T) {
		client := pagedReposServer(t, repos)
		id, err := githubapp.ResolveRepoID(testCtx(), client, "org-uuid", "")
		assert.NilError(t, err)
		assert.Equal(t, id, "")
	})
}
