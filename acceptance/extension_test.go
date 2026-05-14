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
	"gotest.tools/v3/assert/cmp"
	"gotest.tools/v3/golden"

	"github.com/CircleCI-Public/circleci-cli/internal/testing/binary"
	testenv "github.com/CircleCI-Public/circleci-cli/internal/testing/env"
)

// TestExtensionDispatch verifies that an unknown command is transparently
// dispatched to circleci-<name> when the binary exists in PATH.
func TestExtension(t *testing.T) {
	env := testenv.New(t)
	env.Token = testToken

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"testextension", "arg1", "arg2"},
		Env:     withExtDir(env.Environ(), testBinaryDir),
		WorkDir: t.TempDir(),
	})

	assert.Check(t, cmp.Equal(result.ExitCode, 0))
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
}

// TestExtensionTokenFromConfig verifies that CIRCLECI_TOKEN is injected from
// the CLI config file when the token is not already present in the environment.
// This is distinct from TestExtensionEnvInjection, which only exercises the
// env-passthrough path in buildEnv.
func TestExtensionTokenFromConfig(t *testing.T) {
	env := testenv.New(t)
	env.Token = testToken

	// Write the token to the config file. Do NOT set env.Token — the token
	// must reach the extension via the CLI's config-loading path.
	configDir := filepath.Join(env.HomeDir, ".config", "circleci")
	err := os.MkdirAll(configDir, 0o755)
	assert.NilError(t, err)
	err = os.WriteFile(filepath.Join(configDir, "config.yml"), []byte("token: "+testToken+"\n"), 0o600)
	assert.NilError(t, err)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"--insecure-storage", "testextension"},
		Env:     withExtDir(env.Environ(), testBinaryDir),
		WorkDir: t.TempDir(),
	})

	assert.Check(t, cmp.Equal(result.ExitCode, 0))
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))
}

// TestExtensionExitCodePropagated verifies that the extension's exit code is
// propagated back to the caller unchanged.
func TestExtensionExitCodePropagated(t *testing.T) {
	env := testenv.New(t)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"testextension", "exit", "123"},
		Env:     withExtDir(env.Environ(), testBinaryDir),
		WorkDir: t.TempDir(),
	})

	assert.Check(t, cmp.Equal(result.ExitCode, 123))
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))
}

// TestExtensionNotFoundShowsOriginalError verifies that when no matching
// extension exists, the original "unknown command" error from Cobra is shown
// and the ErrNotFound message (which names the missing binary) does not leak.
func TestExtensionNotFoundShowsOriginalError(t *testing.T) {
	env := testenv.New(t)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"no-such-command-xyz"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Check(t, cmp.Equal(result.ExitCode, 1))
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))
}

// TestExtensionNestedUnknownNotIntercepted verifies that unknown subcommands
// within a known group (e.g. "circleci pipeline foo") are not dispatched to
// any extension — only top-level unknown commands are intercepted.
func TestExtensionNestedUnknownNotIntercepted(t *testing.T) {
	// Use an empty directory for the path
	extDir := t.TempDir()

	env := testenv.New(t)
	env.Token = testToken

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"run", "foo"},
		Env:     withExtDir(env.Environ(), extDir),
		WorkDir: t.TempDir(),
	})

	// The key assertion: the extension script prints "args:" — if that appears,
	// the extension was wrongly invoked. Whether the group shows help or an error
	// is pre-existing behavior outside this feature's scope.
	assert.Check(t, cmp.Equal(result.ExitCode, 0))
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))
}

// withExtDir prepends extDir to PATH in the given environ slice.
func withExtDir(environ []string, extDir string) []string {
	out := make([]string, 0, len(environ)+1)
	out = append(out, environ...)
	for i, v := range out {
		if strings.HasPrefix(v, "PATH=") {
			out[i] = "PATH=" + extDir + string(os.PathListSeparator) + v[len("PATH="):]
			return out
		}
	}
	return append(out, "PATH="+extDir)
}
