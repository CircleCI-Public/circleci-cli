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
	"context"
	stderrors "errors"
	"io"
	"strings"
	"testing"

	"gotest.tools/v3/assert"
	"gotest.tools/v3/assert/cmp"
	"gotest.tools/v3/golden"

	cmdconfig "github.com/CircleCI-Public/circleci-cli/internal/cmd/config"
	clierrors "github.com/CircleCI-Public/circleci-cli/internal/errors"
	"github.com/CircleCI-Public/circleci-cli/internal/reposcan"
)

func TestGenerateCmd_RegisteredUnderConfigGroup(t *testing.T) {
	group := cmdconfig.NewConfigCmd()
	var generate, _, _ = group.Find([]string{"generate"})
	assert.Assert(t, generate != nil, "generate must be a subcommand of 'config'")
	assert.Check(t, cmp.Equal(generate.Name(), "generate"))
}

// runGenerate invokes 'circleci config generate' in-process against the given
// project directory and returns the captured stderr (with dir replaced by the
// placeholder "<DIR>" and any Windows path separators normalized to "/" so
// golden files are stable across platforms) plus the error from RunE. Cobra's
// default error rendering is silenced because the production root command
// does the same — errors are formatted by main.go.
func runGenerate(t *testing.T, dir string, extraArgs ...string) (stderr string, err error) {
	t.Helper()
	var buf bytes.Buffer
	group := cmdconfig.NewConfigCmd()
	group.SilenceErrors = true
	group.SilenceUsage = true
	group.SetOut(io.Discard)
	group.SetErr(&buf)
	args := append([]string{"generate", dir}, extraArgs...)
	group.SetArgs(args)
	err = group.Execute()
	s := strings.ReplaceAll(buf.String(), dir, "<DIR>")
	return strings.ReplaceAll(s, `\`, `/`), err
}

// ScannerErrorIsStructured requires SetScanForTest to inject a failure —
// there is no way to trigger a deterministic scanner error from acceptance
// tests without network access. All other error paths (path-not-found,
// skip-existing) are covered by acceptance tests only to avoid duplication.
func TestGenerateCmd_ScannerErrorIsStructured(t *testing.T) {
	dir := t.TempDir()

	cmdconfig.SetScanForTest(t, func(_ context.Context, _ string) (*reposcan.Result, error) {
		return nil, stderrors.New("docker hub unreachable")
	})

	_, err := runGenerate(t, dir)
	assert.Assert(t, err != nil, "expected error from scanner failure")

	var cliErr *clierrors.CLIError
	assert.Assert(t, stderrors.As(err, &cliErr), "expected CLIError, got %T", err)
	assert.Check(t, cmp.Equal(cliErr.ExitCode, clierrors.ExitGeneralError))
	assert.Check(t, golden.String(cliErr.Format(), "scan-failed.error.txt"))
}
