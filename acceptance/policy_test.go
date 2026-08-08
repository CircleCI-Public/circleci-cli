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
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"

	"github.com/CircleCI-Public/circleci-cli/internal/httpcl"
	"gotest.tools/v3/assert"
	"gotest.tools/v3/assert/cmp"
	"gotest.tools/v3/fs"
	"gotest.tools/v3/golden"

	"github.com/CircleCI-Public/circleci-cli/internal/testing/binary"
	testenv "github.com/CircleCI-Public/circleci-cli/internal/testing/env"
	"github.com/CircleCI-Public/circleci-cli/internal/testing/fakes"
	"github.com/CircleCI-Public/circleci-cli/internal/testing/httprecorder"
)

// Elapsed times, JUnit timestamps, and temp-dir paths are non-deterministic, so
// they are normalized to fixed placeholders before golden comparison.
var (
	elapsedSecRe    = regexp.MustCompile(`\d+\.\d{3}`) // markdown table + footer seconds
	jsonElapsedRe   = regexp.MustCompile(`"Elapsed":"[^"]*"`)
	jsonElapsedMSRe = regexp.MustCompile(`"ElapsedMS":\d+`)
	junitTimeRe     = regexp.MustCompile(`time="[0-9.]+"`)
	junitTSRe       = regexp.MustCompile(`timestamp="[^"]*"`)
)

// normalizePolicyOutput replaces the temp directory and every timing value with
// stable placeholders so policy eval/test output can be golden-compared.
func normalizePolicyOutput(s, dir string) string {
	// Replace JSON-escaped path first (backslashes doubled), then the plain path.
	s = strings.ReplaceAll(s, strings.ReplaceAll(dir, `\`, `\\`), "<DIR>")
	s = strings.ReplaceAll(s, dir, "<DIR>")
	// Normalize backslash path separator after the placeholder (Windows).
	s = strings.ReplaceAll(s, `<DIR>\`, "<DIR>/")
	// Normalize Windows-specific syscall names and error messages.
	s = strings.ReplaceAll(s, "GetFileAttributesEx ", "lstat ")
	s = strings.ReplaceAll(s, "The system cannot find the file specified.", "no such file or directory")
	s = jsonElapsedRe.ReplaceAllString(s, `"Elapsed":"<T>"`)
	s = jsonElapsedMSRe.ReplaceAllString(s, `"ElapsedMS":0`)
	s = junitTSRe.ReplaceAllString(s, `timestamp="<TS>"`)
	s = junitTimeRe.ReplaceAllString(s, `time="<T>"`)
	s = elapsedSecRe.ReplaceAllString(s, "<T>")
	return s
}

const testOwnerID = "462d67f8-b232-4da4-a7de-0c86dd667d3f"
const testPolicyCtx = "config"
const testDecisionID = "d0000001-0000-4000-8000-000000000001"

// writePolicyDir creates a temporary directory with a single .rego file and returns its path.
func writePolicyDir(t *testing.T, name, content string) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, name+".rego"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

// Policy/test fixtures carried over verbatim from the v0 branch's
// cmd/policy/testdata so eval/test coverage matches the original scenarios.
const (
	// test0/policy.rego — a config-context policy.
	evalBasicPolicyRego = `package org

policy_name["test"]
enable_rule["branch_is_main"]
branch_is_main = "branch must be main!" { input.branch != "main" }
`
	// test0/subdir/meta-policy-subdir/meta-policy.rego — reads data.meta.
	evalMetaPolicyRego = `package org

policy_name["meta_policy_test"]
enable_rule["enabled"] { data.meta.vcs.branch == "main" }
enable_rule["disabled"] { data.meta.project_id != "test-project-id" }
`
	// test1/test.yml — the input document (deliberately has no "branch" key).
	evalInputYAML = "test: config\n"
	// test0/config.yml — input with a branch.
	evalBranchInputYAML = "branch: main"
	// test1/meta.yml — metadata as a YAML file.
	evalMetaFileYAML = `project_id: test-project-id
vcs:
  branch: main
`

	// test_policies/policy.rego + policy_test.yaml — two passing tests
	// (one PASS on main, one SOFT_FAIL on a feature branch).
	testPolicyRego = `package org

policy_name["test"]

enable_rule["fail_if_not_main"]

fail_if_not_main = "branch must be main!" { data.meta.vcs.branch != "main" }
`
	testPolicyTestsYAML = `test_main:
  meta:
    vcs:
      branch: main
  decision: &root_decision
    status: PASS
    enabled_rules:
      - fail_if_not_main

test_feature:
  meta:
    vcs:
      branch: feature
  decision:
    <<: *root_decision
    status: SOFT_FAIL
    soft_failures:
      - rule: fail_if_not_main
        reason: branch must be main!
`

	// compile_policies/compile.rego + compile_test.yaml — a test that requires
	// config compilation (compile: true + pipeline_parameters).
	compilePolicyRego = `package org

import future.keywords

policy_name["example_compiled"]

enable_hard["enforce_small_jobs"]

enforce_small_jobs[reason] {
	some job_name, job in input._compiled_.jobs
	job.resource_class != "small"
	reason = sprintf("job %s: resource_class must be small", [job_name])
}
`
	compilePolicyTestsYAML = `test_compile_policy:
  compile: true
  pipeline_parameters:
    parameters:
      size: small
  input:
    version: 2.1
    parameters:
      size:
        type: string
        default: medium
    jobs:
      test:
        docker:
          - image: go
        resource_class: << pipeline.parameters.size >>
        steps:
          - run: it
    workflows:
      main:
        jobs:
          - test
  decision:
    status: PASS
    enabled_rules: [enforce_small_jobs]
`
	// A compiled config the fake compile endpoint returns for the compile test:
	// jobs.test.resource_class is "small", so enforce_small_jobs finds no violation.
	compiledSmallJobYAML = `version: "2.1"
jobs:
  test:
    docker:
      - image: go
    resource_class: small
    steps:
      - run: it
workflows:
  main:
    jobs:
      - test
`
)

// stdTestPoliciesDir builds the standard test_policies fixture (policy + its two
// tests) as a temporary directory.
func stdTestPoliciesDir(t *testing.T) *fs.Dir {
	t.Helper()
	return fs.NewDir(t, "policy",
		fs.WithFile("policy.rego", testPolicyRego),
		fs.WithFile("policy_test.yaml", testPolicyTestsYAML),
	)
}

// --- policy eval ---

// TestPolicyEval covers the basic local (uncompiled) raw-OPA evaluation and the
// default "data" query, matching the v0 "config compilation is disabled" case.
func TestPolicyEval(t *testing.T) {
	env := testenv.New(t)
	// No token: --no-compile eval is fully local and must not require auth.

	dir := fs.NewDir(t, "policy",
		fs.WithFile("policy.rego", evalBasicPolicyRego),
		fs.WithFile("input.yml", evalInputYAML),
	)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"policy", "eval", dir.Join("policy.rego"), "--input", dir.Join("input.yml"), "--no-compile"},
		Env:     env.Environ(),
		WorkDir: dir.Path(),
	})

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	assert.Check(t, golden.String(normalizePolicyOutput(result.Stdout, dir.Path()), t.Name()+".txt"))
}

// TestPolicyEval_MetaAndQuery covers --meta / --metafile seeding data.meta and
// the --query flag, matching the four v0 meta-policy eval cases.
func TestPolicyEval_MetaAndQuery(t *testing.T) {
	dir := fs.NewDir(t, "policy",
		fs.WithFile("meta-policy.rego", evalMetaPolicyRego),
		fs.WithFile("config.yml", evalBranchInputYAML),
		fs.WithFile("meta.yml", evalMetaFileYAML),
	)
	policy := dir.Join("meta-policy.rego")
	input := dir.Join("config.yml")
	metaFile := dir.Join("meta.yml")
	metaJSON := `{"project_id": "test-project-id","vcs": {"branch": "main"}}`

	cases := []struct {
		name string
		args []string
	}{
		{"meta full tree", []string{"--meta", metaJSON}},
		{"metafile full tree", []string{"--metafile", metaFile}},
		{"meta with query", []string{"--meta", metaJSON, "--query", "data.org.enable_rule"}},
		{"metafile with query", []string{"--metafile", metaFile, "--query", "data.org.enable_rule"}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			env := testenv.New(t)
			args := append([]string{"policy", "eval", policy, "--input", input, "--no-compile"}, tc.args...)
			result := binary.RunCLI(t, binary.RunOpts{
				Binary:  binaryPath,
				Args:    args,
				Env:     env.Environ(),
				WorkDir: dir.Path(),
			})
			assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
			assert.Check(t, golden.String(normalizePolicyOutput(result.Stdout, dir.Path()), t.Name()+".txt"))
		})
	}
}

// TestPolicyEval_Compile exercises the compile path: the config is compiled via
// the API and spliced under "_compiled_" before evaluation. Mirrors the v0
// "config compilation is enabled" case.
func TestPolicyEval_Compile(t *testing.T) {
	fake := fakes.NewCircleCI(t)
	fake.SetCompileResponse(true, "version: \"2.1\"\nsentinel: compiled-marker\n")

	env := testenv.New(t)
	env.Token = testToken
	env.CircleCIURL = fake.URL()

	dir := fs.NewDir(t, "policy",
		fs.WithFile("policy.rego", evalBasicPolicyRego),
		fs.WithFile("input.yml", evalInputYAML),
	)

	// Query the spliced compiled document to prove compilation happened.
	result := binary.RunCLI(t, binary.RunOpts{
		Binary: binaryPath,
		Args: []string{
			"policy", "eval", dir.Join("policy.rego"),
			"--input", dir.Join("input.yml"),
			"--org", testOwnerID,
			"--query", "input._compiled_.sentinel",
		},
		Env:     env.Environ(),
		WorkDir: dir.Path(),
	})

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	assert.Check(t, golden.String(normalizePolicyOutput(result.Stdout, dir.Path()), t.Name()+".txt"))
	assert.Check(t, cmp.Equal(fake.LastCompileOwnerID(), testOwnerID))
}

// TestPolicyEval_Errors covers the v0 eval failure cases.
func TestPolicyEval_Errors(t *testing.T) {
	dir := fs.NewDir(t, "policy",
		fs.WithFile("policy.rego", evalBasicPolicyRego),
		fs.WithFile("input.yml", evalInputYAML),
	)
	policy := dir.Join("policy.rego")
	input := dir.Join("input.yml")

	cases := []struct {
		name     string
		args     []string
		wantExit int // exact expected exit code
	}{
		// Cobra argument/flag errors are unclassified → ExitGeneralError (1).
		{"missing policy arg", []string{"policy", "eval", "--input", input, "--no-compile"}, 1},
		{"missing input flag", []string{"policy", "eval", policy, "--no-compile"}, 1},
		// Structured CLIErrors carry their own exit codes.
		{"input file not found", []string{"policy", "eval", policy, "--input", dir.Join("no_such.yml"), "--no-compile"}, 2},
		{"policy path not found", []string{"policy", "eval", dir.Join("no_such.rego"), "--input", input, "--no-compile"}, 7},
		{"meta and metafile conflict", []string{"policy", "eval", policy, "--input", input, "--meta", "{}", "--metafile", "somefile", "--no-compile"}, 2},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			env := testenv.New(t)
			result := binary.RunCLI(t, binary.RunOpts{
				Binary:  binaryPath,
				Args:    tc.args,
				Env:     env.Environ(),
				WorkDir: dir.Path(),
			})
			assert.Check(t, cmp.Equal(result.ExitCode, tc.wantExit), "stderr: %s", result.Stderr)
			assert.Check(t, golden.String(normalizePolicyOutput(result.Stderr, dir.Path()), t.Name()+".stderr.txt"))
		})
	}
}

// --- policy test ---

// TestPolicyTest covers the default run over the standard test_policies fixture:
// both tests pass, so the failures-only table is empty and only the summary
// prints.
func TestPolicyTest(t *testing.T) {
	env := testenv.New(t) // no token: neither test compiles.
	dir := stdTestPoliciesDir(t)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"policy", "test", dir.Path()},
		Env:     env.Environ(),
		WorkDir: dir.Path(),
	})

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	assert.Check(t, golden.String(normalizePolicyOutput(result.Stdout, dir.Path()), t.Name()+".txt"))
}

func TestPolicyTest_All(t *testing.T) {
	env := testenv.New(t)
	dir := stdTestPoliciesDir(t)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"policy", "test", dir.Path(), "--all"},
		Env:     env.Environ(),
		WorkDir: dir.Path(),
	})

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	assert.Check(t, golden.String(normalizePolicyOutput(result.Stdout, dir.Path()), t.Name()+".txt"))
}

func TestPolicyTest_AllRun(t *testing.T) {
	env := testenv.New(t)
	dir := stdTestPoliciesDir(t)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"policy", "test", dir.Path(), "--all", "--run", "test_main"},
		Env:     env.Environ(),
		WorkDir: dir.Path(),
	})

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	assert.Check(t, golden.String(normalizePolicyOutput(result.Stdout, dir.Path()), t.Name()+".txt"))
}

// TestPolicyTest_Explain covers --explain, which prints each test's full
// evaluation context.
func TestPolicyTest_Explain(t *testing.T) {
	env := testenv.New(t)
	dir := stdTestPoliciesDir(t)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"policy", "test", dir.Path(), "--explain"},
		Env:     env.Environ(),
		WorkDir: dir.Path(),
	})

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	assert.Check(t, golden.String(normalizePolicyOutput(result.Stdout, dir.Path()), t.Name()+".txt"))
}

// TestPolicyTest_JSON pins the JSON array shape, rendered through the shared
// output helpers.
func TestPolicyTest_JSON(t *testing.T) {
	env := testenv.New(t)
	dir := stdTestPoliciesDir(t)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"policy", "test", dir.Path(), "--json"},
		Env:     env.Environ(),
		WorkDir: dir.Path(),
	})

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	assert.Check(t, golden.String(normalizePolicyOutput(result.Stdout, dir.Path()), t.Name()+".txt"))
}

// TestPolicyTest_JSONJQ pins that --json routes through the shared output
// helpers, so --jq can filter the results array.
func TestPolicyTest_JSONJQ(t *testing.T) {
	env := testenv.New(t)
	dir := stdTestPoliciesDir(t)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"policy", "test", dir.Path(), "--json", "--jq", ".[].Name"},
		Env:     env.Environ(),
		WorkDir: dir.Path(),
	})

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	assert.Check(t, golden.String(normalizePolicyOutput(result.Stdout, dir.Path()), t.Name()+".txt"))
}

func TestPolicyTest_JUnit(t *testing.T) {
	env := testenv.New(t)
	dir := stdTestPoliciesDir(t)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"policy", "test", dir.Path(), "--junit"},
		Env:     env.Environ(),
		WorkDir: dir.Path(),
	})

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	assert.Check(t, golden.String(normalizePolicyOutput(result.Stdout, dir.Path()), t.Name()+".txt"))
}

// TestPolicyTest_JUnitBeatsJSON pins the precedence: --junit wins over --json.
func TestPolicyTest_JUnitBeatsJSON(t *testing.T) {
	env := testenv.New(t)
	dir := stdTestPoliciesDir(t)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"policy", "test", dir.Path(), "--junit", "--json"},
		Env:     env.Environ(),
		WorkDir: dir.Path(),
	})

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	assert.Check(t, golden.String(normalizePolicyOutput(result.Stdout, dir.Path()), t.Name()+".txt"))
}

// TestPolicyTest_Failure covers a failing test: the diff is shown and the
// command exits non-zero.
func TestPolicyTest_Failure(t *testing.T) {
	env := testenv.New(t)
	dir := fs.NewDir(t, "policy",
		fs.WithFile("policy.rego", testPolicyRego),
		fs.WithFile("policy_test.yaml", `test_wrong:
  meta:
    vcs:
      branch: main
  decision:
    status: HARD_FAIL
`),
	)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"policy", "test", dir.Path()},
		Env:     env.Environ(),
		WorkDir: dir.Path(),
	})

	assert.Equal(t, result.ExitCode, 7, "expected ExitValidationFail; stderr: %s", result.Stderr)
	assert.Check(t, golden.String(normalizePolicyOutput(result.Stdout, dir.Path()), t.Name()+".txt"))
}

// TestPolicyTest_Compile runs a test that requires config compilation, served by
// the fake compile endpoint. Mirrors the v0 "compile" case.
func TestPolicyTest_Compile(t *testing.T) {
	fake := fakes.NewCircleCI(t)
	fake.SetCompileResponse(true, compiledSmallJobYAML)

	env := testenv.New(t)
	env.Token = testToken
	env.CircleCIURL = fake.URL()

	dir := fs.NewDir(t, "policy",
		fs.WithFile("compile.rego", compilePolicyRego),
		fs.WithFile("compile_test.yaml", compilePolicyTestsYAML),
	)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"policy", "test", dir.Path(), "--json", "--org", testOwnerID},
		Env:     env.Environ(),
		WorkDir: dir.Path(),
	})

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	assert.Check(t, golden.String(normalizePolicyOutput(result.Stdout, dir.Path()), t.Name()+".txt"))
	assert.Check(t, cmp.Equal(fake.LastCompileOwnerID(), testOwnerID))
}

// --- policy push ---

func TestPolicyPush(t *testing.T) {
	fake := fakes.NewCircleCI(t)

	env := testenv.New(t)
	env.Token = testToken
	env.CircleCIURL = fake.URL()

	dir := writePolicyDir(t, "my_policy", "package main\n")

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"policy", "push", dir, "--org", testOwnerID, "--no-prompt"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	assert.Check(t, strings.Contains(result.Stdout, "created") || strings.Contains(result.Stdout, "updated") || strings.Contains(result.Stdout, "deleted"))

	t.Run("check request", func(t *testing.T) {
		body, err := json.Marshal(map[string]any{
			"policies": map[string]string{
				filepath.Join(dir, "my_policy.rego"): "package main\n",
			},
		})
		assert.NilError(t, err)
		assert.Check(t, cmp.DeepEqual(fake.LastRequest(), &httprecorder.Request{
			Method: http.MethodPost,
			URL:    url.URL{Path: "/api/v2/owner/" + testOwnerID + "/context/" + testPolicyCtx + "/policy-bundle"},
			Header: http.Header{
				"Authorization": {"Bearer test-token"},
				"User-Agent":    {httpcl.UserAgent(runtime.GOOS, runtime.GOARCH, "dev", "")},
			},
			Body: new(string(body)),
		}, ignoreCommonHeaders))
	})
}

func TestPolicyPush_DryRun(t *testing.T) {
	fake := fakes.NewCircleCI(t)

	env := testenv.New(t)
	env.Token = testToken
	env.CircleCIURL = fake.URL()

	dir := writePolicyDir(t, "my_policy", "package main\n")

	// With --no-prompt, it first does a dry-run diff then applies.
	// We test the JSON output matches the diff format.
	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"policy", "push", dir, "--org", testOwnerID, "--no-prompt", "--json"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	var out struct {
		Created []string `json:"created"`
	}
	assert.NilError(t, json.Unmarshal([]byte(result.Stdout), &out))
	assert.Equal(t, len(out.Created), 1)
	assert.Equal(t, out.Created[0], "my_policy.rego")
}

// --- policy diff ---

func TestPolicyDiff(t *testing.T) {
	fake := fakes.NewCircleCI(t)

	env := testenv.New(t)
	env.Token = testToken
	env.CircleCIURL = fake.URL()

	dir := writePolicyDir(t, "my_policy", "package main\n")

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"policy", "diff", dir, "--org", testOwnerID},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	assert.Check(t, strings.Contains(result.Stdout, "created") || strings.Contains(result.Stdout, "updated") || strings.Contains(result.Stdout, "deleted"))
}

// --- policy fetch ---

func TestPolicyFetch(t *testing.T) {
	fake := fakes.NewCircleCI(t)
	fake.AddPolicyBundle(testOwnerID, testPolicyCtx, map[string]string{
		"my_policy": "package main\n",
	})

	env := testenv.New(t)
	env.Token = testToken
	env.CircleCIURL = fake.URL()

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"policy", "fetch", "--org", testOwnerID},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	assert.Check(t, strings.Contains(result.Stdout, "my_policy"))
}

func TestPolicyFetch_JSON(t *testing.T) {
	fake := fakes.NewCircleCI(t)
	fake.AddPolicyBundle(testOwnerID, testPolicyCtx, map[string]string{
		"my_policy": "package main\n",
	})

	env := testenv.New(t)
	env.Token = testToken
	env.CircleCIURL = fake.URL()

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"policy", "fetch", "--org", testOwnerID, "--json"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	var out map[string]any
	assert.NilError(t, json.Unmarshal([]byte(result.Stdout), &out))
	assert.Check(t, out["my_policy"] != nil)
}

func TestPolicyFetch_ByName(t *testing.T) {
	fake := fakes.NewCircleCI(t)
	fake.AddPolicyBundle(testOwnerID, testPolicyCtx, map[string]string{
		"my_policy": "package main\n",
	})

	env := testenv.New(t)
	env.Token = testToken
	env.CircleCIURL = fake.URL()

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"policy", "fetch", "my_policy", "--org", testOwnerID},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	assert.Check(t, strings.Contains(result.Stdout, "my_policy"))
}

// --- policy logs ---

func TestPolicyLogs(t *testing.T) {
	fake := fakes.NewCircleCI(t)
	fake.AddDecisionLog(testOwnerID, testPolicyCtx, map[string]any{
		"id":     testDecisionID,
		"status": "PASS",
	})

	env := testenv.New(t)
	env.Token = testToken
	env.CircleCIURL = fake.URL()

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"policy", "logs", "--org", testOwnerID},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	assert.Check(t, strings.Contains(result.Stdout, testDecisionID))
	assert.Check(t, strings.Contains(result.Stdout, "PASS"))
}

func TestPolicyLogs_ByDecisionID(t *testing.T) {
	fake := fakes.NewCircleCI(t)
	fake.AddDecisionLog(testOwnerID, testPolicyCtx, map[string]any{
		"id":     testDecisionID,
		"status": "SOFT_FAIL",
	})

	env := testenv.New(t)
	env.Token = testToken
	env.CircleCIURL = fake.URL()

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"policy", "logs", testDecisionID, "--org", testOwnerID},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	assert.Check(t, strings.Contains(result.Stdout, "SOFT_FAIL"))
}

// --- policy decide ---

func TestPolicyDecide(t *testing.T) {
	fake := fakes.NewCircleCI(t)
	fake.SetDecisionResult(testOwnerID, testPolicyCtx, map[string]any{
		"status": "PASS",
	})

	env := testenv.New(t)
	env.Token = testToken
	env.CircleCIURL = fake.URL()

	dir := t.TempDir()
	configFile := filepath.Join(dir, "config.yml")
	if err := os.WriteFile(configFile, []byte("version: 2.1\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"policy", "decide", "--org", testOwnerID, "--input", configFile},
		Env:     env.Environ(),
		WorkDir: dir,
	})

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	assert.Check(t, strings.Contains(result.Stdout, "PASS"))

	t.Run("check request", func(t *testing.T) {
		assert.Check(t, cmp.DeepEqual(fake.LastRequest(), &httprecorder.Request{
			Method: http.MethodPost,
			URL:    url.URL{Path: "/api/v2/owner/" + testOwnerID + "/context/" + testPolicyCtx + "/decision"},
			Header: http.Header{
				"Authorization": {"Bearer test-token"},
				"User-Agent":    {httpcl.UserAgent(runtime.GOOS, runtime.GOARCH, "dev", "")},
			},
			Body: new(`{"input":"version: 2.1\n"}`),
		}, ignoreCommonHeaders))
	})
}

func TestPolicyDecide_Strict(t *testing.T) {
	fake := fakes.NewCircleCI(t)
	fake.SetDecisionResult(testOwnerID, testPolicyCtx, map[string]any{
		"status": "HARD_FAIL",
	})

	env := testenv.New(t)
	env.Token = testToken
	env.CircleCIURL = fake.URL()

	dir := t.TempDir()
	configFile := filepath.Join(dir, "config.yml")
	if err := os.WriteFile(configFile, []byte("version: 2.1\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"policy", "decide", "--org", testOwnerID, "--input", configFile, "--strict"},
		Env:     env.Environ(),
		WorkDir: dir,
	})

	assert.Equal(t, result.ExitCode, 7, "expected ExitValidationFail, stderr: %s", result.Stderr) // ExitValidationFail
	assert.Check(t, strings.Contains(result.Stdout, "HARD_FAIL"))
}

// --- policy settings ---

func TestPolicySettingsGet(t *testing.T) {
	fake := fakes.NewCircleCI(t)
	fake.SetPolicyEnabled(testOwnerID, testPolicyCtx, true)

	env := testenv.New(t)
	env.Token = testToken
	env.CircleCIURL = fake.URL()

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"policy", "settings", "get", "--org", testOwnerID},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	assert.Check(t, strings.Contains(result.Stdout, "true"))
}

func TestPolicySettingsGet_JSON(t *testing.T) {
	fake := fakes.NewCircleCI(t)
	fake.SetPolicyEnabled(testOwnerID, testPolicyCtx, true)

	env := testenv.New(t)
	env.Token = testToken
	env.CircleCIURL = fake.URL()

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"policy", "settings", "get", "--org", testOwnerID, "--json"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	var out map[string]any
	assert.NilError(t, json.Unmarshal([]byte(result.Stdout), &out))
	assert.Check(t, out["enabled"] == true)
}

func TestPolicySettingsSet(t *testing.T) {
	fake := fakes.NewCircleCI(t)

	env := testenv.New(t)
	env.Token = testToken
	env.CircleCIURL = fake.URL()

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"policy", "settings", "set", "--org", testOwnerID, "--enabled"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	assert.Check(t, strings.Contains(result.Stdout, "enabled"))

	t.Run("check request", func(t *testing.T) {
		assert.Check(t, cmp.DeepEqual(fake.LastRequest(), &httprecorder.Request{
			Method: http.MethodPatch,
			URL:    url.URL{Path: "/api/v2/owner/" + testOwnerID + "/context/" + testPolicyCtx + "/decision/settings"},
			Header: http.Header{
				"Authorization": {"Bearer test-token"},
				"User-Agent":    {httpcl.UserAgent(runtime.GOOS, runtime.GOARCH, "dev", "")},
			},
			Body: new(`{"enabled":true}`),
		}, ignoreCommonHeaders))
	})
}

func TestPolicySettingsSet_JSON(t *testing.T) {
	fake := fakes.NewCircleCI(t)

	env := testenv.New(t)
	env.Token = testToken
	env.CircleCIURL = fake.URL()

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"policy", "settings", "set", "--org", testOwnerID, "--enabled", "--json"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	var out map[string]any
	assert.NilError(t, json.Unmarshal([]byte(result.Stdout), &out))
	assert.Check(t, out["enabled"] == true)
}

// --- no token ---

func TestPolicy_NoToken(t *testing.T) {
	env := testenv.New(t)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"policy", "fetch", "--org", testOwnerID},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 3) // ExitAuthError
}

// --- removed flag ---

// TestPolicy_RemovedOwnerIDFlag pins the clean break from --owner-id: every
// policy command now takes --org, and the old name must no longer be accepted.
func TestPolicy_RemovedOwnerIDFlag(t *testing.T) {
	env := testenv.New(t)
	env.Token = testToken

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"policy", "fetch", "--owner-id", testOwnerID},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Check(t, result.ExitCode != 0)
	assert.Check(t, cmp.Contains(result.Stderr, "unknown flag: --owner-id"))
}
