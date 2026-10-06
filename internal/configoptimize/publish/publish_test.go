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

//go:build unix

package publish_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"golang.org/x/sys/unix"
	"gotest.tools/v3/assert"
	"gotest.tools/v3/assert/cmp"

	"github.com/CircleCI-Public/circleci-cli/internal/configoptimize/publish"
)

func write(t *testing.T, path, data string, mode os.FileMode) {
	t.Helper()
	assert.NilError(t, os.WriteFile(path, []byte(data), mode))
}

func read(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path) //#nosec:G304 // test temp file
	assert.NilError(t, err)
	return string(b)
}

func TestToFile(t *testing.T) {
	t.Run("writes a new file with the input's mode masked", func(t *testing.T) {
		dir := t.TempDir()
		in := filepath.Join(dir, "config.yml")
		write(t, in, "a: 1\n", 0o600)
		out := filepath.Join(dir, "out.yml")
		assert.NilError(t, publish.ToFile(out, []byte("b: 2\n"), in, false))
		assert.Check(t, cmp.Equal(read(t, out), "b: 2\n"))
		info, err := os.Stat(out)
		assert.NilError(t, err)
		assert.Check(t, cmp.Equal(info.Mode().Perm(), os.FileMode(0o600)))
	})
	t.Run("refuses an existing file without --force", func(t *testing.T) {
		out := filepath.Join(t.TempDir(), "out.yml")
		write(t, out, "old\n", 0o644)
		assert.Check(t, cmp.Equal(refusedCode(t, publish.ToFile(out, []byte("new\n"), "", false)), "output.exists"))
		assert.Check(t, cmp.Equal(read(t, out), "old\n"), "nothing was written")
		assert.NilError(t, publish.ToFile(out, []byte("new\n"), "", true))
		assert.Check(t, cmp.Equal(read(t, out), "new\n"))
	})
	t.Run("refuses the input, even through a symlink or hardlink", func(t *testing.T) {
		dir := t.TempDir()
		in := filepath.Join(dir, "config.yml")
		write(t, in, "a: 1\n", 0o644)
		assert.Check(t, cmp.Equal(refusedCode(t, publish.ToFile(in, []byte("x"), in, true)), "output.is_input"))
		hard := filepath.Join(dir, "hard.yml")
		assert.NilError(t, os.Link(in, hard))
		assert.Check(t, cmp.Equal(refusedCode(t, publish.ToFile(hard, []byte("x"), in, true)), "output.is_input"))
		link := filepath.Join(dir, "link.yml")
		assert.NilError(t, os.Symlink(in, link))
		assert.Check(t, cmp.Equal(refusedCode(t, publish.ToFile(link, []byte("x"), in, true)), "output.is_input"))
		assert.Check(t, cmp.Equal(read(t, in), "a: 1\n"))
	})
	t.Run("honors the umask for stdin input", func(t *testing.T) {
		old := unix.Umask(0o077)
		t.Cleanup(func() { unix.Umask(old) })
		out := filepath.Join(t.TempDir(), "out.yml")
		assert.NilError(t, publish.ToFile(out, []byte("x"), "", false))
		info, err := os.Stat(out)
		assert.NilError(t, err)
		assert.Check(t, cmp.Equal(info.Mode().Perm(), os.FileMode(0o600)))
	})
	t.Run("leaves no temp file behind", func(t *testing.T) {
		dir := t.TempDir()
		assert.NilError(t, publish.ToFile(filepath.Join(dir, "out.yml"), []byte("x"), "", false))
		entries, err := os.ReadDir(dir)
		assert.NilError(t, err)
		assert.Check(t, cmp.Len(entries, 1))
	})
}

// inPlace is what config optimize --in-place does: PreflightInPlace, then
// InPlace with its snapshot.
func inPlace(path string, original, data []byte, force bool) error {
	before, err := publish.PreflightInPlace(path, force)
	if err != nil {
		return err
	}
	return publish.InPlace(path, before, original, data, force)
}

// preflightInPlace is PreflightInPlace without its snapshot.
func preflightInPlace(path string, force bool) error {
	_, err := publish.PreflightInPlace(path, force)
	return err
}

// gitRepo makes a committed git worktree holding config.yml.
func gitRepo(t *testing.T) (string, string) {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yml")
	write(t, path, "a: 1\n", 0o640)
	git(t, dir, "init", "-q")
	commit(t, dir, ".")
	return dir, path
}

func commit(t *testing.T, dir, pathspec string) {
	t.Helper()
	git(t, dir, "add", pathspec)
	git(t, dir, "-c", "user.email=t@example.com", "-c", "user.name=t", "-c", "commit.gpgsign=false", "commit", "-q", "-m", "commit")
}

func TestInPlace(t *testing.T) {
	t.Run("replaces a clean committed file and keeps its mode", func(t *testing.T) {
		_, path := gitRepo(t)
		assert.NilError(t, inPlace(path, []byte("a: 1\n"), []byte("a: 2\n"), false))
		assert.Check(t, cmp.Equal(read(t, path), "a: 2\n"))
		info, err := os.Stat(path)
		assert.NilError(t, err)
		assert.Check(t, cmp.Equal(info.Mode().Perm(), os.FileMode(0o640)))
	})
	t.Run("refuses a dirty tree without --force", func(t *testing.T) {
		dir, path := gitRepo(t)
		write(t, filepath.Join(dir, "untracked.txt"), "x", 0o644)
		assert.Check(t, cmp.Equal(refusedCode(t, inPlace(path, []byte("a: 1\n"), []byte("a: 2\n"), false)), "tree.dirty"))
		assert.NilError(t, inPlace(path, []byte("a: 1\n"), []byte("a: 2\n"), true))
	})
	t.Run("refuses a path outside git without --force", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "config.yml")
		write(t, path, "a: 1\n", 0o644)
		assert.Check(t, cmp.Equal(refusedCode(t, inPlace(path, []byte("a: 1\n"), []byte("a: 2\n"), false)), "tree.not_git"))
	})
	t.Run("refuses a file that changed since it was read, even with --force", func(t *testing.T) {
		_, path := gitRepo(t)
		assert.Check(t, cmp.Equal(refusedCode(t, inPlace(path, []byte("a: 0\n"), []byte("a: 2\n"), true)), "output.changed"))
		assert.Check(t, cmp.Equal(read(t, path), "a: 1\n"))
	})
	t.Run("refuses a symlink and a hard-linked file, even with --force", func(t *testing.T) {
		dir, path := gitRepo(t)
		link := filepath.Join(dir, "link.yml")
		assert.NilError(t, os.Symlink(path, link))
		assert.Check(t, cmp.Equal(refusedCode(t, inPlace(link, []byte("a: 1\n"), []byte("x"), true)), "output.symlink"))
		assert.NilError(t, os.Remove(link))
		assert.NilError(t, os.Link(path, filepath.Join(dir, "hard.yml")))
		assert.Check(t, cmp.Equal(refusedCode(t, inPlace(path, []byte("a: 1\n"), []byte("x"), true)), "output.hardlink"))
	})
	t.Run("refuses an untracked or ignored file without --force", func(t *testing.T) {
		dir, _ := gitRepo(t)
		write(t, filepath.Join(dir, ".gitignore"), "ignored.yml\n", 0o644)
		commit(t, dir, ".gitignore")
		ignored := filepath.Join(dir, "ignored.yml")
		write(t, ignored, "a: 1\n", 0o644)
		assert.Check(t, cmp.Equal(refusedCode(t, inPlace(ignored, []byte("a: 1\n"), []byte("a: 2\n"), false)), "tree.untracked"))
		assert.Check(t, cmp.Equal(read(t, ignored), "a: 1\n"))
	})
	t.Run("keeps the setgid bit", func(t *testing.T) {
		_, path := gitRepo(t)
		assert.NilError(t, os.Chmod(path, 0o640|os.ModeSetgid))
		if info, err := os.Stat(path); err != nil || info.Mode()&os.ModeSetgid == 0 {
			t.Skip("this filesystem or group membership does not allow setgid here")
		}
		assert.NilError(t, inPlace(path, []byte("a: 1\n"), []byte("a: 2\n"), true))
		info, err := os.Stat(path)
		assert.NilError(t, err)
		assert.Check(t, cmp.Equal(info.Mode()&(os.ModePerm|os.ModeSetgid), 0o640|os.ModeSetgid))
	})
	t.Run("refuses a file with user extended attributes without --force", func(t *testing.T) {
		_, path := gitRepo(t)
		if err := unix.Setxattr(path, "user.test", []byte("x"), 0); err != nil {
			t.Skipf("filesystem has no user xattrs: %v", err)
		}
		assert.Check(t, cmp.Equal(refusedCode(t, preflightInPlace(path, false)), "output.xattr"))
	})
	t.Run("refuses a non-regular file, even with --force", func(t *testing.T) {
		fifo := filepath.Join(t.TempDir(), "config.yml")
		assert.NilError(t, unix.Mkfifo(fifo, 0o644))
		assert.Check(t, cmp.Equal(refusedCode(t, preflightInPlace(fifo, true)), "output.not_regular"))
	})
	t.Run("reads the file name literally, not as a pathspec", func(t *testing.T) {
		dir, _ := gitRepo(t)
		// An ignored file whose name is a glob matching the tracked config.
		write(t, filepath.Join(dir, ".gitignore"), "/[*].yml\n", 0o644)
		commit(t, dir, ".gitignore")
		glob := filepath.Join(dir, "*.yml")
		write(t, glob, "a: 1\n", 0o644)
		assert.Check(t, cmp.Equal(refusedCode(t, preflightInPlace(glob, false)), "tree.untracked"))
	})
	// go-git exposes skip-worktree but not assume-unchanged, so only the first
	// is refused.
	t.Run("refuses a config marked --skip-worktree without --force", func(t *testing.T) {
		dir, path := gitRepo(t)
		git(t, dir, "update-index", "--skip-worktree", "config.yml")
		assert.Check(t, cmp.Equal(refusedCode(t, preflightInPlace(path, false)), "tree.index_flag"))
	})
}

// isolateGit keeps the developer's global and system git config (signing,
// hooks, templates) out of the test's repositories.
func isolateGit(t *testing.T) {
	t.Helper()
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
}

func git(t *testing.T, dir string, args ...string) {
	t.Helper()
	isolateGit(t)
	out, err := exec.Command("git", append([]string{"-C", dir}, args...)...).CombinedOutput()
	assert.NilError(t, err, "git %v: %s", args, out)
}

func TestVerifyUnchanged(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yml")
	write(t, path, "a: 1\n", 0o644)
	assert.NilError(t, publish.VerifyUnchanged(path, []byte("a: 1\n")))
	assert.Check(t, cmp.Equal(refusedCode(t, publish.VerifyUnchanged(path, []byte("a: 0\n"))), "output.changed"))

	link := filepath.Join(filepath.Dir(path), "link.yml")
	assert.NilError(t, os.Symlink(path, link))
	assert.Check(t, cmp.Equal(refusedCode(t, publish.VerifyUnchanged(link, []byte("a: 1\n"))), "output.changed"),
		"a symlink with the same content is not the file that was checked")
}
