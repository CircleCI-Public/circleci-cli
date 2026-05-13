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
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gotest.tools/v3/assert"
	"gotest.tools/v3/golden"

	"github.com/CircleCI-Public/circleci-cli/internal/testing/binary"
	testenv "github.com/CircleCI-Public/circleci-cli/internal/testing/env"
)

// TestConfigGenerate_SkipsWhenExists exercises the idempotent re-run path
// from a real binary. It is the only success-path acceptance test for
// config generate; the detected-stack path is unit-tested with a fake
// scanner because the production scanner hits Docker Hub.
func TestConfigGenerate_SkipsWhenExists(t *testing.T) {
	dir := t.TempDir()
	configDir := filepath.Join(dir, ".circleci")
	assert.NilError(t, os.MkdirAll(configDir, 0o755))
	configPath := filepath.Join(configDir, "config.yml")
	original := []byte("# user's existing config\nversion: 2.1\n")
	assert.NilError(t, os.WriteFile(configPath, original, 0o644))

	env := testenv.New(t)
	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"config", "generate", dir},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	assert.Equal(t, result.Stdout, "")

	stderr := strings.ReplaceAll(result.Stderr, dir, "<DIR>")
	assert.Check(t, golden.String(stderr, t.Name()+".stderr.txt"))

	got, readErr := os.ReadFile(configPath)
	assert.NilError(t, readErr)
	assert.DeepEqual(t, got, original)
}

func TestConfigGenerate_PathNotFound(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "does-not-exist")

	env := testenv.New(t)
	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"config", "generate", missing},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 2, "expected ExitBadArguments, stderr: %s", result.Stderr)
	assert.Equal(t, result.Stdout, "")

	stderr := strings.ReplaceAll(result.Stderr, missing, "<MISSING_PATH>")
	assert.Check(t, golden.String(stderr, t.Name()+".stderr.txt"))
}
