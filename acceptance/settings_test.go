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
	"gotest.tools/v3/golden"

	"github.com/CircleCI-Public/circleci-cli/internal/testing/binary"
	testenv "github.com/CircleCI-Public/circleci-cli/internal/testing/env"
)

func TestSettingsListJSON_Defaults(t *testing.T) {
	env := testenv.New(t)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"settings", "list", "--json"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	var out map[string]any
	assert.NilError(t, json.Unmarshal([]byte(result.Stdout), &out))
	assert.Check(t, cmp.Equal(out["token_set"], false))
	assert.Check(t, cmp.Equal(out["host"], "https://circleci.com"))
	assert.Check(t, cmp.Equal(out["telemetry"], true))
	assert.Check(t, golden.String(result.Stdout, t.Name()+".json"))
}

func TestSettingsListJSON_WithToken(t *testing.T) {
	env := testenv.New(t)
	env.Token = "testtoken123"

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"settings", "list", "--json"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	var out map[string]any
	assert.NilError(t, json.Unmarshal([]byte(result.Stdout), &out))
	assert.Check(t, cmp.Equal(out["token_set"], true))
	assert.Check(t, golden.String(result.Stdout, t.Name()+".json"))
}

func TestSettingsListJSON_WithCustomHost(t *testing.T) {
	env := testenv.New(t)
	dir := t.TempDir()

	set := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"settings", "set", "host", "https://circleci.example.com"},
		Env:     env.Environ(),
		WorkDir: dir,
	})
	assert.Equal(t, set.ExitCode, 0, "stderr: %s", set.Stderr)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"settings", "list", "--json"},
		Env:     env.Environ(),
		WorkDir: dir,
	})

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	var out map[string]any
	assert.NilError(t, json.Unmarshal([]byte(result.Stdout), &out))
	assert.Check(t, cmp.Equal(out["host"], "https://circleci.example.com"))
	assert.Check(t, golden.String(result.Stdout, t.Name()+".json"))
}

func TestSettingsListJSON_TelemetryEnvVarOverride(t *testing.T) {
	env := testenv.New(t)
	env.Extra["CIRCLECI_NO_TELEMETRY"] = "1"

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"settings", "list", "--json"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	var out map[string]any
	assert.NilError(t, json.Unmarshal([]byte(result.Stdout), &out))
	assert.Check(t, cmp.Equal(out["telemetry"], false))
	assert.Check(t, golden.String(result.Stdout, t.Name()+".json"))
}

func TestSettingsList_TextOutput(t *testing.T) {
	env := testenv.New(t)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"settings", "list"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	// Strip the path line since it contains a temp directory that changes each run.
	var stable []string
	for _, line := range strings.Split(result.Stdout, "\n") {
		if !strings.HasPrefix(line, "- Path:") {
			stable = append(stable, line)
		}
	}
	assert.Check(t, golden.String(strings.Join(stable, "\n"), t.Name()+".txt"))
}
