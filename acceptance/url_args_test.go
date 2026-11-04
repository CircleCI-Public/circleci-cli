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
	"gotest.tools/v3/assert/cmp"
	"gotest.tools/v3/golden"

	"github.com/CircleCI-Public/circleci-cli/internal/testing/binary"
	testenv "github.com/CircleCI-Public/circleci-cli/internal/testing/env"
	"github.com/CircleCI-Public/circleci-cli/internal/testing/fakes"
)

// These tests cover passing a URL copied from the CircleCI app where a command
// takes a run or job UUID. The fake serves the same run as TestRunGet_ByID, so
// every run-get variant here renders the TestRunGet_ByID golden.

const urlTestSlug = "gh/testorg/testrepo"

func setupURLRunFake(t *testing.T) (*fakes.CircleCI, *testenv.TestEnv) {
	t.Helper()
	fake := fakes.NewCircleCI(t)
	fake.AddRunV3(getRunID, runTestProjectID, fakeRunV3(getRunID, runTestProjectID, "ended", "succeeded", "main", "abc1234def5678"))
	fake.AddRunWorkflowsV3(getRunID, fakeWorkflowV3(testWfID, "build", getRunID, runTestProjectID, "ended", "succeeded"))
	fake.AddWorkflowV3(testWfID, fakeWorkflowV3(testWfID, "build", getRunID, runTestProjectID, "ended", "succeeded"))
	fake.AddWorkflowJobsV3(testWfID,
		fakeJobV3("d0000000-0000-4000-8000-000000000001", "run-tests", testWfID, runTestProjectID),
		fakeJobV3("d0000000-0000-4000-8000-000000000002", "deploy", testWfID, runTestProjectID),
	)
	fake.AddProjectRuns(urlTestSlug, fakes.PipelineV2{ID: getRunID, Number: 1234, State: "created", ProjectSlug: urlTestSlug, Branch: "main", CreatedAt: "2020-01-01T12:00:00Z", UpdatedAt: "2020-01-01T12:00:00Z"})

	env := testenv.New(t)
	env.Token = testToken
	env.CircleCIURL = fake.URL()
	return fake, env
}

func TestRunGet_URL(t *testing.T) {
	for name, arg := range map[string]string{
		"pipeline page":          "https://app.circleci.com/pipelines/" + urlTestSlug + "/1234",
		"workflow page":          "https://app.circleci.com/pipelines/" + urlTestSlug + "/1234/workflows/" + testWfID,
		"details page":           "https://app.circleci.com/pipelines/" + urlTestSlug + "/1234/details?workflowId=" + testWfID + "&job=d0000000-0000-4000-8000-000000000001",
		"cli run url":            "https://app.circleci.com/pipeline/" + getRunID,
		"cli workflow url":       "https://app.circleci.com/workflow/" + testWfID,
		"pipeline page no https": "app.circleci.com/pipelines/" + urlTestSlug + "/1234",
	} {
		t.Run(name, func(t *testing.T) {
			fake, env := setupURLRunFake(t)
			result := binary.RunCLI(t, binary.RunOpts{
				Binary:  binaryPath,
				Args:    []string{"run", "get", arg},
				Env:     env.Environ(),
				WorkDir: t.TempDir(),
			})
			assert.Check(t, cmp.Equal(result.ExitCode, 0), "stderr: %s", result.Stderr)
			assert.Check(t, golden.String(normalizeAppHost(result.Stdout, fake.URL()), "TestRunGet_ByID.txt"))
		})
	}
}

func TestRunGet_URL_NotARunURL(t *testing.T) {
	_, env := setupURLRunFake(t)
	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"run", "get", "https://app.circleci.com/projects/" + urlTestSlug},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})
	assert.Check(t, cmp.Equal(result.ExitCode, 2))
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))
}

func TestRunGet_URL_UnknownPipelineNumber(t *testing.T) {
	_, env := setupURLRunFake(t)
	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"run", "get", "https://app.circleci.com/pipelines/" + urlTestSlug + "/9999"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})
	assert.Check(t, cmp.Equal(result.ExitCode, 5))
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))
}

func TestJobGet_URL(t *testing.T) {
	const wfID = "c266c3a7-0dad-459a-aa7b-dd13e005b9a0"
	for name, arg := range map[string]string{
		"details page": "https://app.circleci.com/pipelines/" + urlTestSlug + "/1234/details?workflowId=" + wfID + "&job=" + testJobID,
		"cli job url":  "https://app.circleci.com/workflow/" + wfID + "/job/" + testJobID,
		"job number":   "https://app.circleci.com/pipelines/" + urlTestSlug + "/1234/workflows/" + wfID + "/jobs/42",
		"legacy job":   "https://circleci.com/" + urlTestSlug + "/42",
	} {
		t.Run(name, func(t *testing.T) {
			fake, env := setupJobGetFake(t)
			fake.AddJobByNumber(urlTestSlug, fakes.JobByNumber{Name: "yaml-lint", Number: 42, WorkflowID: wfID, PipelineID: "682142f3-f35d-4c94-adf7-d855321716e4"})
			fake.AddWorkflowJobsV3(wfID,
				fakeJobV3(testJobID, "yaml-lint", wfID, testProjectID),
				fakeJobV3("d0000000-0000-4000-8000-000000000009", "deploy", wfID, testProjectID),
			)
			result := binary.RunCLI(t, binary.RunOpts{
				Binary:  binaryPath,
				Args:    []string{"job", "get", arg},
				Env:     env.Environ(),
				WorkDir: t.TempDir(),
			})
			assert.Check(t, cmp.Equal(result.ExitCode, 0), "stderr: %s", result.Stderr)
			assert.Check(t, golden.String(normalizeAppHost(result.Stdout, fake.URL()), "TestJobGet.txt"))
		})
	}
}

func TestJobGet_URL_NotAJobURL(t *testing.T) {
	_, env := setupJobGetFake(t)
	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"job", "get", "https://app.circleci.com/pipelines/" + urlTestSlug + "/1234"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})
	assert.Check(t, cmp.Equal(result.ExitCode, 2))
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))
}

func TestJobGet_URL_AmbiguousJobNumber(t *testing.T) {
	const wfID = "c266c3a7-0dad-459a-aa7b-dd13e005b9a0"
	fake, env := setupJobGetFake(t)
	fake.AddJobByNumber(urlTestSlug, fakes.JobByNumber{Name: "yaml-lint", Number: 42, WorkflowID: wfID, PipelineID: "682142f3-f35d-4c94-adf7-d855321716e4"})
	fake.AddWorkflowJobsV3(wfID,
		fakeJobV3(testJobID, "yaml-lint", wfID, testProjectID),
		fakeJobV3("d0000000-0000-4000-8000-000000000009", "yaml-lint", wfID, testProjectID),
	)
	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"job", "get", "https://app.circleci.com/jobs/" + urlTestSlug + "/42"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})
	assert.Check(t, cmp.Equal(result.ExitCode, 2))
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))
}

func TestJobOutputList_URL(t *testing.T) {
	const wfID = "c266c3a7-0dad-459a-aa7b-dd13e005b9a0"
	fake, env := setupJobGetFake(t)
	fake.AddJobStdout(testJobID, 0, 101, []byte("Cloning into 'repo'...\nDone.\n"))
	fake.AddJobStdout(testJobID, 0, 103, []byte(
		"layer: Downloading [==>]\r\x1b[Klayer: Downloading [====>]\r\x1b[Klayer: Download complete\n"+
			"\x1b[32mPASS\x1b[0m\n"))

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"job", "output", "list", "https://app.circleci.com/pipelines/" + urlTestSlug + "/1234/details?workflowId=" + wfID + "&job=" + testJobID},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})
	assert.Check(t, cmp.Equal(result.ExitCode, 0), "stderr: %s", result.Stderr)
	assert.Check(t, golden.String(result.Stdout, "TestJobOutputList.txt"))
}
