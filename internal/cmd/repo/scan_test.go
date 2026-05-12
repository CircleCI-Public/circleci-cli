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

package repo

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"gotest.tools/v3/assert"
	"gotest.tools/v3/assert/cmp"

	clierrors "github.com/CircleCI-Public/circleci-cli-v2/internal/errors"
	"github.com/CircleCI-Public/circleci-cli-v2/internal/iostream"
	"github.com/CircleCI-Public/circleci-cli-v2/internal/reposcan"
)

type fakeScanner struct {
	result *reposcan.Result
	err    error
	gotDir string
}

func (f *fakeScanner) Scan(_ context.Context, dir string) (*reposcan.Result, error) {
	f.gotDir = dir
	return f.result, f.err
}

func ctxWithBuffers() (context.Context, *bytes.Buffer, *bytes.Buffer) {
	var outBuf, errBuf bytes.Buffer
	ctx := iostream.WithStreams(context.Background(), iostream.Streams{
		Out: &outBuf,
		Err: &errBuf,
		In:  strings.NewReader(""),
	})
	ctx = iostream.WithJQFilter(ctx, "")
	return ctx, &outBuf, &errBuf
}

func TestRunScan_PopulatedResult_PrintsHumanOutput_ExitZero(t *testing.T) {
	ctx, _, errBuf := ctxWithBuffers()
	scanner := &fakeScanner{result: &reposcan.Result{
		Stack:        "go",
		Image:        "cimg/go",
		ImageVersion: "1.22",
		Setup: []reposcan.SetupStep{
			{Name: "install", Command: "go mod download"},
			{Name: "test", Command: "go test ./..."},
		},
	}}

	err := runScan(ctx, scanner, t.TempDir(), false)
	assert.NilError(t, err)

	out := errBuf.String()
	assert.Check(t, cmp.Contains(out, "Detected go"), "stderr=%s", out)
	assert.Check(t, cmp.Contains(out, "cimg/go:1.22"), "stderr=%s", out)
	assert.Check(t, cmp.Contains(out, "go mod download"), "stderr=%s", out)
}

func TestRunScan_EmptyResult_PrintsFallback_ExitZero(t *testing.T) {
	ctx, _, errBuf := ctxWithBuffers()
	scanner := &fakeScanner{result: &reposcan.Result{Stack: reposcan.StackUnknown}}

	err := runScan(ctx, scanner, t.TempDir(), false)
	assert.NilError(t, err)

	assert.Check(t, cmp.Contains(errBuf.String(), "No supported stack detected"))
}

func TestRunScan_ScannerError_ReturnsCLIError(t *testing.T) {
	ctx, _, _ := ctxWithBuffers()
	scanner := &fakeScanner{err: errors.New("docker hub timeout")}

	err := runScan(ctx, scanner, t.TempDir(), false)
	assert.Assert(t, err != nil)

	var cli *clierrors.CLIError
	assert.Assert(t, errors.As(err, &cli), "expected *clierrors.CLIError, got %T", err)
	assert.Equal(t, cli.Code, "repo.scan_failed")
	assert.Equal(t, cli.ExitCode, clierrors.ExitGeneralError)
	assert.Check(t, cmp.Contains(cli.Message, "docker hub timeout"))
}

func TestRunScan_JSONFlag_WritesStructuredOutputToStdout(t *testing.T) {
	ctx, outBuf, errBuf := ctxWithBuffers()
	scanner := &fakeScanner{result: &reposcan.Result{
		Stack:        "go",
		Image:        "cimg/go",
		ImageVersion: "1.22",
		Setup: []reposcan.SetupStep{
			{Name: "install", Command: "go mod download"},
		},
	}}

	err := runScan(ctx, scanner, t.TempDir(), true)
	assert.NilError(t, err)

	var got reposcan.Result
	assert.NilError(t, json.Unmarshal(outBuf.Bytes(), &got))
	assert.Equal(t, got.Stack, "go")
	assert.Equal(t, got.Image, "cimg/go")
	assert.Equal(t, got.ImageVersion, "1.22")
	assert.Assert(t, cmp.Len(got.Setup, 1))
	assert.Equal(t, got.Setup[0].Name, "install")

	assert.Check(t, !strings.Contains(errBuf.String(), "Detected"),
		"JSON mode must not print human prose to stderr: %s", errBuf.String())
}

func TestRunScan_JSONFlag_EmptyResult_StillReturnsJSON(t *testing.T) {
	ctx, outBuf, _ := ctxWithBuffers()
	scanner := &fakeScanner{result: &reposcan.Result{Stack: reposcan.StackUnknown}}

	err := runScan(ctx, scanner, t.TempDir(), true)
	assert.NilError(t, err)

	var got reposcan.Result
	assert.NilError(t, json.Unmarshal(outBuf.Bytes(), &got))
	assert.Equal(t, got.Stack, reposcan.StackUnknown)
}

func TestRunScan_PassesCwdToScanner(t *testing.T) {
	ctx, _, _ := ctxWithBuffers()
	scanner := &fakeScanner{result: &reposcan.Result{Stack: "go"}}
	dir := t.TempDir()

	err := runScan(ctx, scanner, dir, false)
	assert.NilError(t, err)
	assert.Equal(t, scanner.gotDir, dir)
}

func TestNewScanCmd_HasJSONFlag(t *testing.T) {
	cmd := newScanCmd()
	assert.Check(t, cmd.Flag("json") != nil, "scan command must have --json flag")
}

func TestNewScanCmd_NoArgsAllowed(t *testing.T) {
	cmd := newScanCmd()
	assert.Assert(t, cmd.Args != nil)
	assert.NilError(t, cmd.Args(cmd, []string{}))
	assert.ErrorContains(t, cmd.Args(cmd, []string{"extra"}), "")
}
