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

// Package preflight runs uncommitted local changes through CircleCI: it
// snapshots the working tree into a commit on a throwaway branch, pushes that
// branch so the project's push trigger starts a run, watches the run and
// reports failures as they happen, then deletes the branch.
package preflight

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
)

// BranchPrefix is the namespace every preflight branch is pushed under. The
// UUID that follows it identifies one preflight, so cleanup can target it.
const BranchPrefix = "cci/preflight/"

// DefaultRemote is the remote preflight branches are pushed to.
const DefaultRemote = "origin"

// fallbackIdent is used for the snapshot commit when the repository has no
// user.name / user.email configured, so a fresh clone in CI or a container can
// still run a preflight. The commit never lands on a real branch.
const (
	fallbackName  = "CircleCI Preflight"
	fallbackEmail = "preflight@circleci.invalid"
)

// BranchName returns the remote branch name for the preflight with the given ID.
func BranchName(id uuid.UUID) string {
	return BranchPrefix + id.String()
}

// ParseBranchName extracts the preflight ID from a branch name produced by
// BranchName. ok is false for any other branch.
func ParseBranchName(name string) (id uuid.UUID, ok bool) {
	rest, found := strings.CutPrefix(name, BranchPrefix)
	if !found {
		return uuid.Nil, false
	}
	id, err := uuid.Parse(rest)
	if err != nil {
		return uuid.Nil, false
	}
	return id, true
}

// GitError is a failed git invocation. Stderr carries git's own explanation,
// which is what a user needs to see (authentication, a rejected push, …).
type GitError struct {
	Args   []string
	Stderr string
	Err    error
}

func (e *GitError) Error() string {
	msg := strings.TrimSpace(e.Stderr)
	if msg == "" {
		msg = e.Err.Error()
	}
	return fmt.Sprintf("git %s: %s", strings.Join(e.Args, " "), msg)
}

func (e *GitError) Unwrap() error { return e.Err }

// ErrNoCommits is returned by Snapshot when HEAD does not point at a commit
// yet. The snapshot is a child of HEAD, so a repository with no history has
// nothing to build it on.
var ErrNoCommits = errors.New("the repository has no commits yet")

// Repo is a git checkout preflight operates on. All git work shells out to the
// git binary rather than using go-git, so ignore rules, credential helpers and
// remote configuration behave exactly as they do for the user's own pushes.
type Repo struct {
	// Dir is the top level of the working tree.
	Dir string
	// Remote is the remote branches are pushed to and deleted from.
	Remote string
}

// OpenRepo returns the repository containing dir, pushing to remote.
func OpenRepo(ctx context.Context, dir, remote string) (*Repo, error) {
	r := &Repo{Dir: dir, Remote: remote}
	top, err := r.git(ctx, nil, "rev-parse", "--show-toplevel")
	if err != nil {
		return nil, err
	}
	r.Dir = top
	return r, nil
}

// Snapshot is the commit a preflight pushes.
type Snapshot struct {
	// Commit is the SHA to push.
	Commit string
	// Base is Commit's parent: where HEAD forked from the remote's default
	// branch, or HEAD itself when that cannot be determined.
	Base string
	// BaseBranch is the default branch Base was found on, or "" when Base
	// fell back to HEAD.
	BaseBranch string
	// Changed is false when the working tree matched Base, in which case
	// Commit is Base itself.
	Changed bool
}

// Snapshot records the whole working tree — staged, unstaged and untracked
// files, minus anything .gitignore excludes — as one commit.
//
// The commit's parent is where HEAD forked from the remote's default branch,
// not HEAD, so its diff shows everything being changed: the branch's own
// commits and the uncommitted work together. The tree built is the working tree
// either way. When the working tree matches the base there is nothing to show,
// and the base itself is returned rather than an empty commit.
//
// Nothing the user can see changes: the work happens in a throwaway copy of the
// index (GIT_INDEX_FILE), so HEAD, the real index, the working tree and the
// stash are all left as they were. Copying the real index first, rather than
// starting from HEAD's tree, keeps git's stat cache, so only files that actually
// changed are re-hashed.
func (r *Repo) Snapshot(ctx context.Context, message string) (Snapshot, error) {
	head, err := r.git(ctx, nil, "rev-parse", "--verify", "--quiet", "HEAD^{commit}")
	if err != nil {
		return Snapshot{}, ErrNoCommits
	}

	tmp, err := os.MkdirTemp("", "circleci-preflight-")
	if err != nil {
		return Snapshot{}, err
	}
	defer func() { _ = os.RemoveAll(tmp) }()
	indexFile := filepath.Join(tmp, "index")

	realIndex, err := r.git(ctx, nil, "rev-parse", "--path-format=absolute", "--git-path", "index")
	if err != nil {
		return Snapshot{}, err
	}
	indexEnv := []string{"GIT_INDEX_FILE=" + indexFile}
	if err := copyFile(realIndex, indexFile); err != nil {
		// No index yet (or unreadable): build the temporary one from HEAD.
		if _, err := r.git(ctx, indexEnv, "read-tree", head); err != nil {
			return Snapshot{}, err
		}
	}

	if _, err := r.git(ctx, indexEnv, "add", "--all", "--", "."); err != nil {
		return Snapshot{}, err
	}
	tree, err := r.git(ctx, indexEnv, "write-tree")
	if err != nil {
		return Snapshot{}, err
	}

	snap := Snapshot{Base: head}
	if base, branch, ok := r.forkPoint(ctx); ok {
		snap.Base, snap.BaseBranch = base, branch
	}
	baseTree, err := r.git(ctx, nil, "rev-parse", snap.Base+"^{tree}")
	if err != nil {
		return Snapshot{}, err
	}
	if tree == baseTree {
		snap.Commit = snap.Base
		return snap, nil
	}
	snap.Commit, err = r.git(ctx, r.identityEnv(ctx), "commit-tree", tree, "-p", snap.Base, "-m", message)
	if err != nil {
		return Snapshot{}, err
	}
	snap.Changed = true
	return snap, nil
}

// forkPoint returns where HEAD forked from the remote's default branch, and
// that branch's name. ok is false when the default branch is unknown or has no
// remote-tracking ref locally; the caller then parents the snapshot on HEAD.
// The tracking ref is used as last fetched, never refreshed: a stale one still
// gives an ancestor of HEAD, just an older one.
func (r *Repo) forkPoint(ctx context.Context) (sha, branch string, ok bool) {
	branch = r.defaultBranch(ctx)
	if branch == "" {
		return "", "", false
	}
	sha, err := r.git(ctx, nil, "merge-base", "HEAD", "refs/remotes/"+r.Remote+"/"+branch)
	if err != nil {
		return "", "", false
	}
	return sha, branch, true
}

// defaultBranch returns the remote's default branch: from the local
// <remote>/HEAD when it is set (as clone sets it), otherwise by asking the
// remote. It returns "" when neither works.
func (r *Repo) defaultBranch(ctx context.Context) string {
	if ref, err := r.git(ctx, nil, "symbolic-ref", "--quiet", "refs/remotes/"+r.Remote+"/HEAD"); err == nil {
		return strings.TrimPrefix(ref, "refs/remotes/"+r.Remote+"/")
	}
	out, err := r.git(ctx, nil, "ls-remote", "--symref", r.Remote, "HEAD")
	if err != nil {
		return ""
	}
	for line := range strings.SplitSeq(out, "\n") {
		if ref, ok := strings.CutPrefix(line, "ref: refs/heads/"); ok {
			name, _, _ := strings.Cut(ref, "\t")
			return name
		}
	}
	return ""
}

// Push pushes sha to branch on the remote. Pre-push hooks are skipped: the
// point of a preflight is to let CI judge work in progress, and a hook that
// rejects that work would defeat it.
func (r *Repo) Push(ctx context.Context, sha, branch string) error {
	_, err := r.git(ctx, nil, "push", "--no-verify", "--quiet", r.Remote, sha+":refs/heads/"+branch)
	return err
}

// DeleteBranch deletes branch from the remote.
func (r *Repo) DeleteBranch(ctx context.Context, branch string) error {
	_, err := r.git(ctx, nil, "push", "--no-verify", "--quiet", r.Remote, "--delete", "refs/heads/"+branch)
	return err
}

// RemoteBranch is a preflight branch that exists on the remote.
type RemoteBranch struct {
	ID     uuid.UUID
	Name   string
	Commit string
}

// ListBranches returns every preflight branch on the remote. Branches under
// BranchPrefix whose name is not a UUID were not made by preflight and are
// left out.
func (r *Repo) ListBranches(ctx context.Context) ([]RemoteBranch, error) {
	out, err := r.git(ctx, nil, "ls-remote", "--heads", r.Remote, "refs/heads/"+BranchPrefix+"*")
	if err != nil {
		return nil, err
	}
	var branches []RemoteBranch
	for line := range strings.SplitSeq(out, "\n") {
		sha, ref, ok := strings.Cut(strings.TrimSpace(line), "\t")
		if !ok {
			continue
		}
		name := strings.TrimPrefix(ref, "refs/heads/")
		id, ok := ParseBranchName(name)
		if !ok {
			continue
		}
		branches = append(branches, RemoteBranch{ID: id, Name: name, Commit: sha})
	}
	return branches, nil
}

// identityEnv supplies a committer and author for the snapshot commit when git
// has none configured; commit-tree refuses to run without one.
func (r *Repo) identityEnv(ctx context.Context) []string {
	if _, err := r.git(ctx, nil, "var", "GIT_COMMITTER_IDENT"); err == nil {
		return nil
	}
	return []string{
		"GIT_AUTHOR_NAME=" + fallbackName, "GIT_AUTHOR_EMAIL=" + fallbackEmail,
		"GIT_COMMITTER_NAME=" + fallbackName, "GIT_COMMITTER_EMAIL=" + fallbackEmail,
	}
}

// git runs one git command in the repository and returns its trimmed stdout.
func (r *Repo) git(ctx context.Context, env []string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", args...) //nolint:gosec // fixed binary; args are built by this package
	cmd.Dir = r.Dir
	cmd.Env = append(os.Environ(), env...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", &GitError{Args: args, Stderr: stderr.String(), Err: err}
	}
	return strings.TrimSpace(stdout.String()), nil
}

func copyFile(src, dst string) error {
	data, err := os.ReadFile(src) //nolint:gosec // src is the repository's index path, as reported by git
	if err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0o600) //nolint:gosec // dst is inside a temp dir this package created
}
