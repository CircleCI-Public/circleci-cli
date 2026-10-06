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

package publish_test

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	gogit "github.com/go-git/go-git/v6"
	"gotest.tools/v3/assert"
	"gotest.tools/v3/assert/cmp"

	"github.com/CircleCI-Public/circleci-cli/internal/configoptimize/publish"
)

// These tests run on every platform, unlike publish_test.go, which needs
// unix file metadata.

func TestToFilePortable(t *testing.T) {
	dir := t.TempDir()
	in := filepath.Join(dir, "config.yml")
	assert.NilError(t, os.WriteFile(in, []byte("a: 1\n"), 0o600))

	t.Run("writes a new file", func(t *testing.T) {
		out := filepath.Join(dir, "new.yml")
		assert.Check(t, publish.ToFile(out, []byte("b: 2\n"), in, false))
		got, err := os.ReadFile(out) //#nosec:G304 // test file
		assert.Check(t, err)
		assert.Check(t, cmp.Equal(string(got), "b: 2\n"))
	})
	t.Run("refuses an existing file without --force", func(t *testing.T) {
		out := filepath.Join(dir, "exists.yml")
		assert.NilError(t, os.WriteFile(out, []byte("old\n"), 0o600))
		assert.Check(t, cmp.Equal(refusedCode(t, publish.ToFile(out, []byte("new\n"), in, false)), "output.exists"))
		assert.Check(t, publish.ToFile(out, []byte("new\n"), in, true))
	})
	t.Run("refuses to write over the input", func(t *testing.T) {
		assert.Check(t, cmp.Equal(refusedCode(t, publish.PreflightToFile(in, in, true)), "output.is_input"))
	})
}

func TestCleanTreePortable(t *testing.T) {
	t.Run("outside a git worktree", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "config.yml")
		assert.NilError(t, os.WriteFile(path, []byte("a: 1\n"), 0o600))
		assert.Check(t, cmp.Equal(refusedCode(t, publish.CleanTree(path)), "tree.not_git"))
	})
	t.Run("an untracked config", func(t *testing.T) {
		dir := t.TempDir()
		_, err := gogit.PlainInit(dir, false)
		assert.NilError(t, err)
		path := filepath.Join(dir, "config.yml")
		assert.NilError(t, os.WriteFile(path, []byte("a: 1\n"), 0o600))
		assert.Check(t, cmp.Equal(refusedCode(t, publish.CleanTree(path)), "tree.untracked"))
	})
}

func refusedCode(t *testing.T, err error) string {
	t.Helper()
	r, ok := errors.AsType[*publish.RefusedError](err)
	assert.Assert(t, ok, "want a refusal, got %v", err)
	return r.Code
}
