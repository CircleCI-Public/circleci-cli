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

package preflight

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/uuid"
	"gotest.tools/v3/assert"
	"gotest.tools/v3/assert/cmp"
)

// newTestRepo creates a repository with one commit and a bare origin, and
// returns the repository's directory.
func newTestRepo(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	origin := filepath.Join(root, "origin.git")
	dir := filepath.Join(root, "work")

	runGit(t, root, "init", "--quiet", "--bare", "--initial-branch=main", origin)
	runGit(t, root, "init", "--quiet", "--initial-branch=main", dir)
	runGit(t, dir, "remote", "add", "origin", origin)
	writeFile(t, dir, ".gitignore", "*.log\n")
	writeFile(t, dir, "committed.txt", "v1\n")
	writeFile(t, dir, "staged.txt", "v1\n")
	writeFile(t, dir, "deleted.txt", "v1\n")
	assert.NilError(t, os.MkdirAll(filepath.Join(dir, "nested", "dir"), 0o750))
	writeFile(t, dir, "nested/dir/file.txt", "v1\n")
	runGit(t, dir, "add", ".")
	runGit(t, dir, "commit", "--quiet", "-m", "initial")
	return dir
}

func runGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=Test", "GIT_AUTHOR_EMAIL=test@example.com",
		"GIT_COMMITTER_NAME=Test", "GIT_COMMITTER_EMAIL=test@example.com",
		"GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_NOSYSTEM=1",
	)
	out, err := cmd.CombinedOutput()
	assert.NilError(t, err, "git %s: %s", strings.Join(args, " "), out)
	return strings.TrimSpace(string(out))
}

// openTestRepo opens dir as a Repo, closing it when the test ends.
func openTestRepo(t *testing.T, dir string) *Repo {
	t.Helper()
	repo, err := OpenRepo(dir, DefaultRemote)
	assert.NilError(t, err)
	t.Cleanup(func() { assert.Check(t, repo.Close()) })
	return repo
}

func writeFile(t *testing.T, dir, name, content string) {
	t.Helper()
	assert.NilError(t, os.WriteFile(filepath.Join(dir, name), []byte(content), 0o600))
}

func TestSnapshot(t *testing.T) {
	ctx := context.Background()
	dir := newTestRepo(t)

	writeFile(t, dir, "committed.txt", "v2 unstaged\n")
	writeFile(t, dir, "staged.txt", "v2 staged\n")
	runGit(t, dir, "add", "staged.txt")
	writeFile(t, dir, "untracked.txt", "new\n")
	writeFile(t, dir, "debug.log", "ignored\n")
	assert.NilError(t, os.Remove(filepath.Join(dir, "deleted.txt")))

	head := runGit(t, dir, "rev-parse", "HEAD")
	statusBefore := runGit(t, dir, "status", "--porcelain")

	repo := openTestRepo(t, dir)
	snap, err := repo.Snapshot(ctx, "preflight test")
	assert.NilError(t, err)
	assert.Check(t, snap.Changed)
	sha := snap.Commit

	t.Run("without a default branch, commit is a child of HEAD", func(t *testing.T) {
		assert.Check(t, cmp.Equal(snap.Base, head))
		assert.Check(t, cmp.Equal(snap.BaseBranch, ""))
		assert.Check(t, cmp.Equal(runGit(t, dir, "rev-parse", sha+"^"), head))
		assert.Check(t, cmp.Equal(runGit(t, dir, "log", "-1", "--format=%s", sha), "preflight test"))
	})

	t.Run("commit holds staged, unstaged and untracked changes", func(t *testing.T) {
		assert.Check(t, cmp.Equal(runGit(t, dir, "show", sha+":committed.txt"), "v2 unstaged"))
		assert.Check(t, cmp.Equal(runGit(t, dir, "show", sha+":staged.txt"), "v2 staged"))
		assert.Check(t, cmp.Equal(runGit(t, dir, "show", sha+":untracked.txt"), "new"))
	})

	t.Run("commit leaves out ignored and deleted files", func(t *testing.T) {
		files := runGit(t, dir, "ls-tree", "-r", "--name-only", sha)
		assert.Check(t, cmp.Equal(files, ".gitignore\ncommitted.txt\nnested/dir/file.txt\nstaged.txt\nuntracked.txt"))
	})

	t.Run("commit is a valid git object graph", func(t *testing.T) {
		runGit(t, dir, "fsck", "--strict", "--no-dangling")
	})

	t.Run("working tree, index and HEAD are untouched", func(t *testing.T) {
		assert.Check(t, cmp.Equal(runGit(t, dir, "rev-parse", "HEAD"), head))
		assert.Check(t, cmp.Equal(runGit(t, dir, "status", "--porcelain"), statusBefore))
		assert.Check(t, cmp.Equal(runGit(t, dir, "stash", "list"), ""))
	})
}

func TestSnapshot_UserExcludes(t *testing.T) {
	ctx := context.Background()
	xdg := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", xdg)
	t.Setenv("HOME", t.TempDir())
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	t.Setenv("GIT_CONFIG_GLOBAL", "")
	assert.NilError(t, os.MkdirAll(filepath.Join(xdg, "git"), 0o750))
	writeFile(t, xdg, "git/ignore", "default.secret\n")
	writeFile(t, xdg, "custom-ignore", "custom.secret\n")

	snapshotFiles := func(t *testing.T, dir string) string {
		t.Helper()
		writeFile(t, dir, "default.secret", "x\n")
		writeFile(t, dir, "custom.secret", "x\n")
		snap, err := openTestRepo(t, dir).Snapshot(ctx, "preflight test")
		assert.NilError(t, err)
		return runGit(t, dir, "ls-tree", "-r", "--name-only", snap.Commit)
	}

	t.Run("without core.excludesFile, the XDG default applies", func(t *testing.T) {
		files := snapshotFiles(t, newTestRepo(t))
		assert.Check(t, !strings.Contains(files, "default.secret"), files)
		assert.Check(t, cmp.Contains(files, "custom.secret"))
	})

	t.Run("core.excludesFile in the XDG global config replaces the default", func(t *testing.T) {
		// Backslash is an escape in git config, so a Windows path is written
		// with forward slashes, as users of git on Windows write it.
		writeFile(t, xdg, "git/config", "[core]\n\texcludesFile = "+filepath.ToSlash(filepath.Join(xdg, "custom-ignore"))+"\n")
		t.Cleanup(func() { assert.Check(t, os.Remove(filepath.Join(xdg, "git", "config"))) })
		files := snapshotFiles(t, newTestRepo(t))
		assert.Check(t, !strings.Contains(files, "custom.secret"), files)
		assert.Check(t, cmp.Contains(files, "default.secret"))
	})

	t.Run("core.excludesFile in the repository config wins", func(t *testing.T) {
		dir := newTestRepo(t)
		runGit(t, dir, "config", "core.excludesFile", filepath.Join(xdg, "custom-ignore"))
		files := snapshotFiles(t, dir)
		assert.Check(t, !strings.Contains(files, "custom.secret"), files)
		assert.Check(t, cmp.Contains(files, "default.secret"))
	})
}

func TestSnapshot_NoCommits(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	runGit(t, dir, "init", "--quiet", dir)

	repo := openTestRepo(t, dir)
	_, err := repo.Snapshot(ctx, "preflight test")
	assert.Check(t, cmp.ErrorIs(err, ErrNoCommits))
}

func TestSnapshot_NoChanges(t *testing.T) {
	ctx := context.Background()
	dir := newTestRepo(t)
	writeFile(t, dir, "debug.log", "ignored files are not changes\n")
	head := runGit(t, dir, "rev-parse", "HEAD")

	repo := openTestRepo(t, dir)
	snap, err := repo.Snapshot(ctx, "preflight test")
	assert.NilError(t, err)
	assert.Check(t, !snap.Changed)
	assert.Check(t, cmp.Equal(snap.Commit, head), "a clean tree pushes HEAD itself, not an empty commit")
}

func TestSnapshot_ForkPoint(t *testing.T) {
	ctx := context.Background()
	dir := newTestRepo(t)
	runGit(t, dir, "push", "--quiet", "--set-upstream", "origin", "main")
	main := runGit(t, dir, "rev-parse", "HEAD")

	runGit(t, dir, "checkout", "--quiet", "-b", "feature")
	writeFile(t, dir, "committed.txt", "v2 committed on feature\n")
	runGit(t, dir, "commit", "--quiet", "-am", "feature work")
	head := runGit(t, dir, "rev-parse", "HEAD")
	writeFile(t, dir, "staged.txt", "v2 uncommitted\n")

	repo := openTestRepo(t, dir)

	check := func(t *testing.T) {
		t.Helper()
		snap, err := repo.Snapshot(ctx, "preflight test")
		assert.NilError(t, err)
		assert.Check(t, snap.Changed)
		assert.Check(t, cmp.Equal(snap.Base, main))
		assert.Check(t, cmp.Equal(snap.BaseBranch, "main"))
		assert.Check(t, cmp.Equal(runGit(t, dir, "rev-parse", snap.Commit+"^"), main), "parent is the fork point, not HEAD")
		assert.Check(t, cmp.Equal(runGit(t, dir, "diff", "--name-only", main, snap.Commit), "committed.txt\nstaged.txt"),
			"diff shows the branch's commits and the uncommitted work")
		assert.Check(t, cmp.Equal(runGit(t, dir, "rev-parse", "HEAD"), head))
	}

	t.Run("default branch read from the remote", check)

	t.Run("default branch read from origin/HEAD", func(t *testing.T) {
		runGit(t, dir, "remote", "set-head", "origin", "main")
		check(t)
	})

	t.Run("clean feature branch still diffs against main", func(t *testing.T) {
		runGit(t, dir, "checkout", "--quiet", "--", ".")
		snap, err := repo.Snapshot(ctx, "preflight test")
		assert.NilError(t, err)
		assert.Check(t, snap.Changed)
		assert.Check(t, cmp.Equal(runGit(t, dir, "rev-parse", snap.Commit+"^{tree}"), runGit(t, dir, "rev-parse", "HEAD^{tree}")))
		assert.Check(t, cmp.Equal(runGit(t, dir, "rev-parse", snap.Commit+"^"), main))
	})
}

func TestPushListDelete(t *testing.T) {
	ctx := context.Background()
	dir := newTestRepo(t)
	repo := openTestRepo(t, dir)

	id := uuid.MustParse("4b1f8e2a-0000-4000-8000-000000000001")
	branch := BranchName(id)
	writeFile(t, dir, "committed.txt", "v2\n")
	snap, err := repo.Snapshot(ctx, "preflight test")
	assert.NilError(t, err)
	sha := snap.Commit

	assert.Assert(t, t.Run("push", func(t *testing.T) {
		assert.NilError(t, repo.Push(ctx, sha, branch))
	}))

	t.Run("list finds the branch and skips foreign ones", func(t *testing.T) {
		runGit(t, dir, "push", "--quiet", "origin", "HEAD:refs/heads/"+BranchPrefix+"not-a-uuid")
		branches, err := repo.ListBranches(ctx)
		assert.NilError(t, err)
		assert.Check(t, cmp.DeepEqual(branches, []RemoteBranch{{ID: id, Name: branch, Commit: sha}}))
	})

	t.Run("delete removes the branch", func(t *testing.T) {
		assert.NilError(t, repo.DeleteBranch(ctx, branch))
		branches, err := repo.ListBranches(ctx)
		assert.NilError(t, err)
		assert.Check(t, cmp.Len(branches, 0))
	})
}

func TestParseBranchName(t *testing.T) {
	id := uuid.MustParse("4b1f8e2a-0000-4000-8000-000000000001")
	tests := []struct {
		name   string
		branch string
		wantID uuid.UUID
		wantOK bool
	}{
		{name: "preflight branch", branch: BranchName(id), wantID: id, wantOK: true},
		{name: "other branch", branch: "main"},
		{name: "prefix without uuid", branch: BranchPrefix + "wip"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := ParseBranchName(tt.branch)
			assert.Check(t, cmp.Equal(ok, tt.wantOK))
			assert.Check(t, cmp.Equal(got, tt.wantID))
		})
	}
}

func TestPush_AuthFailure(t *testing.T) {
	ctx := context.Background()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("WWW-Authenticate", `Basic realm="git"`)
		w.WriteHeader(http.StatusUnauthorized)
	}))
	t.Cleanup(srv.Close)

	dir := newTestRepo(t)
	runGit(t, dir, "remote", "set-url", "origin", srv.URL+"/org/repo.git")
	repo := openTestRepo(t, dir)

	t.Run("push", func(t *testing.T) {
		err := repo.Push(ctx, runGit(t, dir, "rev-parse", "HEAD"), BranchName(uuid.New()))
		assert.Check(t, cmp.ErrorIs(err, ErrRemoteAuth))
	})

	t.Run("list", func(t *testing.T) {
		_, err := repo.ListBranches(ctx)
		assert.Check(t, cmp.ErrorIs(err, ErrRemoteAuth))
	})
}
