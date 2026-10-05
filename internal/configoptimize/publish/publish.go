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

// Package publish writes the optimized config: atomically, never clobbering
// a file without --force, never through a symlink, and never over a file that
// changed while circleci was running. On unix it also refuses to drop a file's
// owner, links, extended attributes or ACLs; on other platforms it writes with
// a plain temporary file and rename.
//
// One race is documented rather than closed: there is no conditional rename,
// so a process that replaces the file between the final check and the rename
// is not detected. The config's directory is assumed not to be written by
// another process at the same moment.
package publish

import (
	"crypto/rand"
	"crypto/sha256"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/go-git/go-git/v6"
)

// RefusedError is a write config optimize will not make. The caller reports it as a
// usage error; nothing was written.
type RefusedError struct {
	Code       string // e.g. "output.exists"
	Reason     string
	Suggestion string // what to do instead, e.g. "Pass --force to overwrite it"
}

func (e *RefusedError) Error() string { return e.Reason }

func refuseHint(code, suggestion, format string, args ...any) error {
	return &RefusedError{Code: code, Reason: fmt.Sprintf(format, args...), Suggestion: suggestion}
}

// IncompleteError reports that the output is in place but a later step
// failed: the write may not survive a crash (output.not_durable), or a
// temporary name was left behind (output.temp_left). It never means
// "nothing written".
type IncompleteError struct {
	Path string
	Code string
	Err  error
}

func (e *IncompleteError) Error() string {
	return fmt.Sprintf("%s was written, but %v", e.Path, e.Err)
}

func (e *IncompleteError) Unwrap() error { return e.Err }

// defaultMode is the mode of a new output file; the umask still applies.
const defaultMode fs.FileMode = 0o644

// PreflightToFile runs ToFile's refusals without writing, so a bad -o fails
// before any analysis.
func PreflightToFile(path, inputPath string, force bool) error {
	if inputPath != "" && inputPath != "-" {
		in, err := os.Stat(inputPath)
		if err != nil {
			return err
		}
		if out, err := os.Stat(path); err == nil && os.SameFile(in, out) {
			return refuseHint("output.is_input", "Use --in-place to replace the input", "-o %s is the input file", path)
		}
	}
	if info, err := os.Lstat(path); err == nil {
		switch {
		case info.Mode()&fs.ModeSymlink != 0:
			return refuseHint("output.symlink", "Pass the path the link points to", "%s is a symlink; writing would replace the link, not its target", path)
		case !force:
			return refuseHint("output.exists", "Pass --force to overwrite it", "%s already exists", path)
		}
	}
	return nil
}

// VerifyUnchanged confirms path still holds original. A no-op --in-place run
// writes nothing but must not report on a file that changed underneath it.
func VerifyUnchanged(path string, original []byte) error {
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return refuseHint("output.changed", "Run the command again", "%s is no longer a regular file", path)
	}
	same, err := sameContent(path, original)
	if err != nil {
		return err
	}
	if !same {
		return refuseHint("output.changed", "Run the command again", "%s changed while circleci was running", path)
	}
	return nil
}

// CleanTree requires path to be tracked in a git worktree with nothing
// staged, unstaged or untracked, so the change can be undone with git. It reads
// the repository with go-git, so no git binary is needed. go-git does not
// expose git's assume-unchanged flag, so only skip-worktree is detected.
func CleanTree(path string) error {
	dir := filepath.Dir(path)
	where := dir
	if where == "." {
		where = "the current directory"
	}
	repo, err := git.PlainOpenWithOptions(dir, &git.PlainOpenOptions{DetectDotGit: true})
	if err != nil {
		return refuseHint("tree.not_git", "Pass --force to write anyway", "%s is not in a git worktree, so the change could not be undone with git", where)
	}
	wt, err := repo.Worktree()
	if err != nil {
		return refuseHint("tree.not_git", "Pass --force to write anyway", "%s is not in a git worktree, so the change could not be undone with git", where)
	}
	rel, err := relativeTo(wt.Filesystem().Root(), path)
	if err != nil {
		return err
	}
	idx, err := repo.Storer.Index()
	if err != nil {
		return fmt.Errorf("read the git index: %w", err)
	}
	entry, err := idx.Entry(rel)
	if err != nil {
		return refuseHint("tree.untracked", "Pass --force to write anyway", "%s is not tracked by git (it may be ignored), so the change could not be undone with git", path)
	}
	if entry.SkipWorktree {
		return refuseHint("tree.index_flag", "Pass --force to write anyway", "%s is marked skip-worktree, so git would not show the change", path)
	}
	status, err := wt.Status()
	if err != nil {
		return fmt.Errorf("read the git status: %w", err)
	}
	if !status.IsClean() {
		return refuseHint("tree.dirty", "Commit or stash the changes, or pass --force", "the git worktree has uncommitted changes")
	}
	return nil
}

// relativeTo is path relative to root, in git's slash form. Both are resolved
// through symlinks first, since a temp or home directory is often one.
func relativeTo(root, path string) (string, error) {
	r, err := filepath.EvalSymlinks(root)
	if err != nil {
		return "", err
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	p, err := filepath.EvalSymlinks(abs)
	if err != nil {
		return "", err
	}
	rel, err := filepath.Rel(r, p)
	if err != nil {
		return "", err
	}
	return filepath.ToSlash(rel), nil
}

// createTemp is os.CreateTemp with a chosen mode (it always uses 0600).
func createTemp(dir string, perm fs.FileMode) (*os.File, error) {
	for range 100 {
		name := filepath.Join(dir, ".circleci-optimize-"+rand.Text())
		f, err := os.OpenFile(name, os.O_RDWR|os.O_CREATE|os.O_EXCL, perm) //#nosec:G304 // a new name in the output's directory
		if !errors.Is(err, fs.ErrExist) {
			return f, err
		}
	}
	return nil, fmt.Errorf("cannot create a temporary file in %s", dir)
}

// sameContent reports whether path still holds original.
func sameContent(path string, original []byte) (bool, error) {
	current, err := os.ReadFile(path) //#nosec:G304 // the user's config
	if err != nil {
		return false, err
	}
	return sha256.Sum256(current) == sha256.Sum256(original), nil
}
