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
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gotest.tools/v3/assert"
	"gotest.tools/v3/assert/cmp"
	"gotest.tools/v3/golden"

	"github.com/CircleCI-Public/circleci-cli/internal/testing/binary"
	testenv "github.com/CircleCI-Public/circleci-cli/internal/testing/env"
	"github.com/CircleCI-Public/circleci-cli/internal/testing/fakes"
)

func TestAPI_Get(t *testing.T) {
	fake := fakes.NewCircleCI(t)
	fake.AddRun(testPipelineID, fakeRun(testPipelineID, 42, "created", testSlug, "main"))

	env := testenv.New(t)
	env.Token = testToken
	env.CircleCIURL = fake.URL()

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"api", "/pipeline/" + testPipelineID},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
}

func TestAPI_Get_JQ(t *testing.T) {
	fake := fakes.NewCircleCI(t)
	fake.AddRun(testPipelineID, fakeRun(testPipelineID, 42, "created", testSlug, "main"))

	env := testenv.New(t)
	env.Token = testToken
	env.CircleCIURL = fake.URL()

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"api", "--jq", ".id", "/pipeline/" + testPipelineID},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	assert.Check(t, cmp.Equal(strings.TrimSpace(result.Stdout), testPipelineID))
}

func TestAPI_Get_Color(t *testing.T) {
	fake := fakes.NewCircleCI(t)
	fake.AddRun(testPipelineID, fakeRun(testPipelineID, 42, "created", testSlug, "main"))

	env := testenv.New(t)
	env.Token = testToken
	env.CircleCIURL = fake.URL()

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"api", "/pipeline/" + testPipelineID},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
		TTY:     true,
	})

	assert.Equal(t, result.ExitCode, 0)
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
}

func TestAPI_RawBody(t *testing.T) {
	fake := fakes.NewCircleCI(t)
	fake.SetTriggerResponse(testSlug, map[string]any{"id": testPipelineID})

	env := testenv.New(t)
	env.Token = testToken
	env.CircleCIURL = fake.URL()

	const body = `{"branch":"main","parameters":{"deploy":true}}`

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"api", "-X", "POST", "/project/" + testSlug + "/pipeline", "-d", body},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)

	got := fake.LastRequest()
	assert.Assert(t, got != nil)
	assert.Equal(t, got.Method, "POST")
	assert.Assert(t, got.Body != nil)
	// The body must be transmitted verbatim, not re-encoded.
	assert.Equal(t, *got.Body, body)
	assert.Assert(t, cmp.Contains(got.Header.Get("Content-Type"), "application/json"))
}

func TestAPI_RawBody_DefaultsToPOST(t *testing.T) {
	fake := fakes.NewCircleCI(t)
	fake.SetTriggerResponse(testSlug, map[string]any{"id": testPipelineID})

	env := testenv.New(t)
	env.Token = testToken
	env.CircleCIURL = fake.URL()

	// No -X: providing -d should default the method to POST.
	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"api", "/project/" + testSlug + "/pipeline", "-d", `{"branch":"main"}`},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	assert.Equal(t, fake.LastRequest().Method, "POST")
}

func TestAPI_RawBody_FromFile(t *testing.T) {
	fake := fakes.NewCircleCI(t)
	fake.SetTriggerResponse(testSlug, map[string]any{"id": testPipelineID})

	env := testenv.New(t)
	env.Token = testToken
	env.CircleCIURL = fake.URL()

	dir := t.TempDir()
	const body = `{"branch":"release"}`
	path := filepath.Join(dir, "body.json")
	assert.NilError(t, os.WriteFile(path, []byte(body), 0o600))

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"api", "-X", "POST", "/project/" + testSlug + "/pipeline", "-d", "@" + path},
		Env:     env.Environ(),
		WorkDir: dir,
	})

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	assert.Equal(t, *fake.LastRequest().Body, body)
}

func TestAPI_RawBody_FromStdin(t *testing.T) {
	fake := fakes.NewCircleCI(t)
	fake.SetTriggerResponse(testSlug, map[string]any{"id": testPipelineID})

	env := testenv.New(t)
	env.Token = testToken
	env.CircleCIURL = fake.URL()

	const body = `{"branch":"from-stdin"}`

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"api", "-X", "POST", "/project/" + testSlug + "/pipeline", "-d", "@-"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
		Stdin:   strings.NewReader(body),
	})

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	assert.Equal(t, *fake.LastRequest().Body, body)
}

func TestAPI_RawBody_InvalidJSON(t *testing.T) {
	env := testenv.New(t)
	env.Token = testToken

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"api", "-X", "POST", "/project/" + testSlug + "/pipeline", "-d", "not json"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 2, "stderr: %s", result.Stderr) // ExitBadArguments
}

func TestAPI_RawBody_ConflictsWithFields(t *testing.T) {
	env := testenv.New(t)
	env.Token = testToken

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"api", "-X", "POST", "/project/" + testSlug + "/pipeline", "-d", `{}`, "-f", "branch=main"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 2, "stderr: %s", result.Stderr) // ExitBadArguments
}

func TestAPI_NotFound(t *testing.T) {
	fake := fakes.NewCircleCI(t)

	env := testenv.New(t)
	env.Token = testToken
	env.CircleCIURL = fake.URL()

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"api", "/pipeline/does-not-exist"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 4, "stderr: %s", result.Stderr) // ExitAPIError
}

func TestAPI_NoToken(t *testing.T) {
	env := testenv.New(t)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"api", "/me"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 3, "stderr: %s", result.Stderr) // ExitAuthError
}

func TestAPI_PathDefaultsToV2(t *testing.T) {
	fake := fakes.NewCircleCI(t)
	fake.AddRun(testPipelineID, fakeRun(testPipelineID, 7, "created", testSlug, "main"))

	env := testenv.New(t)
	env.Token = testToken
	env.CircleCIURL = fake.URL()

	// Path without /api/ prefix should be routed to /api/v2.
	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"api", "/pipeline/" + testPipelineID},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
}

func TestAPI_PathDefaultsToV2_Color(t *testing.T) {
	fake := fakes.NewCircleCI(t)
	fake.AddRun(testPipelineID, fakeRun(testPipelineID, 7, "created", testSlug, "main"))

	env := testenv.New(t)
	env.Token = testToken
	env.CircleCIURL = fake.URL()

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"api", "/pipeline/" + testPipelineID},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
		TTY:     true,
	})

	assert.Equal(t, result.ExitCode, 0)
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
}
