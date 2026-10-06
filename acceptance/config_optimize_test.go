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
	"slices"
	"testing"

	"gotest.tools/v3/assert"
	"gotest.tools/v3/assert/cmp"
	"gotest.tools/v3/golden"

	"github.com/CircleCI-Public/circleci-cli/internal/testing/binary"
	testenv "github.com/CircleCI-Public/circleci-cli/internal/testing/env"
	"github.com/CircleCI-Public/circleci-cli/internal/testing/fakes"
)

// language=yaml
const optimizeConfigYAML = `version: 2.1
jobs:
  test:
    docker:
      - image: cimg/base:current
    resource_class: 2xlarge
    steps:
      - checkout
      - run: make test
workflows:
  main:
    jobs:
      - test
`

// writeOptimizeUsage writes twelve 5-minute runs of test on 2xlarge using
// about 2 of its 16 cores and little memory: a clear one-class downsize.
func writeOptimizeUsage(t *testing.T, dir string) {
	t.Helper()
	cpu := make([]float64, 20)
	mem := make([]float64, 20)
	for i := range cpu {
		cpu[i], mem[i] = 12, 10
	}
	runs := make([]map[string]any, 12)
	for i := range runs {
		runs[i] = map[string]any{"duration_seconds": 300, "executions": []map[string]any{{"index": 0, "cpu_pct": cpu, "memory_pct": mem}}}
	}
	b, err := json.Marshal(map[string]any{"schema_version": 2, "jobs": map[string]any{"test": map[string]any{"resource_class": "2xlarge", "runs": runs}}})
	assert.NilError(t, err)
	writeFile(t, filepath.Join(dir, "usage.json"), string(b))
}

// optimizeDir is a checkout with the config at .circleci/config.yml and
// usage.json beside it.
func optimizeDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	writeConfig(t, dir, optimizeConfigYAML)
	writeOptimizeUsage(t, dir)
	return dir
}

func runOptimize(t *testing.T, fake *fakes.CircleCI, dir string, args ...string) binary.CLIResult {
	t.Helper()
	env := testenv.New(t)
	env.Token = testToken
	env.CircleCIURL = fake.URL()
	return binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    append([]string{"config", "optimize"}, slices.Clone(args)...),
		Env:     env.Environ(),
		WorkDir: dir,
	})
}

func TestConfigOptimize(t *testing.T) {
	fake := fakes.NewCircleCI(t)
	fake.SetCompileResponse(true, optimizeConfigYAML)

	tests := []struct {
		name string
		args []string
	}{
		{name: "reports a downsize from usage", args: []string{"--usage", "usage.json"}},
		{name: "as JSON", args: []string{"--usage", "usage.json", "--json"}},
		{name: "without usage the resource class is a missing input", args: []string{"--only", "resource-class"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := runOptimize(t, fake, optimizeDir(t), tc.args...)
			assert.Check(t, cmp.Equal(result.ExitCode, 0))
			assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
			assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))
		})
	}
}

// compileEcho answers a compile with the config it was sent. For a config with
// no orbs or parameters that is what the real compile returns, so each edit
// config optimize compiles comes back as written.
func compileEcho(config string) fakes.CompileResponse {
	return fakes.CompileResponse{Valid: true, OutputYAML: config}
}

func TestConfigOptimize_Write(t *testing.T) {
	fake := fakes.NewCircleCI(t)
	fake.SetCompileFunc(compileEcho)

	t.Run("-o writes the optimized config to a file", func(t *testing.T) {
		dir := optimizeDir(t)
		result := runOptimize(t, fake, dir, "--usage", "usage.json", "-o", "optimized.yml")
		assert.Check(t, cmp.Equal(result.ExitCode, 0))
		assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
		assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))
		out, err := os.ReadFile(filepath.Join(dir, "optimized.yml")) //#nosec:G304 // test output
		assert.Check(t, err)
		assert.Check(t, golden.String(string(out), t.Name()+".yml.txt"))
	})
	t.Run("-o - writes the optimized config to stdout", func(t *testing.T) {
		result := runOptimize(t, fake, optimizeDir(t), "--usage", "usage.json", "-o", "-")
		assert.Check(t, cmp.Equal(result.ExitCode, 0))
		assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
		assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))
	})
	t.Run("-o refuses an existing file without --force", func(t *testing.T) {
		dir := optimizeDir(t)
		writeFile(t, filepath.Join(dir, "optimized.yml"), "keep me\n")
		result := runOptimize(t, fake, dir, "--usage", "usage.json", "-o", "optimized.yml")
		assert.Check(t, cmp.Equal(result.ExitCode, 2))
		assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
		assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))
	})
	t.Run("--in-place refuses a config outside a git worktree", func(t *testing.T) {
		result := runOptimize(t, fake, optimizeDir(t), "--usage", "usage.json", "--in-place")
		assert.Check(t, cmp.Equal(result.ExitCode, 2))
		assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
		assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))
	})
	t.Run("--in-place --force replaces the config", func(t *testing.T) {
		dir := optimizeDir(t)
		result := runOptimize(t, fake, dir, "--usage", "usage.json", "--in-place", "--force")
		assert.Check(t, cmp.Equal(result.ExitCode, 0))
		assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
		assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))
		out, err := os.ReadFile(filepath.Join(dir, ".circleci", "config.yml")) //#nosec:G304 // test output
		assert.Check(t, err)
		assert.Check(t, golden.String(string(out), t.Name()+".yml.txt"))
	})
}

func TestConfigOptimize_Invalid(t *testing.T) {
	fake := fakes.NewCircleCI(t)
	fake.SetCompileResponse(false, "", "unknown orb 'myorg/unknown@1.0.0'")
	result := runOptimize(t, fake, optimizeDir(t))
	assert.Check(t, cmp.Equal(result.ExitCode, 7))
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))
}

func TestConfigOptimize_BadFlags(t *testing.T) {
	fake := fakes.NewCircleCI(t)
	fake.SetCompileResponse(true, optimizeConfigYAML)
	tests := []struct {
		name     string
		args     []string
		wantExit int
	}{
		{name: "unknown check", args: []string{"--only", "nope"}, wantExit: 2},
		{name: "json to stdout", args: []string{"-o", "-", "--json"}, wantExit: 2},
		{name: "force without a file", args: []string{"--force"}, wantExit: 2},
		{name: "in-place from stdin", args: []string{"--in-place", "-"}, wantExit: 2},
		// Cobra's mutually-exclusive check exits 1, as for runner instance list.
		{name: "output and in-place", args: []string{"-o", "x.yml", "--in-place"}, wantExit: 1},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := runOptimize(t, fake, optimizeDir(t), tc.args...)
			assert.Check(t, cmp.Equal(result.ExitCode, tc.wantExit))
			assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
			assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))
		})
	}
}
