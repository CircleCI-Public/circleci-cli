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

package acceptance_test

import (
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"gotest.tools/v3/assert"
	"gotest.tools/v3/assert/cmp"
	"gotest.tools/v3/golden"

	"github.com/CircleCI-Public/circleci-cli/internal/testing/binary"
	testenv "github.com/CircleCI-Public/circleci-cli/internal/testing/env"
)

// TestSetup_ConfigFlag checks that the legacy setup command writes host and
// token to the file named by -c, not the default config (issue #1901).
func TestSetup_ConfigFlag(t *testing.T) {
	env := testenv.New(t)
	dir := t.TempDir()
	probe := filepath.Join(dir, "probe.yml")
	defaultPath := filepath.Join(env.ConfigDir(), "circleci", "config.yml")

	assert.Assert(t, t.Run("setup", func(t *testing.T) {
		result := binary.RunCLI(t, binary.RunOpts{
			Binary: binaryPath,
			Args: []string{"-c", "probe.yml", "setup", "--no-prompt",
				"--host", "https://example.invalid", "--token", "probe-token"},
			Env:     env.Environ(),
			WorkDir: dir,
		})
		assert.Check(t, cmp.Equal(result.ExitCode, 0))
		assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
		assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))
	}))

	t.Run("probe written", func(t *testing.T) {
		cfg := readConfigFile(t, probe)
		assert.Check(t, cmp.Equal(cfg["host"], "https://example.invalid"))
		assert.Check(t, cmp.Equal(cfg["token"], "probe-token"))
	})

	t.Run("default not written", func(t *testing.T) {
		_, err := os.Stat(defaultPath)
		assert.Check(t, cmp.ErrorIs(err, fs.ErrNotExist), "default config should not be created")
	})
}
