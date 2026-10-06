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
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/CircleCI-Public/circleci-cli/internal/httpcl"
)

// gitHub is a thin client for the GitHub REST API endpoints the release PR needs, over httpcl.
type gitHub struct {
	http *httpcl.Client
	repo string
}

func newGitHub(api, repo, token string) *gitHub {
	return &gitHub{
		http: httpcl.New(httpcl.Config{
			BaseURL:   strings.TrimRight(api, "/"),
			AuthToken: token,
			UserAgent: "circleci-cli-release",
			Timeout:   time.Minute,
		}),
		repo: repo,
	}
}

// call sends a request to a path under the repo, such as "/pulls". Paths are concatenated
// rather than passed as route params, which would escape the slash in release/next.
func (g *gitHub) call(ctx context.Context, method, path string, opts ...func(*httpcl.Request)) error {
	_, err := g.http.Call(ctx, httpcl.NewRequest(method, "/repos/"+g.repo+path, opts...))
	return err
}

func (g *gitHub) owner() string { return strings.Split(g.repo, "/")[0] }

func (g *gitHub) tagExists(ctx context.Context, tag string) (bool, error) {
	err := g.call(ctx, http.MethodGet, "/git/ref/tags/"+tag)
	if httpcl.HasStatusCode(err, http.StatusNotFound) {
		return false, nil
	}
	return err == nil, err
}

type pullRequest struct {
	Number   int    `json:"number"`
	MergedAt string `json:"merged_at"`
	Base     struct {
		Ref string `json:"ref"`
	} `json:"base"`
	Head struct {
		Ref string `json:"ref"`
	} `json:"head"`
	Labels []struct {
		Name string `json:"name"`
	} `json:"labels"`
}

// mergedPRs are the PRs to base merged between the tag and sha, other than release PRs.
func (g *gitHub) mergedPRs(ctx context.Context, tag, sha, base string) ([]pullRequest, error) {
	var commits []string
	for page := 1; ; page++ {
		var cmp struct {
			Commits []struct {
				SHA string `json:"sha"`
			} `json:"commits"`
		}
		if err := g.call(ctx, http.MethodGet, "/compare/"+tag+"..."+sha,
			httpcl.QueryParam("per_page", "100"),
			httpcl.QueryParam("page", strconv.Itoa(page)),
			httpcl.JSONDecoder(&cmp),
		); err != nil {
			return nil, err
		}
		for _, c := range cmp.Commits {
			commits = append(commits, c.SHA)
		}
		if len(cmp.Commits) < 100 {
			break
		}
	}

	seen := map[int]bool{}
	var prs []pullRequest
	for _, c := range commits {
		var cprs []pullRequest
		if err := g.call(ctx, http.MethodGet, "/commits/"+c+"/pulls", httpcl.JSONDecoder(&cprs)); err != nil {
			return nil, err
		}
		for _, pr := range cprs {
			if seen[pr.Number] || pr.MergedAt == "" || pr.Base.Ref != base || pr.Head.Ref == releaseBranch {
				continue
			}
			seen[pr.Number] = true
			prs = append(prs, pr)
		}
	}
	return prs, nil
}

// generateNotes are GitHub's release notes for the PRs merged since previous.
func (g *gitHub) generateNotes(ctx context.Context, tag, previous, sha string) (string, error) {
	var notes struct {
		Body string `json:"body"`
	}
	err := g.call(ctx, http.MethodPost, "/releases/generate-notes",
		httpcl.Body(map[string]string{
			"tag_name":          tag,
			"previous_tag_name": previous,
			"target_commitish":  sha,
		}),
		httpcl.JSONDecoder(&notes),
	)
	return notes.Body, err
}

func (g *gitHub) commitTree(ctx context.Context, sha string) (string, error) {
	var commit struct {
		Tree struct {
			SHA string `json:"sha"`
		} `json:"tree"`
	}
	err := g.call(ctx, http.MethodGet, "/git/commits/"+sha, httpcl.JSONDecoder(&commit))
	return commit.Tree.SHA, err
}

func (g *gitHub) createTree(ctx context.Context, baseTree string, files map[string]string) (string, error) {
	type entry struct {
		Path    string `json:"path"`
		Mode    string `json:"mode"`
		Type    string `json:"type"`
		Content string `json:"content"`
	}
	entries := make([]entry, 0, len(files))
	for path, content := range files {
		entries = append(entries, entry{Path: path, Mode: "100644", Type: "blob", Content: content})
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Path < entries[j].Path })
	var tree struct {
		SHA string `json:"sha"`
	}
	err := g.call(ctx, http.MethodPost, "/git/trees",
		httpcl.Body(map[string]any{"base_tree": baseTree, "tree": entries}),
		httpcl.JSONDecoder(&tree),
	)
	return tree.SHA, err
}

func (g *gitHub) createCommit(ctx context.Context, message, tree, parent string) (string, error) {
	var commit struct {
		SHA string `json:"sha"`
	}
	err := g.call(ctx, http.MethodPost, "/git/commits",
		httpcl.Body(map[string]any{"message": message, "tree": tree, "parents": []string{parent}}),
		httpcl.JSONDecoder(&commit),
	)
	return commit.SHA, err
}

// branch returns the commit the branch points to, or "" if it doesn't exist.
func (g *gitHub) branch(ctx context.Context, name string) (string, error) {
	var ref struct {
		Object struct {
			SHA string `json:"sha"`
		} `json:"object"`
	}
	err := g.call(ctx, http.MethodGet, "/git/ref/heads/"+name, httpcl.JSONDecoder(&ref))
	if httpcl.HasStatusCode(err, http.StatusNotFound) {
		return "", nil
	}
	return ref.Object.SHA, err
}

func (g *gitHub) setBranch(ctx context.Context, name, sha string, exists bool) error {
	if exists {
		return g.call(ctx, http.MethodPatch, "/git/refs/heads/"+name,
			httpcl.Body(map[string]any{"sha": sha, "force": true}))
	}
	return g.call(ctx, http.MethodPost, "/git/refs",
		httpcl.Body(map[string]string{"ref": "refs/heads/" + name, "sha": sha}))
}

func (g *gitHub) deleteBranch(ctx context.Context, name string) error {
	return g.call(ctx, http.MethodDelete, "/git/refs/heads/"+name)
}

// openPR returns the number of the open PR from head, or 0 if there isn't one.
func (g *gitHub) openPR(ctx context.Context, head string) (int, error) {
	var prs []pullRequest
	if err := g.call(ctx, http.MethodGet, "/pulls",
		httpcl.QueryParam("state", "open"),
		httpcl.QueryParam("head", g.owner()+":"+head),
		httpcl.JSONDecoder(&prs),
	); err != nil {
		return 0, err
	}
	if len(prs) == 0 {
		return 0, nil
	}
	return prs[0].Number, nil
}

func (g *gitHub) createPR(ctx context.Context, pr map[string]string) error {
	return g.call(ctx, http.MethodPost, "/pulls", httpcl.Body(pr))
}

func (g *gitHub) updatePR(ctx context.Context, number int, pr map[string]string) error {
	return g.call(ctx, http.MethodPatch, fmt.Sprintf("/pulls/%d", number), httpcl.Body(pr))
}

func (g *gitHub) closePR(ctx context.Context, number int) error {
	return g.updatePR(ctx, number, map[string]string{"state": "closed"})
}
