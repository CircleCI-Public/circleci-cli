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
const testContextOrgID = "a0000001-0000-4000-8000-0000000000a1"
const testContextGroupID = "a0000002-0000-4000-8000-0000000000a2"
const testContextID = "c0000001-0000-4000-8000-000000000001"
const testContextID2 = "c0000002-0000-4000-8000-000000000002"
const testRestrictionID = "e0000001-0000-4000-8000-000000000001"
const testCreatedRestrictionID = "c0000003-0000-4000-8000-000000000003"

// contextFake returns a fake with the test org registered, so the CLI can
// resolve --org gh/testorg to the UUID the v3 context endpoints filter on.
func contextFake(t *testing.T) *fakes.CircleCI {
	t.Helper()
	fake := fakes.NewCircleCI(t)
	fake.AddOrg(testContextOrgID, testOrgSlug, "testorg", "github")
	return fake
}

func fakeContext(id, name string) fakes.Context {
	return fakes.Context{ID: id, Name: name, CreatedAt: "2020-01-01T12:00:00Z", OrgID: testContextOrgID}
}

func fakeContextEnvVar(contextID, variable string) fakes.ContextEnvVar {
	return fakes.ContextEnvVar{
		Variable:       variable,
		TruncatedValue: "abcd",
		ContextID:      contextID,
		CreatedAt:      "2020-01-01T12:00:00Z",
		UpdatedAt:      "2020-06-01T12:00:00Z",
	}
}

func fakeContextRestriction(contextID, id, restrictionType, matchPattern, name string) fakes.ContextRestriction {
	return fakes.ContextRestriction{
		ContextID:       contextID,
		ID:              id,
		RestrictionType: restrictionType,
		MatchPattern:    matchPattern,
		Name:            name,
	}
}

// --- context list ---

func TestContextList(t *testing.T) {
	fake := contextFake(t)
	fake.AddContext(testContextOrgID, fakeContext(testContextID, "my-context"))
	fake.AddContext(testContextOrgID, fakeContext(testContextID2, "other-context"))

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
	fake := contextFake(t)
	fake.AddContext(testContextOrgID, fakeContext(testContextID, "my-context"))
	fake.AddContext(testContextOrgID, fakeContext(testContextID2, "other-context"))

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
	fake := contextFake(t)
	fake.AddContext(testContextOrgID, fakeContext(testContextID, "my-context"))

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
	fake := contextFake(t)

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
	fake := contextFake(t)
	fake.AddContext(testContextOrgID, fakeContext(testContextID, "my-context"))
	fake.AddContext(testContextOrgID, fakeContext(testContextID2, "other-context"))

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
	fake := contextFake(t)
	fake.AddContext(testContextOrgID, fakeContext(testContextID, "my-context"))
	fake.AddContext(testContextOrgID, fakeContext(testContextID2, "other-context"))

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

// TestContextList_OrgUUID covers --org taking an org UUID directly, which skips
// the slug lookup the other tests exercise.
func TestContextList_OrgUUID(t *testing.T) {
	fake := contextFake(t)
	fake.AddContext(testContextOrgID, fakeContext(testContextID, "my-context"))

	env := testenv.New(t)
	env.Token = testToken
	env.CircleCIURL = fake.URL()

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"context", "list", "--org", testContextOrgID, "--json", "--jq", ".[0].name"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	assert.Check(t, cmp.Equal(strings.TrimSpace(result.Stdout), "my-context"))

	t.Run("resolves without an org lookup", func(t *testing.T) {
		for _, req := range fake.AllRequests() {
			assert.Check(t, req.URL.Path != "/api/v3/orgs",
				"a UUID needs no slug resolution, but %s was called", req.URL.Path)
		}
	})
}

// TestContextList_NameFilter pins --name reaching the API as filter[name].
// The match itself is the server's job — it applies upstream, so it narrows
// pagination too, which a local pass could not do.
func TestContextList_NameFilter(t *testing.T) {
	fake := contextFake(t)
	fake.AddContext(testContextOrgID, fakeContext(testContextID, "my-context"))
	fake.AddContext(testContextOrgID, fakeContext(testContextID2, "other-context"))

	env := testenv.New(t)
	env.Token = testToken
	env.CircleCIURL = fake.URL()

	result := binary.RunCLI(t, binary.RunOpts{
		Binary: binaryPath,
		// Upper-case, to pin the match as case-insensitive the way v2's was.
		Args:    []string{"context", "list", "--org", testOrgSlug, "--name", "MY-CONTEXT", "--json"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)

	var out []map[string]any
	assert.NilError(t, json.Unmarshal([]byte(result.Stdout), &out))
	assert.Check(t, cmp.Len(out, 1))
	assert.Check(t, cmp.Equal(out[0]["name"], "my-context"))

	t.Run("sends the filter", func(t *testing.T) {
		var listReq *httprecorder.Request
		reqs := fake.AllRequests()
		for i := range reqs {
			if reqs[i].URL.Path == "/api/v3/contexts" {
				listReq = &reqs[i]
			}
		}
		assert.Assert(t, listReq != nil)
		assert.Check(t, cmp.Equal(listReq.URL.Query().Get("filter[name]"), "MY-CONTEXT"))
	})
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
	fake := contextFake(t)
	fake.AddContext(testContextOrgID, fakeContext(testContextID, "my-context"))
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
	fake := contextFake(t)
	fake.AddContext(testContextOrgID, fakeContext(testContextID, "my-context"))
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

// TestContextGet_GroupRestriction covers the restriction list `context get`
// renders: every type comes from the restrictions endpoint, each carrying its
// resolved name, ordered group-then-project-then-expression.
func TestContextGet_GroupRestriction(t *testing.T) {
	fake := contextFake(t)
	fake.AddContext(testContextOrgID, fakeContext(testContextID, "my-context"))
	fake.AddContextRestriction(testContextID, fakeContextRestriction(
		testContextID, testContextGroupID, "group", testContextGroupID, "All members",
	))
	fake.AddContextRestriction(testContextID, fakeContextRestriction(
		testContextID, testRestrictionID,
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
	rs, ok := out["restrictions"].([]any)
	assert.Assert(t, ok)
	assert.Assert(t, cmp.Len(rs, 2))

	group := rs[0].(map[string]any)
	assert.Check(t, cmp.Equal(group["restriction_type"], "group"))
	assert.Check(t, cmp.Equal(group["name"], "All members"))
	assert.Check(t, cmp.Equal(group["id"], testContextGroupID))
	assert.Check(t, cmp.Equal(group["restriction_value"], testContextGroupID))

	project := rs[1].(map[string]any)
	assert.Check(t, cmp.Equal(project["restriction_type"], "project"))
	assert.Check(t, cmp.Equal(project["name"], "myrepo"))
	assert.Check(t, cmp.Equal(project["restriction_value"], "b0000001-0000-4000-8000-000000000001"))
}

// TestContextList_Forbidden covers a 403 on an org-scoped call: the token
// cannot see the organization, which is not a missing context, so it reports
// access denied and exits 4 rather than borrowing the not-found path.
func TestContextList_Forbidden(t *testing.T) {
	fake := contextFake(t)
	fake.ForbidOrgContexts(testContextOrgID)

	env := testenv.New(t)
	env.Token = testToken
	env.CircleCIURL = fake.URL()

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"context", "list", "--org", testOrgSlug},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Check(t, cmp.Equal(result.ExitCode, 4))
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))
}

// TestContextGet_ByName_SkipsRedundantFetch pins the saving that resolving by
// name buys: the name lookup already returns the whole context, so the by-id
// endpoint must not be called again. It is the slowest of the context reads, so
// re-fetching it would put ~0.5s back on the critical path.
func TestContextGet_ByName_SkipsRedundantFetch(t *testing.T) {
	fake := contextFake(t)
	fake.AddContext(testContextOrgID, fakeContext(testContextID, "my-context"))
	fake.AddContextRestriction(testContextID, fakeContextRestriction(
		testContextID, testContextGroupID, "group", testContextGroupID, "All members",
	))
	fake.AddContextEnvVar(testContextID, fakeContextEnvVar(testContextID, "DB_PASSWORD"))

	env := testenv.New(t)
	env.Token = testToken
	env.CircleCIURL = fake.URL()

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"context", "get", "my-context", "--org", testOrgSlug, "--json"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)

	t.Run("never fetches the context by id", func(t *testing.T) {
		byID := "/api/v3/contexts/" + testContextID
		for _, req := range fake.AllRequests() {
			assert.Check(t, req.URL.Path != byID,
				"by-name resolution already has the context; %s is a wasted round trip", req.URL.Path)
		}
	})

	t.Run("still renders everything, groups included", func(t *testing.T) {
		var out map[string]any
		assert.NilError(t, json.Unmarshal([]byte(result.Stdout), &out))
		// org_id comes from the resolved org, since the list response omits it.
		assert.Check(t, cmp.Equal(out["org_id"], testContextOrgID))
		assert.Check(t, cmp.Equal(out["name"], "my-context"))

		evs, ok := out["environment_variables"].([]any)
		assert.Assert(t, ok)
		assert.Check(t, cmp.Len(evs, 1))

		rs, ok := out["restrictions"].([]any)
		assert.Assert(t, ok)
		assert.Assert(t, cmp.Len(rs, 1))
		assert.Check(t, cmp.Equal(rs[0].(map[string]any)["name"], "All members"))
	})
}

func TestContextGet_NotFound(t *testing.T) {
	fake := contextFake(t)

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
	fake := contextFake(t)
	fake.AddContext(testContextOrgID, fakeContext(testContextID, "my-context"))
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
	fake := contextFake(t)
	fake.AddContext(testContextOrgID, fakeContext(testContextID, "my-context"))

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
	fake := contextFake(t)

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
			URL:    url.URL{Path: "/api/v3/contexts"},
			Header: http.Header{
				"Authorization": {"Bearer test-token"},
				"User-Agent":    {httpcl.UserAgent(runtime.GOOS, runtime.GOARCH, "dev", "chunk")},
			},
			Body: new(`{"data":{"attributes":{"name":"new-context"},` +
				`"references":{"org":{"id":"` + testContextOrgID + `"}}}}`),
		}, ignoreCommonHeaders))
	})
}

func TestContextCreate_JSON(t *testing.T) {
	fake := contextFake(t)

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
	fake := contextFake(t)
	fake.AddContext(testContextOrgID, fakeContext(testContextID, "my-context"))

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
			URL:    url.URL{Path: "/api/v3/contexts/" + testContextID},
			Header: http.Header{
				"Authorization": {"Bearer test-token"},
				"User-Agent":    {httpcl.UserAgent(runtime.GOOS, runtime.GOARCH, "dev", "")},
			},
			Body: new(""),
		}, ignoreCommonHeaders))
	})
}

func TestContextDelete_RequiresForce(t *testing.T) {
	fake := contextFake(t)
	fake.AddContext(testContextOrgID, fakeContext(testContextID, "my-context"))

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
	fake := contextFake(t)

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
	fake := contextFake(t)
	fake.AddContext(testContextOrgID, fakeContext(testContextID, "my-context"))

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
	fake := contextFake(t)
	fake.AddContext(testContextOrgID, fakeContext(testContextID, "my-context"))

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
	fake := contextFake(t)
	fake.AddContext(testContextOrgID, fakeContext(testContextID, "my-context"))
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
	fake := contextFake(t)
	fake.AddContext(testContextOrgID, fakeContext(testContextID, "my-context"))
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
	fake := contextFake(t)

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
	fake := contextFake(t)
	fake.AddContext(testContextOrgID, fakeContext(testContextID, "my-context"))
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
	fake := contextFake(t)
	fake.AddContext(testContextOrgID, fakeContext(testContextID, "my-context"))

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
			Method: http.MethodPost,
			URL:    url.URL{Path: "/api/v3/contexts/" + testContextID + "/env-vars/set"},
			Header: http.Header{
				"Authorization": {"Bearer test-token"},
				"User-Agent":    {httpcl.UserAgent(runtime.GOOS, runtime.GOARCH, "dev", "")},
			},
			Body: new(`{"name":"MY_VAR","value":"s3cr3t"}`),
		}, ignoreCommonHeaders))
	})
}

// TestContextSecretSet_Forbidden covers a by-id 403: the caller can see the
// context but lacks the permission to write to it — the read-only-member case,
// and the only thing a by-id 403 means, since a missing or cross-tenant context
// answers 404. So the message can say so plainly instead of falling through to
// the generic API error.
func TestContextSecretSet_Forbidden(t *testing.T) {
	fake := contextFake(t)
	fake.AddContext(testContextOrgID, fakeContext(testContextID, "my-context"))
	fake.ForbidContextAction(testContextID)

	env := testenv.New(t)
	env.Token = testToken
	env.CircleCIURL = fake.URL()

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"context", "secret", "set", testContextID, "--name", "MY_VAR", "--value", "s3cr3t"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Check(t, cmp.Equal(result.ExitCode, 4))
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))
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
	fake := contextFake(t)
	fake.AddContext(testContextOrgID, fakeContext(testContextID, "my-context"))

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
	fake := contextFake(t)
	fake.AddContext(testContextOrgID, fakeContext(testContextID, "my-context"))
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
			URL: url.URL{
				Path:     "/api/v3/contexts/" + testContextID + "/env-vars",
				RawQuery: "filter%5Bname%5D=MY_VAR",
			},
			Header: http.Header{
				"Authorization": {"Bearer test-token"},
				"User-Agent":    {httpcl.UserAgent(runtime.GOOS, runtime.GOARCH, "dev", "")},
			},
			Body: new(""),
		}, ignoreCommonHeaders))
	})
}

func TestContextSecretDelete_RequiresForce(t *testing.T) {
	fake := contextFake(t)
	fake.AddContext(testContextOrgID, fakeContext(testContextID, "my-context"))
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
	fake := contextFake(t)

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
	fake := contextFake(t)
	fake.AddContext(testContextOrgID, fakeContext(testContextID, "my-context"))
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
	fake := contextFake(t)
	fake.AddContext(testContextOrgID, fakeContext(testContextID, "my-context"))

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
			URL:    url.URL{Path: "/api/v3/context-restrictions"},
			Header: http.Header{
				"Authorization": {"Bearer test-token"},
				"User-Agent":    {httpcl.UserAgent(runtime.GOOS, runtime.GOARCH, "dev", "")},
			},
			Body: new(`{"data":{"attributes":{"match_pattern":"p0000001-0000-4000-8000-000000000001",` +
				`"restriction_type":"project"},"references":{"context":{"id":"` + testContextID + `"}}}}`),
		}, ignoreCommonHeaders))
	})
}

func TestContextRestrictionCreate_Color(t *testing.T) {
	fake := contextFake(t)
	fake.AddContext(testContextOrgID, fakeContext(testContextID, "my-context"))

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
	fake := contextFake(t)
	fake.AddContext(testContextOrgID, fakeContext(testContextID, "my-context"))

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

// TestContextRestrictionCreate_Group covers --type group. Groups are stored
// differently upstream, but the API hides that: the same endpoint takes a group
// id as its match pattern, so the CLI needs no special case.
func TestContextRestrictionCreate_Group(t *testing.T) {
	fake := contextFake(t)
	fake.AddContext(testContextOrgID, fakeContext(testContextID, "my-context"))

	env := testenv.New(t)
	env.Token = testToken
	env.CircleCIURL = fake.URL()

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"context", "restriction", "create", testContextID, "--type", "group", "--value", testContextGroupID, "--json"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)

	var out map[string]any
	assert.NilError(t, json.Unmarshal([]byte(result.Stdout), &out))
	assert.Check(t, cmp.Equal(out["restriction_type"], "group"))
	assert.Check(t, cmp.Equal(out["restriction_value"], testContextGroupID))
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
	fake := contextFake(t)

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
	fake := contextFake(t)
	fake.AddContext(testContextOrgID, fakeContext(testContextID, "my-context"))

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
	fake := contextFake(t)
	fake.AddContext(testContextOrgID, fakeContext(testContextID, "my-context"))
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
			URL: url.URL{
				Path:     "/api/v3/context-restrictions/" + testRestrictionID,
				RawQuery: "filter%5Bcontext_id%5D=" + testContextID,
			},
			Header: http.Header{
				"Authorization": {"Bearer test-token"},
				"User-Agent":    {httpcl.UserAgent(runtime.GOOS, runtime.GOARCH, "dev", "")},
			},
			Body: new(""),
		}, ignoreCommonHeaders))
	})
}

func TestContextRestrictionDelete_Color(t *testing.T) {
	fake := contextFake(t)
	fake.AddContext(testContextOrgID, fakeContext(testContextID, "my-context"))
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
	fake := contextFake(t)
	fake.AddContext(testContextOrgID, fakeContext(testContextID, "my-context"))
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
	fake := contextFake(t)
	fake.AddContext(testContextOrgID, fakeContext(testContextID, "my-context"))

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
	fake := contextFake(t)
	fake.AddContext(testContextOrgID, fakeContext(testContextID, "my-context"))
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
