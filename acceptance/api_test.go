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
	"context"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/segmentio/analytics-go/v3"
	"github.com/shirou/gopsutil/v4/host"
	"gotest.tools/v3/assert"
	"gotest.tools/v3/assert/cmp"
	"gotest.tools/v3/golden"
	"gotest.tools/v3/poll"

	"github.com/CircleCI-Public/circleci-cli/internal/config"
	"github.com/CircleCI-Public/circleci-cli/internal/iostream"
	"github.com/CircleCI-Public/circleci-cli/internal/telemetry"
	"github.com/CircleCI-Public/circleci-cli/internal/testing/binary"
	testenv "github.com/CircleCI-Public/circleci-cli/internal/testing/env"
	"github.com/CircleCI-Public/circleci-cli/internal/testing/fakesegment"
	"github.com/CircleCI-Public/circleci-cli/internal/testing/fakes"
)

// IDs used by the {project-id}/{org-id} substitution tests.
const (
	apiProjectID = "a0000000-0000-4000-8000-0000000a0001"
	apiOrgID     = "a0000000-0000-4000-8000-0000000a0002"
)

// writeInfoYML writes .circleci/info.yml into dir so that gitremote.Detect
// resolves the given slug without needing a real git checkout. This is what
// the {project-id}/{org-id} placeholder resolution relies on.
func writeInfoYML(t *testing.T, dir, slug string) {
	t.Helper()
	ccDir := filepath.Join(dir, ".circleci")
	assert.NilError(t, os.MkdirAll(ccDir, 0o755))
	body := "project:\n  slug: " + slug + "\n"
	assert.NilError(t, os.WriteFile(filepath.Join(ccDir, "info.yml"), []byte(body), 0o644))
}

func TestAPI_Get(t *testing.T) {
	fake := fakes.NewCircleCI(t)
	fake.AddRunV3(getRunID, runTestProjectID,
		fakeRunV3(getRunID, runTestProjectID, "ended", "succeeded", "main", "abc1234def5678"))

	env := testenv.New(t)
	env.Token = testToken
	env.CircleCIURL = fake.URL()

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"api", "api/v3/runs/" + getRunID},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Check(t, cmp.Equal(result.ExitCode, 0))
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))
}

func TestAPI_Get_JQ(t *testing.T) {
	fake := fakes.NewCircleCI(t)
	fake.AddRunV3(getRunID, runTestProjectID,
		fakeRunV3(getRunID, runTestProjectID, "ended", "succeeded", "main", "abc1234def5678"))

	env := testenv.New(t)
	env.Token = testToken
	env.CircleCIURL = fake.URL()

	// V3 wraps the resource in {"data": ...}, so the id lives at .data.id.
	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"api", "--jq", ".data.id", "api/v3/runs/" + getRunID},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Check(t, cmp.Equal(result.ExitCode, 0))
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))
}

func TestAPI_Get_Color(t *testing.T) {
	fake := fakes.NewCircleCI(t)
	fake.AddRunV3(getRunID, runTestProjectID,
		fakeRunV3(getRunID, runTestProjectID, "ended", "succeeded", "main", "abc1234def5678"))

	env := testenv.New(t)
	env.Token = testToken
	env.CircleCIURL = fake.URL()

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"api", "api/v3/runs/" + getRunID},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
		TTY:     true,
	})

	assert.Check(t, cmp.Equal(result.ExitCode, 0))
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))
}

func TestAPI_RawBody(t *testing.T) {
	fake := fakes.NewCircleCI(t)

	env := testenv.New(t)
	env.Token = testToken
	env.CircleCIURL = fake.URL()

	const body = `{"scope":{"project_ids":["` + runTestProjectID + `"]}}`

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"api", "-X", "POST", "api/v3/runs/search", "-d", body},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Check(t, cmp.Equal(result.ExitCode, 0))
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))

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

	env := testenv.New(t)
	env.Token = testToken
	env.CircleCIURL = fake.URL()

	// No -X: providing -d should default the method to POST.
	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"api", "api/v3/runs/search", "-d", `{"scope":{"project_ids":[]}}`},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Check(t, cmp.Equal(result.ExitCode, 0))
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))

	assert.Equal(t, fake.LastRequest().Method, "POST")
}

func TestAPI_RawBody_FromFile(t *testing.T) {
	fake := fakes.NewCircleCI(t)

	env := testenv.New(t)
	env.Token = testToken
	env.CircleCIURL = fake.URL()

	dir := t.TempDir()
	const body = `{"scope":{"project_ids":[]},"page":{"limit":5}}`
	path := filepath.Join(dir, "body.json")
	assert.NilError(t, os.WriteFile(path, []byte(body), 0o600))

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"api", "-X", "POST", "api/v3/runs/search", "-d", "@" + path},
		Env:     env.Environ(),
		WorkDir: dir,
	})

	assert.Check(t, cmp.Equal(result.ExitCode, 0))
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))

	assert.Equal(t, *fake.LastRequest().Body, body)
}

func TestAPI_RawBody_FromStdin(t *testing.T) {
	fake := fakes.NewCircleCI(t)

	env := testenv.New(t)
	env.Token = testToken
	env.CircleCIURL = fake.URL()

	const body = `{"scope":{"project_ids":[]},"page":{"limit":1}}`

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"api", "-X", "POST", "api/v3/runs/search", "-d", "@-"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
		Stdin:   strings.NewReader(body),
	})

	assert.Check(t, cmp.Equal(result.ExitCode, 0))
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))

	assert.Equal(t, *fake.LastRequest().Body, body)
}

func TestAPI_RawBody_InvalidJSON(t *testing.T) {
	env := testenv.New(t)
	env.Token = testToken

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"api", "-X", "POST", "api/v3/runs/search", "-d", "not json"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Check(t, cmp.Equal(result.ExitCode, 2)) // ExitBadArguments
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))
}

func TestAPI_RawBody_ConflictsWithFields(t *testing.T) {
	env := testenv.New(t)
	env.Token = testToken

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"api", "-X", "POST", "api/v3/runs/search", "-d", `{}`, "-f", "limit=5"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Check(t, cmp.Equal(result.ExitCode, 2)) // ExitBadArguments
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))
}

func TestAPI_NotFound(t *testing.T) {
	fake := fakes.NewCircleCI(t)

	env := testenv.New(t)
	env.Token = testToken
	env.CircleCIURL = fake.URL()

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"api", "api/v3/runs/does-not-exist"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Check(t, cmp.Equal(result.ExitCode, 4)) // ExitAPIError
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))
}

func TestAPI_NoToken(t *testing.T) {
	env := testenv.New(t)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"api", "api/v3/users"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Check(t, cmp.Equal(result.ExitCode, 3)) // ExitAuthError
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))
}

// A path with no version prefix is routed to /api/v3 by default.
func TestAPI_PathDefaultsToV3(t *testing.T) {
	fake := fakes.NewCircleCI(t)
	fake.AddRunV3(getRunID, runTestProjectID,
		fakeRunV3(getRunID, runTestProjectID, "ended", "succeeded", "main", "abc1234def5678"))

	env := testenv.New(t)
	env.Token = testToken
	env.CircleCIURL = fake.URL()

	// "runs/<id>" has no version prefix, so it must resolve to /api/v3/runs/<id>.
	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"api", "runs/" + getRunID},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Check(t, cmp.Equal(result.ExitCode, 0))
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))

	got := fake.LastRequest()
	assert.Assert(t, got != nil)
	assert.Equal(t, got.URL.Path, "/api/v3/runs/"+getRunID)
}

// {project-id} is replaced with the project UUID resolved from the current
// repository, then used against the V3 GET /api/v3/projects/{id} endpoint.
func TestAPI_ProjectIDSubstitution(t *testing.T) {
	fake := fakes.NewCircleCI(t)
	// Slug → UUID resolution (the CLI's internal lookup).
	fake.AddProjectBySlug(testSlug, apiProjectID, "testrepo", apiOrgID)
	// The V3 project the substituted path actually targets.
	fake.AddProjectV3(apiProjectID, map[string]any{
		"id":         apiProjectID,
		"attributes": map[string]any{"name": "testrepo", "slug": testSlug},
	})

	env := testenv.New(t)
	env.Token = testToken
	env.CircleCIURL = fake.URL()

	dir := t.TempDir()
	writeInfoYML(t, dir, testSlug)

	// No version prefix → defaults to /api/v3/projects/<resolved-uuid>.
	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"api", "projects/{project-id}"},
		Env:     env.Environ(),
		WorkDir: dir,
	})

	assert.Check(t, cmp.Equal(result.ExitCode, 0))
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))

	// {project-id} must have been substituted into the outgoing request path.
	got := fake.LastRequest()
	assert.Assert(t, got != nil)
	assert.Equal(t, got.URL.Path, "/api/v3/projects/"+apiProjectID)
}

// When a placeholder is used but no project can be detected (no git remote and
// no info.yml), the command fails with a bad-arguments exit code.
func TestAPI_ProjectIDSubstitution_NoProject(t *testing.T) {
	fake := fakes.NewCircleCI(t)

	env := testenv.New(t)
	env.Token = testToken
	env.CircleCIURL = fake.URL()

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"api", "projects/{project-id}"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(), // empty dir → no git remote, no info.yml
	})

	assert.Check(t, cmp.Equal(result.ExitCode, 2)) // ExitBadArguments
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
	// The git-detect failure embeds git's own (version/platform-dependent)
	// message, so match a stable substring rather than golden the whole thing.
	assert.Check(t, cmp.Contains(result.Stderr, "could not read git remote"))
}

// TestAPI_Telemetry verifies that a circleci api call emits a command_invocation
// event with the api_path property set to the raw path the user typed.
func TestAPI_Telemetry(t *testing.T) {
	ctx := iostream.Testing(context.Background())

	fake := fakes.NewCircleCI(t)
	fake.AddRunV3(getRunID, runTestProjectID,
		fakeRunV3(getRunID, runTestProjectID, "ended", "succeeded", "main", "abc1234def5678"))

	env := testenv.New(t)
	env.Token = testToken
	env.CircleCIURL = fake.URL()
	env.Telemetry = true
	fs := fakesegment.New(ctx, telemetry.SegmentKey)
	fsSrv := httptest.NewServer(fs)
	t.Cleanup(fsSrv.Close)
	env.Extra["CIRCLE_TELEMETRY_ENDPOINT"] = fsSrv.URL

	const apiPath = "api/v3/runs/" + getRunID

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"api", apiPath},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})
	assert.Check(t, cmp.Equal(result.ExitCode, 0))

	cfg, err := config.Load(ctx, env.ConfigDir()+"/circleci/config.yml", false)
	assert.NilError(t, err)

	hostInfo, err := host.Info()
	assert.NilError(t, err)

	poll.WaitOn(t, func(t poll.LogT) poll.Result {
		batches := fs.Batches()
		now := time.Now()
		return poll.Compare(cmp.DeepEqual(batches, []fakesegment.Batch{
			{
				SentAt: now,
				Messages: []analytics.Track{
					{
						Type:      "track",
						MessageId: "ignored",
						Timestamp: now,
						UserId:    telemetry.AnonymousID.String(),
						Event:     "command_invocation",
						Properties: analytics.Properties{
							"command":  "circleci api",
							"flags":    "debug,insecure-storage,theme",
							"api_path": apiPath,
						},
						Context: &analytics.Context{
							App: analytics.AppInfo{Name: "circleci-cli", Version: "dev"},
							Device: analytics.DeviceInfo{
								Id:    cfg.DeviceID().String(),
								Model: hostInfo.KernelArch,
								Type:  hostInfo.PlatformFamily,
							},
							OS: analytics.OSInfo{Name: hostInfo.OS, Version: hostInfo.PlatformVersion},
							Traits: map[string]any{
								"agent":          "",
								"is_self_hosted": true, // fake server URL is not https://circleci.com
								"is_tty":         false,
							},
						},
						Integrations: analytics.NewIntegrations().Enable("Amplitude"),
					},
				},
			},
		}, fakesegment.CompareTrack, fakesegment.CompareTime))
	})
}

// When the project lookup itself fails (slug resolves but the API has no such
// project), the command surfaces an API error.
func TestAPI_ProjectIDSubstitution_LookupFails(t *testing.T) {
	fake := fakes.NewCircleCI(t)
	// Deliberately register no project info, so the slug→UUID lookup 404s.

	env := testenv.New(t)
	env.Token = testToken
	env.CircleCIURL = fake.URL()

	dir := t.TempDir()
	writeInfoYML(t, dir, testSlug)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"api", "projects/{project-id}"},
		Env:     env.Environ(),
		WorkDir: dir,
	})

	assert.Check(t, cmp.Equal(result.ExitCode, 4)) // ExitAPIError
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))
}
