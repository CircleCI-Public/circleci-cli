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
	"io/fs"
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

const testArtifactJobID = "cccccccc-0000-0000-0000-000000000001"

func fakeArtifactV3(path, url string, execution int) fakes.Artifact {
	return fakes.Artifact{Path: path, URL: url, Execution: execution}
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
		Args:    []string{"artifact", testArtifactJobID, "--output", downloadDir},
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
		Args:    []string{"artifact", jobID, "--output", downloadDir},
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

// The V3 artifacts endpoint answers 404 for a job that stored no artifacts as
// well as for a job that does not exist (AEX-99). A job that exists and stored
// nothing is an empty list, not "No resource found".
func TestArtifact_JobWithNoArtifacts(t *testing.T) {
	const jobID = "cccccccc-0000-0000-0000-000000000002"
	setup := func(t *testing.T) *testenv.TestEnv {
		t.Helper()
		fake := fakes.NewCircleCI(t)
		fake.AddJobV3(fakes.JobV3{ID: jobID, Name: "test-linux", Phase: "ended", Outcome: "success"})
		env := testenv.New(t)
		env.Token = testToken
		env.CircleCIURL = fake.URL()
		return env
	}

	for _, args := range [][]string{{"artifact", jobID}, {"job", "artifact", jobID}} {
		t.Run(args[0], func(t *testing.T) {
			result := binary.RunCLI(t, binary.RunOpts{
				Binary:  binaryPath,
				Args:    args,
				Env:     setup(t).Environ(),
				WorkDir: t.TempDir(),
			})

			assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
			assert.Check(t, cmp.Equal(result.Stdout, ""))
			assert.Check(t, golden.String(result.Stderr, "TestArtifact_JobWithNoArtifacts.stderr.txt"))
		})
	}
}

func TestArtifact_JobWithNoArtifacts_JSON(t *testing.T) {
	const jobID = "cccccccc-0000-0000-0000-000000000002"
	fake := fakes.NewCircleCI(t)
	fake.AddJobV3(fakes.JobV3{ID: jobID, Name: "test-linux", Phase: "ended", Outcome: "success"})
	env := testenv.New(t)
	env.Token = testToken
	env.CircleCIURL = fake.URL()

	for _, args := range [][]string{{"artifact", "--json", jobID}, {"job", "artifact", "--json", jobID}} {
		t.Run(args[0], func(t *testing.T) {
			result := binary.RunCLI(t, binary.RunOpts{
				Binary:  binaryPath,
				Args:    args,
				Env:     env.Environ(),
				WorkDir: t.TempDir(),
			})

			assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)

			var out []map[string]any
			err := json.Unmarshal([]byte(result.Stdout), &out)
			assert.NilError(t, err)
			assert.Check(t, cmp.DeepEqual(out, []map[string]any{}))

			assert.Check(t, golden.String(result.Stdout, "TestArtifact_JobWithNoArtifacts_JSON.json"))
		})
	}
}

// With --json and --output, an empty job prints only [] and downloads nothing.
func TestArtifact_JobWithNoArtifacts_JSONOutput(t *testing.T) {
	const jobID = "cccccccc-0000-0000-0000-000000000002"
	fake := fakes.NewCircleCI(t)
	fake.AddJobV3(fakes.JobV3{ID: jobID, Name: "test-linux", Phase: "ended", Outcome: "success"})
	env := testenv.New(t)
	env.Token = testToken
	env.CircleCIURL = fake.URL()

	for _, args := range [][]string{{"artifact"}, {"job", "artifact"}} {
		t.Run(args[0], func(t *testing.T) {
			outDir := filepath.Join(t.TempDir(), "out")
			result := binary.RunCLI(t, binary.RunOpts{
				Binary:  binaryPath,
				Args:    append(args, "--json", "--output", outDir, jobID),
				Env:     env.Environ(),
				WorkDir: t.TempDir(),
			})

			assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
			assert.Check(t, cmp.Equal(result.Stderr, ""))
			assert.Check(t, golden.String(result.Stdout, "TestArtifact_JobWithNoArtifacts_JSON.json"))

			_, err := os.Stat(outDir)
			assert.Check(t, cmp.ErrorIs(err, fs.ErrNotExist))
		})
	}
}

// A job ID the job endpoint does not know is still not found.
func TestArtifact_UnknownJob(t *testing.T) {
	fake := fakes.NewCircleCI(t)
	env := testenv.New(t)
	env.Token = testToken
	env.CircleCIURL = fake.URL()

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"artifact", "cccccccc-0000-0000-0000-000000000003"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 5) // ExitNotFound
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))
}
