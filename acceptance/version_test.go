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
	"strings"
	"testing"

	"gotest.tools/v3/assert"
	"gotest.tools/v3/assert/cmp"

	"github.com/CircleCI-Public/circleci-cli-v2/internal/testing/binary"
	testenv "github.com/CircleCI-Public/circleci-cli-v2/internal/testing/env"
)

func TestVersionText(t *testing.T) {
	env := testenv.New(t)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"version"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	// Format: `circleci <version> (<commit>)\n`. The acceptance binary is
	// built without ldflags, so the version is the default "dev".
	assert.Check(t, strings.HasPrefix(result.Stdout, "circleci dev ("),
		"unexpected version output: %q", result.Stdout)
	assert.Check(t, strings.HasSuffix(strings.TrimSpace(result.Stdout), ")"),
		"version output should end with a closing paren: %q", result.Stdout)
}

func TestVersionJSON(t *testing.T) {
	env := testenv.New(t)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"version", "--json"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)

	var info struct {
		Version  string `json:"version"`
		Commit   string `json:"commit"`
		Modified bool   `json:"modified"`
	}
	err := json.Unmarshal([]byte(result.Stdout), &info)
	assert.NilError(t, err, "stdout was not valid JSON: %q", result.Stdout)

	assert.Equal(t, info.Version, "dev", "expected default version 'dev', got %q", info.Version)
	assert.Check(t, info.Commit != "", "commit field should be set")
}

func TestVersionFlagMatchesSubcommand(t *testing.T) {
	// Cobra exposes --version as a built-in flag and the subcommand prints
	// the same version string. Both should report the same version so the
	// two surfaces stay consistent.
	env := testenv.New(t)

	flagResult := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"--version"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})
	assert.Equal(t, flagResult.ExitCode, 0, "stderr: %s", flagResult.Stderr)

	subResult := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"version"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})
	assert.Equal(t, subResult.ExitCode, 0, "stderr: %s", subResult.Stderr)

	// Both outputs contain "circleci dev"; the subcommand additionally
	// includes the commit hash in parens.
	assert.Check(t, cmp.Contains(flagResult.Stdout, "dev"),
		"--version output: %q", flagResult.Stdout)
	assert.Check(t, cmp.Contains(subResult.Stdout, "dev"),
		"version subcommand output: %q", subResult.Stdout)
}
