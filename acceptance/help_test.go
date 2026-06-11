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
	"strings"
	"testing"

	"gotest.tools/v3/assert"
	"gotest.tools/v3/assert/cmp"

	"github.com/CircleCI-Public/circleci-cli/internal/testing/binary"
	testenv "github.com/CircleCI-Public/circleci-cli/internal/testing/env"
)

// TestHelpGettingStarted verifies that a "Getting Started" section appears in
// root help when no token is configured, and is absent when one is present.
func TestHelpGettingStarted(t *testing.T) {
	t.Run("shown when unauthenticated", func(t *testing.T) {
		for _, args := range [][]string{{"--help"}, {"help"}} {
			t.Run(strings.Join(args, " "), func(t *testing.T) {
				env := testenv.New(t)
				env.Extra["NO_COLOR"] = "1"

				result := binary.RunCLI(t, binary.RunOpts{
					Binary:  binaryPath,
					Args:    args,
					Env:     env.Environ(),
					WorkDir: t.TempDir(),
				})

				assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
				assert.Check(t, cmp.Contains(result.Stdout, "Getting Started"))
				assert.Check(t, cmp.Contains(result.Stdout, "circleci auth signup"))
				assert.Check(t, cmp.Contains(result.Stdout, "circleci auth login"))
				assert.Check(t, cmp.Contains(result.Stdout, "circleci settings set token"))
			})
		}
	})

	t.Run("hidden when authenticated", func(t *testing.T) {
		env := testenv.New(t)
		env.Token = testToken
		env.Extra["NO_COLOR"] = "1"

		result := binary.RunCLI(t, binary.RunOpts{
			Binary:  binaryPath,
			Args:    []string{"--help"},
			Env:     env.Environ(),
			WorkDir: t.TempDir(),
		})

		assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
		assert.Check(t, !strings.Contains(result.Stdout, "Getting Started"), "unexpected Getting Started section in output")
	})

	t.Run("hidden on subcommand help", func(t *testing.T) {
		env := testenv.New(t)
		env.Extra["NO_COLOR"] = "1"

		result := binary.RunCLI(t, binary.RunOpts{
			Binary:  binaryPath,
			Args:    []string{"pipeline", "--help"},
			Env:     env.Environ(),
			WorkDir: t.TempDir(),
		})

		assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
		assert.Check(t, !strings.Contains(result.Stdout, "Getting Started"), "unexpected Getting Started section in output")
	})
}

// TestHelpNoStderr guards against telemetry lifecycle bugs (e.g. double-close)
// that leak error messages onto stderr when help is requested.
func TestHelpNoStderr(t *testing.T) {
	cases := []struct {
		name string
		args []string
	}{
		{"subcommand", []string{"help"}},
		{"flag", []string{"--help"}},
		{"subcommand topic", []string{"help", "pipeline"}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			env := testenv.New(t)

			result := binary.RunCLI(t, binary.RunOpts{
				Binary:  binaryPath,
				Args:    tc.args,
				Env:     env.Environ(),
				WorkDir: t.TempDir(),
			})

			assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
			assert.Check(t, cmp.Equal(result.Stderr, ""), "expected no stderr output, got: %q", result.Stderr)
		})
	}
}
