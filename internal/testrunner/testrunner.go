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

// Package testrunner runs a repository's detected test command using the
// Dockerfile emitted by chunk-cli's env-builder library.
package testrunner

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/CircleCI-Public/chunk-cli/envbuilder"

	"github.com/CircleCI-Public/circleci-cli/internal/reposcan"
)

// Outcome is the high-level result of a test run.
type Outcome string

const (
	// OutcomePass means the test command exited successfully.
	OutcomePass Outcome = "pass"
	// OutcomeFail means the test command ran and exited non-zero.
	OutcomeFail Outcome = "fail"
	// OutcomeError means the runner could not execute the test command.
	OutcomeError Outcome = "error"
)

// RunResult is the complete result of building and running the detected tests.
type RunResult struct {
	Outcome  Outcome
	ExitCode int
	Stdout   string
	Stderr   string
	Err      error
	Skipped  bool
}

// Runner builds the env-builder Dockerfile and runs the resulting test image.
type Runner interface {
	Build(ctx context.Context, dir, tag string) error
	Run(ctx context.Context, tag string) (RunResult, error)
}

// Run writes Dockerfile.test with env-builder, builds a Docker image, and uses
// the container exit code as the test pass/fail signal.
func Run(ctx context.Context, dir string, scan *reposcan.Result, runner Runner) RunResult {
	if runner == nil {
		runner = NewDefaultRunner()
	}

	if scan.IsEmpty() || testCommand(scan) == "" {
		return RunResult{Outcome: OutcomePass, Skipped: true}
	}

	env := environmentFromScan(scan)
	if _, err := envbuilder.WriteDockerfile(dir, env); err != nil {
		return RunResult{Outcome: OutcomeError, Err: fmt.Errorf("write Dockerfile.test: %w", err)}
	}

	tag := imageTag(dir)
	if err := runner.Build(ctx, dir, tag); err != nil {
		return RunResult{Outcome: OutcomeError, Err: err}
	}

	result, err := runner.Run(ctx, tag)
	if err != nil {
		if result.Outcome == "" {
			result.Outcome = OutcomeError
		}
		if result.Err == nil {
			result.Err = err
		}
		return result
	}
	if result.Outcome == "" {
		if result.ExitCode == 0 {
			result.Outcome = OutcomePass
		} else {
			result.Outcome = OutcomeFail
		}
	}
	return result
}

func environmentFromScan(scan *reposcan.Result) *envbuilder.Environment {
	env := &envbuilder.Environment{
		Stack:        scan.Stack,
		Image:        scan.Image,
		ImageVersion: scan.ImageVersion,
	}
	for _, step := range scan.Setup {
		env.Setup = append(env.Setup, envbuilder.Step{Name: step.Name, Command: step.Command})
	}
	return env
}

func testCommand(scan *reposcan.Result) string {
	if scan == nil {
		return ""
	}
	for _, step := range scan.Setup {
		if step.Name == "test" {
			return step.Command
		}
	}
	return ""
}

func imageTag(dir string) string {
	sum := sha256.Sum256([]byte(fmt.Sprintf("%s:%d", dir, time.Now().UnixNano())))
	return "circleci-init-test:" + hex.EncodeToString(sum[:])[:12]
}
