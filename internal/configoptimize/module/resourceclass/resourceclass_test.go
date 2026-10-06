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

package resourceclass_test

import (
	"context"
	"slices"
	"testing"

	"gotest.tools/v3/assert"
	"gotest.tools/v3/assert/cmp"

	"github.com/CircleCI-Public/circleci-cli/internal/configoptimize/engine"
	"github.com/CircleCI-Public/circleci-cli/internal/configoptimize/module"
	rc "github.com/CircleCI-Public/circleci-cli/internal/configoptimize/module/resourceclass"
	"github.com/CircleCI-Public/circleci-cli/internal/configoptimize/pipelineconfig"
	"github.com/CircleCI-Public/circleci-cli/internal/configoptimize/pricing"
	"github.com/CircleCI-Public/circleci-cli/internal/configoptimize/registry"
	"github.com/CircleCI-Public/circleci-cli/internal/configoptimize/report"
	"github.com/CircleCI-Public/circleci-cli/internal/configoptimize/usage"
)

type fixedCompiler struct{ compiled []byte }

func (f fixedCompiler) Compile(context.Context, pipelineconfig.CompileInput) (pipelineconfig.CompileResult, error) {
	return pipelineconfig.CompileResult{CompiledYAML: f.compiled, Compiler: "circleci (fixture)"}, nil
}

// analyze runs the module through the engine and returns its findings by job.
func analyze(t *testing.T, source, compiled string, p pricing.Provider, u *usage.Data) map[string]report.Finding {
	t.Helper()
	doc, err := engine.Analyze(context.Background(), engine.AnalyzeInput{
		Path: "config.yml", Config: []byte(source),
		Modules:  []module.Analyzer{rc.New(p, u)},
		Compiler: fixedCompiler{compiled: []byte(compiled)},
		Policy:   registry.Policy(),
	})
	assert.NilError(t, err)
	byJob := map[string]report.Finding{}
	for _, f := range doc.Findings {
		byJob[f.Target.Job] = f
	}
	return byJob
}

// findingFor is byJob's finding on job; the test stops when there is none.
func findingFor(t *testing.T, byJob map[string]report.Finding, job string) report.Finding {
	t.Helper()
	f, ok := byJob[job]
	assert.Assert(t, ok, "no finding for %s", job)
	return f
}

// outcome is a finding's verdict, or its reason code for no change.
func outcome(f report.Finding) string {
	if f.Verdict == string(rc.VerdictNoChange) && len(f.ReasonCodes) == 1 {
		return f.ReasonCodes[0]
	}
	return f.Verdict
}

// anyMateriality turns off R1's materiality threshold for tests of the
// rule's other conditions, whose fixture has many equal jobs.
func anyMateriality(t *testing.T) {
	t.Helper()
	old := rc.MinPipelineSaving
	rc.MinPipelineSaving = 0
	t.Cleanup(func() { rc.MinPipelineSaving = old })
}

// plainJobs compile to themselves.
const plainJobs = `  oversized: {docker: [{image: x}], resource_class: xlarge, steps: [checkout]}
  saturated: {docker: [{image: x}], resource_class: xlarge, steps: [checkout]}
  few-runs: {docker: [{image: x}], resource_class: xlarge, steps: [checkout]}
  short: {docker: [{image: x}], resource_class: xlarge, steps: [checkout]}
  memory-bound: {docker: [{image: x}], resource_class: large, steps: [checkout]}
  no-memory: {docker: [{image: x}], resource_class: xlarge, steps: [checkout]}
  small-class: {docker: [{image: x}], resource_class: medium, steps: [checkout]}
  class-mismatch: {docker: [{image: x}], resource_class: xlarge, steps: [checkout]}
  gen2: {docker: [{image: x}], resource_class: xlarge.gen2, steps: [checkout]}
  machine: {machine: {image: ubuntu-2204:current}, resource_class: large, steps: [checkout]}
  no-usage: {docker: [{image: x}], resource_class: xlarge, steps: [checkout]}
`

// jobsSource adds jobs that inherit their class from a shared executor or
// a merge key; jobsCompiled is what the compiler makes of them.
const (
	jobsSource = `version: 2.1
x-big: &big
  resource_class: xlarge
executors:
  shared-big: {docker: [{image: x}], resource_class: xlarge}
jobs:
` + plainJobs + `  shared-a: {executor: shared-big, steps: [checkout]}
  shared-b: {executor: shared-big, steps: [checkout]}
  merged:
    docker: [{image: x}]
    <<: *big
    steps: [checkout]
  merged-own:
    docker: [{image: x}]
    <<: *big
    resource_class: xlarge
    steps: [checkout]
`
	jobsCompiled = `version: 2
jobs:
` + plainJobs + `  shared-a: {docker: [{image: x}], resource_class: xlarge, steps: [checkout]}
  shared-b: {docker: [{image: x}], resource_class: xlarge, steps: [checkout]}
  merged: {docker: [{image: x}], resource_class: xlarge, steps: [checkout]}
  merged-own: {docker: [{image: x}], resource_class: xlarge, steps: [checkout]}
`
)

// idle is n two-minute runs at 8% CPU and 15% memory: 0.64 of xlarge's 8
// cores, so a step down to large costs no time.
func idle(n int) []usage.Run { return rc.Runs(n, 120, rc.Samples(8, 8), rc.Samples(8, 15)) }

// jobsUsage is first-format usage for jobsSource; shared-b and no-usage have
// none.
func jobsUsage() *usage.Data {
	return &usage.Data{Jobs: map[string]usage.Job{
		"oversized": {ResourceClass: "xlarge", Runs: idle(12)},
		// One heavy run in twelve, at 95% of the ceiling.
		"saturated":    {ResourceClass: "xlarge", Runs: append(idle(11), rc.Runs(1, 120, rc.Samples(8, 95), rc.Samples(8, 15))...)},
		"few-runs":     {ResourceClass: "xlarge", Runs: idle(rc.MinRuns - 1)},
		"short":        {ResourceClass: "xlarge", Runs: rc.Runs(12, 10, rc.Samples(1, 8), rc.Samples(1, 15))},
		"memory-bound": {ResourceClass: "large", Runs: rc.Runs(12, 120, rc.Samples(8, 8), rc.Samples(8, 66))}, // 5.3 of 8 GB; medium+ has 6
		"no-memory":    {ResourceClass: "xlarge", Runs: rc.Runs(12, 120, rc.Samples(8, 8), nil)},
		"small-class":  {ResourceClass: "medium", Runs: idle(12)},
		// Measured on another class than the config names.
		"class-mismatch": {ResourceClass: "2xlarge", Runs: idle(12)},
		"gen2":           {ResourceClass: "xlarge-gen2", Runs: idle(12)},
		"machine":        {ResourceClass: "large", Runs: idle(12)},
		"shared-a":       {ResourceClass: "xlarge", Runs: idle(12)},
		"merged":         {ResourceClass: "xlarge", Runs: idle(12)},
		"merged-own":     {ResourceClass: "xlarge", Runs: idle(12)},
	}}
}

func TestDownsize(t *testing.T) {
	anyMateriality(t)
	byJob := analyze(t, jobsSource, jobsCompiled, rc.PublicLadder, jobsUsage())

	tests := []struct {
		job  string
		want string // the verdict, or the reason code for no change
		why  string
	}{
		{job: "oversized", want: string(rc.VerdictDownsize)},
		{job: "saturated", want: string(rc.VerdictDownsize), why: "saturation is a caveat, not a gate"},
		{job: "shared-a", want: string(rc.VerdictDownsize), why: "the executor is shared with shared-b"},
		{job: "merged", want: string(rc.VerdictDownsize), why: "the class comes from a merge key"},
		{job: "merged-own", want: string(rc.VerdictDownsize), why: "the job's own class sits beside a merge key"},
		{job: "few-runs", want: rc.CodeFewRuns},
		{job: "short", want: rc.CodeShortJob},
		{job: "memory-bound", want: rc.CodeMemory},
		{job: "no-memory", want: rc.CodeNoMemory},
		{job: "small-class", want: rc.CodeSmallClass},
		{job: "class-mismatch", want: rc.CodeClassMismatch},
		{job: "gen2", want: rc.CodeUnsized, why: "xlarge-gen2 is the usage API's spelling of xlarge.gen2, which the ladder lacks"},
		{job: "machine", want: rc.CodeNotDocker},
		{job: "no-usage", want: rc.CodeNoUsage, why: "a job with no usage is reported, not skipped"},
	}
	for _, tc := range tests {
		t.Run(tc.job, func(t *testing.T) {
			f := findingFor(t, byJob, tc.job)
			assert.Check(t, cmp.Equal(outcome(f), tc.want), "%s; note: %s", tc.why, f.Suggestion.Note)
			if tc.want == string(rc.VerdictDownsize) {
				// An inherited class is set on the job itself; the shared
				// source is left alone.
				assert.Check(t, cmp.Equal(f.Disposition, "actionable"))
				assert.Check(t, cmp.Equal(f.Suggestion.Value, "large"))
				assert.Check(t, cmp.DeepEqual(f.AuthoredPaths, []string{"jobs." + tc.job + ".resource_class"}))
			}
		})
	}
	t.Run("the downsize states its prediction", func(t *testing.T) {
		f := findingFor(t, byJob, "oversized")
		assert.Check(t, cmp.Contains(f.Impact.Basis, "1.00× the duration on large at 0.50× the rate"))
		assert.Check(t, cmp.Contains(f.Confidence.Reason, "paired real runs decide"))
	})
	t.Run("a saturated run is named as a caveat", func(t *testing.T) {
		f := findingFor(t, byJob, "saturated")
		assert.Check(t, slices.ContainsFunc(f.Evidence, func(e report.Evidence) bool { return e.Claim == "caveat" }))
	})
}

// Rates, sizes and the ladder come only from the pricing provider.
func TestPricingProvider(t *testing.T) {
	anyMateriality(t)
	const cfg = "version: 2\njobs:\n  j: {docker: [{image: x}], resource_class: xlarge, steps: [checkout]}\n"
	u := &usage.Data{Jobs: map[string]usage.Job{"j": {ResourceClass: "xlarge", Runs: idle(12)}}}
	tests := []struct {
		name    string
		pricing pricing.Provider
		want    string
	}{
		{name: "the public ladder", pricing: rc.PublicLadder, want: string(rc.VerdictDownsize)},
		{name: "nothing below xlarge", pricing: rc.PublicLadder[4:], want: rc.CodeNoSmaller},
		{name: "no table", pricing: pricing.None{}, want: rc.CodeUnsized},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			f := findingFor(t, analyze(t, cfg, cfg, tc.pricing, u), "j")
			assert.Check(t, cmp.Equal(outcome(f), tc.want), "note: %s", f.Suggestion.Note)
		})
	}
}

// Without every job priced, the pipeline's credits are unknown, so no
// share, and no materiality, can be claimed.
func TestIncompletePipeline(t *testing.T) {
	f := findingFor(t, analyze(t, jobsSource, jobsCompiled, rc.PublicLadder, jobsUsage()), "oversized")
	assert.Check(t, cmp.DeepEqual(f.ReasonCodes, []string{rc.CodeShareUnknown}))
	assert.Check(t, cmp.Contains(f.Suggestion.Note, "no-usage"), "the note names the unpriced jobs")
}

// Materiality on a pipeline with four 2xlarge jobs: the saving must be
// material for the pipeline, not just for the job.
func TestMateriality(t *testing.T) {
	const cfg = `version: 2
jobs:
  test: {docker: [{image: x}], resource_class: 2xlarge, parallelism: 2, steps: [checkout]}
  cover: {docker: [{image: x}], resource_class: 2xlarge, steps: [checkout]}
  build: {docker: [{image: x}], resource_class: 2xlarge, steps: [checkout]}
  lint: {docker: [{image: x}], resource_class: 2xlarge, steps: [checkout]}
`
	// Each job: its duration, and how many of 20 samples are busy at what
	// CPU (the rest are idle at 10%).
	shape := map[string]struct {
		seconds  float64
		busy     int
		busyPct  float64
		expected string
	}{
		"test":  {369, 7, 90, string(rc.VerdictDownsize)}, // 35% of samples at 14.4 cores
		"cover": {344, 4, 80, string(rc.VerdictDownsize)}, // 20% at 12.8 cores
		"build": {184, 15, 100, rc.CodeRightSized},        // 75% at 16 cores
		"lint":  {80, 10, 80, rc.CodeImmaterial},          // 50% at 12.8, but a small job
	}
	jobs := map[string]usage.Job{}
	for name, s := range shape {
		// A leading idle sample, which the per-container format drops as
		// usually skewed, then the 20 that count.
		cpu := slices.Concat(rc.Samples(1, 10), rc.Samples(s.busy, s.busyPct), rc.Samples(20-s.busy, 10))
		containers := 1
		if name == "test" {
			containers = 2
		}
		runs := make([]usage.Run, 12)
		for i := range runs {
			runs[i] = usage.Run{DurationSeconds: s.seconds, Outcome: "succeeded"}
			for c := range containers {
				runs[i].Executions = append(runs[i].Executions, usage.Execution{Index: c, CPUPct: cpu, MemoryPct: rc.Samples(2, 10)})
			}
		}
		jobs[name] = usage.Job{ResourceClass: "2xlarge", Runs: runs}
	}
	byJob := analyze(t, cfg, cfg, rc.PublicLadder, &usage.Data{SchemaVersion: usage.SchemaPerContainer, Jobs: jobs})
	assert.Check(t, cmp.Len(byJob, len(shape)))
	for name, s := range shape {
		t.Run(name, func(t *testing.T) {
			f := findingFor(t, byJob, name)
			assert.Check(t, cmp.Equal(outcome(f), s.expected), "note: %s", f.Suggestion.Note)
			assert.Check(t, slices.ContainsFunc(f.Evidence, func(e report.Evidence) bool { return e.Claim == "predicted" }),
				"every R1 verdict states its prediction")
		})
	}
}

// A usage file in another shape (such as a Usage API export) is refused.
func TestUsageShape(t *testing.T) {
	_, err := usage.Parse([]byte(`{"jobs": {"a": {"runs": [{"duration_seconds": 60, "MAX_CPU_UTILIZATION_PCT": 100}]}}}`))
	assert.Check(t, cmp.ErrorContains(err, "unknown field"))
}
