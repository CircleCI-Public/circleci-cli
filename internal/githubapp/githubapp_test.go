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

// connectionsServer serves the provider connections endpoint, returning one
// connection per provider named, and records the paths it was asked for so a test
// can assert the install flow was not entered.
func connectionsServer(t *testing.T, providers ...string) (*apiclient.Client, *[]string) {
	t.Helper()
	return connectionsServerWithSetup(t, map[string]any{
		"next_step": "redirect",
		"url":       "https://github.com/apps/circleci/installations/new?state=test",
	}, providers...)
}

// connectionsServerWithSetup serves the connections list, one connection per
// provider named, and answers a setup call with setupAttrs. The paths it was asked
// for are recorded so a test can assert whether an install was started.
func connectionsServerWithSetup(t *testing.T, setupAttrs any, providers ...string) (*apiclient.Client, *[]string) {
	t.Helper()

	var paths []string
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v3/provider/connections", func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.URL.Path)
		data := make([]map[string]any, 0, len(providers))
		for _, p := range providers {
			data = append(data, map[string]any{
				"id":         "conn-" + p,
				"attributes": map[string]any{"provider": p, "login": "myorg"},
			})
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"data": data})
	})
	mux.HandleFunc("/api/v3/provider/connections/setup", func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": map[string]any{"attributes": setupAttrs},
		})
	})

	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	return apiclient.New(apiclient.Config{BaseURL: srv.URL, Token: "test-token"}), &paths
}

func TestEnsureInstalled(t *testing.T) {
	t.Run("an existing connection needs no install", func(t *testing.T) {
		client, paths := connectionsServer(t, "github_app")

		installed, err := githubapp.EnsureInstalled(testCtx(), client, "org-uuid", "https://app.circleci.com/x", true)

		assert.NilError(t, err)
		assert.Check(t, installed)
		assert.Check(t, cmp.DeepEqual(*paths, []string{"/api/v3/provider/connections"}),
			"an installed app must not start an install")
	})

	t.Run("another provider's connection does not count", func(t *testing.T) {
		// Only the app's own provider means installed; a different integration on the
		// same org must still send the user through the install.
		client, _ := connectionsServer(t, "github_oauth")

		installed, err := githubapp.EnsureInstalled(testCtx(), client, "org-uuid", "https://app.circleci.com/x", true)

		assert.NilError(t, err)
		assert.Check(t, !installed)
	})

	t.Run("no connections sends the user to install", func(t *testing.T) {
		client, paths := connectionsServer(t)

		// noBrowser prints the URL and returns rather than polling.
		installed, err := githubapp.EnsureInstalled(testCtx(), client, "org-uuid", "https://app.circleci.com/x", true)

		assert.NilError(t, err)
		assert.Check(t, !installed)
		assert.Check(t, cmp.DeepEqual(*paths, []string{
			"/api/v3/provider/connections",
			"/api/v3/provider/connections/setup",
		}), "an absent connection must start one")
	})

	t.Run("a setup the CLI cannot finish is reported", func(t *testing.T) {
		// Registering an app from a manifest is a browser flow with no terminal
		// equivalent, so it must surface rather than open an empty URL.
		client, _ := connectionsServerWithSetup(t, map[string]any{
			"next_step":   "register",
			"state_token": "tok",
		})

		installed, err := githubapp.EnsureInstalled(testCtx(), client, "org-uuid", "https://app.circleci.com/x", true)

		assert.Check(t, err != nil, "a register step has no CLI path and must not pass silently")
		assert.Check(t, !installed)
	})

	t.Run("a failed check is an error, not a missing install", func(t *testing.T) {
		// Reporting "not installed" here would walk the user into an install flow to
		// fix a problem that is not a missing installation.
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		}))
		t.Cleanup(srv.Close)
		client := apiclient.New(apiclient.Config{BaseURL: srv.URL, Token: "test-token"})

		installed, err := githubapp.EnsureInstalled(testCtx(), client, "org-uuid", "https://app.circleci.com/x", true)

		assert.Check(t, err != nil, "a 500 from the connections endpoint must surface")
		assert.Check(t, !installed)
	})
}
