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

package config

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"gotest.tools/v3/assert"
)

// TestStateDir locks in the GitHub-CLI-style precedence: XDG_STATE_HOME wins on
// every platform; %LocalAppData% is the Windows default; ~/.local/state is the
// fallback elsewhere.
func TestStateDir(t *testing.T) {
	t.Run("XDG_STATE_HOME wins on every platform", func(t *testing.T) {
		xdg := t.TempDir()
		t.Setenv("XDG_STATE_HOME", xdg)
		// LocalAppData must be ignored when XDG is set, including on Windows.
		t.Setenv("LocalAppData", t.TempDir())

		dir, err := StateDir()
		assert.NilError(t, err)
		assert.Equal(t, dir, filepath.Join(xdg, "circleci"))
	})

	t.Run("platform default", func(t *testing.T) {
		t.Setenv("XDG_STATE_HOME", "")

		if runtime.GOOS == "windows" {
			// Windows uses LocalAppData (machine-local), not roaming AppData.
			lad := t.TempDir()
			t.Setenv("LocalAppData", lad)

			dir, err := StateDir()
			assert.NilError(t, err)
			assert.Equal(t, dir, filepath.Join(lad, "circleci"))
			return
		}

		home, err := os.UserHomeDir()
		assert.NilError(t, err)

		dir, err := StateDir()
		assert.NilError(t, err)
		assert.Equal(t, dir, filepath.Join(home, ".local", "state", "circleci"))
	})
}

func TestStatePath(t *testing.T) {
	xdg := t.TempDir()
	t.Setenv("XDG_STATE_HOME", xdg)

	path, err := StatePath()
	assert.NilError(t, err)
	assert.Equal(t, path, filepath.Join(xdg, "circleci", "state.yml"))
}
