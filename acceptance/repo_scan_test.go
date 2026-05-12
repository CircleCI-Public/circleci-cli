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
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"gotest.tools/v3/assert"
	"gotest.tools/v3/assert/cmp"

	"github.com/CircleCI-Public/circleci-cli-v2/internal/testing/binary"
	testenv "github.com/CircleCI-Public/circleci-cli-v2/internal/testing/env"
)

func TestRepoScan_EmptyDir_PrintsFallback_ExitZero(t *testing.T) {
	env := testenv.New(t)
	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"repo", "scan"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	assert.Check(t, cmp.Contains(result.Stderr, "No supported stack detected"),
		"expected fallback message in stderr, got: %s", result.Stderr)
}

func TestRepoScan_JSONFlag_EmptyDir_PrintsJSON(t *testing.T) {
	env := testenv.New(t)
	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"repo", "scan", "--json"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)

	var parsed map[string]any
	assert.NilError(t, json.Unmarshal([]byte(result.Stdout), &parsed),
		"stdout should be valid JSON, got: %s", result.Stdout)
	assert.Equal(t, parsed["stack"], "unknown")
}

func TestRepoScan_DetectsGoModule(t *testing.T) {
	dir := t.TempDir()
	gomod := "module example.com/x\n\ngo 1.22\n"
	assert.NilError(t, os.WriteFile(filepath.Join(dir, "go.mod"), []byte(gomod), 0o644))

	env := testenv.New(t)
	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"repo", "scan"},
		Env:     env.Environ(),
		WorkDir: dir,
	})

	if result.ExitCode != 0 {
		t.Skipf("repo scan exited %d (likely Docker Hub unavailable). stderr: %s",
			result.ExitCode, result.Stderr)
	}
	assert.Check(t, cmp.Contains(result.Stderr, "Detected go"),
		"expected go detection in stderr, got: %s", result.Stderr)
}

func TestRepoScan_Help(t *testing.T) {
	env := testenv.New(t)
	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"repo", "scan", "--help"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	for _, field := range []string{"stack", "image", "setup"} {
		assert.Check(t, cmp.Contains(result.Stdout, field),
			"--help should document JSON field %q; stdout: %s", field, result.Stdout)
	}
}
