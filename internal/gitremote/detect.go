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

// Package gitremote resolves the CircleCI project slug for the current
// working directory. Resolution prefers the per-checkout .circleci/info.yml
// recorded by `circleci project link` (so repository renames and standalone
// projects stay addressable), falling back to parsing the git remote URL.
package gitremote

import (
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"

	"github.com/CircleCI-Public/circleci-cli/internal/projectref"
)

// ProjectInfo holds the information needed to identify a CircleCI project.
type ProjectInfo struct {
	// Slug is the CircleCI project slug, e.g. "gh/myorg/myrepo".
	Slug string
	// Branch is the current git branch name.
	Branch string
	// DefaultBranch is the default branch name.
	DefaultBranch string
}

var (
	// matches git@github.com:org/repo.git (SCP-style)
	sshRemote = regexp.MustCompile(`^git@([^:]+):([^/]+)/(.+?)(?:\.git)?$`)
	// matches ssh://git@github.com/org/repo.git (protocol-style)
	sshProtoRemote = regexp.MustCompile(`^ssh://git@([^/]+)/([^/]+)/(.+?)(?:\.git)?$`)
	// matches https://github.com/org/repo.git
	httpsRemote = regexp.MustCompile(`^https?://([^/]+)/([^/]+)/(.+?)(?:\.git)?$`)
)

// DetectNamespace returns the organization name (namespace) from the git remote.
// For a slug like "gh/myorg/myrepo" it returns "myorg".
func DetectNamespace() (string, error) {
	info, err := Detect()
	if err != nil {
		return "", err
	}
	parts := strings.Split(info.Slug, "/")
	if len(parts) != 3 {
		return "", fmt.Errorf("unexpected slug format: %q", info.Slug)
	}
	return parts[1], nil
}

// Detect resolves the CircleCI project for the current working directory.
//
// Resolution priority:
//  1. .circleci/info.yml in the working directory (written by `circleci project link`).
//     When this file carries both project_id and organization_id, the canonical
//     "circleci/<orgID>/<projectID>" slug is returned so lookups survive VCS-side
//     renames; otherwise the file's stored slug is returned verbatim.
//  2. The git remote "origin" URL.
//
// The branch is always read from git (best-effort when info.yml supplied the slug,
// since the branch is per-checkout and never persisted in info.yml).
func Detect() (*ProjectInfo, error) {
	if cwd, err := os.Getwd(); err == nil {
		if ref, err := projectref.Read(cwd); err == nil {
			// Branch info is per-checkout and never persisted in info.yml, so
			// read it from the repo best-effort — a missing or non-git dir just
			// leaves the fields empty.
			var branch, defaultBranch string
			if repo, err := openRepo(); err == nil {
				branch, _ = gitCurrentBranch(repo)
				defaultBranch, _ = gitDefaultBranch(repo)
			}
			return &ProjectInfo{
				Slug:          ref.EffectiveSlug(),
				Branch:        branch,
				DefaultBranch: defaultBranch,
			}, nil
		}
	}
	return DetectFromRemote()
}

// DetectFromRemote resolves the project from the git "origin" remote without
// consulting .circleci/info.yml. Use this from the `project link` command itself
// — reading info.yml there would short-circuit the very write that link is
// about to perform.
func DetectFromRemote() (*ProjectInfo, error) {
	// Both "not a git repo" and "repo without an origin remote" surface as the
	// same user-facing failure, matching the previous `git remote get-url`
	// behaviour.
	repo, err := openRepo()
	if err != nil {
		return nil, fmt.Errorf("could not read git remote: %w", err)
	}

	remoteURL, err := gitOriginURL(repo)
	if err != nil {
		return nil, fmt.Errorf("could not read git remote: %w", err)
	}

	slug, err := slugFromRemote(remoteURL)
	if err != nil {
		return nil, err
	}

	branch, err := gitCurrentBranch(repo)
	if err != nil {
		return nil, fmt.Errorf("could not determine current branch: %w", err)
	}

	defaultBranch, err := gitDefaultBranch(repo)
	if err != nil {
		return nil, fmt.Errorf("could not determine default branch: %w", err)
	}

	return &ProjectInfo{
		Slug:          slug,
		Branch:        branch,
		DefaultBranch: defaultBranch,
	}, nil
}

// SlugFromRemote is exported for testing.
func SlugFromRemote(remoteURL string) (string, error) {
	return slugFromRemote(remoteURL)
}

func slugFromRemote(remoteURL string) (string, error) {
	remoteURL = strings.TrimSpace(remoteURL)

	if m := sshRemote.FindStringSubmatch(remoteURL); m != nil {
		host, org, repo := m[1], m[2], m[3]
		return buildSlug(host, org, repo)
	}

	if m := sshProtoRemote.FindStringSubmatch(remoteURL); m != nil {
		host, org, repo := m[1], m[2], m[3]
		return buildSlug(host, org, repo)
	}

	if m := httpsRemote.FindStringSubmatch(remoteURL); m != nil {
		host, org, repo := m[1], m[2], m[3]
		return buildSlug(host, org, repo)
	}

	return "", fmt.Errorf("unrecognised git remote URL format: %q", remoteURL)
}

func buildSlug(host, org, repo string) (string, error) {
	var vcs string
	switch {
	case strings.Contains(host, "github"):
		vcs = "gh"
	case strings.Contains(host, "bitbucket"):
		vcs = "bb"
	case strings.Contains(host, "gitlab"):
		vcs = "gl"
	default:
		return "", fmt.Errorf("unsupported VCS host %q (expected github.com, bitbucket.org, or gitlab.com)", host)
	}
	return fmt.Sprintf("%s/%s/%s", vcs, org, repo), nil
}

// openRepo opens the git repository containing the current working directory,
// walking up parent directories to find the .git dir (like the git CLI does).
func openRepo() (*git.Repository, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	return git.PlainOpenWithOptions(cwd, &git.PlainOpenOptions{DetectDotGit: true})
}

// gitOriginURL returns the first configured URL for the "origin" remote,
// equivalent to `git remote get-url origin`.
func gitOriginURL(repo *git.Repository) (string, error) {
	remote, err := repo.Remote("origin")
	if err != nil {
		return "", err
	}
	urls := remote.Config().URLs
	if len(urls) == 0 {
		return "", fmt.Errorf("remote %q has no URL configured", "origin")
	}
	return urls[0], nil
}

// gitCurrentBranch returns the short name of the checked-out branch, or "HEAD"
// in detached-HEAD state — matching `git rev-parse --abbrev-ref HEAD`.
func gitCurrentBranch(repo *git.Repository) (string, error) {
	head, err := repo.Head()
	if err != nil {
		return "", err
	}
	return head.Name().Short(), nil
}

// gitDefaultBranch returns the short name of the remote default branch (e.g.
// "main"), read from the symbolic ref refs/remotes/origin/HEAD. This is the
// "origin/"-stripped equivalent of `git rev-parse --abbrev-ref origin/HEAD`.
func gitDefaultBranch(repo *git.Repository) (string, error) {
	ref, err := repo.Reference(plumbing.ReferenceName("refs/remotes/origin/HEAD"), false)
	if err != nil {
		return "", err
	}
	target := ref.Target()
	if target == "" {
		return "", fmt.Errorf("origin/HEAD is not a symbolic reference")
	}
	return strings.TrimPrefix(target.Short(), "origin/"), nil
}
