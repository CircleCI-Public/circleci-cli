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

package gitremote

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/go-git/go-git/v6"
	"github.com/go-git/go-git/v6/config"
	"github.com/go-git/go-git/v6/plumbing"
	"github.com/go-git/go-git/v6/plumbing/object"
	"gotest.tools/v3/assert"
	"gotest.tools/v3/assert/cmp"

	"github.com/CircleCI-Public/circleci-cli/internal/projectref"
)

func TestSlugFromRemote(t *testing.T) {
	type testCase struct {
		name      string
		url       string
		wantSlug  string
		wantError string
	}
	// Each group below exercises one of the three remote-URL regexes; run drives
	// a group's cases through SlugFromRemote and asserts the slug or the error.
	run := func(t *testing.T, cases []testCase) {
		t.Helper()
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				slug, err := SlugFromRemote(tc.url)
				if tc.wantError != "" {
					assert.Check(t, cmp.ErrorContains(err, tc.wantError))
					return
				}
				assert.NilError(t, err)
				assert.Check(t, cmp.Equal(slug, tc.wantSlug))
			})
		}
	}

	t.Run("SCP-style SSH URLs (git@host:org/repo) map each host to its VCS prefix", func(t *testing.T) {
		run(t, []testCase{
			{name: "github with .git suffix", url: "git@github.com:myorg/myrepo.git", wantSlug: "gh/myorg/myrepo"},
			{name: "github without .git suffix", url: "git@github.com:myorg/myrepo", wantSlug: "gh/myorg/myrepo"},
			{name: "bitbucket", url: "git@bitbucket.org:myorg/myrepo.git", wantSlug: "bb/myorg/myrepo"},
			{name: "gitlab", url: "git@gitlab.com:myorg/myrepo.git", wantSlug: "gl/myorg/myrepo"},
			{name: "self-hosted gitlab", url: "git@gitlab.mycompany.com:myorg/myrepo.git", wantSlug: "gl/myorg/myrepo"},
		})
	})

	t.Run("protocol-style ssh:// URLs (ssh://git@host/org/repo)", func(t *testing.T) {
		run(t, []testCase{
			{name: "github with .git suffix", url: "ssh://git@github.com/myorg/myrepo.git", wantSlug: "gh/myorg/myrepo"},
			{name: "github without .git suffix", url: "ssh://git@github.com/myorg/myrepo", wantSlug: "gh/myorg/myrepo"},
		})
	})

	t.Run("HTTP(S) URLs (https://host/org/repo)", func(t *testing.T) {
		run(t, []testCase{
			{name: "https github with .git suffix", url: "https://github.com/myorg/myrepo.git", wantSlug: "gh/myorg/myrepo"},
			{name: "https github without .git suffix", url: "https://github.com/myorg/myrepo", wantSlug: "gh/myorg/myrepo"},
			{name: "plain http github", url: "http://github.com/myorg/myrepo.git", wantSlug: "gh/myorg/myrepo"},
			{name: "https bitbucket", url: "https://bitbucket.org/myorg/myrepo.git", wantSlug: "bb/myorg/myrepo"},
			{name: "https gitlab", url: "https://gitlab.com/myorg/myrepo.git", wantSlug: "gl/myorg/myrepo"},
		})
	})

	t.Run("surrounding whitespace is trimmed before parsing", func(t *testing.T) {
		run(t, []testCase{
			{name: "trailing newline", url: "git@github.com:myorg/myrepo.git\n", wantSlug: "gh/myorg/myrepo"},
			{name: "leading and trailing spaces", url: "  https://github.com/myorg/myrepo.git  ", wantSlug: "gh/myorg/myrepo"},
		})
	})

	t.Run("unsupported hosts and unparseable URLs return an error", func(t *testing.T) {
		run(t, []testCase{
			{name: "unsupported host", url: "git@codeberg.org:myorg/myrepo.git", wantError: `unsupported VCS host "codeberg.org"`},
			{name: "unrecognised format", url: "not-a-url", wantError: "unrecognised git remote URL format"},
			{name: "empty string", url: "", wantError: "unrecognised git remote URL format"},
		})
	})
}

// TestDetect_PrefersInfoYml verifies that an existing .circleci/info.yml takes
// priority over the git remote, and that a UUID-bearing record is normalised
// to the canonical "circleci/<orgID>/<projectID>" slug.
func TestDetect_PrefersInfoYml(t *testing.T) {
	tests := []struct {
		name      string
		info      projectref.Info
		wantSlug  string
		wantOrgID string
	}{
		{
			name: "uuids present yields canonical slug",
			info: projectref.Info{
				Organization: projectref.Organization{ID: "E6i3yYZeWZhcf8UNqcKfjN"},
				Project: projectref.Project{
					Slug: "gh/myorg/myrepo",
					ID:   "13c8F7nusayivoSxC6GMsw",
				},
			},
			wantSlug:  "circleci/E6i3yYZeWZhcf8UNqcKfjN/13c8F7nusayivoSxC6GMsw",
			wantOrgID: "E6i3yYZeWZhcf8UNqcKfjN",
		},
		{
			name:     "slug-only falls through verbatim",
			info:     projectref.Info{Project: projectref.Project{Slug: "gh/myorg/legacy"}},
			wantSlug: "gh/myorg/legacy",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			assert.NilError(t, projectref.Write(dir, &tc.info))

			cwd, err := os.Getwd()
			assert.NilError(t, err)
			t.Cleanup(func() { _ = os.Chdir(cwd) })
			assert.NilError(t, os.Chdir(dir))

			info, err := Detect()
			assert.NilError(t, err)
			assert.Check(t, cmp.Equal(info.Slug, tc.wantSlug))
			// The org ID recorded by `project link` is surfaced verbatim so
			// callers can use it without a project lookup.
			assert.Check(t, cmp.Equal(info.OrgID, tc.wantOrgID))
		})
	}
}

// TestDetect_SurfacesMalformedInfoYml verifies that a present-but-invalid
// info.yml is reported as an error rather than silently falling back to the git
// remote: a broken link is a real problem the user should hear about, not a
// reason to guess the project from elsewhere.
func TestDetect_SurfacesMalformedInfoYml(t *testing.T) {
	dir := t.TempDir()
	assert.NilError(t, os.MkdirAll(filepath.Join(dir, ".circleci"), 0o755))
	// Valid YAML, but missing the required project.slug field.
	assert.NilError(t, os.WriteFile(
		filepath.Join(dir, projectref.FilePath),
		[]byte("organization:\n  id: OID\n"), 0o644,
	))

	cwd, err := os.Getwd()
	assert.NilError(t, err)
	t.Cleanup(func() { _ = os.Chdir(cwd) })
	assert.NilError(t, os.Chdir(dir))

	_, err = Detect()
	assert.Check(t, err != nil, "expected Detect to surface a malformed info.yml rather than fall back")
}

// Sanity check that DetectFromRemote does not consult info.yml — used by
// `project link` to avoid short-circuiting against an existing entry.
func TestDetectFromRemote_IgnoresInfoYml(t *testing.T) {
	dir := t.TempDir()
	assert.NilError(t, projectref.Write(dir, &projectref.Info{
		Organization: projectref.Organization{ID: "OID"},
		Project: projectref.Project{
			Slug: "gh/myorg/myrepo",
			ID:   "PID",
		},
	}))

	cwd, err := os.Getwd()
	assert.NilError(t, err)
	t.Cleanup(func() { _ = os.Chdir(cwd) })
	assert.NilError(t, os.Chdir(dir))

	t.Run("detection bypasses info.yml and fails without a git remote", func(t *testing.T) {
		_, err := DetectFromRemote()
		assert.Check(t, err != nil, "expected git-remote detection to fail in a non-git temp dir")
	})

	t.Run("info.yml is left in place on disk", func(t *testing.T) {
		_, statErr := os.Stat(filepath.Join(dir, projectref.FilePath))
		assert.NilError(t, statErr)
	})
}

// TestDetectFromRemote_Worktree verifies detection works from inside a linked
// git worktree. In a worktree, .git is a file pointing at
// <main>/.git/worktrees/<name>/, which holds only per-worktree state (HEAD);
// the origin remote and origin/HEAD live in the shared common dir. Detection
// must follow the "commondir" pointer to read them — otherwise the origin URL
// and default branch are invisible and Detect fails.
func TestDetectFromRemote_Worktree(t *testing.T) {
	mainDir := t.TempDir() // the shared repo the worktree links back to
	wtDir := t.TempDir()   // the linked worktree's working directory
	wtGitDir := filepath.Join(mainDir, ".git", "worktrees", "wt1")

	var repo *git.Repository

	// Every group below the final one builds one piece of the fixture and is
	// mandatory: assert.Assert(t, t.Run(...)) aborts the whole test if a step
	// fails, so detection never runs against a half-built fixture. go-git creates
	// normal repos but not worktrees, and the package must not depend on the OS
	// git binary, so the layout `git worktree add` produces is written by hand.
	assert.Assert(t, t.Run("the main repo has an origin remote", func(t *testing.T) {
		var err error
		repo, err = git.PlainInit(mainDir, false)
		assert.NilError(t, err)
		_, err = repo.CreateRemote(&config.RemoteConfig{
			Name: "origin",
			URLs: []string{"git@github.com:myorg/myrepo.git"},
		})
		assert.NilError(t, err)
	}))

	assert.Assert(t, t.Run("origin/HEAD names the default branch in the common dir", func(t *testing.T) {
		// gitDefaultBranch reads this symbolic ref, which lives in the common dir
		// alongside config — not in the worktree's private gitdir.
		assert.NilError(t, repo.Storer.SetReference(plumbing.NewSymbolicReference(
			"refs/remotes/origin/HEAD", "refs/remotes/origin/main",
		)))
	}))

	assert.Assert(t, t.Run("the worktree branch points at a real commit", func(t *testing.T) {
		// repo.Head() resolves the branch ref to a commit, so an unborn branch
		// would fail exactly as a real but empty worktree cannot exist.
		wt, err := repo.Worktree()
		assert.NilError(t, err)
		assert.NilError(t, os.WriteFile(filepath.Join(mainDir, "README.md"), []byte("x\n"), 0o644))
		_, err = wt.Add("README.md")
		assert.NilError(t, err)
		commit, err := wt.Commit("init", &git.CommitOptions{
			Author: &object.Signature{Name: "test", Email: "test@test.com"},
		})
		assert.NilError(t, err)
		assert.NilError(t, repo.Storer.SetReference(plumbing.NewHashReference(
			"refs/heads/feature", commit,
		)))
	}))

	assert.Assert(t, t.Run("the worktree's .git file points at its private gitdir", func(t *testing.T) {
		assert.NilError(t, os.MkdirAll(wtGitDir, 0o755))
		assert.NilError(t, os.WriteFile(
			filepath.Join(wtDir, ".git"),
			[]byte("gitdir: "+wtGitDir+"\n"), 0o644,
		))
	}))

	assert.Assert(t, t.Run("commondir points back to the shared .git", func(t *testing.T) {
		// The path is relative to the worktree gitdir: ../.. resolves to <main>/.git.
		assert.NilError(t, os.WriteFile(
			filepath.Join(wtGitDir, "commondir"), []byte("../..\n"), 0o644,
		))
	}))

	assert.Assert(t, t.Run("per-worktree HEAD is checked out on its own branch", func(t *testing.T) {
		assert.NilError(t, os.WriteFile(
			filepath.Join(wtGitDir, "HEAD"), []byte("ref: refs/heads/feature\n"), 0o644,
		))
	}))

	t.Run("detection from inside the worktree resolves slug, branch and default branch", func(t *testing.T) {
		cwd, err := os.Getwd()
		assert.NilError(t, err)
		t.Cleanup(func() { _ = os.Chdir(cwd) })
		assert.NilError(t, os.Chdir(wtDir))

		info, err := DetectFromRemote()
		assert.NilError(t, err)
		assert.Check(t, cmp.Equal(info.Slug, "gh/myorg/myrepo"))
		assert.Check(t, cmp.Equal(info.Branch, "feature"))
		assert.Check(t, cmp.Equal(info.DefaultBranch, "main"))
	})
}
