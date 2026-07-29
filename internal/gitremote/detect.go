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
	"errors"
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/go-git/go-git/v6"
	"github.com/go-git/go-git/v6/plumbing"

	"github.com/CircleCI-Public/circleci-cli/internal/closer"
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
	// OrgID is the organization ID recorded by `circleci project link`
	// (.circleci/info.yml). It is empty when the project was resolved from the
	// git remote, because the org ID is not derivable from a remote URL without
	// an API lookup. Its form is whatever link persisted (a UUID, or a compact
	// base62 ID); consumers that need a UUID must parse and fall back on failure.
	OrgID string
}

var (
	// matches git@github.com:org/repo.git (SCP-style)
	sshRemote = regexp.MustCompile(`^git@([^:]+):([^/]+)/(.+?)(?:\.git)?$`)
	// matches ssh://git@github.com/org/repo.git (protocol-style)
	sshProtoRemote = regexp.MustCompile(`^ssh://git@([^/]+)/([^/]+)/(.+?)(?:\.git)?$`)
	// matches https://github.com/org/repo.git
	httpsRemote = regexp.MustCompile(`^https?://([^/]+)/([^/]+)/(.+?)(?:\.git)?$`)
)

var (
	// ErrSHARepoInaccessible is returned by ExpandSHA when the local git
	// repository cannot be opened, so a short SHA cannot be expanded.
	ErrSHARepoInaccessible = errors.New("local git repository is not accessible")
	// ErrSHANotFound is returned by ExpandSHA when the short SHA does not
	// resolve to any object in the local repository.
	ErrSHANotFound = errors.New("SHA not found in local repository")
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

// DetectRepoName returns the repository name from the git remote, or "" if it
// cannot be detected.
func DetectRepoName() string {
	info, err := Detect()
	if err != nil {
		return ""
	}
	parts := strings.Split(info.Slug, "/")
	if len(parts) == 3 {
		return parts[2]
	}
	return ""
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
	info, err := detectLinkedProject()
	if err != nil {
		return nil, err
	}
	if info != nil {
		return info, nil
	}
	return DetectFromRemote()
}

// detectLinkedProject resolves the project from the .circleci/info.yml written
// by `circleci project link`. It returns (nil, nil) when no info.yml is present
// so Detect can fall back to the git remote; a malformed or unreadable info.yml
// is a real error and is surfaced rather than silently ignored.
func detectLinkedProject() (*ProjectInfo, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("could not determine working directory: %w", err)
	}

	ref, err := projectref.Read(cwd)
	if errors.Is(err, projectref.ErrNotFound) {
		return nil, nil // not linked — caller falls back to the git remote
	}
	if err != nil {
		return nil, err
	}

	// Branch and default branch are per-checkout and never persisted in
	// info.yml, so they are read from git best-effort.
	branch, defaultBranch := gitBranches()
	return &ProjectInfo{
		Slug:          ref.EffectiveSlug(),
		Branch:        branch,
		DefaultBranch: defaultBranch,
		OrgID:         ref.Organization.ID,
	}, nil
}

// gitBranches reads the current and default branch of the repository in the
// working directory, best-effort: any failure (not a git repo, detached HEAD,
// no origin/HEAD) leaves that field empty. The repository handle is always
// closed, so callers on Windows can delete the checkout afterwards.
func gitBranches() (branch, defaultBranch string) {
	repo, err := openRepo()
	if err != nil {
		return "", ""
	}
	defer func() { _ = repo.Close() }()
	branch, _ = gitCurrentBranch(repo)
	defaultBranch, _ = gitDefaultBranch(repo)
	return branch, defaultBranch
}

// DetectFromRemote resolves the project from the git "origin" remote without
// consulting .circleci/info.yml. Use this from the `project link` command itself
// — reading info.yml there would short-circuit the very write that link is
// about to perform.
func DetectFromRemote() (_ *ProjectInfo, err error) {
	// Both "not a git repo" and "repo without an origin remote" surface as the
	// same user-facing failure, matching the previous `git remote get-url`
	// behaviour.
	repo, err := openRepo()
	if err != nil {
		return nil, fmt.Errorf("could not read git remote: %w", err)
	}
	defer closer.ErrorHandler(repo, &err)

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
//
// Linked worktrees resolve correctly: inside a worktree, .git is a file pointing
// at <main>/.git/worktrees/<name>/, which holds only per-worktree state (HEAD,
// index), while the shared config, packed-refs, and refs/remotes live in the
// common dir. go-git v6 always follows the worktree's "commondir" pointer, so
// the origin remote and default branch are visible from a worktree without any
// extra option. Callers must Close the returned repository to release its file
// handles (Windows cannot delete files with open handles — see the tests).
func openRepo() (*git.Repository, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	return openRepoAt(cwd)
}

// openRepoAt is openRepo for an explicit starting directory, letting tests point
// at a temporary checkout instead of mutating the process working directory. The
// same worktree resolution and handle-closing notes on openRepo apply.
func openRepoAt(dir string) (*git.Repository, error) {
	return git.PlainOpenWithOptions(dir, &git.PlainOpenOptions{DetectDotGit: true})
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

// ExpandSHA resolves an abbreviated git SHA against the repository containing
// the current working directory. See ExpandSHAIn for the contract.
func ExpandSHA(sha string) (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return sha, ErrSHARepoInaccessible
	}
	return ExpandSHAIn(cwd, sha)
}

// ExpandSHAIn attempts to resolve an abbreviated git SHA to its full
// 40-character form using the repository containing dir. It returns the
// (possibly expanded) SHA and nil on success, or the original input and either
// ErrSHARepoInaccessible or ErrSHANotFound on failure. A SHA that is already 40
// characters is returned as-is without opening a repository, so callers holding
// a full SHA never depend on local git state.
//
// sha must already be known to be hex; callers validate that themselves, since a
// non-SHA argument is a bad-argument error rather than a git failure.
// ResolveRevision accepts any revision expression — branch names, tags, HEAD~3 —
// so passing unvalidated input here would silently resolve those instead.
func ExpandSHAIn(dir, sha string) (string, error) {
	if len(sha) == 40 {
		return sha, nil
	}
	repo, err := openRepoAt(dir)
	if err != nil {
		return sha, ErrSHARepoInaccessible
	}
	defer func() { _ = repo.Close() }()

	hash, err := repo.ResolveRevision(plumbing.Revision(sha))
	if err != nil {
		return sha, ErrSHANotFound
	}
	return hash.String(), nil
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
