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

package dlc_test

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"

	"gotest.tools/v3/assert"
	"gotest.tools/v3/assert/cmp"
	"gotest.tools/v3/golden"

	"github.com/CircleCI-Public/circleci-cli/internal/cmdutil"
	"github.com/CircleCI-Public/circleci-cli/internal/configoptimize/engine"
	"github.com/CircleCI-Public/circleci-cli/internal/configoptimize/module"
	"github.com/CircleCI-Public/circleci-cli/internal/configoptimize/module/dlc"
	"github.com/CircleCI-Public/circleci-cli/internal/configoptimize/pipelineconfig"
	"github.com/CircleCI-Public/circleci-cli/internal/configoptimize/pricing"
	"github.com/CircleCI-Public/circleci-cli/internal/configoptimize/reconcile"
	"github.com/CircleCI-Public/circleci-cli/internal/configoptimize/registry"
	"github.com/CircleCI-Public/circleci-cli/internal/configoptimize/report"
)

// fixedCompiler returns the committed real-compiler output for a fixture.
type fixedCompiler struct{ compiled []byte }

func (f fixedCompiler) Compile(context.Context, pipelineconfig.CompileInput) (pipelineconfig.CompileResult, error) {
	return pipelineconfig.CompileResult{CompiledYAML: f.compiled, Compiler: "circleci (fixture)"}, nil
}

// flatRate is a pricing.Provider with one flat charge, injected the way the
// catalog injects the real table. Tests may not import the table either.
type flatRate struct{ pricing.None }

func (flatRate) FlatCharge(kind string) (float64, bool) {
	if kind == "dlc" {
		return 200, true
	}
	return 0, false
}

// analyze runs the whole analyze pipeline on testdata/<name>.yml with only
// the dlc module, as `--modules=dlc` would. With no policy it tests
// eligibility; the product applies registry.Policy(), which keeps dlc
// report-only.
func analyze(t *testing.T, name string, p pricing.Provider, policy reconcile.Policy) report.Document {
	t.Helper()
	src, err := os.ReadFile(filepath.Join("testdata", name+".yml")) //#nosec:G304 // test fixture
	assert.NilError(t, err)
	compiled, err := os.ReadFile(filepath.Join("testdata", name+".compiled.yml")) //#nosec:G304 // test fixture
	assert.NilError(t, err)
	doc, err := engine.Analyze(context.Background(), engine.AnalyzeInput{
		Path:     name + ".yml",
		Config:   src,
		Modules:  []module.Analyzer{dlc.New(p)},
		Compiler: fixedCompiler{compiled: compiled},
		Policy:   policy,
	})
	assert.NilError(t, err)
	return doc
}

// findingOn is doc's finding on job; the test stops when there is none.
func findingOn(t *testing.T, doc report.Document, job string) report.Finding {
	t.Helper()
	i := slices.IndexFunc(doc.Findings, func(f report.Finding) bool { return f.Target.Job == job })
	assert.Assert(t, i >= 0, "no finding for %s", job)
	return doc.Findings[i]
}

// result is the part of a finding that tells one case from another.
type result struct {
	Job         string
	Verdict     string
	Disposition string
	Reason      string   // the reason's label: what keeps it report-only
	Change      []string // the authored paths an actionable change edits
	Codes       []string
	// Evidence is where DLC is enabled, then what the steps showed: each
	// build, each step that cannot be inspected, or how many were read.
	Evidence []string
}

func results(doc report.Document) []result {
	out := make([]result, 0, len(doc.Findings))
	for _, f := range doc.Findings {
		label, _, _ := strings.Cut(f.Reason, ":")
		r := result{f.Target.Job, f.Verdict, f.Disposition, label, f.AuthoredPaths, f.ReasonCodes, nil}
		for _, e := range f.Evidence {
			if e.Value == "docker_layer_caching: true" {
				r.Evidence = append(r.Evidence, strings.TrimPrefix(e.Source, "config:"))
				continue
			}
			r.Evidence = append(r.Evidence, e.Value)
		}
		out = append(out, r)
	}
	return out
}

func onMachine(job string) string { return "jobs." + job + ".machine.docker_layer_caching" }

func onRemote(job string, step int) string {
	return "jobs." + job + ".steps." + strconv.Itoa(step) + ".setup_remote_docker.docker_layer_caching"
}

const (
	unused       = "DLC_UNUSED"
	built        = "DLC_BUILD_DETECTED"
	unverifiable = "DLC_UNVERIFIABLE"
	actionable   = "actionable"
	reportOnly   = "report_only"
	noChange     = "no change proposed"
)

// TestFindings is the dlc module's specification: every finding of every
// fixture, in report order. A job that is absent has no finding.
func TestFindings(t *testing.T) {
	tests := []struct {
		fixture string
		want    []result
	}{
		// Text that only mentions a build: in echo, in a comment, as a grep pattern.
		{fixture: "false-positive-echo", want: []result{
			{Job: "build", Verdict: unused, Disposition: actionable, Change: []string{onMachine("build")},
				Evidence: []string{onMachine("build"), "3 steps inspected"}},
		}},
		{fixture: "steps", want: []result{
			// One finding however many places enable DLC.
			{Job: "both", Verdict: unused, Disposition: actionable,
				Change:   []string{onMachine("both"), onRemote("both", 0)},
				Evidence: []string{onMachine("both"), onRemote("both", 0), "2 steps inspected"}},
			// Plain, buildx, compose, chained and multi-line builds are each found.
			{Job: "builds", Verdict: built, Disposition: reportOnly, Reason: noChange, Evidence: []string{
				onMachine("builds"), "step 1: docker build", "step 2: docker buildx build",
				"step 3: docker compose build", "step 4: docker build", "step 5: docker build",
			}},
			// The compiler writes a job environment as a list of maps.
			{Job: "job-env", Verdict: unverifiable, Disposition: reportOnly, Reason: noChange, Codes: []string{"SHELL_STATE"},
				Evidence: []string{onMachine("job-env"), "job: environment sets BASH_ENV, which can change what the steps run"}},
			{Job: "job-shell", Verdict: unverifiable, Disposition: reportOnly, Reason: noChange, Codes: []string{"UNSUPPORTED_SHELL"},
				Evidence: []string{onMachine("job-shell"), "step 0: runs under the shell pwsh"}},
			{Job: "opaque", Verdict: unverifiable, Disposition: reportOnly, Reason: noChange,
				Codes: []string{"COMPOSE_UP", "EXTERNAL_SCRIPT", "TASK_RUNNER", "TEST_RUNNER"}, Evidence: []string{
					onMachine("opaque"),
					"step 1: runs the script ./scripts/ci.sh",
					"step 2: go test runs tests, which can build images",
					"step 3: docker-compose up builds missing service images, and the compose file is not in the config",
					"step 4: go tool runs a tool or target the config does not show",
				}},
			{Job: "parallel", Verdict: unused, Disposition: actionable, Change: []string{onMachine("parallel")},
				Evidence: []string{onMachine("parallel"), "2 steps inspected"}},
			// docker pull and docker images build nothing.
			{Job: "remote", Verdict: unused, Disposition: actionable, Change: []string{onRemote("remote", 1)},
				Evidence: []string{onRemote("remote", 1), "3 steps inspected"}},
			// Environment on a docker image and on a run step.
			{Job: "step-env", Verdict: unverifiable, Disposition: reportOnly, Reason: noChange, Codes: []string{"SHELL_STATE"},
				Evidence: []string{
					onRemote("step-env", 0),
					"job: environment sets LD_PRELOAD, which can change what the steps run",
					"step 1: environment sets GOFLAGS, which can change what the steps run",
				}},
		}},
		// Detected on every job, then routed to report-only: never dropped.
		{fixture: "sharing", want: []result{
			{Job: "building", Verdict: built, Disposition: reportOnly, Reason: noChange,
				Evidence: []string{onMachine("building"), "step 0: docker build"}},
			// The build is only in the authored config, behind a pipeline parameter.
			{Job: "gated", Verdict: unused, Disposition: reportOnly, Reason: "input-dependent",
				Evidence: []string{onRemote("gated", 0), "1 steps inspected"}},
			// Shared with building, whose executor is a parameter.
			{Job: "safe", Verdict: unused, Disposition: reportOnly, Reason: "shared-executor",
				Evidence: []string{onMachine("safe"), "1 steps inspected"}},
			// Its only step is a command, reached through a YAML alias, that reads a pipeline value.
			{Job: "target", Verdict: unused, Disposition: reportOnly, Reason: "input-dependent",
				Evidence: []string{onMachine("target"), "1 steps inspected"}},
		}},
		// A registry orb's compiled commands are read like any other step.
		{fixture: "orb-registry-justified", want: []result{
			{Job: "docker/publish", Verdict: built, Disposition: reportOnly, Reason: noChange,
				Evidence: []string{onRemote("docker/publish", 1), "step 3: docker buildx build"}},
		}},
	}
	for _, tc := range tests {
		t.Run(tc.fixture, func(t *testing.T) {
			assert.Check(t, cmp.DeepEqual(results(analyze(t, tc.fixture, flatRate{}, reconcile.Policy{})), tc.want))
		})
	}
}

// TestReportJSON pins the whole document, envelope included, for one
// actionable and one report-only finding. Regenerate with
// `task test -- ./internal/configoptimize/module/dlc/... -update`.
func TestReportJSON(t *testing.T) {
	for _, name := range []string{"false-positive-echo", "orb-registry-justified"} {
		t.Run(name, func(t *testing.T) {
			var buf bytes.Buffer
			assert.NilError(t, cmdutil.WriteJSON(&buf, analyze(t, name, flatRate{}, reconcile.Policy{})))
			assert.Check(t, golden.String(buf.String(), filepath.Join("golden", name+".json")))
		})
	}
}

func TestShapeOfEachVerdict(t *testing.T) {
	doc := analyze(t, "steps", flatRate{}, reconcile.Policy{})

	t.Run("each verdict", func(t *testing.T) {
		tests := []struct {
			job, op, cost, confidence string
		}{
			{job: "remote", op: "remove", cost: "better", confidence: "high"},
			{job: "builds", op: "none", cost: "neutral", confidence: "high"},
			{job: "opaque", op: "none", cost: "unknown", confidence: "low"},
		}
		for _, tc := range tests {
			f := findingOn(t, doc, tc.job)
			assert.Check(t, cmp.Equal(f.Suggestion.Op, tc.op), tc.job)
			assert.Check(t, cmp.Equal(f.Impact.Cost, tc.cost), tc.job)
			assert.Check(t, cmp.Equal(f.Confidence.Level, tc.confidence), tc.job)
		}
	})
	t.Run("a shared change is located but not authored", func(t *testing.T) {
		f := findingOn(t, analyze(t, "sharing", flatRate{}, reconcile.Policy{}), "safe")
		assert.Check(t, cmp.DeepEqual(f.Locations, []string{onMachine("safe")}))
		assert.Check(t, cmp.Equal(f.Mutability, "shared-executor"))
		assert.Check(t, cmp.Contains(f.Reason, "executor shared is used by 2 jobs"))
	})
	t.Run("in the product, dlc is report-only but shows the eligible change", func(t *testing.T) {
		f := findingOn(t, analyze(t, "steps", flatRate{}, registry.Policy()), "both")
		assert.Check(t, cmp.Equal(f.Disposition, "report_only"))
		assert.Check(t, cmp.Contains(f.Reason, "report-only module"))
		assert.Check(t, cmp.Len(f.AuthoredPaths, 2), "the eligible change is still shown")
	})
	t.Run("the rate comes from pricing, never a run-count total", func(t *testing.T) {
		f := findingOn(t, doc, "remote")
		assert.Assert(t, f.Impact.UnitCredits != nil)
		assert.Check(t, cmp.Equal(*f.Impact.UnitCredits, 200.0))
		assert.Check(t, cmp.Nil(f.Impact.CostCredits), "no run count in v1, so no credit total")
		assert.Check(t, !strings.Contains(f.Impact.Basis, "parallelism"), f.Impact.Basis)
	})
	t.Run("without a pricing table no rate is shown", func(t *testing.T) {
		f := findingOn(t, analyze(t, "steps", pricing.None{}, reconcile.Policy{}), "remote")
		assert.Check(t, cmp.Nil(f.Impact.UnitCredits))
		assert.Check(t, cmp.Contains(f.Impact.Basis, "no rate was given"))
	})
	t.Run("parallelism is noted against the per-run charge", func(t *testing.T) {
		assert.Check(t, cmp.Contains(findingOn(t, doc, "parallel").Impact.Basis, "parallelism 4"))
	})
	t.Run("the report charges once per job however many places enable DLC", func(t *testing.T) {
		// both, parallel and remote: four enabling locations, three jobs.
		var buf bytes.Buffer
		assert.NilError(t, report.WriteText(&buf, doc, report.TextOptions{Verbose: true}))
		assert.Check(t, cmp.Contains(buf.String(),
			"DLC_UNUSED on 3 jobs: 200 credits per job run each, 600 credits per pipeline run in which all of them run"))
	})
}
