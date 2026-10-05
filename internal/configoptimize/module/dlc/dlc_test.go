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
	"testing"

	"gotest.tools/v3/assert"
	"gotest.tools/v3/assert/cmp"
	"gotest.tools/v3/golden"

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

func samplePricing(*testing.T) pricing.Provider { return flatRate{} }

// analyzeFixture runs the whole analyze pipeline on testdata/<name>.yml with
// only the dlc module, as `--modules=dlc` would.
func analyzeFixture(t *testing.T, name string, p pricing.Provider) report.Document {
	t.Helper()
	return analyzeFixtureWith(t, name, p, reconcile.Policy{})
}

// analyzeFixtureWith runs the pipeline with a module policy. The fixture
// table uses no policy, so it tests eligibility; the product applies
// registry.Policy(), which keeps dlc report-only.
func analyzeFixtureWith(t *testing.T, name string, p pricing.Provider, policy reconcile.Policy) report.Document {
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

// firstFinding is doc's first finding; the test stops when there is none.
func firstFinding(t *testing.T, doc report.Document) report.Finding {
	t.Helper()
	assert.Assert(t, len(doc.Findings) > 0, "no findings")
	return doc.Findings[0]
}

// findingOn is doc's finding on job; the test stops when there is none.
func findingOn(t *testing.T, doc report.Document, job string) report.Finding {
	t.Helper()
	i := slices.IndexFunc(doc.Findings, func(f report.Finding) bool { return f.Target.Job == job })
	assert.Assert(t, i >= 0, "no finding for %s", job)
	return doc.Findings[i]
}

// want is one expected finding: its job, verdict and disposition.
type want struct {
	Job         string
	Verdict     string
	Disposition string
}

// TestFixtures is the dlc module's test table. Each case states its expected
// findings explicitly, and the full JSON document is also a generated golden
// (`task test -- ./internal/configoptimize/module/dlc/... -update`). The explicit list is the
// specification; the golden only catches unintended drift in the rest.
func TestFixtures(t *testing.T) {
	const (
		actionable = "actionable"
		reportOnly = "report_only"
	)
	tests := []struct {
		name string
		want []want
	}{
		// The two cases that catch real bugs, first.
		{name: "false-positive-echo", want: []want{{"build", "DLC_UNUSED", actionable}}},
		{name: "unverifiable-script", want: []want{{"build", "DLC_UNVERIFIABLE", reportOnly}}},

		{name: "unused-simple", want: []want{{"test", "DLC_UNUSED", actionable}}},
		{name: "justified-build", want: []want{{"image", "DLC_BUILD_DETECTED", reportOnly}}},
		{name: "justified-compose", want: []want{{"image", "DLC_BUILD_DETECTED", reportOnly}}},
		{name: "justified-buildx", want: []want{{"image", "DLC_BUILD_DETECTED", reportOnly}}},
		{name: "justified-multiline", want: []want{{"image", "DLC_BUILD_DETECTED", reportOnly}}},
		{name: "justified-chained", want: []want{{"image", "DLC_BUILD_DETECTED", reportOnly}}},
		{name: "setup-remote-docker", want: []want{{"test", "DLC_UNUSED", actionable}}},
		{name: "machine-executor", want: []want{{"test", "DLC_UNUSED", actionable}}},
		// Detected on every job, then routed to report-only: never dropped.
		{name: "shared-executor", want: []want{
			{"docs", "DLC_UNUSED", reportOnly},
			{"lint", "DLC_UNUSED", reportOnly},
			{"test", "DLC_UNUSED", reportOnly},
		}},
		{name: "orb-sourced", want: []want{{"tools/test", "DLC_UNUSED", reportOnly}}},
		{name: "orb-registry-justified", want: []want{{"docker/publish", "DLC_BUILD_DETECTED", reportOnly}}},
		{name: "not-enabled", want: nil},
		// Regression cases from code review.
		{name: "test-runner", want: []want{{"test", "DLC_UNVERIFIABLE", reportOnly}}},
		{name: "compose-and-task", want: []want{{"test", "DLC_UNVERIFIABLE", reportOnly}}},
		{name: "job-shell", want: []want{{"build", "DLC_UNVERIFIABLE", reportOnly}}},
		{name: "when-parameter", want: []want{{"build", "DLC_UNUSED", reportOnly}}},
		{name: "when-command", want: []want{{"build", "DLC_UNUSED", reportOnly}}},
		{name: "alias-key", want: []want{{"build", "DLC_UNUSED", reportOnly}}},
		{name: "machine-and-remote", want: []want{{"build", "DLC_UNUSED", actionable}}},
		// More regression cases from code review.
		{name: "env-job", want: []want{{"build", "DLC_UNVERIFIABLE", reportOnly}}},
		{name: "env-step", want: []want{{"build", "DLC_UNVERIFIABLE", reportOnly}}},
		{name: "shell-rcfile", want: []want{{"build", "DLC_UNVERIFIABLE", reportOnly}}},
		{name: "parameterized-executor", want: []want{
			{"building", "DLC_BUILD_DETECTED", reportOnly},
			{"safe", "DLC_UNUSED", reportOnly},
		}},
		{name: "alias-step", want: []want{{"target", "DLC_UNUSED", reportOnly}}},
		{name: "parallel-builds", want: []want{
			{"integration", "DLC_UNUSED", actionable},
			{"lint", "DLC_UNUSED", actionable},
			{"unit", "DLC_UNUSED", actionable},
		}},
	}
	p := samplePricing(t)
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			doc := analyzeFixture(t, tc.name, p)

			var got []want
			for _, f := range doc.Findings {
				got = append(got, want{f.Target.Job, f.Verdict, f.Disposition})
			}
			assert.Check(t, cmp.DeepEqual(got, tc.want))

			var buf bytes.Buffer
			assert.NilError(t, report.WriteJSON(&buf, doc))
			assert.Check(t, golden.String(buf.String(), filepath.Join("golden", tc.name+".json")))
		})
	}
}

func TestShapeOfEachVerdict(t *testing.T) {
	p := samplePricing(t)

	t.Run("unused proposes removal at the enabling path", func(t *testing.T) {
		f := firstFinding(t, analyzeFixture(t, "setup-remote-docker", p))
		assert.Check(t, cmp.Equal(f.Target.Path, "jobs.test"))
		assert.Check(t, cmp.DeepEqual(f.Locations, []string{"jobs.test.steps.1.setup_remote_docker.docker_layer_caching"}))
		assert.Check(t, cmp.DeepEqual(f.AuthoredPaths, []string{"jobs.test.steps.1.setup_remote_docker.docker_layer_caching"}))
		assert.Check(t, cmp.Equal(f.Suggestion.Op, "remove"))
		assert.Check(t, cmp.Equal(f.Impact.Cost, "better"))
		assert.Check(t, cmp.Equal(f.Confidence.Level, "high"))
	})
	t.Run("every enabling location is one all-or-nothing change, counted once", func(t *testing.T) {
		doc := analyzeFixture(t, "machine-and-remote", p)
		assert.Assert(t, cmp.Len(doc.Findings, 1))
		assert.Check(t, cmp.DeepEqual(doc.Findings[0].AuthoredPaths, []string{
			"jobs.build.machine.docker_layer_caching",
			"jobs.build.steps.0.setup_remote_docker.docker_layer_caching",
		}))
		var buf bytes.Buffer
		assert.NilError(t, report.WriteText(&buf, doc, report.TextOptions{Verbose: true}))
		assert.Check(t, cmp.Contains(buf.String(), "DLC_UNUSED on 1 job: 200 credits per job run each"))
	})
	t.Run("in the product, dlc is report-only but shows the eligible change", func(t *testing.T) {
		doc := analyzeFixtureWith(t, "machine-and-remote", p, registry.Policy())
		f := firstFinding(t, doc)
		assert.Check(t, cmp.Equal(f.Disposition, "report_only"))
		assert.Check(t, cmp.Contains(f.Reason, "report-only module"))
		assert.Check(t, cmp.Len(f.AuthoredPaths, 2), "the eligible change is still shown")
	})
	t.Run("a build that depends on pipeline inputs is never actionable", func(t *testing.T) {
		for _, name := range []string{"when-parameter", "when-command"} {
			t.Run(name, func(t *testing.T) {
				f := firstFinding(t, analyzeFixture(t, name, p))
				assert.Check(t, cmp.Contains(f.Reason, "input-dependent"))
			})
		}
	})
	t.Run("an executor a parameterized job may use is shared", func(t *testing.T) {
		f := findingOn(t, analyzeFixture(t, "parameterized-executor", p), "safe")
		assert.Check(t, cmp.Equal(f.Mutability, "shared-executor"), f.Reason)
	})
	t.Run("a command reached through an alias step is followed", func(t *testing.T) {
		f := firstFinding(t, analyzeFixture(t, "alias-step", p))
		assert.Check(t, cmp.Contains(f.Reason, "input-dependent"), f.Reason)
	})
	t.Run("an anchored key is shared", func(t *testing.T) {
		f := firstFinding(t, analyzeFixture(t, "alias-key", p))
		assert.Check(t, cmp.Equal(f.Mutability, "shared-source"), f.Reason)
	})
	t.Run("uninspectable jobs carry reason codes", func(t *testing.T) {
		tests := []struct {
			name  string // the fixture
			codes []string
		}{
			{name: "test-runner", codes: []string{"TEST_RUNNER"}},
			{name: "compose-and-task", codes: []string{"COMPOSE_UP", "TASK_RUNNER"}},
			{name: "job-shell", codes: []string{"UNSUPPORTED_SHELL"}},
			{name: "unverifiable-script", codes: []string{"EXTERNAL_SCRIPT"}},
			{name: "env-job", codes: []string{"SHELL_STATE"}},
			{name: "env-step", codes: []string{"SHELL_STATE"}},
			{name: "shell-rcfile", codes: []string{"UNSUPPORTED_SHELL"}},
		}
		for _, tc := range tests {
			t.Run(tc.name, func(t *testing.T) {
				f := firstFinding(t, analyzeFixture(t, tc.name, p))
				assert.Check(t, cmp.DeepEqual(f.ReasonCodes, tc.codes))
			})
		}
	})
	t.Run("unverifiable is low confidence and proposes nothing", func(t *testing.T) {
		f := firstFinding(t, analyzeFixture(t, "unverifiable-script", p))
		assert.Check(t, cmp.Equal(f.Suggestion.Op, "none"))
		assert.Check(t, cmp.Equal(f.Confidence.Level, "low"))
		assert.Check(t, cmp.Equal(f.Impact.Cost, "unknown"))
	})
	t.Run("justified proposes nothing", func(t *testing.T) {
		f := firstFinding(t, analyzeFixture(t, "justified-build", p))
		assert.Check(t, cmp.Equal(f.Suggestion.Op, "none"))
		assert.Check(t, cmp.Equal(f.Impact.Cost, "neutral"))
	})
	t.Run("the rate comes from pricing, never a run-count total", func(t *testing.T) {
		f := firstFinding(t, analyzeFixture(t, "unused-simple", p))
		assert.Assert(t, f.Impact.UnitCredits != nil)
		assert.Check(t, cmp.Equal(*f.Impact.UnitCredits, 200.0))
		assert.Check(t, cmp.Nil(f.Impact.CostCredits), "no run count in v1, so no credit total")
	})
	t.Run("without a pricing table no rate is shown", func(t *testing.T) {
		f := firstFinding(t, analyzeFixture(t, "unused-simple", pricing.None{}))
		assert.Check(t, cmp.Nil(f.Impact.UnitCredits))
		assert.Check(t, cmp.Contains(f.Impact.Basis, "no rate was given"))
	})
	t.Run("parallelism is noted against the per-run charge", func(t *testing.T) {
		f := findingOn(t, analyzeFixture(t, "parallel-builds", p), "unit")
		assert.Check(t, cmp.Contains(f.Impact.Basis, "parallelism 4"))
	})
	t.Run("the report multiplies the fee across jobs", func(t *testing.T) {
		var buf bytes.Buffer
		assert.NilError(t, report.WriteText(&buf, analyzeFixture(t, "parallel-builds", p), report.TextOptions{Verbose: true}))
		assert.Check(t, cmp.Contains(buf.String(),
			"DLC_UNUSED on 3 jobs: 200 credits per job run each, 600 credits per pipeline run in which all of them run"))
	})
}
