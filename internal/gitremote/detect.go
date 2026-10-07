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
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/go-git/go-git/v6"
	"github.com/go-git/go-git/v6/plumbing"

	"github.com/CircleCI-Public/circleci-cli/clikit/closer"
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
	// ErrSHAAmbiguous is returned by ExpandSHA when the short SHA is a prefix of
	// more than one commit in the local repository, so expanding it would pick
	// one arbitrarily.
	ErrSHAAmbiguous = errors.New("SHA prefix matches more than one commit")
	// ErrSHATooShort is returned by ExpandSHA for a prefix below
	// MinSHAPrefixLen, which git itself refuses to abbreviate to.
	ErrSHATooShort = errors.New("SHA prefix is too short to identify a commit")
)

// MinSHAPrefixLen is the shortest abbreviated SHA that may be expanded, matching
// git's own floor (MINIMUM_ABBREV). Below it, a prefix carries so little
// information that it is far more likely to be a typo than an abbreviation.
const MinSHAPrefixLen = 4

// fullSHALen is the length of an unabbreviated SHA-1 object ID in hex.
const fullSHALen = 40

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

// DetectRepoName returns a human-readable name for the checkout in the working
// directory, or "" when none can be determined. Callers use it as the suggested
// project name.
//
// The git remote is preferred. It is read directly rather than through Detect,
// which prefers .circleci/info.yml: a linked standalone project's slug is
// "circleci/<orgID>/<projectID>", so its last segment is an opaque ID — useless as
// a name to show a user.
//
// When the remote cannot be read — no origin, no origin/HEAD, an unsupported host
// — the name recorded by `circleci project link` is used instead. That is the only
// other place a readable name for this checkout exists, and without it a linked
// repository with no usable remote would offer no name at all.
func DetectRepoName() string {
	if info, err := DetectFromRemote(); err == nil {
		if parts := strings.Split(info.Slug, "/"); len(parts) == 3 {
			return parts[2]
		}
	}
	cwd, err := os.Getwd()
	if err != nil {
		return ""
	}
	ref, err := projectref.Read(cwd)
	if err != nil {
		return ""
	}
	return ref.Project.Name
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
	return DetectFromRemoteIn("")
}

// DetectFromRemoteIn is DetectFromRemote scoped to the repository containing dir.
// An empty dir means the process working directory.
//
// Commands that accept a directory argument must use this: reading the process
// working directory instead would describe a different repository than the one
// being operated on — or an enclosing one, since detection walks upward.
func DetectFromRemoteIn(dir string) (_ *ProjectInfo, err error) {
	// Both "not a git repo" and "repo without an origin remote" surface as the
	// same user-facing failure, matching the previous `git remote get-url`
	// behaviour.
	repo, err := openRepoIn(dir)
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

	// The slug is the only required field. Branch and default branch are
	// best-effort, matching gitBranches() on the linked-project path: a
	// locally-created repo (git init + remote add + push, rather than clone)
	// has no refs/remotes/origin/HEAD, and a fresh or detached checkout has no
	// current branch. None of these should turn a resolvable project into a
	// "could not detect project" failure — no consumer of ProjectInfo requires
	// either branch to be non-empty.
	branch, _ := gitCurrentBranch(repo)
	defaultBranch, _ := gitDefaultBranch(repo)

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

// RemoteRef identifies a repository as its remote URL names it, before any
// mapping onto CircleCI's own vocabulary.
type RemoteRef struct {
	// Host is the remote's host, e.g. "github.com".
	Host string
	// Owner and Repo name the repository, e.g. "acme" and "web".
	Owner, Repo string
}

// FullName is the "owner/repo" form the provider APIs use as a repository key.
func (r RemoteRef) FullName() string {
	if r.Owner == "" || r.Repo == "" {
		return ""
	}
	return r.Owner + "/" + r.Repo
}

// DetectRemoteRefIn reads the origin remote of the repository containing dir and
// returns what the URL says, without requiring the host to be one CircleCI has a
// slug form for.
//
// This is the parse to use when a caller resolves the integration itself (see
// internal/provider). DetectFromRemoteIn is the slug-producing path, and it
// rejects any host that has no slug segment.
func DetectRemoteRefIn(dir string) (_ RemoteRef, err error) {
	repo, err := openRepoIn(dir)
	if err != nil {
		return RemoteRef{}, fmt.Errorf("could not read git remote: %w", err)
	}
	defer closer.ErrorHandler(repo, &err)

	remoteURL, err := gitOriginURL(repo)
	if err != nil {
		return RemoteRef{}, fmt.Errorf("could not read git remote: %w", err)
	}
	return refFromRemote(remoteURL)
}

// refFromRemote parses a git remote URL into its host, owner and repository.
func refFromRemote(remoteURL string) (RemoteRef, error) {
	remoteURL = strings.TrimSpace(remoteURL)

	for _, re := range []*regexp.Regexp{sshRemote, sshProtoRemote, httpsRemote} {
		if m := re.FindStringSubmatch(remoteURL); m != nil {
			return RemoteRef{Host: m[1], Owner: m[2], Repo: m[3]}, nil
		}
	}
	return RemoteRef{}, fmt.Errorf("unrecognised git remote URL format: %q", remoteURL)
}

func slugFromRemote(remoteURL string) (string, error) {
	ref, err := refFromRemote(remoteURL)
	if err != nil {
		return "", err
	}
	return buildSlug(ref.Host, ref.Owner, ref.Repo)
}

func buildSlug(host, org, repo string) (string, error) {
	var vcs string
	switch {
	case strings.Contains(host, "github"):
		vcs = "gh"
	case strings.Contains(host, "bitbucket"):
		vcs = "bb"
	default:
		return "", fmt.Errorf("unsupported VCS host %q (expected github.com or bitbucket.org)", host)
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
	return openRepoIn("")
}

// RepoRootIn returns the root of the working tree containing dir, or an error
// when dir is not inside a git repository. An empty dir means the process working
// directory.
//
// Callers that write files describing the repository use this so that running
// from a subdirectory records them at the root, where they belong, rather than
// wherever the command happened to be invoked.
func RepoRootIn(dir string) (_ string, err error) {
	repo, err := openRepoIn(dir)
	if err != nil {
		return "", err
	}
	defer closer.ErrorHandler(repo, &err)

	wt, err := repo.Worktree()
	if err != nil {
		return "", err
	}
	return wt.Filesystem().Root(), nil
}

// openRepoIn opens the repository containing dir, walking upward to find it. An
// empty dir means the process working directory.
func openRepoIn(dir string) (*git.Repository, error) {
	if dir == "" {
		cwd, err := os.Getwd()
		if err != nil {
			return nil, err
		}
		dir = cwd
	}
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
// 40-character form using the repository containing dir. It returns the expanded
// SHA, lowercased, and nil on success. On failure it returns the original input
// unchanged — so callers can quote what the user typed — and one of
// ErrSHATooShort, ErrSHARepoInaccessible, ErrSHANotFound or ErrSHAAmbiguous.
// A SHA that is already 40 characters is lowercased and returned without opening
// a repository, so callers holding a full SHA never depend on local git state.
//
// sha must already be known to be hex; callers validate that themselves, since a
// non-SHA argument is a bad-argument error rather than a git failure.
//
// Expansion matches object-ID prefixes only. Deliberately not ResolveRevision:
// that resolves any revision expression, so a hex-looking branch or tag name
// ("1234", "deadbeef", a 20260101 date tag) would expand to that ref's tip and
// silently watch the wrong commit, and it reports no error for a prefix matching
// several commits — it returns whichever the object store enumerates first.
func ExpandSHAIn(dir, sha string) (string, error) {
	norm := strings.ToLower(sha)
	if len(norm) == fullSHALen {
		return norm, nil
	}
	if len(norm) < MinSHAPrefixLen {
		return sha, ErrSHATooShort
	}
	repo, err := openRepoIn(dir)
	if err != nil {
		return sha, ErrSHARepoInaccessible
	}
	defer func() { _ = repo.Close() }()

	commits := commitsWithPrefix(repo, norm)
	switch len(commits) {
	case 0:
		return sha, ErrSHANotFound
	case 1:
		return commits[0].String(), nil
	default:
		return sha, ErrSHAAmbiguous
	}
}

// commitsWithPrefix returns the distinct commits whose object ID starts with the
// lowercase hex prefix. Objects that are not commits are skipped, and an
// annotated tag is peeled to the commit it points at, so the result holds only
// hashes that can appear as a pipeline revision. A prefix naming several objects
// that peel to the same commit counts once, since there is nothing ambiguous
// about it from the caller's point of view.
func commitsWithPrefix(repo *git.Repository, prefix string) []plumbing.Hash {
	// hex.DecodeString only decodes whole bytes, so a prefix of odd length is
	// narrowed by its even part and the dangling nybble is filtered below.
	evenHex := prefix[:len(prefix)&^1]
	raw, err := hex.DecodeString(evenHex)
	if err != nil {
		return nil
	}

	var (
		out  []plumbing.Hash
		seen = map[plumbing.Hash]bool{}
	)
	for _, h := range hashesWithPrefix(repo, raw) {
		if !strings.HasPrefix(h.String(), prefix) {
			continue
		}
		commit, ok := peelToCommit(repo, h)
		if !ok || seen[commit] {
			continue
		}
		seen[commit] = true
		out = append(out, commit)
	}
	return out
}

// hashesWithPrefix lists every object ID in the repository beginning with the
// given raw byte prefix. The filesystem object storage can answer this directly;
// anything else is walked object by object.
func hashesWithPrefix(repo *git.Repository, prefix []byte) []plumbing.Hash {
	type prefixIndexer interface {
		HashesWithPrefix(prefix []byte) ([]plumbing.Hash, error)
	}
	if idx, ok := repo.Storer.(prefixIndexer); ok {
		hashes, err := idx.HashesWithPrefix(prefix)
		if err != nil {
			return nil
		}
		return hashes
	}

	iter, err := repo.Storer.IterEncodedObjects(plumbing.AnyObject)
	if err != nil {
		return nil
	}
	var hashes []plumbing.Hash
	_ = iter.ForEach(func(obj plumbing.EncodedObject) error {
		if h := obj.Hash(); h.HasPrefix(prefix) {
			hashes = append(hashes, h)
		}
		return nil
	})
	return hashes
}

// peelToCommit resolves an object ID to a commit: commits are returned as-is,
// annotated tags are followed to their target, and anything else (a blob or a
// tree) reports false.
func peelToCommit(repo *git.Repository, h plumbing.Hash) (plumbing.Hash, bool) {
	if _, err := repo.CommitObject(h); err == nil {
		return h, true
	}
	tag, err := repo.TagObject(h)
	if err != nil {
		return plumbing.ZeroHash, false
	}
	commit, err := tag.Commit()
	if err != nil {
		return plumbing.ZeroHash, false
	}
	return commit.Hash, true
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
