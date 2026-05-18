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

package configgen_test

import (
	"bytes"
	"context"
	stderrors "errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gotest.tools/v3/assert"
	"gotest.tools/v3/assert/cmp"
	"gotest.tools/v3/golden"

	"github.com/CircleCI-Public/circleci-cli/internal/configgen"
	clierrors "github.com/CircleCI-Public/circleci-cli/internal/errors"
	"github.com/CircleCI-Public/circleci-cli/internal/iostream"
	"github.com/CircleCI-Public/circleci-cli/internal/reposcan"
)

// runGenerate invokes Generate with a fresh context wired to a captured
// stderr buffer. It returns the stderr (with dir normalized to "<DIR>" and
// any Windows path separators flipped to "/" so goldens are stable across
// platforms) and the function's error.
func runGenerate(t *testing.T, dir string, result *reposcan.Result) (stderr string, err error) {
	t.Helper()
	var buf bytes.Buffer
	ctx := iostream.WithStreams(context.Background(), iostream.Streams{
		Out: &bytes.Buffer{},
		Err: &buf,
		In:  strings.NewReader(""),
	})
	err = configgen.Generate(ctx, dir, result)
	s := strings.ReplaceAll(buf.String(), dir, "<DIR>")
	return strings.ReplaceAll(s, `\`, `/`), err
}

func nodeResult() *reposcan.Result {
	return &reposcan.Result{
		Stack:        "node",
		Image:        "cimg/node",
		ImageVersion: "20.10",
		Setup: []reposcan.SetupStep{
			{Name: "install", Command: "npm ci"},
			{Name: "test", Command: "npm test"},
		},
	}
}

func TestGenerate_DetectedStack_WritesYAML(t *testing.T) {
	dir := t.TempDir()

	stderr, err := runGenerate(t, dir, nodeResult())
	assert.NilError(t, err)

	written, readErr := os.ReadFile(filepath.Join(dir, ".circleci", "config.yml"))
	assert.NilError(t, readErr)
	assert.Check(t, golden.Bytes(written, "detected-node.yml.golden"))
	assert.Check(t, golden.String(stderr, "detected-node.stderr.golden"))
}

func TestGenerate_UnknownStack_WritesFallback(t *testing.T) {
	dir := t.TempDir()

	stderr, err := runGenerate(t, dir, &reposcan.Result{Stack: reposcan.StackUnknown})
	assert.NilError(t, err)

	written, readErr := os.ReadFile(filepath.Join(dir, ".circleci", "config.yml"))
	assert.NilError(t, readErr)
	assert.Check(t, golden.Bytes(written, "fallback.yml.golden"))
	assert.Check(t, golden.String(stderr, "fallback.stderr.golden"))
}

func TestGenerate_WriteFailure_CleansUpTempFile(t *testing.T) {
	dir := t.TempDir()

	configgen.SetRenameForTest(t, func(_, _ string) error {
		return stderrors.New("simulated rename failure")
	})

	_, err := runGenerate(t, dir, &reposcan.Result{Stack: reposcan.StackUnknown})
	assert.Assert(t, err != nil, "expected error when rename fails")

	var cliErr *clierrors.CLIError
	assert.Assert(t, stderrors.As(err, &cliErr), "expected CLIError, got %T", err)
	assert.Check(t, cmp.Equal(cliErr.ExitCode, clierrors.ExitGeneralError))

	entries, _ := os.ReadDir(filepath.Join(dir, ".circleci"))
	var leftover []string
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), "config.") && strings.HasSuffix(e.Name(), ".tmp") {
			leftover = append(leftover, e.Name())
		}
	}
	assert.Check(t, cmp.Len(leftover, 0), "expected no temp files left behind, got %v", leftover)
}
