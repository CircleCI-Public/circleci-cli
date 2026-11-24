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
	"strings"
	"testing"

	"gotest.tools/v3/assert"
	"gotest.tools/v3/assert/cmp"
	"gotest.tools/v3/golden"

	"github.com/CircleCI-Public/circleci-cli/internal/testing/binary"
	testenv "github.com/CircleCI-Public/circleci-cli/internal/testing/env"
	"github.com/CircleCI-Public/circleci-cli/internal/testing/fakes"
)

const (
	forgejoRemote = "https://forgejo.example.com/acme/web.git"
	forgejoURL    = "https://forgejo.example.com"
	forgejoRepoID = "12345"
	forgejoSecret = "whsec-shown-once"
	setupPath     = "/api/v3/provider/connections/setup"
)

// forgejoFlags are the values that connect Forgejo without a prompt.
var forgejoFlags = []string{
	"--provider", "forgejo",
	"--connection-audience", "u:1:conn",
	"--checkout-audience", "u:1:checkout",
}

// forgejoRepo is a checkout on a self-hosted Forgejo whose org can see acme/web
// once it is connected.
func forgejoRepo(t *testing.T) (string, *fakes.CircleCI, *testenv.TestEnv) {
	t.Helper()
	dir, fake, env := onboardRepoWithRemote(t, forgejoRemote)
	fake.AddProviderRepository(onboardOrgID, fakes.ProviderRepo{
		ID:           forgejoRepoID,
		RepoFullName: "acme/web",
		RepoName:     "web",
		Owner:        "acme",
		Provider:     "forgejo",
	})
	addFirstPipelineResponses(fake)
	return dir, fake, env
}

// forgejoConfigured is the 201 a successful Forgejo setup answers with.
func forgejoConfigured() fakes.SetupResponse {
	return fakes.SetupResponse{
		Status: http.StatusCreated,
		Body: map[string]any{"data": map[string]any{
			"id": "conn-forgejo-1",
			"attributes": map[string]any{
				"next_step": "configure",
				"owner":     map[string]any{"login": "acme", "id": "3", "type": "organization"},
				"webhook": map[string]any{
					"url":    "https://internal.circleci.com/private/soc/forgejo/webhook/conn-forgejo-1",
					"secret": forgejoSecret,
					"events": []string{"push", "pull_request"},
				},
			},
		}},
	}
}

// forgejoSection is the part of onboard's stdout the Forgejo connection prints.
// The rest carries a temporary directory and the fake's port.
func forgejoSection(stdout string) string {
	_, section, _ := strings.Cut(stdout, "Connect Forgejo to CircleCI")
	section, _, _ = strings.Cut(section, "✓ Found repository")
	return "Connect Forgejo to CircleCI" + section
}

// requestBodies returns the bodies of every POST to path, in order.
func requestBodies(fake *fakes.CircleCI, path string) []string {
	var bodies []string
	for _, req := range fake.AllRequests() {
		if req.Method == http.MethodPost && req.URL.Path == path && req.Body != nil {
			bodies = append(bodies, *req.Body)
		}
	}
	return bodies
}

func TestOnboard_Forgejo_AllFlags(t *testing.T) {
	dir, fake, env := forgejoRepo(t)
	fake.QueueProviderConnectionSetupResponses(forgejoConfigured())

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    append([]string{"onboard", "--scan", "-H", "X-Gateway: tok"}, forgejoFlags...),
		Env:     env.Environ(),
		WorkDir: dir,
	})

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	assert.Check(t, golden.String(forgejoSection(result.Stdout), t.Name()+".txt"))

	t.Run("sends the setup values, defaulting url and owner from the remote", func(t *testing.T) {
		bodies := requestBodies(fake, setupPath)
		assert.Assert(t, cmp.Len(bodies, 1))
		var body map[string]any
		err := json.Unmarshal([]byte(bodies[0]), &body)
		assert.NilError(t, err)
		assert.Check(t, cmp.DeepEqual(body, map[string]any{
			"type": "vcs",
			"vcs": map[string]any{
				"provider":            "forgejo",
				"url":                 forgejoURL,
				"owner":               "acme",
				"connection_audience": "u:1:conn",
				"checkout_audience":   "u:1:checkout",
				"request_headers":     map[string]any{"X-Gateway": "tok"},
			},
		}))
	})

	t.Run("shows the webhook and its secret", func(t *testing.T) {
		assert.Check(t, cmp.Contains(result.Stdout, "Secret (save this — it will not be shown again):\n"+forgejoSecret))
		assert.Check(t, cmp.Contains(result.Stdout, "https://internal.circleci.com/private/soc/forgejo/webhook/conn-forgejo-1"))
	})

	t.Run("sets the pipeline up on forgejo", func(t *testing.T) {
		defs := requestBodies(fake, "/api/v3/pipelines")
		assert.Assert(t, cmp.Len(defs, 1))
		assert.Check(t, cmp.Contains(defs[0], `"provider":"forgejo"`))
		assert.Check(t, cmp.Contains(defs[0], `"repo_id":"`+forgejoRepoID+`"`))
		assert.Check(t, cmp.Contains(defs[0], `"repo_full_name":"acme/web"`))
		assert.Check(t, cmp.Contains(result.Stdout, "Trigger created: all-pushes"))
	})
}

// An org already connected to the remote's instance is recognised without
// --provider, and set up without connecting again.
func TestOnboard_Forgejo_ExistingConnection(t *testing.T) {
	dir, fake, env := forgejoRepo(t)
	fake.SetProviderConnectionURL(onboardOrgID, "forgejo", forgejoURL)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"onboard", "--scan"},
		Env:     env.Environ(),
		WorkDir: dir,
	})

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	assert.Check(t, cmp.Contains(result.Stdout, "Using this organization's Forgejo connection to "+forgejoURL))
	assert.Check(t, cmp.Contains(result.Stdout, "Found repository acme/web"))
	assert.Check(t, cmp.Contains(result.Stdout, "Trigger created: all-pushes"))
	assert.Check(t, cmp.Len(requestBodies(fake, setupPath), 0), "an existing connection must not be set up again")
}

// A remote on another instance than the org's connection is not connected.
func TestOnboard_Forgejo_ConnectedToAnotherInstance(t *testing.T) {
	dir, fake, env := forgejoRepo(t)
	fake.SetProviderConnectionURL(onboardOrgID, "forgejo", "https://code.example.org")

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    append([]string{"onboard", "--scan"}, forgejoFlags...),
		Env:     env.Environ(),
		WorkDir: dir,
	})

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	assert.Check(t, cmp.Contains(result.Stderr, "already connected to the Forgejo instance at https://code.example.org"))
	assert.Check(t, cmp.Len(requestBodies(fake, setupPath), 0))
	assert.Check(t, cmp.Len(requestBodies(fake, "/api/v3/pipelines"), 0))
}

func TestOnboard_Forgejo_RejectedValue(t *testing.T) {
	dir, fake, env := forgejoRepo(t)
	fake.QueueProviderConnectionSetupResponses(fakes.SetupResponse{
		Status: http.StatusBadRequest,
		Body: map[string]any{"error": map[string]any{
			"title":  "Invalid Input",
			"detail": "the checkout integration lacks scope read:repository",
		}},
	})

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    append([]string{"onboard", "--scan"}, forgejoFlags...),
		Env:     env.Environ(),
		WorkDir: dir,
	})

	assert.Check(t, cmp.Equal(result.ExitCode, 4), "stderr: %s", result.Stderr)
	assert.Check(t, cmp.Contains(result.Stderr, "the checkout integration lacks scope read:repository"))
	assert.Check(t, cmp.Contains(result.Stderr, "--checkout-audience"))
	assert.Check(t, cmp.Len(requestBodies(fake, "/api/v3/pipelines"), 0))
}

// Without a TTY a missing audience cannot be prompted for, so the steps that
// produce it are printed and the error names the flags.
func TestOnboard_Forgejo_MissingAudiences(t *testing.T) {
	dir, fake, env := forgejoRepo(t)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"onboard", "--scan", "--provider", "forgejo"},
		Env:     env.Environ(),
		WorkDir: dir,
	})

	assert.Check(t, cmp.Equal(result.ExitCode, 2), "stderr: %s", result.Stderr)
	assert.Check(t, golden.String(forgejoSection(result.Stdout), t.Name()+".txt"))
	assert.Check(t, cmp.Contains(result.Stderr, "needs --connection-audience and --checkout-audience"))
	assert.Check(t, cmp.Len(requestBodies(fake, setupPath), 0))
}

func TestOnboard_Forgejo_InvalidProvider(t *testing.T) {
	dir, fake, env := forgejoRepo(t)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"onboard", "--scan", "--provider", "github_app"},
		Env:     env.Environ(),
		WorkDir: dir,
	})

	assert.Check(t, cmp.Equal(result.ExitCode, 2))
	assert.Check(t, cmp.Contains(result.Stderr, `"github_app" is not a provider onboard can connect`))
	assert.Check(t, cmp.Contains(result.Stderr, "Valid values: forgejo"))
	assert.Check(t, cmp.Len(fake.AllRequests(), 0), "a bad flag must fail before any request")
}

func TestOnboard_UnclaimedHost_Interactive_Declines(t *testing.T) {
	dir, fake, env := onboardRepoWithRemote(t, "https://gitlab.com/myorg/my-repo.git")

	console := binary.RunCLIInteractive(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"onboard", "--scan"},
		Env:     env.Environ(),
		WorkDir: dir,
	})

	assert.Assert(t, t.Run("reaches the provider picker", func(t *testing.T) {
		for _, step := range []string{"Press Enter to continue", "Project name"} {
			_, err := console.ExpectString(step)
			assert.NilError(t, err)
			_, err = console.Send("\r")
			assert.NilError(t, err)
		}
		_, err := console.ExpectString("Which provider hosts myorg/my-repo?")
		assert.NilError(t, err)
	}))

	assert.Assert(t, t.Run("Enter takes none of these", func(t *testing.T) {
		_, err := console.Send("\r")
		assert.NilError(t, err)
		_, err = console.ExpectString("is not on a provider CircleCI can look up")
		assert.NilError(t, err)
		_, err = console.ExpectString("--provider forgejo")
		assert.NilError(t, err)
	}))

	assert.Check(t, cmp.Len(requestBodies(fake, setupPath), 0))
}

func TestOnboard_Forgejo_Interactive(t *testing.T) {
	dir, fake, env := forgejoRepo(t)
	fake.QueueProviderConnectionSetupResponses(forgejoConfigured())

	console := binary.RunCLIInteractive(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"onboard", "--scan"},
		Env:     env.Environ(),
		WorkDir: dir,
	})

	answer := func(t *testing.T, prompt, input string) {
		t.Helper()
		_, err := console.ExpectString(prompt)
		assert.NilError(t, err)
		_, err = console.Send(input)
		assert.NilError(t, err)
	}

	assert.Assert(t, t.Run("picks Forgejo", func(t *testing.T) {
		answer(t, "Press Enter to continue", "\r")
		answer(t, "Project name", "\r")
		answer(t, "Which provider hosts acme/web?", "\x1b[A")
		_, err := console.Send("\r")
		assert.NilError(t, err)
	}))

	assert.Assert(t, t.Run("takes the defaults and the audiences", func(t *testing.T) {
		answer(t, "Forgejo instance URL", "\r")
		answer(t, "Owner", "\r")
		_, err := console.ExpectString("Create a bot user")
		assert.NilError(t, err)
		_, err = console.ExpectString("https://oidc.circleci.com/org/" + onboardOrgID)
		assert.NilError(t, err)
		answer(t, "Connection integration audience", "u:1:conn\r")
		answer(t, "Checkout integration audience", "u:1:checkout\r")
		answer(t, "Does your instance need extra request headers", "\r")
	}))

	assert.Assert(t, t.Run("shows the webhook and waits for it", func(t *testing.T) {
		_, err := console.ExpectString(forgejoSecret)
		assert.NilError(t, err)
		answer(t, "Press Enter to continue", "\r")
		_, err = console.ExpectString("Found repository acme/web")
		assert.NilError(t, err)
		_, err = console.ExpectString("Trigger created: all-pushes")
		assert.NilError(t, err)
	}))

	t.Run("sent what was entered", func(t *testing.T) {
		bodies := requestBodies(fake, setupPath)
		assert.Assert(t, cmp.Len(bodies, 1))
		assert.Check(t, cmp.Contains(bodies[0], `"url":"`+forgejoURL+`"`), "body: %s", bodies[0])
		assert.Check(t, cmp.Contains(bodies[0], `"owner":"acme"`), "body: %s", bodies[0])
		assert.Check(t, cmp.Contains(bodies[0], `"checkout_audience":"u:1:checkout"`), "body: %s", bodies[0])
		assert.Check(t, !strings.Contains(bodies[0], "request_headers"), "body: %s", bodies[0])
	})
}

// When interactive, a value the API rejects is asked for again rather than
// failing the run.
func TestOnboard_Forgejo_Interactive_RepromptsRejectedValue(t *testing.T) {
	dir, fake, env := forgejoRepo(t)
	fake.QueueProviderConnectionSetupResponses(
		fakes.SetupResponse{
			Status: http.StatusBadRequest,
			Body: map[string]any{"error": map[string]any{
				"title":  "Invalid Input",
				"detail": "the checkout integration lacks scope read:repository",
			}},
		},
		forgejoConfigured(),
	)

	console := binary.RunCLIInteractive(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    append([]string{"onboard", "--scan"}, forgejoFlags...),
		Env:     env.Environ(),
		WorkDir: dir,
	})

	answer := func(t *testing.T, prompt, input string) {
		t.Helper()
		_, err := console.ExpectString(prompt)
		assert.NilError(t, err)
		_, err = console.Send(input)
		assert.NilError(t, err)
	}

	assert.Assert(t, t.Run("asks only for what no flag supplied", func(t *testing.T) {
		answer(t, "Press Enter to continue", "\r")
		answer(t, "Project name", "\r")
		answer(t, "Forgejo instance URL", "\r")
		answer(t, "Owner", "\r")
		answer(t, "Does your instance need extra request headers", "\r")
	}))

	assert.Assert(t, t.Run("shows the rejection and asks for that value again", func(t *testing.T) {
		_, err := console.ExpectString("Setup failed: the checkout integration lacks scope read:repository.")
		assert.NilError(t, err)
		answer(t, "Checkout integration audience", "u:1:fixed\r")
		_, err = console.ExpectString(forgejoSecret)
		assert.NilError(t, err)
		answer(t, "Press Enter to continue", "\r")
		_, err = console.ExpectString("Trigger created: all-pushes")
		assert.NilError(t, err)
	}))

	t.Run("retried with the new value", func(t *testing.T) {
		bodies := requestBodies(fake, setupPath)
		assert.Assert(t, cmp.Len(bodies, 2))
		assert.Check(t, cmp.Contains(bodies[1], `"checkout_audience":"u:1:fixed"`), "body: %s", bodies[1])
		assert.Check(t, cmp.Contains(bodies[1], `"connection_audience":"u:1:conn"`), "body: %s", bodies[1])
	})
}

func TestOnboard_UnclaimedHost_Interactive_Cancels(t *testing.T) {
	dir, _, env := onboardRepoWithRemote(t, "https://gitlab.com/myorg/my-repo.git")

	console := binary.RunCLIInteractiveExitCode(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"onboard", "--scan"},
		Env:     env.Environ(),
		WorkDir: dir,
	}, 6)

	for _, step := range []string{"Press Enter to continue", "Project name"} {
		_, err := console.ExpectString(step)
		assert.NilError(t, err)
		_, err = console.Send("\r")
		assert.NilError(t, err)
	}
	_, err := console.ExpectString("Which provider hosts myorg/my-repo?")
	assert.NilError(t, err)
	_, err = console.Send("\x1b")
	assert.NilError(t, err)
	_, err = console.ExpectString("No provider selected.")
	assert.NilError(t, err)
}
