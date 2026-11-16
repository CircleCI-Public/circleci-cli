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
	"testing"

	"gotest.tools/v3/assert"
	"gotest.tools/v3/assert/cmp"
	"gotest.tools/v3/golden"

	"github.com/CircleCI-Public/circleci-cli/internal/testing/binary"
	testenv "github.com/CircleCI-Public/circleci-cli/internal/testing/env"
	"github.com/CircleCI-Public/circleci-cli/internal/testing/fakes"
)

const (
	raRunID       = "e0000000-0000-4000-8000-0000000000a1"
	raWorkflowID  = "b0000000-0000-4000-8000-0000000000a1"
	raTestJobID   = "c0000000-0000-4000-8000-0000000000a1"
	raLintJobID   = "c0000000-0000-4000-8000-0000000000a2"
	raHoldJobID   = "c0000000-0000-4000-8000-0000000000a3"
	raFailedRunID = "e0000000-0000-4000-8000-0000000000a2"
)

// setupRunArtifactsFake registers one successful run on main with one workflow
// of three jobs: "test" stored two artifacts, "lint" stored none, and "hold" is
// an approval job, which has no artifacts endpoint to ask. The artifact files
// themselves are served so -o can download them.
func setupRunArtifactsFake(t *testing.T) (*fakes.CircleCI, *testenv.TestEnv) {
	t.Helper()
	fake := fakes.NewCircleCI(t)
	addProjectBySlug(fake, testSlug, runTestProjectID)

	fake.AddRunV3(raRunID, runTestProjectID, fakeRunV3(raRunID, runTestProjectID, "ended", "succeeded", "main", "abc1234def5678"))
	fake.AddRunWorkflowsV3(raRunID, fakeWorkflowV3(raWorkflowID, "build-and-test", raRunID, runTestProjectID, "ended", "succeeded"))

	testJob := fakeJobV3(raTestJobID, "test", raWorkflowID, runTestProjectID)
	lintJob := fakeJobV3(raLintJobID, "lint", raWorkflowID, runTestProjectID)
	holdJob := fakeJobV3(raHoldJobID, "hold", raWorkflowID, runTestProjectID)
	holdJob.Type = "approval"
	fake.AddWorkflowJobsV3(raWorkflowID, testJob, lintJob, holdJob)
	// lint exists but stored no artifacts: its artifacts endpoint 404s and the
	// job endpoint answers.
	fake.AddJobV3(lintJob)

	fake.AddJobArtifactsV3(raTestJobID,
		fakeArtifactV3("coverage/index.html", fake.URL()+"/artifacts/coverage/index.html", 0),
		fakeArtifactV3("test-results.xml", fake.URL()+"/artifacts/test-results.xml", 0),
	)
	fake.AddStaticFile("/artifacts/coverage/index.html", "<html>coverage</html>")
	fake.AddStaticFile("/artifacts/test-results.xml", "<testsuites/>")

	env := testenv.New(t)
	env.Token = testToken
	env.CircleCIURL = fake.URL()
	return fake, env
}

func TestRunArtifacts_ByRunID(t *testing.T) {
	_, env := setupRunArtifactsFake(t)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"run", "artifacts", raRunID},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))
}

func TestRunArtifacts_ByRunID_JSON(t *testing.T) {
	_, env := setupRunArtifactsFake(t)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"run", "artifacts", raRunID, "--json"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	assert.Check(t, golden.String(result.Stdout, t.Name()+".json"))

	type job struct {
		ID        string           `json:"id"`
		Name      string           `json:"name"`
		Type      string           `json:"type"`
		Artifacts []map[string]any `json:"artifacts"`
	}
	type workflow struct {
		ID   string `json:"id"`
		Name string `json:"name"`
		Jobs []job  `json:"jobs"`
	}
	var out struct {
		ID        string     `json:"id"`
		Phase     string     `json:"phase"`
		Branch    string     `json:"branch"`
		Workflows []workflow `json:"workflows"`
	}
	err := json.Unmarshal([]byte(result.Stdout), &out)
	assert.NilError(t, err)
	// RunCLI rewrites the fake's port in output to a fixed 8000.
	const artifactBase = "http://127.0.0.1:8000/artifacts/"
	assert.Check(t, cmp.DeepEqual(out.Workflows, []workflow{{
		ID:   raWorkflowID,
		Name: "build-and-test",
		Jobs: []job{
			{ID: raTestJobID, Name: "test", Type: "build", Artifacts: []map[string]any{
				{"path": "coverage/index.html", "url": artifactBase + "coverage/index.html", "execution": float64(0)},
				{"path": "test-results.xml", "url": artifactBase + "test-results.xml", "execution": float64(0)},
			}},
			{ID: raLintJobID, Name: "lint", Type: "build", Artifacts: []map[string]any{}},
			{ID: raHoldJobID, Name: "hold", Type: "approval", Artifacts: []map[string]any{}},
		},
	}}))
	assert.Check(t, cmp.Equal(out.ID, raRunID))
	assert.Check(t, cmp.Equal(out.Phase, "ended"))
	assert.Check(t, cmp.Equal(out.Branch, "main"))
}

// TestRunArtifacts_LatestStatus checks that with no run ID the latest run is
// resolved server-side by --status: a newer failed run on main is passed over
// for the successful one.
func TestRunArtifacts_LatestStatus(t *testing.T) {
	fake := fakes.NewCircleCI(t)
	addProjectBySlug(fake, testSlug, runTestProjectID)
	// Registered first, so the fake's search returns it first: the newer run.
	fake.AddRunV3(raFailedRunID, runTestProjectID, fakeRunV3(raFailedRunID, runTestProjectID, "ended", "failed", "main", "deadbeef12345678"))
	fake.AddRunV3(raRunID, runTestProjectID, fakeRunV3(raRunID, runTestProjectID, "ended", "succeeded", "main", "abc1234def5678"))
	fake.AddRunWorkflowsV3(raRunID, fakeWorkflowV3(raWorkflowID, "build-and-test", raRunID, runTestProjectID, "ended", "succeeded"))
	fake.AddWorkflowJobsV3(raWorkflowID, fakeJobV3(raTestJobID, "test", raWorkflowID, runTestProjectID))
	fake.AddJobArtifactsV3(raTestJobID, fakeArtifactV3("coverage/index.html", fake.URL()+"/artifacts/coverage/index.html", 0))

	env := testenv.New(t)
	env.Token = testToken
	env.CircleCIURL = fake.URL()

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"run", "artifacts", "--project", testSlug, "--status", "success"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(), // not a git repo: --project must be enough
	})

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
}

// TestRunArtifacts_NoArtifacts checks that a run whose jobs stored nothing says
// so on stderr and still succeeds.
func TestRunArtifacts_NoArtifacts(t *testing.T) {
	fake := fakes.NewCircleCI(t)
	fake.AddRunV3(raRunID, runTestProjectID, fakeRunV3(raRunID, runTestProjectID, "ended", "succeeded", "main", "abc1234def5678"))
	fake.AddRunWorkflowsV3(raRunID, fakeWorkflowV3(raWorkflowID, "build-and-test", raRunID, runTestProjectID, "ended", "succeeded"))
	lintJob := fakeJobV3(raLintJobID, "lint", raWorkflowID, runTestProjectID)
	fake.AddWorkflowJobsV3(raWorkflowID, lintJob)
	fake.AddJobV3(lintJob)

	env := testenv.New(t)
	env.Token = testToken
	env.CircleCIURL = fake.URL()

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"run", "artifacts", raRunID},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Check(t, cmp.Equal(result.ExitCode, 0))
	assert.Check(t, cmp.Equal(result.Stdout, ""))
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))
}

// TestRunArtifacts_Download checks that -o writes each job's artifacts under
// <dir>/<workflow>/<job>/ and still prints the listing.
func TestRunArtifacts_Download(t *testing.T) {
	_, env := setupRunArtifactsFake(t)
	dir := t.TempDir()

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"run", "artifacts", raRunID, "-o", dir},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
	assert.Check(t, cmp.Contains(result.Stderr, "Downloaded 2 artifact(s)"))

	for path, want := range map[string]string{
		"build-and-test/test/coverage/index.html": "<html>coverage</html>",
		"build-and-test/test/test-results.xml":    "<testsuites/>",
	} {
		got, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(path)))
		if assert.Check(t, err, "reading %s", path) {
			assert.Check(t, cmp.Equal(string(got), want), "content of %s", path)
		}
	}
}

// TestRunArtifacts_Download_JSON checks that --json keeps stderr free of the
// download confirmation while -o still writes the files.
func TestRunArtifacts_Download_JSON(t *testing.T) {
	_, env := setupRunArtifactsFake(t)
	dir := t.TempDir()

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"run", "artifacts", raRunID, "-o", dir, "--json"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	assert.Check(t, cmp.Equal(result.Stderr, ""))
	assert.Check(t, json.Valid([]byte(result.Stdout)), "stdout is JSON")
	_, err := os.Stat(filepath.Join(dir, "build-and-test", "test", "coverage", "index.html"))
	assert.Check(t, err)
}

func TestRunArtifacts_RunNotFound(t *testing.T) {
	fake := fakes.NewCircleCI(t)

	env := testenv.New(t)
	env.Token = testToken
	env.CircleCIURL = fake.URL()

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"run", "artifacts", "00000000-0000-0000-0000-000000000000"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Check(t, cmp.Equal(result.ExitCode, 5)) // ExitNotFound
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))
}

// TestRunArtifacts_NoMatchingRun checks that a --status that matches no run is
// a not-found error naming what was searched.
func TestRunArtifacts_NoMatchingRun(t *testing.T) {
	fake := fakes.NewCircleCI(t)
	addProjectBySlug(fake, testSlug, runTestProjectID)
	fake.AddRunV3(raFailedRunID, runTestProjectID, fakeRunV3(raFailedRunID, runTestProjectID, "ended", "failed", "main", "deadbeef12345678"))

	env := testenv.New(t)
	env.Token = testToken
	env.CircleCIURL = fake.URL()

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"run", "artifacts", "--project", testSlug, "--status", "success"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Check(t, cmp.Equal(result.ExitCode, 5)) // ExitNotFound
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))
}

// TestRunArtifacts_StatusWithRunID checks that --status, which only selects the
// latest run, is rejected alongside an explicit run ID.
func TestRunArtifacts_StatusWithRunID(t *testing.T) {
	fake := fakes.NewCircleCI(t)

	env := testenv.New(t)
	env.Token = testToken
	env.CircleCIURL = fake.URL()

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"run", "artifacts", raRunID, "--status", "success"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Check(t, cmp.Equal(result.ExitCode, 2)) // ExitBadArguments
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))
	assert.Check(t, cmp.Nil(fake.LastRequest()), "no API call should be made")
}
