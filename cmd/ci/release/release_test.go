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

package main

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"gotest.tools/v3/assert"
	"gotest.tools/v3/assert/cmp"

	"github.com/CircleCI-Public/circleci-cli/clikit/iostream"
)

// fakeGitHub answers the requests release makes from canned responses, keyed by method and
// path, and records the bodies it was sent.
type fakeGitHub struct {
	t         *testing.T
	responses map[string]string
	mu        sync.Mutex
	sent      map[string]string
}

func newFakeGitHub(t *testing.T, responses map[string]string) (*fakeGitHub, *gitHub) {
	f := &fakeGitHub{t: t, responses: responses, sent: map[string]string{}}
	srv := httptest.NewServer(f)
	t.Cleanup(srv.Close)
	return f, newGitHub(srv.URL, "o/r", "gh")
}

func (f *fakeGitHub) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	key := r.Method + " " + strings.TrimPrefix(r.URL.Path, "/repos/o/r")
	if r.URL.RawQuery != "" {
		key += "?" + r.URL.RawQuery
	}
	body, _ := io.ReadAll(r.Body)
	f.mu.Lock()
	f.sent[key] = string(body)
	f.mu.Unlock()
	resp, ok := f.responses[key]
	if !ok {
		http.NotFound(w, r)
		return
	}
	_, _ = io.WriteString(w, resp)
}

func (f *fakeGitHub) wasSent(key string) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	_, ok := f.sent[key]
	return ok
}

func (f *fakeGitHub) body(key string) map[string]any {
	f.t.Helper()
	f.mu.Lock()
	defer f.mu.Unlock()
	s, ok := f.sent[key]
	if !ok {
		f.t.Fatalf("no request %s; got %v", key, f.sent)
	}
	var m map[string]any
	if err := json.Unmarshal([]byte(s), &m); err != nil {
		f.t.Fatalf("%s: %v", key, err)
	}
	return m
}

func testConfig() config {
	return config{
		Base: "main", SHA: "head",
		now: time.Date(2026, 10, 2, 12, 0, 0, 0, time.UTC),
	}
}

func TestRunPR(t *testing.T) {
	t.Run("leaves the PR alone while the version is unreleased", func(t *testing.T) {
		t.Chdir(t.TempDir())
		err := os.WriteFile(changelogFile, []byte(changelog), 0o644)
		assert.NilError(t, err)
		f, gh := newFakeGitHub(t, map[string]string{})

		err = runPR(iostream.Testing(context.Background()), testConfig(), gh)
		assert.NilError(t, err)

		assert.Check(t, f.wasSent("GET /git/ref/tags/v1.3.1"))
		assert.Check(t, !f.wasSent("GET /compare/v1.3.1...head?page=1&per_page=100"),
			"looked for merged PRs before the release was tagged")
	})
}

func TestUpdateReleasePR(t *testing.T) {
	t.Run("opens it", func(t *testing.T) {
		f, gh := newFakeGitHub(t, map[string]string{
			"GET /compare/v1.3.1...head?page=1&per_page=100": `{"commits": [{"sha": "a"}, {"sha": "b"}]}`,
			"GET /commits/a/pulls": `[
				{"number": 9, "merged_at": "2026-10-01T00:00:00Z", "base": {"ref": "main"}, "head": {"ref": "fix"}}
			]`,
			"GET /commits/b/pulls": `[
				{"number": 9, "merged_at": "2026-10-01T00:00:00Z", "base": {"ref": "main"}, "head": {"ref": "fix"}},
				{"number": 10, "merged_at": null, "base": {"ref": "main"}, "head": {"ref": "closed"}}
			]`,
			"GET /pulls?head=o%3Arelease%2Fnext&state=open": `[]`,
			"POST /releases/generate-notes": `{
				"body": "## What's Changed\n* Fix it by @me in https://github.com/o/r/pull/9"
			}`,
			"GET /git/commits/head": `{"tree": {"sha": "head-tree"}}`,
			"POST /git/trees":       `{"sha": "new-tree"}`,
			"POST /git/commits":     `{"sha": "release-commit"}`,
			"POST /git/refs":        `{}`,
			"POST /pulls":           `{}`,
		})

		err := updateReleasePR(iostream.Testing(context.Background()), testConfig(), gh, version{1, 3, 1}, changelog)
		assert.NilError(t, err)

		t.Run("asks for notes on the next version", func(t *testing.T) {
			notes := f.body("POST /releases/generate-notes")
			assert.Check(t, cmp.DeepEqual(notes, map[string]any{
				"tag_name": "v1.4.0", "previous_tag_name": "v1.3.1", "target_commitish": "head",
			}))
		})

		t.Run("adds the next version to the changelog", func(t *testing.T) {
			files := map[string]string{}
			tree := f.body("POST /git/trees")
			for _, e := range tree["tree"].([]any) {
				e := e.(map[string]any)
				files[e["path"].(string)] = e["content"].(string)
			}
			assert.Check(t, cmp.Equal(tree["base_tree"], "head-tree"))
			assert.Check(t, cmp.Contains(files["CHANGELOG.md"], "## [1.4.0] - 2026-10-02\n\n### What's Changed\n* Fix it"))
		})

		t.Run("pushes the branch and opens the PR", func(t *testing.T) {
			ref := f.body("POST /git/refs")
			assert.Check(t, cmp.DeepEqual(ref, map[string]any{"ref": "refs/heads/release/next", "sha": "release-commit"}))
			pr := f.body("POST /pulls")
			assert.Check(t, cmp.Equal(pr["title"], "Release v1.4.0"))
			assert.Check(t, cmp.Equal(pr["head"], "release/next"))
			assert.Check(t, cmp.Equal(pr["base"], "main"))
		})
	})

	t.Run("leaves an up-to-date branch", func(t *testing.T) {
		f, gh := newFakeGitHub(t, map[string]string{
			"GET /compare/v1.3.1...head?page=1&per_page=100": `{"commits": [{"sha": "a"}]}`,
			"GET /commits/a/pulls": `[
				{"number": 9, "merged_at": "2026-10-01T00:00:00Z", "base": {"ref": "main"}, "head": {"ref": "fix"},
					"labels": [{"name": "release:patch"}]}
			]`,
			"GET /pulls?head=o%3Arelease%2Fnext&state=open": `[{"number": 12}]`,
			"POST /releases/generate-notes":                 `{"body": "notes"}`,
			"GET /git/commits/head":                         `{"tree": {"sha": "head-tree"}}`,
			"POST /git/trees":                               `{"sha": "new-tree"}`,
			"GET /git/ref/heads/release/next":               `{"object": {"sha": "old-release-commit"}}`,
			"GET /git/commits/old-release-commit":           `{"tree": {"sha": "new-tree"}}`,
			"PATCH /pulls/12":                               `{}`,
		})

		err := updateReleasePR(iostream.Testing(context.Background()), testConfig(), gh, version{1, 3, 1}, changelog)
		assert.NilError(t, err)

		assert.Check(t, !f.wasSent("POST /git/commits"), "made a commit for an up-to-date branch")
		pr := f.body("PATCH /pulls/12")
		assert.Check(t, cmp.Equal(pr["title"], "Release v1.3.2"))
	})

	t.Run("closes it with nothing merged", func(t *testing.T) {
		f, gh := newFakeGitHub(t, map[string]string{
			"GET /compare/v1.4.0...head?page=1&per_page=100": `{"commits": []}`,
			"GET /pulls?head=o%3Arelease%2Fnext&state=open":  `[{"number": 12}]`,
			"PATCH /pulls/12":                     `{}`,
			"DELETE /git/refs/heads/release/next": ``,
		})

		err := updateReleasePR(iostream.Testing(context.Background()), testConfig(), gh, version{1, 4, 0}, changelog)
		assert.NilError(t, err)

		pr := f.body("PATCH /pulls/12")
		assert.Check(t, cmp.DeepEqual(pr, map[string]any{"state": "closed"}))
		assert.Check(t, f.wasSent("DELETE /git/refs/heads/release/next"), "didn't delete the release branch")
	})
}
