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
	"io"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"net/url"
	"runtime"
	"strings"
	"sync/atomic"
	"testing"

	"gotest.tools/v3/assert"
	"gotest.tools/v3/assert/cmp"
	"gotest.tools/v3/golden"

	"github.com/CircleCI-Public/circleci-cli/internal/httpcl"
	"github.com/CircleCI-Public/circleci-cli/internal/testing/binary"
	testenv "github.com/CircleCI-Public/circleci-cli/internal/testing/env"
	"github.com/CircleCI-Public/circleci-cli/internal/testing/fakes"
	"github.com/CircleCI-Public/circleci-cli/internal/testing/httprecorder"
)

// `circleci run trigger` and `circleci pipeline run` share one implementation
// on the V3 runs API, so every test here runs against both.
var triggerCommands = []struct {
	name string
	args []string
}{
	{name: "run_trigger", args: []string{"run", "trigger"}},
	{name: "pipeline_run", args: []string{"pipeline", "run"}},
}

const (
	triggerSlug = "gh/myorg/myrepo"
	// triggerOneSlug names a project with a single pipeline, as an OAuth
	// project's synthetic pipeline is.
	triggerOneSlug      = "gh/myorg/oauthrepo"
	triggerOneProjectID = "a0000000-0000-4000-8000-0000000b0003"
	triggerOnePipeline  = "pdef-uuid-0009"
	triggerEmptySlug    = "gh/myorg/empty"
)

// setupRunTriggerFake serves three projects: triggerSlug with the two pipelines
// in pipelineListFixtures, triggerOneSlug with a single pipeline, and
// triggerEmptySlug with none.
func setupRunTriggerFake(t *testing.T) (*fakes.CircleCI, *testenv.TestEnv) {
	t.Helper()
	fake := fakes.NewCircleCI(t)
	fake.AddProjectBySlug(triggerSlug, pipelineProjectID, "myrepo", testOrgID)
	for _, d := range pipelineListFixtures {
		fake.AddPipelineDefinition(pipelineProjectID, d)
	}
	fake.AddProjectBySlug(triggerOneSlug, triggerOneProjectID, "oauthrepo", testOrgID)
	fake.AddPipelineDefinition(triggerOneProjectID, fakePipelineDefPayload(
		triggerOnePipeline, "oauthrepo", "github", pipelineRepoID, ".circleci/config.yml", "github", pipelineRepoID))
	fake.AddProjectBySlug(triggerEmptySlug, "a0000000-0000-4000-8000-0000000b0002", "empty", testOrgID)

	env := testenv.New(t)
	env.Token = testToken
	env.CircleCIURL = fake.URL()
	return fake, env
}

func execTrigger(t *testing.T, env *testenv.TestEnv, cmd []string, args ...string) binary.CLIResult {
	t.Helper()
	return binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    append(append([]string{}, cmd...), args...),
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})
}

// createRunRequest is the POST /api/v3/runs request the CLI should send.
func createRunRequest(projectID, body string) *httprecorder.Request {
	return &httprecorder.Request{
		Method: http.MethodPost,
		URL:    url.URL{Path: "/api/v3/runs", RawQuery: "filter%5Bproject_id%5D=" + projectID},
		Header: http.Header{
			"Authorization": {"Bearer test-token"},
			"User-Agent":    {httpcl.UserAgent(runtime.GOOS, runtime.GOARCH, "dev", "")},
		},
		Body: new(body),
	}
}

func TestTrigger_SinglePipeline(t *testing.T) {
	for _, c := range triggerCommands {
		t.Run(c.name, func(t *testing.T) {
			fake, env := setupRunTriggerFake(t)

			result := execTrigger(t, env, c.args, "--project", triggerOneSlug, "--branch", "main")

			assert.Check(t, cmp.Equal(result.ExitCode, 0), "stderr: %s", result.Stderr)
			assert.Check(t, golden.String(result.Stdout, "TestTrigger_SinglePipeline.txt"))
			assert.Check(t, golden.String(result.Stderr, "TestTrigger_SinglePipeline.stderr.txt"))

			t.Run("check request", func(t *testing.T) {
				assert.Check(t, cmp.DeepEqual(fake.LastRequest(), createRunRequest(triggerOneProjectID,
					`{"data":{"attributes":{"checkout":{"branch":"main"},"config":{"branch":"main"}},"references":{"pipeline":{"id":"`+triggerOnePipeline+`"}}}}`),
					ignoreCommonHeaders))
			})
		})
	}
}

func TestTrigger_JSON(t *testing.T) {
	for _, c := range triggerCommands {
		t.Run(c.name, func(t *testing.T) {
			_, env := setupRunTriggerFake(t)

			result := execTrigger(t, env, c.args, "--project", triggerOneSlug, "--tag", "v1.2.3", "--json")

			assert.Assert(t, cmp.Equal(result.ExitCode, 0), "stderr: %s", result.Stderr)
			var out map[string]any
			assert.NilError(t, json.Unmarshal([]byte(result.Stdout), &out))
			assert.Check(t, cmp.DeepEqual(out, map[string]any{
				"id":          fakes.CreatedRunID,
				"pipeline_id": triggerOnePipeline,
				"project_id":  triggerOneProjectID,
				"created_at":  "2024-06-01T00:00:00Z",
				"tag":         "v1.2.3",
			}))
			assert.Check(t, cmp.Equal(result.Stderr, ""))
		})
	}
}

func TestTrigger_RefRequired(t *testing.T) {
	// The runs API requires a ref. Outside a checkout of the project there is
	// none to infer, so the command fails before any API call.
	for _, c := range triggerCommands {
		t.Run(c.name, func(t *testing.T) {
			fake, env := setupRunTriggerFake(t)

			result := execTrigger(t, env, c.args, "--project", triggerOneSlug)

			assert.Check(t, cmp.Equal(result.ExitCode, 2))
			assert.Check(t, golden.String(result.Stderr, "TestTrigger_RefRequired.stderr.txt"))
			assert.Check(t, cmp.Nil(fake.LastRequest()), "no API call should be made")
		})
	}
}

func TestTrigger_DefinitionByName(t *testing.T) {
	for _, c := range triggerCommands {
		t.Run(c.name, func(t *testing.T) {
			fake, env := setupRunTriggerFake(t)

			result := execTrigger(t, env, c.args,
				"--project", triggerSlug, "--definition", "Nightly", "--branch", "main",
				"--param", "deploy_env=staging", "--param", "run_tests=true")

			assert.Check(t, cmp.Equal(result.ExitCode, 0), "stderr: %s", result.Stderr)
			assert.Check(t, cmp.DeepEqual(fake.LastRequest(), createRunRequest(pipelineProjectID,
				`{"data":{"attributes":{"checkout":{"branch":"main"},"config":{"branch":"main"},"parameters":{"deploy_env":"staging","run_tests":true}},"references":{"pipeline":{"id":"pdef-uuid-0002"}}}}`),
				ignoreCommonHeaders))
		})
	}
}

func TestTrigger_DefinitionID(t *testing.T) {
	for _, c := range triggerCommands {
		t.Run(c.name, func(t *testing.T) {
			fake, env := setupRunTriggerFake(t)

			result := execTrigger(t, env, c.args, "--project", triggerSlug, "--definition-id", pipelineDefID, "--branch", "main")

			assert.Check(t, cmp.Equal(result.ExitCode, 0), "stderr: %s", result.Stderr)
			assert.Check(t, cmp.DeepEqual(fake.LastRequest(), createRunRequest(pipelineProjectID,
				`{"data":{"attributes":{"checkout":{"branch":"main"},"config":{"branch":"main"}},"references":{"pipeline":{"id":"`+pipelineDefID+`"}}}}`),
				ignoreCommonHeaders))
		})
	}
}

func TestTrigger_DefinitionIDNotInProject(t *testing.T) {
	_, env := setupRunTriggerFake(t)

	result := execTrigger(t, env, []string{"run", "trigger"},
		"--project", triggerSlug, "--definition-id", triggerOnePipeline, "--branch", "main")

	assert.Check(t, cmp.Equal(result.ExitCode, 5))
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))
}

func TestTrigger_SeveralPipelinesNeedDefinition(t *testing.T) {
	for _, c := range triggerCommands {
		t.Run(c.name, func(t *testing.T) {
			_, env := setupRunTriggerFake(t)

			result := execTrigger(t, env, c.args, "--project", triggerSlug, "--branch", "main")

			assert.Check(t, cmp.Equal(result.ExitCode, 2))
			assert.Check(t, cmp.Equal(result.Stdout, ""))
			assert.Check(t, golden.String(result.Stderr, "TestTrigger_SeveralPipelinesNeedDefinition.stderr.txt"))
		})
	}
}

func TestTrigger_DefinitionNameErrors(t *testing.T) {
	cases := []struct {
		name     string
		slug     string
		def      string
		exitCode int
	}{
		{name: "not_found", slug: triggerSlug, def: "weekly", exitCode: 5},
		{name: "no_definitions", slug: triggerEmptySlug, def: "nightly", exitCode: 5},
		{name: "ambiguous", slug: triggerSlug, def: "Nightly", exitCode: 2},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fake, env := setupRunTriggerFake(t)
			if tc.name == "ambiguous" {
				fake.AddPipelineDefinition(pipelineProjectID, fakePipelineDefPayload(
					"pdef-uuid-0003", "NIGHTLY", "github_app", pipelineRepoID, ".circleci/config.yml", "github_app", pipelineRepoID))
			}

			result := execTrigger(t, env, []string{"run", "trigger"}, "--project", tc.slug, "--definition", tc.def, "--branch", "main")

			assert.Check(t, cmp.Equal(result.ExitCode, tc.exitCode))
			assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))
		})
	}
}

func TestTrigger_ProjectNotFound(t *testing.T) {
	_, env := setupRunTriggerFake(t)

	result := execTrigger(t, env, []string{"run", "trigger"}, "--project", "gh/myorg/typo", "--branch", "main")

	assert.Check(t, cmp.Equal(result.ExitCode, 5))
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))
}

func TestTrigger_InvalidArgs(t *testing.T) {
	// Bad input is reported before any API call.
	cases := []struct {
		name string
		args []string
	}{
		{name: "branch_and_tag", args: []string{"--branch", "main", "--tag", "v1"}},
		{name: "definition_and_id", args: []string{"--definition", "nightly", "--definition-id", pipelineDefID}},
		{name: "bad_param", args: []string{"--param", "notakeyvalue"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fake, env := setupRunTriggerFake(t)

			result := execTrigger(t, env, []string{"pipeline", "run"}, append([]string{"--project", triggerSlug}, tc.args...)...)

			assert.Check(t, cmp.Equal(result.ExitCode, 2))
			assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))
			assert.Check(t, cmp.Nil(fake.LastRequest()), "no API call should be made")
		})
	}
}

func TestTrigger_EmptyDefinitionIDIgnored(t *testing.T) {
	// MCP calls fill every flag, so an empty --definition-id must not clash
	// with --definition.
	fake, env := setupRunTriggerFake(t)

	result := execTrigger(t, env, []string{"pipeline", "run"},
		"--project", triggerSlug, "--definition", "nightly", "--definition-id", "", "--branch", "main")

	assert.Check(t, cmp.Equal(result.ExitCode, 0), "stderr: %s", result.Stderr)
	assert.Check(t, cmp.Contains(*fake.LastRequest().Body, `"pipeline":{"id":"pdef-uuid-0002"}`))
}

func TestRunTrigger_DeprecatedParameterFlag(t *testing.T) {
	fake, env := setupRunTriggerFake(t)

	result := execTrigger(t, env, []string{"run", "trigger"},
		"--project", triggerOneSlug, "--parameter", "deploy_env=staging", "--branch", "main")

	assert.Check(t, cmp.Equal(result.ExitCode, 0), "stderr: %s", result.Stderr)
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))
	assert.Check(t, cmp.Contains(*fake.LastRequest().Body, `"parameters":{"deploy_env":"staging"}`))
}

func TestTrigger_NoToken(t *testing.T) {
	env := testenv.New(t)

	result := execTrigger(t, env, []string{"run", "trigger"}, "--project", triggerSlug, "--branch", "main")

	assert.Check(t, cmp.Equal(result.ExitCode, 3))
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))
}

func TestTrigger_Interactive_SelectDefinition(t *testing.T) {
	fake, env := setupRunTriggerFake(t)

	console := binary.RunCLIInteractive(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"pipeline", "run", "--project", triggerSlug},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Assert(t, t.Run("enter a branch", func(t *testing.T) {
		_, err := console.ExpectString("Branch to run")
		assert.NilError(t, err)
		_, err = console.Send("feature/x\r")
		assert.NilError(t, err)
	}))
	assert.Assert(t, t.Run("select the second definition", func(t *testing.T) {
		_, err := console.ExpectString("Select a pipeline definition")
		assert.NilError(t, err)
		_, err = console.Send(keyDown + "\r")
		assert.NilError(t, err)
	}))
	t.Run("run is triggered", func(t *testing.T) {
		_, err := console.ExpectString("Triggered run " + fakes.CreatedRunID)
		assert.NilError(t, err)
		// The pty stays open until cleanup, so wait for the last line rather
		// than EOF.
		_, err = console.ExpectString("Watch it with: circleci run watch " + fakes.CreatedRunID)
		assert.NilError(t, err)
		assert.Check(t, cmp.Contains(*fake.LastRequest().Body, `"pipeline":{"id":"pdef-uuid-0002"}`))
		assert.Check(t, cmp.Contains(*fake.LastRequest().Body, `"checkout":{"branch":"feature/x"}`))
	})
}

// dropRunCreation stands in front of fake: it closes the connection on the
// POST that creates a run without answering, as if the response were lost, and
// passes every other request through. It returns the URL to point the CLI at
// and a count of the POSTs it dropped.
func dropRunCreation(t *testing.T, fake *fakes.CircleCI) (string, *atomic.Int32) {
	t.Helper()
	target, err := url.Parse(fake.URL())
	assert.NilError(t, err)
	proxy := httputil.NewSingleHostReverseProxy(target)

	var posts atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost && r.URL.Path == "/api/v3/runs" {
			_, _ = io.Copy(io.Discard, r.Body)
			posts.Add(1)
			panic(http.ErrAbortHandler) // close the connection without answering
		}
		proxy.ServeHTTP(w, r)
	}))
	t.Cleanup(srv.Close)
	return srv.URL, &posts
}

// A trigger whose response is lost may still have started a run, so the error
// says so rather than leaving the caller to trigger it again blindly.
func TestTrigger_ResponseLost(t *testing.T) {
	for _, c := range triggerCommands {
		t.Run(c.name, func(t *testing.T) {
			fake, env := setupRunTriggerFake(t)
			srvURL, posts := dropRunCreation(t, fake)
			env.CircleCIURL = srvURL

			result := execTrigger(t, env, c.args, "--project", triggerOneSlug, "--branch", "main")

			assert.Check(t, cmp.Equal(posts.Load(), int32(1)), "the trigger must be sent exactly once")
			assert.Check(t, cmp.Equal(result.ExitCode, 4)) // ExitAPIError
			assert.Check(t, golden.String(result.Stdout, "TestTrigger_ResponseLost.txt"))
			stderr := strings.ReplaceAll(result.Stderr, srvURL, "<SERVER>")
			assert.Check(t, golden.String(stderr, "TestTrigger_ResponseLost.stderr.txt"))
		})
	}
}

// With --json the lost-response error carries its own code, so a script or
// agent can tell it apart without matching the message.
func TestTrigger_ResponseLost_JSON(t *testing.T) {
	for _, c := range triggerCommands {
		t.Run(c.name, func(t *testing.T) {
			fake, env := setupRunTriggerFake(t)
			srvURL, _ := dropRunCreation(t, fake)
			env.CircleCIURL = srvURL

			result := execTrigger(t, env, c.args, "--project", triggerOneSlug, "--branch", "main", "--json")

			assert.Check(t, cmp.Equal(result.ExitCode, 4)) // ExitAPIError
			assert.Check(t, cmp.Equal(result.Stdout, ""))

			var errOut map[string]any
			assert.NilError(t, json.Unmarshal([]byte(result.Stderr), &errOut), "stderr should be JSON: %s", result.Stderr)
			assert.Check(t, cmp.DeepEqual(errOut, map[string]any{
				"error": true,
				"code":  "api.outcome_unknown",
				"message": `Post "` + srvURL + `/api/v3/runs?filter%5Bproject_id%5D=` + triggerOneProjectID + `": EOF` +
					"\nNo response arrived after the request was sent, so CircleCI may already have acted on it.",
				"exit_code": float64(4),
				"suggestions": []any{
					"Check whether the request took effect before you send it again",
				},
			}))
		})
	}
}
