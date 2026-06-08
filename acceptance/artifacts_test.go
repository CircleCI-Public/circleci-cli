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

	"github.com/google/uuid"
	"gotest.tools/v3/assert"
	"gotest.tools/v3/assert/cmp"
	"gotest.tools/v3/golden"

	"github.com/CircleCI-Public/circleci-cli/internal/testing/binary"
	testenv "github.com/CircleCI-Public/circleci-cli/internal/testing/env"
	"github.com/CircleCI-Public/circleci-cli/internal/testing/fakes"
)

const testArtifactJobID = "cccccccc-0000-0000-0000-000000000001"

func fakeArtifactV3(path, url string, execution int) map[string]any {
	return map[string]any{
		"id": uuid.NewString(),
		"attributes": map[string]any{
			"path":      path,
			"url":       url,
			"execution": execution,
		},
	}
}

// setupArtifactFake builds a fake server with one job → two V3 artifacts.
func setupArtifactFake(t *testing.T) (*fakes.CircleCI, *testenv.TestEnv) {
	t.Helper()
	fake := fakes.NewCircleCI(t)

	fake.AddJobArtifactsV3(testArtifactJobID,
		fakeArtifactV3("coverage/index.html", fake.URL()+"/artifacts/coverage/index.html", 0),
		fakeArtifactV3("test-results.xml", fake.URL()+"/artifacts/test-results.xml", 0),
	)

	env := testenv.New(t)
	env.Token = testToken
	env.CircleCIURL = fake.URL()

	return fake, env
}

func TestArtifact_ByJobID(t *testing.T) {
	_, env := setupArtifactFake(t)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"artifact", testArtifactJobID},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
}

func TestArtifact_ByJobID_Color(t *testing.T) {
	_, env := setupArtifactFake(t)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"artifact", testArtifactJobID},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
		TTY:     true,
	})

	assert.Equal(t, result.ExitCode, 0)
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
}

func TestArtifact_MultiExecution(t *testing.T) {
	const jobID = "cccccccc-0000-0000-0000-000000000002"
	fake := fakes.NewCircleCI(t)
	fake.AddJobArtifactsV3(jobID,
		fakeArtifactV3("test-0.xml", fake.URL()+"/a/test-0.xml", 0),
		fakeArtifactV3("test-1.xml", fake.URL()+"/a/test-1.xml", 1),
		fakeArtifactV3("test-3.xml", fake.URL()+"/a/test-3.xml", 3),
		fakeArtifactV3("coverage.html", fake.URL()+"/a/coverage.html", 0),
	)

	env := testenv.New(t)
	env.Token = testToken
	env.CircleCIURL = fake.URL()

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"artifact", jobID},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
}

func TestArtifact_ByJobID_JSON(t *testing.T) {
	_, env := setupArtifactFake(t)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"artifact", "--json", testArtifactJobID},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)

	var out []map[string]any
	err := json.Unmarshal([]byte(result.Stdout), &out)
	assert.NilError(t, err)
	assert.Check(t, cmp.Len(out, 2))
	assert.Check(t, cmp.Equal(out[0]["path"], "coverage/index.html"))
	assert.Check(t, cmp.Equal(out[1]["path"], "test-results.xml"))

	assert.Check(t, golden.String(result.Stdout, t.Name()+".json"))
}

func TestArtifact_ByJobID_JSON_Color(t *testing.T) {
	_, env := setupArtifactFake(t)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"artifact", "--json", testArtifactJobID},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
		TTY:     true,
	})

	assert.Equal(t, result.ExitCode, 0)
	assert.Check(t, golden.String(result.Stdout, t.Name()+".json"))
}

func TestJobArtifact_ByJobID(t *testing.T) {
	_, env := setupArtifactFake(t)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"job", "artifact", testArtifactJobID},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
}

func TestJobArtifact_ByJobID_Color(t *testing.T) {
	_, env := setupArtifactFake(t)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"job", "artifact", testArtifactJobID},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
		TTY:     true,
	})

	assert.Equal(t, result.ExitCode, 0)
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
}

func TestArtifact_Download(t *testing.T) {
	fake, env := setupArtifactFake(t)

	fake.AddStaticFile("/artifacts/coverage/index.html", "<html>coverage</html>")
	fake.AddStaticFile("/artifacts/test-results.xml", "<xml/>")

	downloadDir := t.TempDir()
	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"artifact", testArtifactJobID, "--download", downloadDir},
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

func TestArtifact_Download_MultiExecution(t *testing.T) {
	const jobID = "cccccccc-0000-0000-0000-000000000003"
	fake := fakes.NewCircleCI(t)
	fake.AddJobArtifactsV3(jobID,
		fakeArtifactV3("results.xml", fake.URL()+"/artifacts/exec0/results.xml", 0),
		fakeArtifactV3("results.xml", fake.URL()+"/artifacts/exec1/results.xml", 1),
	)
	fake.AddStaticFile("/artifacts/exec0/results.xml", "<xml>exec0</xml>")
	fake.AddStaticFile("/artifacts/exec1/results.xml", "<xml>exec1</xml>")

	env := testenv.New(t)
	env.Token = testToken
	env.CircleCIURL = fake.URL()

	downloadDir := t.TempDir()
	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"artifact", jobID, "--download", downloadDir},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)

	data, err := os.ReadFile(filepath.Join(downloadDir, "exec-0000", "results.xml"))
	assert.NilError(t, err)
	assert.Check(t, cmp.Contains(string(data), "exec0"))

	data, err = os.ReadFile(filepath.Join(downloadDir, "exec-0001", "results.xml"))
	assert.NilError(t, err)
	assert.Check(t, cmp.Contains(string(data), "exec1"))
}

func TestArtifact_ByJobID_Quiet(t *testing.T) {
	_, env := setupArtifactFake(t)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"artifact", "--quiet", testArtifactJobID},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	assert.Check(t, cmp.Equal(result.Stderr, ""), "expected empty stderr with --quiet")
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
}

func TestArtifact_NoToken(t *testing.T) {
	env := testenv.New(t)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"artifact", testArtifactJobID},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 3) // ExitAuthError
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))
}
