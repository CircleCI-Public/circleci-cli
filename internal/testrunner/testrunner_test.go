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

package testrunner

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"gotest.tools/v3/assert"

	"github.com/CircleCI-Public/circleci-cli/internal/reposcan"
)

type fakeRunner struct {
	buildErr error
	run      RunResult
	runErr   error
	built    bool
	ran      bool
}

func (f *fakeRunner) Build(_ context.Context, _, _ string) error {
	f.built = true
	return f.buildErr
}

func (f *fakeRunner) Run(_ context.Context, _ string) (RunResult, error) {
	f.ran = true
	return f.run, f.runErr
}

func TestRun_Pass(t *testing.T) {
	dir := writeGoModule(t)
	runner := &fakeRunner{run: RunResult{Outcome: OutcomePass, ExitCode: 0}}

	result := Run(context.Background(), dir, scanResult(), runner)

	assert.Equal(t, result.Outcome, OutcomePass)
	assert.Check(t, runner.built)
	assert.Check(t, runner.ran)
	assertFileExists(t, filepath.Join(dir, "Dockerfile.test"))
}

func TestRun_TestFailure(t *testing.T) {
	dir := writeGoModule(t)
	runner := &fakeRunner{run: RunResult{Outcome: OutcomeFail, ExitCode: 1, Stderr: "failed"}}

	result := Run(context.Background(), dir, scanResult(), runner)

	assert.Equal(t, result.Outcome, OutcomeFail)
	assert.Equal(t, result.ExitCode, 1)
	assert.Equal(t, result.Stderr, "failed")
}

func TestRun_BuildFailure(t *testing.T) {
	dir := writeGoModule(t)
	buildErr := errors.New("build failed")
	runner := &fakeRunner{buildErr: buildErr}

	result := Run(context.Background(), dir, scanResult(), runner)

	assert.Equal(t, result.Outcome, OutcomeError)
	assert.ErrorContains(t, result.Err, "build failed")
	assert.Check(t, runner.built)
	assert.Check(t, !runner.ran)
}

func TestRun_DockerMissing(t *testing.T) {
	dir := writeGoModule(t)
	runner := &fakeRunner{buildErr: ErrRunnerUnavailable}

	result := Run(context.Background(), dir, scanResult(), runner)

	assert.Equal(t, result.Outcome, OutcomeError)
	assert.Check(t, errors.Is(result.Err, ErrRunnerUnavailable))
}

func TestRun_NoTestCommandSkips(t *testing.T) {
	runner := &fakeRunner{}

	result := Run(context.Background(), t.TempDir(), &reposcan.Result{Stack: reposcan.StackUnknown}, runner)

	assert.Equal(t, result.Outcome, OutcomePass)
	assert.Check(t, result.Skipped)
	assert.Check(t, !runner.built)
	assert.Check(t, !runner.ran)
}

func scanResult() *reposcan.Result {
	return &reposcan.Result{
		Stack:        "go",
		Image:        "cimg/go",
		ImageVersion: "1.22",
		Setup: []reposcan.SetupStep{
			{Name: "install", Command: "go mod download"},
			{Name: "test", Command: "go test ./..."},
		},
	}
}

func writeGoModule(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	assert.NilError(t, os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com/fixture\n\ngo 1.22\n"), 0o644))
	assert.NilError(t, os.WriteFile(filepath.Join(dir, "go.sum"), nil, 0o644))
	return dir
}

func assertFileExists(t *testing.T, path string) {
	t.Helper()
	_, err := os.Stat(path)
	assert.NilError(t, err)
}
