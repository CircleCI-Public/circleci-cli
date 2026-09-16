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
	"testing"

	"gotest.tools/v3/assert"
	"gotest.tools/v3/assert/cmp"
	"gotest.tools/v3/golden"

	"github.com/CircleCI-Public/circleci-cli/internal/testing/binary"
	testenv "github.com/CircleCI-Public/circleci-cli/internal/testing/env"
	"github.com/CircleCI-Public/circleci-cli/internal/testing/fakes"
)

// setupOrgFake registers the orgs served by GET /api/v3/orgs: a VCS-backed one
// and a standalone CircleCI org, which has no VCS provider.
//
// The same two orgs are registered as collaborations, because that is where
// their slugs come from: GET /api/v3/orgs carries none. A standalone org's slug
// is circleci/<short id>, which is not derivable from its UUID.
func setupOrgFake(t *testing.T) (*fakes.CircleCI, *testenv.TestEnv) {
	t.Helper()
	fake := fakes.NewCircleCI(t)
	fake.AddOrg("a0000000-0000-4000-8000-0000000c0002", "gh/myorg", "myorg", "github")
	fake.AddOrg("a0000000-0000-4000-8000-0000000c0003", "circleci/2V3EWrY7hKnRnfkk8cm4Ai", "standalone-org", "")
	fake.SetCollaborations(
		fakes.Collaboration{
			ID: "a0000000-0000-4000-8000-0000000c0002", Name: "myorg",
			Slug: "gh/myorg", VCSType: "github",
		},
		fakes.Collaboration{
			ID: "a0000000-0000-4000-8000-0000000c0003", Name: "standalone-org",
			Slug: "circleci/2V3EWrY7hKnRnfkk8cm4Ai", VCSType: "circleci",
		},
	)

	env := testenv.New(t)
	env.Token = testToken
	env.CircleCIURL = fake.URL()

	return fake, env
}

func TestOrgList(t *testing.T) {
	_, env := setupOrgFake(t)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"org", "list"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))
}

func TestOrgList_JSON(t *testing.T) {
	_, env := setupOrgFake(t)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"org", "list", "--json"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)

	var out []map[string]any
	assert.NilError(t, json.Unmarshal([]byte(result.Stdout), &out))
	assert.Check(t, cmp.DeepEqual(out, []map[string]any{
		{
			"id":       "a0000000-0000-4000-8000-0000000c0002",
			"name":     "myorg",
			"slug":     "gh/myorg",
			"vcs_type": "github",
		},
		{
			"id":       "a0000000-0000-4000-8000-0000000c0003",
			"name":     "standalone-org",
			"slug":     "circleci/2V3EWrY7hKnRnfkk8cm4Ai",
			"vcs_type": "",
		},
	}))
}

// TestOrgList_NoSlugsAvailable covers the org list whose supplementary slug
// lookup came back with nothing: the listing is still the answer, so it prints
// with the slug omitted rather than failing.
func TestOrgList_NoSlugsAvailable(t *testing.T) {
	fake := fakes.NewCircleCI(t)
	fake.AddOrg("a0000000-0000-4000-8000-0000000c0002", "gh/myorg", "myorg", "github")

	env := testenv.New(t)
	env.Token = testToken
	env.CircleCIURL = fake.URL()

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"org", "list", "--json"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)

	var out []map[string]any
	assert.NilError(t, json.Unmarshal([]byte(result.Stdout), &out))
	assert.Check(t, cmp.DeepEqual(out, []map[string]any{
		{
			"id":       "a0000000-0000-4000-8000-0000000c0002",
			"name":     "myorg",
			"vcs_type": "github",
		},
	}))
}

func TestOrgList_Empty(t *testing.T) {
	fake := fakes.NewCircleCI(t)

	env := testenv.New(t)
	env.Token = testToken
	env.CircleCIURL = fake.URL()

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"org", "list"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	assert.Check(t, cmp.Contains(result.Stderr, "No organizations found"))
}
