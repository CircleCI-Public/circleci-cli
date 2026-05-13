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

package cmdconfig_test

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gotest.tools/v3/assert"
	"gotest.tools/v3/assert/cmp"
	"gotest.tools/v3/golden"

	"github.com/CircleCI-Public/circleci-cli/internal/cmd/cmdconfig"
)

func TestGenerateCmd_RegisteredUnderConfigGroup(t *testing.T) {
	group := cmdconfig.NewConfigCmd()
	var generate, _, _ = group.Find([]string{"generate"})
	assert.Assert(t, generate != nil, "generate must be a subcommand of 'config'")
	assert.Check(t, cmp.Equal(generate.Name(), "generate"))
}

func TestGenerateCmd_HelpText(t *testing.T) {
	group := cmdconfig.NewConfigCmd()
	generate, _, err := group.Find([]string{"generate"})
	assert.NilError(t, err)

	assert.Check(t, generate.Short != "", "Short must be non-empty")
	assert.Check(t, generate.Long != "", "Long must be non-empty")
	assert.Check(t, generate.Example != "", "Example must be non-empty")

	// CLAUDE.md rule #6: Example must contain at least 3 examples. Examples in
	// this codebase start each example on its own line beginning with the CLI
	// name; count those occurrences.
	exampleCount := strings.Count(generate.Example, "circleci config generate")
	assert.Check(t, exampleCount >= 3,
		"Example must show at least 3 invocations of 'circleci config generate', got %d", exampleCount)
}

// runGenerate invokes 'circleci config generate' in-process against the given
// project directory and returns the captured stderr (with dir replaced by the
// placeholder "<DIR>" so golden files are stable across machines) plus the
// error from RunE.
func runGenerate(t *testing.T, dir string, extraArgs ...string) (stderr string, err error) {
	t.Helper()
	var buf bytes.Buffer
	group := cmdconfig.NewConfigCmd()
	group.SetOut(io.Discard)
	group.SetErr(&buf)
	args := append([]string{"generate", dir}, extraArgs...)
	group.SetArgs(args)
	err = group.Execute()
	return strings.ReplaceAll(buf.String(), dir, "<DIR>"), err
}

func TestGenerateCmd_SkipsWhenConfigExists(t *testing.T) {
	dir := t.TempDir()

	configDir := filepath.Join(dir, ".circleci")
	assert.NilError(t, os.MkdirAll(configDir, 0o755))
	configPath := filepath.Join(configDir, "config.yml")
	original := []byte("# user's existing config\nversion: 2.1\n")
	assert.NilError(t, os.WriteFile(configPath, original, 0o644))

	stderr, err := runGenerate(t, dir)
	assert.NilError(t, err)

	// File must be untouched.
	got, readErr := os.ReadFile(configPath)
	assert.NilError(t, readErr)
	assert.DeepEqual(t, got, original)

	assert.Check(t, golden.String(stderr, "skip-existing.stderr.golden"))
}
