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

package engine_test

import (
	"os"
	"path/filepath"
	"testing"

	gogit "github.com/go-git/go-git/v6"
	"gotest.tools/v3/assert"
	"gotest.tools/v3/assert/cmp"

	"github.com/CircleCI-Public/circleci-cli/internal/configoptimize/engine"
)

func TestRepoRoot(t *testing.T) {
	writeConfig := func(t *testing.T, dir string) string {
		t.Helper()
		assert.NilError(t, os.MkdirAll(filepath.Join(dir, ".circleci"), 0o700))
		path := filepath.Join(dir, ".circleci", "config.yml")
		assert.NilError(t, os.WriteFile(path, []byte("version: 2.1\n"), 0o600))
		return path
	}
	resolved := func(t *testing.T, dir string) string {
		t.Helper()
		r, err := filepath.EvalSymlinks(dir)
		assert.NilError(t, err)
		return r
	}

	t.Run("the git worktree that holds the config", func(t *testing.T) {
		dir := t.TempDir()
		_, err := gogit.PlainInit(dir, false)
		assert.NilError(t, err)
		sub := filepath.Join(dir, "service")
		path := writeConfig(t, sub)
		got, err := filepath.EvalSymlinks(engine.RepoRoot(path))
		assert.Check(t, err)
		assert.Check(t, cmp.Equal(got, resolved(t, dir)))
	})
	t.Run("outside git, the directory above .circleci", func(t *testing.T) {
		dir := t.TempDir()
		got, err := filepath.EvalSymlinks(engine.RepoRoot(writeConfig(t, dir)))
		assert.Check(t, err)
		assert.Check(t, cmp.Equal(got, resolved(t, dir)))
	})
	t.Run("stdin has no checkout", func(t *testing.T) {
		assert.Check(t, cmp.Equal(engine.RepoRoot(""), ""))
	})
}
