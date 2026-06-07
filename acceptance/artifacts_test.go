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
	"time"

	"gotest.tools/v3/assert"
	"gotest.tools/v3/assert/cmp"
	"gotest.tools/v3/golden"

	"github.com/CircleCI-Public/circleci-cli/internal/testing/binary"
	testenv "github.com/CircleCI-Public/circleci-cli/internal/testing/env"
	"github.com/CircleCI-Public/circleci-cli/internal/testing/fakes"
)

const (
	testPipelineID = "aaaaaaaa-0000-0000-0000-000000000001"
	testWorkflowID = "bbbbbbbb-0000-0000-0000-000000000001"
	testSlug       = "gh/testorg/testrepo"
)

func fakeWorkflow(id, name string) map[string]any {
	created := time.Date(2020, 1, 1, 12, 0, 0, 0, time.UTC)
	stopped := created.Add(2*time.Minute + 34*time.Second)
	return map[string]any{
		"id":         id,
		"name":       name,
		"status":     "success",
		"created_at": created.Format(time.RFC3339),
		"stopped_at": stopped.Format(time.RFC3339),
	}
}

func fakeJob(id, name string, jobNumber int64, slug string) map[string]any {
	return map[string]any{
		"id":           id,
		"name":         name,
		"job_number":   jobNumber,
		"status":       "success",
		"type":         "build",
		"project_slug": slug,
		"started_at":   time.Now().UTC().Format(time.RFC3339),
		"stopped_at":   time.Now().UTC().Format(time.RFC3339),
	}
}

func fakeArtifact(path, url string) map[string]any {
	return map[string]any{
		"path":       path,
		"url":        url,
		"node_index": 0,
	}
}

// setupArtifactFake builds a fake server with one pipeline → one workflow →
// one job → two artifacts.
func setupArtifactFake(t *testing.T) (*fakes.CircleCI, *testenv.TestEnv) {
	t.Helper()
	fake := fakes.NewCircleCI(t)

	fake.AddRun(testPipelineID,
		fakeRun(testPipelineID, 7, "created", testSlug, "main"))
	fake.AddProjectRuns(testSlug,
		fakeRun(testPipelineID, 7, "created", testSlug, "main"))
	fake.AddRunWorkflows(testPipelineID,
		fakeWorkflow(testWorkflowID, "build"))
	fake.AddWorkflowJobs(testWorkflowID,
		fakeJob("job-uuid-1", "build", 42, testSlug))
	fake.AddJobArtifacts(testSlug, 42,
		fakeArtifact("coverage/index.html", fake.URL()+"/artifacts/coverage/index.html"),
		fakeArtifact("test-results.xml", fake.URL()+"/artifacts/test-results.xml"),
	)

	env := testenv.New(t)
	env.Token = testToken
	env.CircleCIURL = fake.URL()

	return fake, env
}

func TestArtifacts_ByPipelineID(t *testing.T) {
	_, env := setupArtifactFake(t)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"artifacts", testPipelineID},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
}

func TestArtifacts_ByPipelineID_Color(t *testing.T) {
	_, env := setupArtifactFake(t)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"artifacts", testPipelineID},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
		TTY:     true,
	})

	assert.Equal(t, result.ExitCode, 0)
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
}

func TestArtifacts_ByPipelineID_JSON(t *testing.T) {
	_, env := setupArtifactFake(t)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"artifacts", "--json", testPipelineID},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)

	var out []map[string]any
	err := json.Unmarshal([]byte(result.Stdout), &out)
	assert.NilError(t, err)
	assert.Check(t, cmp.Len(out, 2))
	assert.Check(t, cmp.Equal(out[0]["job_name"], "build"))
	assert.Check(t, cmp.Equal(out[0]["path"], "coverage/index.html"))
	assert.Check(t, cmp.Equal(out[1]["path"], "test-results.xml"))

	assert.Check(t, golden.String(result.Stdout, t.Name()+".json"))
}

func TestArtifacts_ByPipelineID_JSON_Color(t *testing.T) {
	_, env := setupArtifactFake(t)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"artifacts", "--json", testPipelineID},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
		TTY:     true,
	})

	assert.Equal(t, result.ExitCode, 0)
	assert.Check(t, golden.String(result.Stdout, t.Name()+".json"))
}

func TestArtifacts_ByJobNumber(t *testing.T) {
	_, env := setupArtifactFake(t)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"artifacts", "--job", "42", "--project", testSlug},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
}

func TestArtifacts_ByJobNumber_Color(t *testing.T) {
	_, env := setupArtifactFake(t)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"artifacts", "--job", "42", "--project", testSlug},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
		TTY:     true,
	})

	assert.Equal(t, result.ExitCode, 0)
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
}

func TestJobArtifacts_ByJobNumber(t *testing.T) {
	_, env := setupArtifactFake(t)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"job", "artifacts", "42", "--project", testSlug},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
}

func TestJobArtifacts_ByJobNumber_Color(t *testing.T) {
	_, env := setupArtifactFake(t)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"job", "artifacts", "42", "--project", testSlug},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
		TTY:     true,
	})

	assert.Equal(t, result.ExitCode, 0)
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
}

func TestArtifacts_Download(t *testing.T) {
	fake, env := setupArtifactFake(t)

	// Serve fake artifact content from the fake server.
	// We add a simple static handler via the underlying httptest server.
	// Since our fake uses gin, we register the route on the fake directly.
	fake.AddStaticFile("/artifacts/coverage/index.html", "<html>coverage</html>")
	fake.AddStaticFile("/artifacts/test-results.xml", "<xml/>")

	downloadDir := t.TempDir()
	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"artifacts", testPipelineID, "--download", downloadDir},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)

	data, err := os.ReadFile(filepath.Join(downloadDir, "coverage", "index.html"))
	assert.NilError(t, err)
	assert.Check(t, cmp.Contains(string(data), "coverage"))

	_, err = os.ReadFile(filepath.Join(downloadDir, "test-results.xml"))
	assert.NilError(t, err)
}

func TestArtifacts_ByPipelineID_Quiet(t *testing.T) {
	_, env := setupArtifactFake(t)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"artifacts", "--quiet", testPipelineID},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	assert.Check(t, cmp.Equal(result.Stderr, ""), "expected empty stderr with --quiet")
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
}

func TestArtifacts_NoToken(t *testing.T) {
	env := testenv.New(t)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"artifacts", testPipelineID},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 3) // ExitAuthError
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))
}
