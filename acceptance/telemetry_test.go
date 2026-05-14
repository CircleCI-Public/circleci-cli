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

	"github.com/CircleCI-Public/circleci-cli/internal/testing/binary"
	testenv "github.com/CircleCI-Public/circleci-cli/internal/testing/env"
)

func TestTelemetryEnable(t *testing.T) {
	env := testenv.New(t)
	dir := t.TempDir()

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"settings", "telemetry", "enable"},
		Env:     env.Environ(),
		WorkDir: dir,
	})
	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	assert.Check(t, strings.Contains(result.Stderr, "Telemetry enabled"),
		"expected 'Telemetry enabled' in stderr, got: %q", result.Stderr)

	verify := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"settings", "list", "--json"},
		Env:     env.Environ(),
		WorkDir: dir,
	})
	assert.Equal(t, verify.ExitCode, 0, "stderr: %s", verify.Stderr)
	assert.Check(t, strings.Contains(verify.Stdout, `"telemetry_enabled":true`),
		"expected telemetry_enabled:true in settings list output, got: %q", verify.Stdout)
}

func TestTelemetryDisable(t *testing.T) {
	env := testenv.New(t)
	dir := t.TempDir()

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"settings", "telemetry", "disable"},
		Env:     env.Environ(),
		WorkDir: dir,
	})
	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	assert.Check(t, strings.Contains(result.Stderr, "Telemetry disabled"),
		"expected 'Telemetry disabled' in stderr, got: %q", result.Stderr)

	verify := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"settings", "list", "--json"},
		Env:     env.Environ(),
		WorkDir: dir,
	})
	assert.Equal(t, verify.ExitCode, 0, "stderr: %s", verify.Stderr)
	assert.Check(t, strings.Contains(verify.Stdout, `"telemetry_enabled":false`),
		"expected telemetry_enabled:false in settings list output, got: %q", verify.Stdout)
}

func TestTelemetryEnableWithEnvVarOverride(t *testing.T) {
	env := testenv.New(t)
	env.Extra["CIRCLECI_NO_TELEMETRY"] = "1"

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"settings", "telemetry", "enable"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	assert.Check(t, strings.Contains(result.Stderr, "Telemetry enabled"),
		"expected 'Telemetry enabled' in stderr, got: %q", result.Stderr)
	assert.Check(t, strings.Contains(result.Stderr, "CIRCLECI_NO_TELEMETRY"),
		"expected env var override notice in stderr, got: %q", result.Stderr)
}

func TestTelemetryUnknownSubcommand(t *testing.T) {
	env := testenv.New(t)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"settings", "telemetry", "bogus"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 2, "expected exit code 2 for unknown subcommand")
}

func TestTelemetryNoArgs(t *testing.T) {
	env := testenv.New(t)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"settings", "telemetry"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	assert.Check(t, strings.Contains(result.Stdout, "enable"),
		"expected help output listing subcommands in stdout, got: %q", result.Stdout)
}
