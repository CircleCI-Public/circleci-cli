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
	"strings"
	"testing"

	"gotest.tools/v3/assert"

	"github.com/CircleCI-Public/circleci-cli-v2/internal/testing/binary"
	testenv "github.com/CircleCI-Public/circleci-cli-v2/internal/testing/env"
	"github.com/CircleCI-Public/circleci-cli-v2/internal/testing/fakes"
)

func TestAPI_Get(t *testing.T) {
	fake := fakes.NewCircleCI(t)
	fake.AddPipeline(testPipelineID, fakePipeline(testPipelineID, 42, "created", testSlug, "main"))

	env := testenv.New(t)
	env.Token = "testtoken"
	env.CircleCIURL = fake.URL()

	result := binary.RunCLI(t,
		[]string{"api", "/pipeline/" + testPipelineID},
		env.Environ(), t.TempDir())

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	assert.Assert(t, strings.Contains(result.Stdout, testPipelineID), "stdout: %s", result.Stdout)
}

func TestAPI_Get_JSON(t *testing.T) {
	fake := fakes.NewCircleCI(t)
	fake.AddPipeline(testPipelineID, fakePipeline(testPipelineID, 42, "created", testSlug, "main"))

	env := testenv.New(t)
	env.Token = "testtoken"
	env.CircleCIURL = fake.URL()

	result := binary.RunCLI(t,
		[]string{"api", "--json", "/pipeline/" + testPipelineID},
		env.Environ(), t.TempDir())

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)

	// --json must produce valid, indented JSON.
	var out map[string]any
	assert.NilError(t, json.Unmarshal([]byte(result.Stdout), &out), "stdout: %s", result.Stdout)
	assert.Equal(t, out["id"], testPipelineID)
}

func TestAPI_NotFound(t *testing.T) {
	fake := fakes.NewCircleCI(t)

	env := testenv.New(t)
	env.Token = "testtoken"
	env.CircleCIURL = fake.URL()

	result := binary.RunCLI(t,
		[]string{"api", "/pipeline/does-not-exist"},
		env.Environ(), t.TempDir())

	assert.Equal(t, result.ExitCode, 4, "stderr: %s", result.Stderr) // ExitAPIError
}

func TestAPI_NoToken(t *testing.T) {
	env := testenv.New(t)

	result := binary.RunCLI(t,
		[]string{"api", "/me"},
		env.Environ(), t.TempDir())

	assert.Equal(t, result.ExitCode, 3, "stderr: %s", result.Stderr) // ExitAuthError
}

func TestAPI_PathDefaultsToV2(t *testing.T) {
	fake := fakes.NewCircleCI(t)
	fake.AddPipeline(testPipelineID, fakePipeline(testPipelineID, 7, "created", testSlug, "main"))

	env := testenv.New(t)
	env.Token = "testtoken"
	env.CircleCIURL = fake.URL()

	// Path without /api/ prefix should be routed to /api/v2.
	result := binary.RunCLI(t,
		[]string{"api", "/pipeline/" + testPipelineID},
		env.Environ(), t.TempDir())

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	assert.Assert(t, strings.Contains(result.Stdout, testPipelineID))
}
