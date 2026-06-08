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
	"testing"

	"gotest.tools/v3/assert"
	"gotest.tools/v3/golden"

	"github.com/CircleCI-Public/circleci-cli/internal/testing/binary"
	testenv "github.com/CircleCI-Public/circleci-cli/internal/testing/env"
	"github.com/CircleCI-Public/circleci-cli/internal/testing/fakes"
)

const (
	testJobID     = "8e50c384-0083-43d0-bc8f-93f0db589d6b"
	testProjectID = "3936b1ba-3289-44a2-96d8-d0b4fe366795"
)

func setupJobGetFake(t *testing.T) (*fakes.CircleCI, *testenv.TestEnv) {
	t.Helper()
	fake := fakes.NewCircleCI(t)

	fake.AddJobV3(testJobID, map[string]any{
		"data": map[string]any{
			"id": testJobID,
			"attributes": map[string]any{
				"name":       "yaml-lint",
				"type":       "build",
				"phase":      "ended",
				"outcome":    "succeeded",
				"started_at": "2026-05-19T20:29:13.938Z",
				"ended_at":   "2026-05-19T20:29:22.564Z",
				"parallel_executions": []map[string]any{
					{
						"steps": []map[string]any{
							{
								"name":       "Spin up environment",
								"type":       "spinup_environment",
								"num":        0,
								"phase":      "ended",
								"outcome":    "succeeded",
								"started_at": "2026-05-19T20:29:13.521Z",
								"ended_at":   "2026-05-19T20:29:14.871Z",
							},
							{
								"name":       "Checkout code",
								"type":       "checkout",
								"num":        101,
								"phase":      "ended",
								"outcome":    "succeeded",
								"started_at": "2026-05-19T20:29:15.235Z",
								"ended_at":   "2026-05-19T20:29:16.204Z",
							},
							{
								"name":       "task lint-yaml",
								"type":       "run",
								"num":        103,
								"phase":      "ended",
								"outcome":    "succeeded",
								"exit_code":  0,
								"command":    "#!/bin/bash -eo pipefail\ntask lint-yaml",
								"started_at": "2026-05-19T20:29:19.964Z",
								"ended_at":   "2026-05-19T20:29:20.613Z",
							},
						},
					},
					{
						"steps": []map[string]any{
							{
								"name":       "Spin up environment",
								"type":       "spinup_environment",
								"num":        0,
								"phase":      "ended",
								"outcome":    "succeeded",
								"started_at": "2026-05-19T20:29:13.800Z",
								"ended_at":   "2026-05-19T20:29:15.100Z",
							},
							{
								"name":       "Checkout code",
								"type":       "checkout",
								"num":        101,
								"phase":      "ended",
								"outcome":    "succeeded",
								"started_at": "2026-05-19T20:29:15.500Z",
								"ended_at":   "2026-05-19T20:29:16.450Z",
							},
							{
								"name":       "task lint-yaml",
								"type":       "run",
								"num":        103,
								"phase":      "ended",
								"outcome":    "failed",
								"exit_code":  1,
								"command":    "#!/bin/bash -eo pipefail\ntask lint-yaml\ntask lint-extra",
								"started_at": "2026-05-19T20:29:20.100Z",
								"ended_at":   "2026-05-19T20:29:21.750Z",
							},
						},
					},
				},
			},
			"references": map[string]any{
				"project":  map[string]any{"id": testProjectID},
				"pipeline": map[string]any{"id": "682142f3-f35d-4c94-adf7-d855321716e4"},
				"workflow": map[string]any{"id": "c266c3a7-0dad-459a-aa7b-dd13e005b9a0"},
				"user":     map[string]any{"id": "206f4e13-bac7-48c2-aedf-ebdd8282fba8"},
			},
		},
	})

	env := testenv.New(t)
	env.Token = "testtoken"
	env.CircleCIURL = fake.URL()

	return fake, env
}

func TestJobGet(t *testing.T) {
	_, env := setupJobGetFake(t)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"job", "get", testJobID},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
}

func TestJobGet_JSON(t *testing.T) {
	_, env := setupJobGetFake(t)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"job", "get", testJobID, "--json"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	assert.Check(t, golden.String(result.Stdout, t.Name()+".json"))
}

func TestJobGet_NotFound(t *testing.T) {
	fake := fakes.NewCircleCI(t)
	env := testenv.New(t)
	env.Token = "testtoken"
	env.CircleCIURL = fake.URL()

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"job", "get", "00000000-0000-0000-0000-000000000000"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 5, "stderr: %s", result.Stderr) // ExitNotFound
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))
}

func TestJobGet_MissingArg(t *testing.T) {
	fake := fakes.NewCircleCI(t)
	env := testenv.New(t)
	env.Token = "testtoken"
	env.CircleCIURL = fake.URL()

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"job", "get"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 2, "stderr: %s", result.Stderr) // ExitBadArguments
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))
}
