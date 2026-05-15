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
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"strings"

	"github.com/CircleCI-Public/circleci-cli/internal/iostream"
)

// ErrRunnerUnavailable identifies local environment failures that prevent the
// test runner from starting, such as Docker not being installed.
var ErrRunnerUnavailable = errors.New("test runner unavailable")

// DockerRunner runs tests by building and running the env-builder Dockerfile.
type DockerRunner struct{}

// NewDefaultRunner returns the production test runner.
func NewDefaultRunner() *DockerRunner {
	return &DockerRunner{}
}

func (r *DockerRunner) Build(ctx context.Context, dir, tag string) error {
	if err := dockerAvailable(ctx); err != nil {
		return err
	}

	cmd := exec.CommandContext(ctx, "docker", "build", "-f", "Dockerfile.test", "-t", tag, ".") //#nosec:G204 // fixed docker invocation, dir/tag are internally controlled
	cmd.Dir = dir
	cmd.Stdout = iostream.Err(ctx)
	cmd.Stderr = iostream.Err(ctx)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("docker build failed: %w", err)
	}
	return nil
}

func (r *DockerRunner) Run(ctx context.Context, tag string) (RunResult, error) {
	var stdout, stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, "docker", "run", "--rm", tag) //#nosec:G204 // fixed docker invocation, tag is internally controlled
	cmd.Stdout = io.MultiWriter(iostream.Out(ctx), &stdout)
	cmd.Stderr = io.MultiWriter(iostream.Err(ctx), &stderr)

	err := cmd.Run()
	result := RunResult{
		ExitCode: 0,
		Stdout:   stdout.String(),
		Stderr:   stderr.String(),
	}
	if err == nil {
		result.Outcome = OutcomePass
		return result, nil
	}

	if exitErr, ok := errors.AsType[*exec.ExitError](err); ok {
		result.Outcome = OutcomeFail
		result.ExitCode = exitErr.ExitCode()
		return result, nil
	}

	result.Outcome = OutcomeError
	result.Err = fmt.Errorf("docker run failed: %w", err)
	return result, result.Err
}

func dockerAvailable(ctx context.Context) error {
	if _, err := exec.LookPath("docker"); err != nil {
		return fmt.Errorf("%w: Docker is required to run tests via env-builder", ErrRunnerUnavailable)
	}

	cmd := exec.CommandContext(ctx, "docker", "info", "--format", "{{.ServerVersion}}") //#nosec:G204 // fixed docker invocation
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		return fmt.Errorf("%w: Docker is installed but the daemon is not reachable: %s", ErrRunnerUnavailable, msg)
	}
	return nil
}
