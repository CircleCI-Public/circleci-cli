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

//go:build smoke

// Smoke tests for the runner command group. Unlike acceptance/, these run the
// compiled binary against the live CircleCI API, so they need real credentials
// and are excluded from `task test` by the smoke build tag. See README.md in
// this directory for how to run them.
package smoke_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"gopkg.in/yaml.v3"
	"gotest.tools/v3/assert"
	"gotest.tools/v3/assert/cmp"

	clierrors "github.com/CircleCI-Public/circleci-cli/clikit/errors"
	"github.com/CircleCI-Public/circleci-cli/internal/testing/binary"
	testenv "github.com/CircleCI-Public/circleci-cli/internal/testing/env"
)

const (
	envToken     = "CIRCLE_TOKEN"
	envNamespace = "CIRCLE_SMOKE_NAMESPACE"
	envProject   = "CIRCLE_SMOKE_PROJECT"
	envReadOnly  = "CIRCLE_SMOKE_READONLY_NAMESPACE"

	// runnerAPI is where the agent-facing endpoints live. Agent traffic
	// (claim, unclaim, task config) is served on the runner domain rather than
	// through the circleci.com external API, and this matches the machine runner 3
	// agent's own api.url default.
	runnerAPI = "https://runner.circleci.com"

	// namePrefix marks the resource classes this suite creates and is the only
	// thing the orphan sweep will delete. It has to be unambiguous in a shared
	// namespace: circleci-runner already holds prod-smoke-test-* from the
	// runner-smoke-tests suite and provisioner-smoke-test-* from
	// runner-provisioner, and sweeping either would destroy another team's
	// fixtures. TestCreatedAtFromName pins that it does not.
	namePrefix = "cli-smoke-"

	// orphanAge is how old one of our resource classes must be before a later run
	// treats it as abandoned and deletes it. It has to exceed the longest a run
	// can legitimately hold one, so that a sweep never deletes a live run's
	// resource class out from under it.
	orphanAge = time.Hour

	// claimBranch is the branch the queued-task test triggers. The project's
	// committed config is what names the resource class, so this has to be a
	// branch where that config exists.
	claimBranch = "main"

	// claimParam is the pipeline parameter the queued-task test passes its
	// resource class in. The project's config declares it and interpolates it into
	// the job's resource_class, so the job runs on the resource class this suite
	// created rather than on a fixed one the suite has to match.
	claimParam = "runner_resource_class"

	// claimVersion is the agent version the claim announces. Recognisable on
	// purpose: the instance-list read asserts it, which is what makes that read
	// prove the listed instance is the one this run registered.
	claimVersion = "0.0.0-smoke"

	// claimPollAttempts attempts with claimPollInterval between them bounds how
	// long the queued-task test waits for the distributor to make the task
	// claimable. The first attempt does not sleep, so the budget is
	// (attempts-1) x interval.
	claimPollAttempts = 24
	claimPollInterval = 5 * time.Second
)

var binaryPath string

// earliestPlausible bounds the timestamp createdAtFromName will accept. This
// suite did not exist before it, so anything older is a name that merely looks
// like ours -- and treating it as ancient would delete it immediately.
var earliestPlausible = time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC)

func TestMain(m *testing.M) {
	path, cleanup, err := binary.Build("circleci", "..", "./cmd/circleci")
	if err != nil {
		// Exit 0, not 1: a broken build should not read as a smoke failure.
		_, _ = fmt.Fprintf(os.Stderr, "skipping smoke tests: %v\n", err)
		os.Exit(0)
	}
	binaryPath = path
	code := m.Run()
	cleanup()
	os.Exit(code)
}

// secrets redacts values the test has learned out of any gotest.tools failure
// message derived from a CLIResult. A failed cmp comparison prints the whole
// buffer it was given, so without this a live token could land in captured
// output. It deliberately claims no more than that: RunCLI tees the child's
// stdout to the terminal before scrub ever runs, so anything the CLI actually
// prints is already gone.
//
// ponytail: this is the backstop, not the primary defence. The smoke path never
// prints a token in the first place -- `runner config` writes to --output and
// `token create` projects with --jq '.id'. Keep it that way: RunCLI tees child
// stdout straight to the terminal, so dropping either flag puts a credential on
// screen before any redaction can run.
type secrets struct {
	mu   sync.Mutex
	vals []string
}

// add registers v for redaction. Nothing this suite handles is short -- API
// tokens are 40 characters, runner and task tokens longer -- so a short value
// means the field it was decoded from was empty or malformed. That has to be
// loud rather than skipped: add("") would make scrub's ReplaceAll splice
// [REDACTED] between every rune of all later output.
func (s *secrets) add(t *testing.T, v string) {
	t.Helper()

	if len(v) < 8 {
		t.Fatalf("refusing to register a %d-character value as a secret: "+
			"the field it came from is empty or malformed", len(v))
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.vals = append(s.vals, v)
}

func (s *secrets) scrub(out string) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	// Longest first: if one registered value contains another, replacing the
	// shorter one first would fragment the longer so it never matches.
	slices.SortFunc(s.vals, func(a, b string) int { return len(b) - len(a) })
	for _, v := range s.vals {
		out = strings.ReplaceAll(out, v, "[REDACTED]")
	}
	return out
}

type smoke struct {
	token     string
	namespace string
	workDir   string
	secrets   *secrets
}

func newSmoke(t *testing.T) *smoke {
	t.Helper()

	token, namespace := os.Getenv(envToken), os.Getenv(envNamespace)
	if token == "" || namespace == "" {
		t.Skipf("set %s and %s to run the runner smoke tests (see smoke/README.md)",
			envToken, envNamespace)
	}

	s := &smoke{
		token:     token,
		namespace: namespace,
		// A temp working directory has no git remote, so every command must be
		// told its namespace or resource class explicitly. That is deliberate:
		// the suite must not depend on where it was invoked from.
		workDir: t.TempDir(),
		secrets: &secrets{},
	}
	s.secrets.add(t, token)
	return s
}

// run executes the CLI with the configured credentials, redacting known secrets
// from the captured output.
func (s *smoke) run(t *testing.T, args ...string) binary.CLIResult {
	t.Helper()
	return s.runAsToken(t, s.token, args...)
}

// runAsToken runs the CLI with token in place of the configured one. An empty
// token means no CIRCLE_TOKEN is set at all.
func (s *smoke) runAsToken(t *testing.T, token string, args ...string) binary.CLIResult {
	t.Helper()

	env := testenv.New(t)
	env.Token = token
	if token != "" {
		// Self-registering, so a caller cannot hand this an unredacted credential.
		s.secrets.add(t, token)
	}

	res := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    args,
		Env:     env.Environ(),
		WorkDir: s.workDir,
	})
	res.Stdout = s.secrets.scrub(res.Stdout)
	res.Stderr = s.secrets.scrub(res.Stderr)
	return res
}

type resourceClass struct {
	ID            string `json:"id"`
	ResourceClass string `json:"resource_class"`
	Description   string `json:"description"`
}

type runnerToken struct {
	ID            string `json:"id"`
	ResourceClass string `json:"resource_class"`
	Nickname      string `json:"nickname"`
	CreatedAt     string `json:"created_at"`
}

type runnerInstance struct {
	ResourceClass string `json:"resource_class"`
	Hostname      string `json:"hostname"`
	Name          string `json:"name"`
	Version       string `json:"version"`
}

// agentConfig mirrors the fields of the machine runner 3 schema that this suite
// checks. Mirrored rather than imported: the upstream types live in another
// repository and hold the token in a secret.String whose whole purpose is to stop
// it being read back, and a contract test that shares a schema with the thing it
// checks verifies nothing.
//
// `circleci runner config` emits only api.auth_token (internal/cmd/runner/
// config.go). It emits neither api.url -- which is why runnerAPI above is a
// constant -- nor runner.name, which circleci-runner's
// config/machine/driver/config.go marks required:"true" and enforces separately in
// Validate. That is why claimTask supplies a name of its own. Pointers, so an
// absent key is distinguishable from one present and empty; otherwise the
// assertion below could not tell "not emitted" from "emitted as empty".
type agentConfig struct {
	API struct {
		AuthToken string  `yaml:"auth_token"`
		URL       *string `yaml:"url"`
	} `yaml:"api"`
	Runner struct {
		Name *string `yaml:"name"`
	} `yaml:"runner"`
}

// runnerAPICall makes one request to the runner API with a bearer token,
// decoding a 200 JSON body into out when out is non-nil, and returns the status
// code.
func (s *smoke) runnerAPICall(t *testing.T, method, path, token string, in, out any) int {
	t.Helper()

	var body io.Reader
	if in != nil {
		raw, err := json.Marshal(in)
		assert.NilError(t, err)
		body = bytes.NewReader(raw)
	}

	req, err := http.NewRequest(method, runnerAPI+path, body)
	assert.NilError(t, err)
	req.Header.Set("Authorization", "Bearer "+token)
	if in != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := (&http.Client{Timeout: 30 * time.Second}).Do(req)
	assert.NilError(t, err)
	defer func() { _ = resp.Body.Close() }()

	if out != nil && resp.StatusCode == http.StatusOK {
		assert.NilError(t, json.NewDecoder(resp.Body).Decode(out))
	}
	return resp.StatusCode
}

// claimResponse is the subset of the claim body this suite inspects.
type claimResponse struct {
	TaskToken     string `json:"task_token"`
	AgentVersion  string `json:"agent_version"`
	ResourceClass string `json:"resource_class"`
	Allocation    string `json:"allocation"`
	// Warning carries a registration failure the API declined to fail the claim
	// over. It is the one field that explains an instance missing from
	// `instance list` after a successful claim.
	Warning string `json:"warning"`
}

// claimTask makes the same request a runner agent makes on start-up: POST
// /api/v3/runner/claim with the resource-class token as a bearer credential.
//
// The API registers the runner before it asks the distributor for work, so a
// claim against an empty resource class still creates the instance and returns
// 204. TestRunnerSmoke relies on that: a brand-new resource class that no
// .circleci/config.yml references cannot have queued work, so 204 is its pass
// condition, and a 200 there would mean it was handed somebody else's task.
// TestRunnerSmokeClaimQueuedTask queues a real job and expects the 200.
//
// Registration is best-effort, not guaranteed: runner-admin degrades a store
// failure to the response's warning field and returns the claim anyway
// (api/external/claim_task.go). So a 204 is not proof the instance exists, which
// is why the instance-list read retries and logs any warning.
func (s *smoke) claimTask(t *testing.T, token, name, hostname string) (int, claimResponse) {
	t.Helper()

	in := map[string]string{
		"name":     name,
		"hostname": hostname,
		"ip":       "",
		"version":  claimVersion,
	}

	var claimed claimResponse
	status := s.runnerAPICall(t, http.MethodPost, "/api/v3/runner/claim", token, in, &claimed)
	if claimed.Warning != "" {
		t.Logf("claim returned a warning, so the runner may not be registered: %s",
			s.secrets.scrub(claimed.Warning))
	}
	if status == http.StatusOK {
		s.secrets.add(t, claimed.TaskToken)
	}
	return status, claimed
}

// unclaimTask hands a claimed task back, the way an agent does when it turns out
// it cannot run one. Cancelling the run is not sufficient on its own: a claimed
// task keeps its job queued until the claim times out, so without this every run
// of the queued-task test leaves a job sitting in the project.
//
// The task id is not in the claim response. An agent reads it from
// GET /api/v2/task/config, authenticated with the task token rather than the
// resource-class token, so this does the same.
//
// The unclaim route sits outside runner-admin's resource-class authenticator, so
// the bearer header it sends is inert: the task_token in the body is what
// authorizes the call. The header is only there for consistency with the rest.
func (s *smoke) unclaimTask(t *testing.T, taskToken string) {
	t.Helper()

	var cfg struct {
		TaskID string `json:"task_id"`
	}
	if status := s.runnerAPICall(t, http.MethodGet, "/api/v2/task/config",
		taskToken, nil, &cfg); status != http.StatusOK || cfg.TaskID == "" {
		t.Errorf("could not read the task config to unclaim (status %d): the claimed "+
			"task will hold its job queued until the claim times out", status)
		return
	}

	in := map[string]string{"task_id": cfg.TaskID, "task_token": taskToken}
	if status := s.runnerAPICall(t, http.MethodPost, "/api/v3/runner/unclaim",
		taskToken, in, nil); status != http.StatusOK {
		t.Errorf("unclaim returned %d for task %s", status, cfg.TaskID)
	}
}

// reap deletes rc, tolerating "already gone" so that the explicit delete step and
// this backstop can both run.
//
// `resource-class delete` always calls DELETE /api/v3/runner/resource/{id}/force,
// regardless of the CLI's --force flag, which only skips the confirmation prompt.
// The force endpoint deletes the class's tokens before the class; the non-force
// endpoint answers 409 while any token exists. So no separate token pass is
// needed, and that is load-bearing rather than a convenience.
//
// ExitNotFound cannot be taken at face value: runner-admin answers a permission
// denial with 404 too, so "not found" and "not allowed" arrive identically. Read
// back rather than assume, or a denied delete would look like a completed one.
func (s *smoke) reap(t *testing.T, rc string) {
	t.Helper()

	res := s.run(t, "runner", "resource-class", "delete", rc, "--force")
	switch res.ExitCode {
	case 0:
		return
	case clierrors.ExitNotFound:
		if s.resourceClassExists(t, rc) {
			t.Errorf("leaked resource class %s: delete reported not-found but it is "+
				"still listed, so the delete was most likely denied", rc)
		}
	default:
		t.Errorf("leaked resource class %s: exit %d\nstderr: %s",
			rc, res.ExitCode, res.Stderr)
	}
}

// resourceClassExists reports whether rc is still listed in its namespace.
func (s *smoke) resourceClassExists(t *testing.T, rc string) bool {
	t.Helper()

	ns, _, _ := strings.Cut(rc, "/")
	res := s.run(t, "runner", "resource-class", "list", "--namespace", ns, "--json")
	if res.ExitCode != 0 {
		t.Logf("could not list %s to confirm %s is gone: exit %d", ns, rc, res.ExitCode)
		return false
	}

	var classes []resourceClass
	if err := json.Unmarshal([]byte(res.Stdout), &classes); err != nil {
		t.Logf("could not decode the resource class list: %v", err)
		return false
	}
	return containsFunc(classes, func(c resourceClass) bool { return c.ResourceClass == rc })
}

// sweepOrphans deletes cli-smoke-* resource classes left behind by an interrupted
// earlier run. t.Cleanup covers assertion failures and t.Fatal, but not a
// Ctrl-C or a `go test -timeout` kill, so without this the namespace
// accumulates resource classes forever. The age comes out of the name because
// `resource-class list --json` returns no timestamp.
func (s *smoke) sweepOrphans(t *testing.T) {
	t.Helper()

	res := s.run(t, "runner", "resource-class", "list", "--namespace", s.namespace, "--json")
	assert.Assert(t, cmp.Equal(res.ExitCode, 0), "listing resource classes: %s", res.Stderr)

	var classes []resourceClass
	assert.NilError(t, json.Unmarshal([]byte(res.Stdout), &classes))

	now := time.Now()
	for _, rc := range classes {
		_, name, found := strings.Cut(rc.ResourceClass, "/")
		if !found || !shouldSweep(name, now) {
			continue
		}
		t.Logf("sweeping orphaned resource class %s", rc.ResourceClass)
		s.reap(t, rc.ResourceClass)
	}
}

// smokeName returns a resource class name carrying its own creation time, in a
// form both the resource class API and the claim request's name regex accept.
// The seconds and the pid together are what make it unique: the pid separates
// concurrent runs on one machine, the timestamp separates runs that share a pid,
// and the timestamp is also what the sweep dates the name by.
func smokeName() string {
	return fmt.Sprintf("%s%d-%d", namePrefix, time.Now().Unix(), os.Getpid())
}

// shouldSweep decides whether a resource class name belongs to an abandoned
// earlier run. It is the whole destructive decision, in one place, so it can be
// tested without touching an organization: the sweep deletes only names this
// suite generates, and only once they are too old for a run to still be using
// them.
func shouldSweep(name string, now time.Time) bool {
	created, ok := createdAtFromName(name)
	return ok && now.Sub(created) >= orphanAge
}

// createdAtFromName recovers the creation time smokeName encoded. The timestamp
// travels in the name because `resource-class list --json` returns none.
//
// The window is deliberate. A name is only datable if it parses to a plausible
// time: a nonsense value must not resolve to 1970 and be deleted on sight, nor
// to a far-future date that is never swept.
func createdAtFromName(name string) (time.Time, bool) {
	rest, ok := strings.CutPrefix(name, namePrefix)
	if !ok {
		return time.Time{}, false
	}
	secs, err := strconv.ParseInt(strings.Split(rest, "-")[0], 10, 64)
	if err != nil {
		return time.Time{}, false
	}
	created := time.Unix(secs, 0)
	if created.Before(earliestPlausible) || created.After(time.Now()) {
		return time.Time{}, false
	}
	return created, true
}

// preflightNamespace fails early and clearly when the namespace is not claimed.
// Runner resource classes live under the organization's orb namespace, and a call
// against an unclaimed one is denied before permissions are even consulted.
//
// The resulting 404 surfaces two different ways, which is why this check exists
// rather than letting the suite stumble into either. The list paths map it to
// "Self-hosted runners are not available for this token or account" and exit
// ExitAPIError, which sends you inspecting the token when the namespace is what is
// missing; create and delete-by-name report "No runner resource found" and exit
// ExitNotFound, which is what the failure cases below assert.
func (s *smoke) preflightNamespace(t *testing.T) {
	t.Helper()

	res := s.run(t, "namespace", "get", s.namespace, "--json")
	if res.ExitCode == 0 {
		return
	}
	t.Fatalf("namespace %q is not usable (exit %d).\n"+
		"Runner resource classes live under the org's namespace. Claim it once with:\n"+
		"    circleci namespace create %s --org <vcs>/<org>\n"+
		"stderr: %s", s.namespace, res.ExitCode, s.namespace, res.Stderr)
}

func TestRunnerSmoke(t *testing.T) {
	s := newSmoke(t)
	s.preflightNamespace(t)
	s.sweepOrphans(t)

	const description = "ONP-3563 smoke test"

	name := smokeName()
	rc := s.namespace + "/" + name
	configPath := filepath.Join(s.workDir, "circleci-runner-config.yaml")

	// Registered before the create runs: if create half-succeeds, the resource
	// class still gets reaped.
	t.Cleanup(func() { s.reap(t, rc) })

	var agentToken, tokenID string

	// Gated: every subtest below operates on this resource class, so without the
	// gate one failed create becomes six failures with the cause buried.
	assert.Assert(t, t.Run("resource class create", func(t *testing.T) {
		res := s.run(t, "runner", "resource-class", "create", rc,
			"--description", description, "--json")
		assert.Assert(t, cmp.Equal(res.ExitCode, 0), "stderr: %s", res.Stderr)

		var created resourceClass
		assert.NilError(t, json.Unmarshal([]byte(res.Stdout), &created))
		// Fatal: the cleanup registered above reaps rc, so if the API created
		// something else, that something else would leak.
		assert.Assert(t, cmp.Equal(created.ResourceClass, rc))
		assert.Check(t, created.ID != "", "resource class has no id")
		assert.Check(t, cmp.Equal(created.Description, description))
	}))

	t.Run("agent config generate", func(t *testing.T) {
		// --output keeps the minted token off stdout entirely.
		res := s.run(t, "runner", "config", rc, "--output", configPath,
			"--nickname", "smoke-agent")
		assert.Assert(t, cmp.Equal(res.ExitCode, 0), "stderr: %s", res.Stderr)
		// Reports the length, never the value, and fatally: a non-empty stdout here
		// means a live token was just printed, and secrets does not know it yet.
		assert.Assert(t, len(res.Stdout) == 0,
			"runner config wrote %d bytes to stdout despite --output", len(res.Stdout))

		raw, err := os.ReadFile(configPath)
		assert.NilError(t, err)

		var cfg agentConfig
		assert.NilError(t, yaml.Unmarshal(raw, &cfg))
		assert.Assert(t, cfg.API.AuthToken != "", "generated config has no api.auth_token")

		agentToken = cfg.API.AuthToken
		s.secrets.add(t, agentToken)

		// The agent requires runner.name and the CLI does not emit it, nor api.url.
		// Asserted so a change in either direction is caught here rather than by a
		// customer whose agent refuses to start. See ONP-3558.
		assert.Check(t, cfg.Runner.Name == nil,
			"runner config now emits runner.name -- update the claim below")
		assert.Check(t, cfg.API.URL == nil,
			"runner config now emits api.url -- runnerAPI need not be a constant")
	})

	t.Run("agent claims a task", func(t *testing.T) {
		assert.Assert(t, agentToken != "", "no agent token from the previous step")

		status, claimed := s.claimTask(t, agentToken, name, name)
		switch status {
		case http.StatusNoContent:
			// Expected: token authenticated, runner registered, no work queued.
		case http.StatusOK:
			// Should be unreachable, but a claimed task holds its job queued until
			// the claim times out, so hand it back before failing rather than
			// stranding somebody's work to make a point.
			s.unclaimTask(t, claimed.TaskToken)
			t.Fatalf("claim returned 200: this run was handed a real task for %s", rc)
		default:
			t.Fatalf("claim returned %d, want 204", status)
		}
	})

	t.Run("token create", func(t *testing.T) {
		// --jq projects away the token value, so only the id reaches stdout.
		res := s.run(t, "runner", "token", "create", rc,
			"--nickname", "smoke-extra", "--json", "--jq", ".id")
		assert.Assert(t, cmp.Equal(res.ExitCode, 0), "stderr: %s", res.Stderr)

		// --jq emits a bare string rather than a JSON-quoted one, so this is
		// not JSON to unmarshal. Trim quotes anyway so the test survives the
		// CLI switching to quoted output.
		tokenID = strings.Trim(strings.TrimSpace(res.Stdout), `"`)
		assert.Assert(t, tokenID != "", "token create returned no id")
		assert.Check(t, !strings.Contains(tokenID, "\n"),
			"--jq returned more than the id: %q", tokenID)
	})

	t.Run("reads", func(t *testing.T) {
		t.Run("resource class list", func(t *testing.T) {
			res := s.run(t, "runner", "resource-class", "list",
				"--namespace", s.namespace, "--json")
			assert.Assert(t, cmp.Equal(res.ExitCode, 0), "stderr: %s", res.Stderr)

			var classes []resourceClass
			assert.NilError(t, json.Unmarshal([]byte(res.Stdout), &classes))
			assert.Check(t, containsFunc(classes, func(c resourceClass) bool {
				return c.ResourceClass == rc
			}), "resource class %s missing from list", rc)
		})

		t.Run("token list", func(t *testing.T) {
			res := s.run(t, "runner", "token", "list", "--resource-class", rc, "--json")
			assert.Assert(t, cmp.Equal(res.ExitCode, 0), "stderr: %s", res.Stderr)

			var tokens []runnerToken
			assert.NilError(t, json.Unmarshal([]byte(res.Stdout), &tokens))
			// Two tokens by now: one from `runner config`, one from `token create`.
			assert.Check(t, cmp.Len(tokens, 2))
			assert.Check(t, containsFunc(tokens, func(tk runnerToken) bool {
				return tk.ID == tokenID && tk.Nickname == "smoke-extra"
			}), "token %s nicknamed smoke-extra missing from list", tokenID)
		})

		t.Run("instance list", func(t *testing.T) {
			// The claim registers the runner synchronously, so this retry only
			// covers lag on the read path.
			var instances []runnerInstance
			for attempt := range 3 {
				if attempt > 0 {
					time.Sleep(2 * time.Second)
				}
				res := s.run(t, "runner", "instance", "list", "--resource-class", rc, "--json")
				assert.Assert(t, cmp.Equal(res.ExitCode, 0), "stderr: %s", res.Stderr)
				assert.NilError(t, json.Unmarshal([]byte(res.Stdout), &instances))
				if len(instances) > 0 {
					break
				}
			}
			assert.Check(t, containsFunc(instances, func(i runnerInstance) bool {
				return i.Hostname == name && i.Version == claimVersion
			}), "claimed runner %s at version %s missing from instance list",
				name, claimVersion)
		})
	})

	t.Run("token delete", func(t *testing.T) {
		assert.Assert(t, tokenID != "", "no token id from the create step")

		res := s.run(t, "runner", "token", "delete", tokenID, "--force")
		assert.Assert(t, cmp.Equal(res.ExitCode, 0), "stderr: %s", res.Stderr)

		res = s.run(t, "runner", "token", "list", "--resource-class", rc, "--json")
		assert.Assert(t, cmp.Equal(res.ExitCode, 0), "stderr: %s", res.Stderr)
		var tokens []runnerToken
		assert.NilError(t, json.Unmarshal([]byte(res.Stdout), &tokens))
		assert.Check(t, !containsFunc(tokens, func(tk runnerToken) bool {
			return tk.ID == tokenID
		}), "token %s still listed after delete", tokenID)
	})

	t.Run("resource class delete", func(t *testing.T) {
		res := s.run(t, "runner", "resource-class", "delete", rc, "--force")
		assert.Assert(t, cmp.Equal(res.ExitCode, 0), "stderr: %s", res.Stderr)

		res = s.run(t, "runner", "resource-class", "list",
			"--namespace", s.namespace, "--json")
		assert.Assert(t, cmp.Equal(res.ExitCode, 0), "stderr: %s", res.Stderr)
		var classes []resourceClass
		assert.NilError(t, json.Unmarshal([]byte(res.Stdout), &classes))
		assert.Check(t, !containsFunc(classes, func(c resourceClass) bool {
			return c.ResourceClass == rc
		}), "resource class %s still listed after delete", rc)
	})
}

func TestRunnerSmokeFailures(t *testing.T) {
	s := newSmoke(t)
	s.preflightNamespace(t)

	// Both of these exit ExitAuthError, so the exit code alone cannot tell them
	// apart. The structured error code can, and the distinction is the point: one
	// is refused locally, the other is a live 401 the CLI has to map. Asserting
	// only the exit code would let a regression that never sends the token pass
	// the bad-token case.
	t.Run("no token", func(t *testing.T) {
		res := s.runAsToken(t, "", "runner", "resource-class", "list",
			"--namespace", s.namespace, "--json")
		assert.Check(t, cmp.Equal(res.ExitCode, clierrors.ExitAuthError))
		assert.Check(t, cmp.Contains(res.Stderr, "auth.token_missing"))
	})

	t.Run("bad token", func(t *testing.T) {
		// In the environment, never in argv: RunCLI logs the full argument list.
		res := s.runAsToken(t, "not-a-real-token", "runner", "resource-class", "list",
			"--namespace", s.namespace, "--json")
		assert.Check(t, cmp.Equal(res.ExitCode, clierrors.ExitAuthError))
		assert.Check(t, cmp.Contains(res.Stderr, "auth.token_invalid"),
			"expected a rejected-token error, not a missing-token one")
	})

	t.Run("resource class not found", func(t *testing.T) {
		res := s.run(t, "runner", "resource-class", "delete",
			s.namespace+"/"+namePrefix+"does-not-exist", "--force")
		assert.Check(t, cmp.Equal(res.ExitCode, clierrors.ExitNotFound))
	})

	// Insufficient permission is proved with this identity aimed at a namespace it
	// may read but not administer, which needs no second account. It lands on
	// ExitNotFound rather than a permission-shaped code because runner-admin
	// answers a denial with 404 so callers cannot probe what exists.
	t.Run("insufficient permissions", func(t *testing.T) {
		ns := os.Getenv(envReadOnly)
		if ns == "" {
			t.Skipf("set %s to a namespace this token can read but not administer",
				envReadOnly)
		}

		assert.Assert(t, ns != s.namespace,
			"%s must differ from %s, or the create below would be permitted and this "+
				"would prove nothing", envReadOnly, envNamespace)

		// Reading it must work, or this proves nothing either: a namespace that does
		// not resolve to an org is denied before permissions are ever checked.
		res := s.run(t, "runner", "resource-class", "list", "--namespace", ns, "--json")
		assert.Assert(t, cmp.Equal(res.ExitCode, 0),
			"%s=%q is not even readable, so a denial would not prove anything: %s",
			envReadOnly, ns, res.Stderr)

		// Registered before the create, because the sweep cannot help here: it only
		// ever looks at s.namespace, so a resource class created in someone else's
		// namespace would leak permanently.
		rc := ns + "/" + smokeName()
		t.Cleanup(func() {
			if s.resourceClassExists(t, rc) {
				s.reap(t, rc)
			}
		})

		res = s.run(t, "runner", "resource-class", "create", rc, "--json")
		assert.Assert(t, cmp.Equal(res.ExitCode, clierrors.ExitNotFound),
			"creating %s was not denied, so this token can administer %s after all",
			rc, ns)
	})
}

// TestRunnerSmokeClaimQueuedTask proves the whole dispatch path, not just that a
// token authenticates: it queues a real job on the resource class and claims it,
// which is a 200 with a task token rather than the 204 that TestRunnerSmoke
// asserts.
//
// It needs a project whose committed config declares a runner_resource_class
// pipeline parameter and interpolates it into a machine job's resource_class, so
// the queued job targets the resource class this test just created. Opt-in and
// separate from TestRunnerSmoke because it triggers a real pipeline and waits on the
// distributor, which takes an order of magnitude longer than the rest of the suite.
//
// Nothing executes the task: there is no agent, only the claim request an agent
// would make. The run is cancelled afterwards so the job does not sit queued.
func TestRunnerSmokeClaimQueuedTask(t *testing.T) {
	project := os.Getenv(envProject)
	if project == "" {
		t.Skipf("set %s (e.g. gh/org/repo) to a project whose config declares the %s "+
			"pipeline parameter, to run the queued-task claim test", envProject, claimParam)
	}

	s := newSmoke(t)
	s.preflightNamespace(t)
	s.sweepOrphans(t)

	// The same per-run name the rest of the suite uses: unique, so concurrent runs
	// do not collide, and carrying its own timestamp so the sweep can reclaim it if
	// this run is interrupted.
	rc := s.namespace + "/" + smokeName()

	configPath := filepath.Join(s.workDir, "claim-agent-config.yaml")
	var agentToken string

	// Subtests shadow t, and cleanups belong to the t they are registered on, so
	// anything that must outlive a subtest is registered here.
	parent := t
	parent.Cleanup(func() { s.reap(parent, rc) })

	t.Run("create the resource class and its agent config", func(t *testing.T) {
		res := s.run(t, "runner", "resource-class", "create", rc,
			"--description", "ONP-3563 queued-task claim", "--json")
		assert.Assert(t, cmp.Equal(res.ExitCode, 0), "stderr: %s", res.Stderr)

		res = s.run(t, "runner", "config", rc, "--output", configPath)
		assert.Assert(t, cmp.Equal(res.ExitCode, 0), "stderr: %s", res.Stderr)

		raw, err := os.ReadFile(configPath)
		assert.NilError(t, err)
		var cfg agentConfig
		assert.NilError(t, yaml.Unmarshal(raw, &cfg))
		assert.Assert(t, cfg.API.AuthToken != "", "generated config has no api.auth_token")
		agentToken = cfg.API.AuthToken
		s.secrets.add(t, agentToken)
	})

	var runID, taskToken string

	t.Run("trigger a run that targets it", func(t *testing.T) {
		res := s.run(t, "run", "trigger", "--project", project,
			"--branch", claimBranch, "--parameter", claimParam+"="+rc,
			"--json", "--jq", ".id")
		assert.Assert(t, cmp.Equal(res.ExitCode, 0), "stderr: %s", res.Stderr)

		runID = strings.Trim(strings.TrimSpace(res.Stdout), `"`)
		assert.Assert(t, runID != "", "run trigger returned no id")
		t.Logf("triggered run %s on %s@%s", runID, project, claimBranch)

		// Registered on the parent, not on this subtest's t: a cleanup belongs to
		// the t it is called on, so registering it here would cancel the run the
		// moment this subtest returned -- before the claim below ever ran.
		parent.Cleanup(func() { s.cancelRun(parent, project, runID) })
	})

	t.Run("agent claims the queued task", func(t *testing.T) {
		assert.Assert(t, agentToken != "", "no agent token from the create step")
		assert.Assert(t, runID != "", "no run from the trigger step")

		hostname := namePrefix + "claim-" + strconv.Itoa(os.Getpid())

		var status int
		var claimed claimResponse
		for attempt := range claimPollAttempts {
			if attempt > 0 {
				time.Sleep(claimPollInterval)
			}
			status, claimed = s.claimTask(t, agentToken, hostname, hostname)
			if status == http.StatusOK {
				break
			}
			assert.Assert(t, cmp.Equal(status, http.StatusNoContent),
				"claim returned %d while waiting for the task", status)

			if ended, errs := s.runEnded(t, project, runID); ended {
				t.Fatalf("run %s ended before any task could be claimed (%s).\n"+
					"Does %s@%s declare a %s pipeline parameter and interpolate it into "+
					"a machine job's resource_class? See smoke/README.md.",
					runID, errs, project, claimBranch, claimParam)
			}
		}

		assert.Assert(t, cmp.Equal(status, http.StatusOK),
			"no task became claimable within %s -- does %s@%s declare a %s pipeline "+
				"parameter and interpolate it into a machine job's resource_class? "+
				"see smoke/README.md",
			time.Duration(claimPollAttempts-1)*claimPollInterval, project, claimBranch,
			claimParam)

		// Assert presence, never the values: task_token is a credential and
		// allocation is opaque.
		assert.Check(t, claimed.TaskToken != "", "claimed task has no task_token")
		assert.Check(t, claimed.Allocation != "", "claimed task has no allocation")

		taskToken = claimed.TaskToken
		t.Logf("claimed a task on %s (agent_version=%q, resource_class=%q)",
			rc, claimed.AgentVersion, claimed.ResourceClass)
	})

	t.Run("agent unclaims the task", func(t *testing.T) {
		assert.Assert(t, taskToken != "", "no task token from the claim step")
		// Before the run is cancelled: cancelling drops the task from the
		// distributor, after which unclaim answers 404.
		s.unclaimTask(t, taskToken)
	})

	t.Run("cancel the run", func(t *testing.T) {
		assert.Assert(t, runID != "", "no run from the trigger step")
		res := s.run(t, "run", "cancel", runID, "--force", "--project", project)
		assert.Check(t, cmp.Equal(res.ExitCode, 0), "stderr: %s", res.Stderr)
	})
}

// runEnded reports whether the run has already reached a terminal phase, with any
// errors it recorded. The claim poll consults it so that a run which cannot
// produce a task fails in seconds rather than waiting out the full timeout: an
// undeclared pipeline parameter, for instance, ends the run immediately with a
// config error.
//
// A read failure reports false so the poll continues, but says so: silently
// returning false would let 24 failed reads look like a run still in progress,
// and the eventual timeout would then blame the project's config for something
// that was never the cause.
func (s *smoke) runEnded(t *testing.T, project, runID string) (bool, string) {
	t.Helper()

	res := s.run(t, "run", "get", runID, "--project", project, "--json")
	if res.ExitCode != 0 {
		t.Logf("could not read run %s (exit %d), still waiting: %s",
			runID, res.ExitCode, res.Stderr)
		return false, ""
	}

	var got struct {
		Phase  string `json:"phase"`
		Errors []struct {
			Type    string `json:"type"`
			Message string `json:"message"`
		} `json:"errors"`
	}
	if err := json.Unmarshal([]byte(res.Stdout), &got); err != nil {
		t.Logf("could not decode `run get --json` for %s, still waiting: %v", runID, err)
		return false, ""
	}
	// An empty phase means the field moved or was renamed, which would turn this
	// fast-fail into a permanent no-op. Say so rather than reporting "not ended".
	if got.Phase == "" {
		t.Logf("`run get --json` returned no phase for %s: has the output shape "+
			"changed? the claim poll's fast-fail depends on it", runID)
		return false, ""
	}
	if got.Phase != "ended" {
		return false, ""
	}

	msgs := make([]string, 0, len(got.Errors))
	for _, e := range got.Errors {
		msgs = append(msgs, e.Type+": "+e.Message)
	}
	if len(msgs) == 0 {
		return true, "no errors reported"
	}
	return true, strings.Join(msgs, "; ")
}

// cancelRun cancels runID so the triggered pipeline does not stay live, and logs
// the phase it reaches. It deliberately does not assert a terminal phase: a run
// whose job was queued on a runner resource class takes minutes to settle after
// cancellation, which is platform timing this suite does not control. What must
// hold is that the cancel request itself was accepted.
func (s *smoke) cancelRun(t *testing.T, project, runID string) {
	t.Helper()

	// ExitBadArguments is the CLI's "no active workflows to cancel", i.e. the
	// explicit step already cancelled it or it ended on its own. Anything else --
	// auth, API, a timeout -- leaves a live run burning credits in someone's
	// project, so it fails rather than logging.
	res := s.run(t, "run", "cancel", runID, "--force", "--project", project)
	if res.ExitCode != 0 && res.ExitCode != clierrors.ExitBadArguments {
		t.Errorf("could not cancel run %s: exit %d\nstderr: %s",
			runID, res.ExitCode, res.Stderr)
	}

	res = s.run(t, "run", "get", runID, "--project", project, "--json", "--jq", ".phase")
	if res.ExitCode == 0 {
		t.Logf("run %s left in phase %s", runID, strings.TrimSpace(res.Stdout))
	}
}

func containsFunc[T any](items []T, match func(T) bool) bool {
	for _, item := range items {
		if match(item) {
			return true
		}
	}
	return false
}

// TestCreatedAtFromName covers the naming half of the sweep decision: which names
// this suite will even claim as its own. Pure logic, so it needs no credentials.
func TestCreatedAtFromName(t *testing.T) {
	for _, tc := range []struct {
		name   string
		dated  bool
		reason string
	}{
		{"cli-smoke-1787956903-25211", true, "one of ours, datable"},
		{"prod-smoke-test-deb-amd64-3794", false, "another suite's, wrong prefix"},
		{"provisioner-smoke-test-40151-aws-rhel", false, "another suite's, wrong prefix"},
		{"cli-smoke-not-a-timestamp", false, "malformed, not assumed ancient"},
		{"cli-smoke-1-0", false, "1970 is implausible, so not deleted on sight"},
		{"cli-smoke-99999999999999-0", false, "the far future would never be swept"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, ok := createdAtFromName(tc.name)
			assert.Check(t, cmp.Equal(ok, tc.dated), tc.reason)
		})
	}
}

// TestShouldSweep covers the half that decides a deletion. The age gate is the
// only thing standing between the sweep and a concurrently running suite's
// resource class, so it is worth pinning without a live organization.
func TestShouldSweep(t *testing.T) {
	now := time.Date(2026, time.June, 1, 12, 0, 0, 0, time.UTC)
	at := func(d time.Duration) string {
		return fmt.Sprintf("%s%d-1", namePrefix, now.Add(-d).Unix())
	}

	for _, tc := range []struct {
		name   string
		sweep  bool
		reason string
	}{
		{at(0), false, "a run that just started is still using it"},
		{at(orphanAge - time.Minute), false, "still inside the window a run may hold it"},
		{at(orphanAge), true, "at the boundary it is abandoned"},
		{at(24 * time.Hour), true, "long abandoned"},
		{"prod-smoke-test-deb-amd64-3794", false, "another suite's, never ours to delete"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			assert.Check(t, cmp.Equal(shouldSweep(tc.name, now), tc.sweep), tc.reason)
		})
	}
}
