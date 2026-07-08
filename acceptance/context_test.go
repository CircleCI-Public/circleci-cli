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
	"net/http"
	"net/url"
	"runtime"
	"strings"
	"testing"

	"github.com/CircleCI-Public/circleci-cli/internal/httpcl"
	"gotest.tools/v3/assert"
	"gotest.tools/v3/assert/cmp"
	"gotest.tools/v3/golden"

	"github.com/CircleCI-Public/circleci-cli/internal/testing/binary"
	testenv "github.com/CircleCI-Public/circleci-cli/internal/testing/env"
	"github.com/CircleCI-Public/circleci-cli/internal/testing/fakes"
	"github.com/CircleCI-Public/circleci-cli/internal/testing/httprecorder"
)

const testOrgSlug = "gh/testorg"
const testContextID = "c0000001-0000-4000-8000-000000000001"
const testContextID2 = "c0000002-0000-4000-8000-000000000002"
const testRestrictionID = "e0000001-0000-4000-8000-000000000001"
const testCreatedRestrictionID = "c0000003-0000-4000-8000-000000000003"

func fakeContext(id, name string) map[string]any {
	return map[string]any{
		"id":         id,
		"name":       name,
		"created_at": "2020-01-01T12:00:00Z",
	}
}

func fakeContextEnvVar(contextID, variable string) map[string]any {
	return map[string]any{
		"variable":        variable,
		"truncated_value": "abcd",
		"context_id":      contextID,
		"created_at":      "2020-01-01T12:00:00Z",
		"updated_at":      "2020-06-01T12:00:00Z",
	}
}

func fakeContextRestriction(contextID, id, restrictionType, restrictionValue, name string) map[string]any {
	return map[string]any{
		"context_id":        contextID,
		"id":                id,
		"restriction_type":  restrictionType,
		"restriction_value": restrictionValue,
		"name":              name,
	}
}

// --- context list ---

func TestContextList(t *testing.T) {
	fake := fakes.NewCircleCI(t)
	fake.AddContext(testOrgSlug, fakeContext(testContextID, "my-context"))
	fake.AddContext(testOrgSlug, fakeContext(testContextID2, "other-context"))

	env := testenv.New(t)
	env.Token = testToken
	env.CircleCIURL = fake.URL()

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"context", "list", "--org", testOrgSlug},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
}

func TestContextList_JSON(t *testing.T) {
	fake := fakes.NewCircleCI(t)
	fake.AddContext(testOrgSlug, fakeContext(testContextID, "my-context"))
	fake.AddContext(testOrgSlug, fakeContext(testContextID2, "other-context"))

	env := testenv.New(t)
	env.Token = testToken
	env.CircleCIURL = fake.URL()

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"context", "list", "--org", testOrgSlug, "--json"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)

	var out []map[string]any
	assert.NilError(t, json.Unmarshal([]byte(result.Stdout), &out))
	assert.Check(t, cmp.Len(out, 2))
	assert.Check(t, cmp.Equal(out[0]["id"], testContextID))
	assert.Check(t, cmp.Equal(out[0]["name"], "my-context"))
	assert.Check(t, cmp.Equal(out[1]["name"], "other-context"))
}

func TestContextList_JQ(t *testing.T) {
	fake := fakes.NewCircleCI(t)
	fake.AddContext(testOrgSlug, fakeContext(testContextID, "my-context"))

	env := testenv.New(t)
	env.Token = testToken
	env.CircleCIURL = fake.URL()

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"context", "list", "--org", testOrgSlug, "--json", "--jq", ".[0].name"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	assert.Check(t, cmp.Equal(strings.TrimSpace(result.Stdout), "my-context"))
}

func TestContextList_Empty(t *testing.T) {
	fake := fakes.NewCircleCI(t)

	env := testenv.New(t)
	env.Token = testToken
	env.CircleCIURL = fake.URL()

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"context", "list", "--org", testOrgSlug},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))
}

func TestContextList_Name(t *testing.T) {
	fake := fakes.NewCircleCI(t)
	fake.AddContext(testOrgSlug, fakeContext(testContextID, "my-context"))
	fake.AddContext(testOrgSlug, fakeContext(testContextID2, "other-context"))

	env := testenv.New(t)
	env.Token = testToken
	env.CircleCIURL = fake.URL()

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"context", "list", "--org", testOrgSlug, "--name", "my"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
}

func TestContextList_Name_JSON(t *testing.T) {
	fake := fakes.NewCircleCI(t)
	fake.AddContext(testOrgSlug, fakeContext(testContextID, "my-context"))
	fake.AddContext(testOrgSlug, fakeContext(testContextID2, "other-context"))

	env := testenv.New(t)
	env.Token = testToken
	env.CircleCIURL = fake.URL()

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"context", "list", "--org", testOrgSlug, "--name", "my", "--json"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	var out []map[string]any
	assert.NilError(t, json.Unmarshal([]byte(result.Stdout), &out))
	assert.Check(t, cmp.Len(out, 1))
	assert.Check(t, cmp.Equal(out[0]["name"], "my-context"))
}

func TestContextList_NoToken(t *testing.T) {
	env := testenv.New(t)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"context", "list", "--org", testOrgSlug},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 3) // ExitAuthError
}

// --- context get ---

func TestContextGet(t *testing.T) {
	fake := fakes.NewCircleCI(t)
	fake.AddContext(testOrgSlug, fakeContext(testContextID, "my-context"))
	fake.AddContextEnvVar(testContextID, fakeContextEnvVar(testContextID, "DB_PASSWORD"))
	fake.AddContextEnvVar(testContextID, fakeContextEnvVar(testContextID, "API_KEY"))
	fake.AddContextRestriction(testContextID, fakeContextRestriction(
		testContextID, "e0000001-0000-4000-8000-000000000001",
		"project", "b0000001-0000-4000-8000-000000000001", "myrepo",
	))

	env := testenv.New(t)
	env.Token = testToken
	env.CircleCIURL = fake.URL()

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"context", "get", testContextID},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
}

func TestContextGet_JSON(t *testing.T) {
	fake := fakes.NewCircleCI(t)
	fake.AddContext(testOrgSlug, fakeContext(testContextID, "my-context"))
	fake.AddContextEnvVar(testContextID, fakeContextEnvVar(testContextID, "DB_PASSWORD"))
	fake.AddContextRestriction(testContextID, fakeContextRestriction(
		testContextID, "e0000001-0000-4000-8000-000000000001",
		"project", "b0000001-0000-4000-8000-000000000001", "myrepo",
	))

	env := testenv.New(t)
	env.Token = testToken
	env.CircleCIURL = fake.URL()

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"context", "get", testContextID, "--json"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)

	var out map[string]any
	assert.NilError(t, json.Unmarshal([]byte(result.Stdout), &out))
	assert.Check(t, cmp.Equal(out["id"], testContextID))
	assert.Check(t, cmp.Equal(out["name"], "my-context"))
	assert.Check(t, out["org_id"] != nil)
	evs := out["environment_variables"].([]any)
	assert.Check(t, cmp.Len(evs, 1))
	assert.Check(t, cmp.Equal(evs[0].(map[string]any)["variable"], "DB_PASSWORD"))
	rs := out["restrictions"].([]any)
	assert.Check(t, cmp.Len(rs, 1))
	assert.Check(t, cmp.Equal(rs[0].(map[string]any)["restriction_type"], "project"))
}

func TestContextGet_NotFound(t *testing.T) {
	fake := fakes.NewCircleCI(t)

	env := testenv.New(t)
	env.Token = testToken
	env.CircleCIURL = fake.URL()

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"context", "get", "00000000-0000-0000-0000-000000000000"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 5) // ExitNotFound
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))
}

func TestContextGet_MissingArg(t *testing.T) {
	env := testenv.New(t)
	env.Token = testToken

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"context", "get"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 2) // ExitBadArguments
}

func TestContextGet_NoToken(t *testing.T) {
	env := testenv.New(t)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"context", "get", testContextID},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 3) // ExitAuthError
}

func TestContextGet_ByName(t *testing.T) {
	fake := fakes.NewCircleCI(t)
	fake.AddContext(testOrgSlug, fakeContext(testContextID, "my-context"))
	fake.AddContextEnvVar(testContextID, fakeContextEnvVar(testContextID, "DB_PASSWORD"))

	env := testenv.New(t)
	env.Token = testToken
	env.CircleCIURL = fake.URL()

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"context", "get", "my-context", "--org", testOrgSlug},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
}

func TestContextGet_ByName_NotFound(t *testing.T) {
	fake := fakes.NewCircleCI(t)
	fake.AddContext(testOrgSlug, fakeContext(testContextID, "my-context"))

	env := testenv.New(t)
	env.Token = testToken
	env.CircleCIURL = fake.URL()

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"context", "get", "nonexistent", "--org", testOrgSlug},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 5) // ExitNotFound
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))
}

// --- context create ---

func TestContextCreate(t *testing.T) {
	fake := fakes.NewCircleCI(t)

	env := testenv.New(t)
	env.Token = testToken
	env.CircleCIURL = fake.URL()
	env.Extra = map[string]string{
		"AI_AGENT": "chunk",
	}

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"context", "create", "new-context", "--org", testOrgSlug},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))

	t.Run("check request", func(t *testing.T) {
		assert.Check(t, cmp.DeepEqual(fake.LastRequest(), &httprecorder.Request{
			Method: http.MethodPost,
			URL:    url.URL{Path: "/api/v2/context"},
			Header: http.Header{
				"Authorization": {"Bearer test-token"},
				"User-Agent":    {httpcl.UserAgent(runtime.GOOS, runtime.GOARCH, "dev", "chunk")},
			},
			Body: new(`{"name":"new-context","owner":{"slug":"gh/testorg","type":"organization"}}`),
		}, ignoreCommonHeaders))
	})
}

func TestContextCreate_JSON(t *testing.T) {
	fake := fakes.NewCircleCI(t)

	env := testenv.New(t)
	env.Token = testToken
	env.CircleCIURL = fake.URL()

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"context", "create", "new-context", "--org", testOrgSlug, "--json"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)

	var out map[string]any
	assert.NilError(t, json.Unmarshal([]byte(result.Stdout), &out))
	assert.Check(t, cmp.Equal(out["name"], "new-context"))
	assert.Check(t, out["id"] != nil)
	assert.Check(t, out["created_at"] != nil)
}

func TestContextCreate_MissingArg(t *testing.T) {
	env := testenv.New(t)
	env.Token = testToken

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"context", "create"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 2) // ExitBadArguments
}

func TestContextCreate_NoToken(t *testing.T) {
	env := testenv.New(t)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"context", "create", "new-context", "--org", testOrgSlug},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 3) // ExitAuthError
}

// --- context delete ---

func TestContextDelete(t *testing.T) {
	fake := fakes.NewCircleCI(t)
	fake.AddContext(testOrgSlug, fakeContext(testContextID, "my-context"))

	env := testenv.New(t)
	env.Token = testToken
	env.CircleCIURL = fake.URL()

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"context", "delete", testContextID, "--force"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))

	t.Run("check request", func(t *testing.T) {
		assert.Check(t, cmp.DeepEqual(fake.LastRequest(), &httprecorder.Request{
			Method: http.MethodDelete,
			URL:    url.URL{Path: "/api/v2/context/" + testContextID},
			Header: http.Header{
				"Authorization": {"Bearer test-token"},
				"User-Agent":    {httpcl.UserAgent(runtime.GOOS, runtime.GOARCH, "dev", "")},
			},
			Body: new(""),
		}, ignoreCommonHeaders))
	})
}

func TestContextDelete_RequiresForce(t *testing.T) {
	fake := fakes.NewCircleCI(t)
	fake.AddContext(testOrgSlug, fakeContext(testContextID, "my-context"))

	env := testenv.New(t)
	env.Token = testToken
	env.CircleCIURL = fake.URL()

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"context", "delete", testContextID},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 6) // ExitCancelled
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))
}

func TestContextDelete_NotFound(t *testing.T) {
	fake := fakes.NewCircleCI(t)

	env := testenv.New(t)
	env.Token = testToken
	env.CircleCIURL = fake.URL()

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"context", "delete", "00000000-0000-0000-0000-000000000000", "--force"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 5) // ExitNotFound
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))
}

func TestContextDelete_MissingArg(t *testing.T) {
	env := testenv.New(t)
	env.Token = testToken

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"context", "delete"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 2) // ExitBadArguments
}

func TestContextDelete_ByName(t *testing.T) {
	fake := fakes.NewCircleCI(t)
	fake.AddContext(testOrgSlug, fakeContext(testContextID, "my-context"))

	env := testenv.New(t)
	env.Token = testToken
	env.CircleCIURL = fake.URL()

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"context", "delete", "my-context", "--org", testOrgSlug, "--force"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
}

func TestContextDelete_ByName_NotFound(t *testing.T) {
	fake := fakes.NewCircleCI(t)
	fake.AddContext(testOrgSlug, fakeContext(testContextID, "my-context"))

	env := testenv.New(t)
	env.Token = testToken
	env.CircleCIURL = fake.URL()

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"context", "delete", "nonexistent", "--org", testOrgSlug, "--force"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 5) // ExitNotFound
}

// --- context secret list ---

func TestContextSecretList(t *testing.T) {
	fake := fakes.NewCircleCI(t)
	fake.AddContext(testOrgSlug, fakeContext(testContextID, "my-context"))
	fake.AddContextEnvVar(testContextID, fakeContextEnvVar(testContextID, "DB_PASSWORD"))
	fake.AddContextEnvVar(testContextID, fakeContextEnvVar(testContextID, "API_KEY"))

	env := testenv.New(t)
	env.Token = testToken
	env.CircleCIURL = fake.URL()

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"context", "secret", "list", testContextID},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
}

func TestContextSecretList_JSON(t *testing.T) {
	fake := fakes.NewCircleCI(t)
	fake.AddContext(testOrgSlug, fakeContext(testContextID, "my-context"))
	fake.AddContextEnvVar(testContextID, fakeContextEnvVar(testContextID, "DB_PASSWORD"))

	env := testenv.New(t)
	env.Token = testToken
	env.CircleCIURL = fake.URL()

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"context", "secret", "list", testContextID, "--json"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)

	var out []map[string]any
	assert.NilError(t, json.Unmarshal([]byte(result.Stdout), &out))
	assert.Check(t, cmp.Len(out, 1))
	assert.Check(t, cmp.Equal(out[0]["variable"], "DB_PASSWORD"))
	assert.Check(t, cmp.Equal(out[0]["context_id"], testContextID))
}

func TestContextSecretList_Empty(t *testing.T) {
	fake := fakes.NewCircleCI(t)

	env := testenv.New(t)
	env.Token = testToken
	env.CircleCIURL = fake.URL()

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"context", "secret", "list", testContextID},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))
}

func TestContextSecretList_MissingArg(t *testing.T) {
	env := testenv.New(t)
	env.Token = testToken

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"context", "secret", "list"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 2) // ExitBadArguments
}

func TestContextSecretList_ByName(t *testing.T) {
	fake := fakes.NewCircleCI(t)
	fake.AddContext(testOrgSlug, fakeContext(testContextID, "my-context"))
	fake.AddContextEnvVar(testContextID, fakeContextEnvVar(testContextID, "DB_PASSWORD"))
	fake.AddContextEnvVar(testContextID, fakeContextEnvVar(testContextID, "API_KEY"))

	env := testenv.New(t)
	env.Token = testToken
	env.CircleCIURL = fake.URL()

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"context", "secret", "list", "my-context", "--org", testOrgSlug},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
}

// --- context secret set ---

func TestContextSecretSet(t *testing.T) {
	fake := fakes.NewCircleCI(t)
	fake.AddContext(testOrgSlug, fakeContext(testContextID, "my-context"))

	env := testenv.New(t)
	env.Token = testToken
	env.CircleCIURL = fake.URL()

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"context", "secret", "set", testContextID, "--name", "MY_VAR", "--value", "s3cr3t"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))

	t.Run("check request", func(t *testing.T) {
		assert.Check(t, cmp.DeepEqual(fake.LastRequest(), &httprecorder.Request{
			Method: http.MethodPut,
			URL:    url.URL{Path: "/api/v2/context/" + testContextID + "/environment-variable/MY_VAR"},
			Header: http.Header{
				"Authorization": {"Bearer test-token"},
				"User-Agent":    {httpcl.UserAgent(runtime.GOOS, runtime.GOARCH, "dev", "")},
			},
			Body: new(`{"value":"s3cr3t"}`),
		}, ignoreCommonHeaders))
	})
}

func TestContextSecretSet_MissingName(t *testing.T) {
	env := testenv.New(t)
	env.Token = testToken

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"context", "secret", "set", testContextID, "--value", "s3cr3t"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 2) // ExitBadArguments
}

func TestContextSecretSet_MissingValue(t *testing.T) {
	env := testenv.New(t)
	env.Token = testToken

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"context", "secret", "set", testContextID, "--name", "MY_VAR"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 2) // ExitBadArguments
}

func TestContextSecretSet_MissingContextID(t *testing.T) {
	env := testenv.New(t)
	env.Token = testToken

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"context", "secret", "set"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 2) // ExitBadArguments
}

func TestContextSecretSet_NoToken(t *testing.T) {
	env := testenv.New(t)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"context", "secret", "set", testContextID, "--name", "MY_VAR", "--value", "val"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 3) // ExitAuthError
}

func TestContextSecretSet_ByName(t *testing.T) {
	fake := fakes.NewCircleCI(t)
	fake.AddContext(testOrgSlug, fakeContext(testContextID, "my-context"))

	env := testenv.New(t)
	env.Token = testToken
	env.CircleCIURL = fake.URL()

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"context", "secret", "set", "my-context", "--org", testOrgSlug, "--name", "MY_VAR", "--value", "s3cr3t"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
}

// --- context secret delete ---

func TestContextSecretDelete(t *testing.T) {
	fake := fakes.NewCircleCI(t)
	fake.AddContext(testOrgSlug, fakeContext(testContextID, "my-context"))
	fake.AddContextEnvVar(testContextID, fakeContextEnvVar(testContextID, "MY_VAR"))

	env := testenv.New(t)
	env.Token = testToken
	env.CircleCIURL = fake.URL()

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"context", "secret", "delete", testContextID, "--name", "MY_VAR", "--force"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))

	t.Run("check request", func(t *testing.T) {
		assert.Check(t, cmp.DeepEqual(fake.LastRequest(), &httprecorder.Request{
			Method: http.MethodDelete,
			URL:    url.URL{Path: "/api/v2/context/" + testContextID + "/environment-variable/MY_VAR"},
			Header: http.Header{
				"Authorization": {"Bearer test-token"},
				"User-Agent":    {httpcl.UserAgent(runtime.GOOS, runtime.GOARCH, "dev", "")},
			},
			Body: new(""),
		}, ignoreCommonHeaders))
	})
}

func TestContextSecretDelete_RequiresForce(t *testing.T) {
	fake := fakes.NewCircleCI(t)
	fake.AddContext(testOrgSlug, fakeContext(testContextID, "my-context"))
	fake.AddContextEnvVar(testContextID, fakeContextEnvVar(testContextID, "MY_VAR"))

	env := testenv.New(t)
	env.Token = testToken
	env.CircleCIURL = fake.URL()

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"context", "secret", "delete", testContextID, "--name", "MY_VAR"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 6) // ExitCancelled
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))
}

func TestContextSecretDelete_NotFound(t *testing.T) {
	fake := fakes.NewCircleCI(t)

	env := testenv.New(t)
	env.Token = testToken
	env.CircleCIURL = fake.URL()

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"context", "secret", "delete", testContextID, "--name", "NONEXISTENT", "--force"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 5) // ExitNotFound
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))
}

func TestContextSecretDelete_MissingArgs(t *testing.T) {
	env := testenv.New(t)
	env.Token = testToken

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"context", "secret", "delete", testContextID},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 2) // ExitBadArguments
}

func TestContextSecretDelete_ByName(t *testing.T) {
	fake := fakes.NewCircleCI(t)
	fake.AddContext(testOrgSlug, fakeContext(testContextID, "my-context"))
	fake.AddContextEnvVar(testContextID, fakeContextEnvVar(testContextID, "MY_VAR"))

	env := testenv.New(t)
	env.Token = testToken
	env.CircleCIURL = fake.URL()

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"context", "secret", "delete", "my-context", "--org", testOrgSlug, "--name", "MY_VAR", "--force"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
}

// --- context restriction create ---

func TestContextRestrictionCreate(t *testing.T) {
	fake := fakes.NewCircleCI(t)
	fake.AddContext(testOrgSlug, fakeContext(testContextID, "my-context"))

	env := testenv.New(t)
	env.Token = testToken
	env.CircleCIURL = fake.URL()

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"context", "restriction", "create", testContextID, "--type", "project", "--value", "p0000001-0000-4000-8000-000000000001"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))

	t.Run("check request", func(t *testing.T) {
		assert.Check(t, cmp.DeepEqual(fake.LastRequest(), &httprecorder.Request{
			Method: http.MethodPost,
			URL:    url.URL{Path: "/api/v2/context/" + testContextID + "/restrictions"},
			Header: http.Header{
				"Authorization": {"Bearer test-token"},
				"User-Agent":    {httpcl.UserAgent(runtime.GOOS, runtime.GOARCH, "dev", "")},
			},
			Body: new(`{"restriction_type":"project","restriction_value":"p0000001-0000-4000-8000-000000000001"}`),
		}, ignoreCommonHeaders))
	})
}

func TestContextRestrictionCreate_Color(t *testing.T) {
	fake := fakes.NewCircleCI(t)
	fake.AddContext(testOrgSlug, fakeContext(testContextID, "my-context"))

	env := testenv.New(t)
	env.Token = testToken
	env.CircleCIURL = fake.URL()

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"context", "restriction", "create", testContextID, "--type", "project", "--value", "p0000001-0000-4000-8000-000000000001"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
		TTY:     true,
	})

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
}

func TestContextRestrictionCreate_JSON(t *testing.T) {
	fake := fakes.NewCircleCI(t)
	fake.AddContext(testOrgSlug, fakeContext(testContextID, "my-context"))

	env := testenv.New(t)
	env.Token = testToken
	env.CircleCIURL = fake.URL()

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"context", "restriction", "create", testContextID, "--type", "project", "--value", "p0000001-0000-4000-8000-000000000001", "--json"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)

	var out map[string]any
	assert.NilError(t, json.Unmarshal([]byte(result.Stdout), &out))
	assert.Check(t, cmp.Equal(out["id"], testCreatedRestrictionID))
	assert.Check(t, cmp.Equal(out["restriction_type"], "project"))
	assert.Check(t, cmp.Equal(out["restriction_value"], "p0000001-0000-4000-8000-000000000001"))
}

func TestContextRestrictionCreate_MissingArg(t *testing.T) {
	env := testenv.New(t)
	env.Token = testToken

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"context", "restriction", "create", "--type", "project", "--value", "some-value"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 2) // ExitBadArguments
}

func TestContextRestrictionCreate_MissingType(t *testing.T) {
	env := testenv.New(t)
	env.Token = testToken

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"context", "restriction", "create", testContextID, "--value", "some-value"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 2) // ExitBadArguments
}

func TestContextRestrictionCreate_MissingValue(t *testing.T) {
	env := testenv.New(t)
	env.Token = testToken

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"context", "restriction", "create", testContextID, "--type", "project"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 2) // ExitBadArguments
}

func TestContextRestrictionCreate_InvalidType(t *testing.T) {
	fake := fakes.NewCircleCI(t)

	env := testenv.New(t)
	env.Token = testToken
	env.CircleCIURL = fake.URL()

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"context", "restriction", "create", testContextID, "--type", "invalid", "--value", "some-value"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 2) // ExitBadArguments
}

func TestContextRestrictionCreate_ByName(t *testing.T) {
	fake := fakes.NewCircleCI(t)
	fake.AddContext(testOrgSlug, fakeContext(testContextID, "my-context"))

	env := testenv.New(t)
	env.Token = testToken
	env.CircleCIURL = fake.URL()

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"context", "restriction", "create", "my-context", "--org", testOrgSlug, "--type", "project", "--value", "p0000001-0000-4000-8000-000000000001"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
}

// --- context restriction delete ---

func TestContextRestrictionDelete(t *testing.T) {
	fake := fakes.NewCircleCI(t)
	fake.AddContext(testOrgSlug, fakeContext(testContextID, "my-context"))
	fake.AddContextRestriction(testContextID, fakeContextRestriction(testContextID, testRestrictionID, "project", "p0000001-0000-4000-8000-000000000001", "myrepo"))

	env := testenv.New(t)
	env.Token = testToken
	env.CircleCIURL = fake.URL()

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"context", "restriction", "delete", testContextID, "--restriction-id", testRestrictionID, "--force"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))

	t.Run("check request", func(t *testing.T) {
		assert.Check(t, cmp.DeepEqual(fake.LastRequest(), &httprecorder.Request{
			Method: http.MethodDelete,
			URL:    url.URL{Path: "/api/v2/context/" + testContextID + "/restrictions/" + testRestrictionID},
			Header: http.Header{
				"Authorization": {"Bearer test-token"},
				"User-Agent":    {httpcl.UserAgent(runtime.GOOS, runtime.GOARCH, "dev", "")},
			},
			Body: new(""),
		}, ignoreCommonHeaders))
	})
}

func TestContextRestrictionDelete_Color(t *testing.T) {
	fake := fakes.NewCircleCI(t)
	fake.AddContext(testOrgSlug, fakeContext(testContextID, "my-context"))
	fake.AddContextRestriction(testContextID, fakeContextRestriction(testContextID, testRestrictionID, "project", "p0000001-0000-4000-8000-000000000001", "myrepo"))

	env := testenv.New(t)
	env.Token = testToken
	env.CircleCIURL = fake.URL()

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"context", "restriction", "delete", testContextID, "--restriction-id", testRestrictionID, "--force"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
		TTY:     true,
	})

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
}

func TestContextRestrictionDelete_RequiresForce(t *testing.T) {
	fake := fakes.NewCircleCI(t)
	fake.AddContext(testOrgSlug, fakeContext(testContextID, "my-context"))
	fake.AddContextRestriction(testContextID, fakeContextRestriction(testContextID, testRestrictionID, "project", "p0000001-0000-4000-8000-000000000001", "myrepo"))

	env := testenv.New(t)
	env.Token = testToken
	env.CircleCIURL = fake.URL()

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"context", "restriction", "delete", testContextID, "--restriction-id", testRestrictionID},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 6) // ExitCancelled
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))
}

func TestContextRestrictionDelete_NotFound(t *testing.T) {
	fake := fakes.NewCircleCI(t)
	fake.AddContext(testOrgSlug, fakeContext(testContextID, "my-context"))

	env := testenv.New(t)
	env.Token = testToken
	env.CircleCIURL = fake.URL()

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"context", "restriction", "delete", testContextID, "--restriction-id", testRestrictionID, "--force"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 5) // ExitNotFound
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))
}

func TestContextRestrictionDelete_MissingArg(t *testing.T) {
	env := testenv.New(t)
	env.Token = testToken

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"context", "restriction", "delete", "--restriction-id", testRestrictionID, "--force"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 2) // ExitBadArguments
}

func TestContextRestrictionDelete_ByName(t *testing.T) {
	fake := fakes.NewCircleCI(t)
	fake.AddContext(testOrgSlug, fakeContext(testContextID, "my-context"))
	fake.AddContextRestriction(testContextID, fakeContextRestriction(testContextID, testRestrictionID, "project", "p0000001-0000-4000-8000-000000000001", "myrepo"))

	env := testenv.New(t)
	env.Token = testToken
	env.CircleCIURL = fake.URL()

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"context", "restriction", "delete", "my-context", "--org", testOrgSlug, "--restriction-id", testRestrictionID, "--force"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
}
