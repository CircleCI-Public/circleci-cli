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
	"os"
	"path/filepath"
	"slices"
	"testing"

	"gotest.tools/v3/assert"
	"gotest.tools/v3/assert/cmp"

	"github.com/CircleCI-Public/circleci-cli/internal/configoptimize/engine"
	"github.com/CircleCI-Public/circleci-cli/internal/configoptimize/module"
	"github.com/CircleCI-Public/circleci-cli/internal/configoptimize/module/resourceclass"
	"github.com/CircleCI-Public/circleci-cli/internal/configoptimize/pipelineconfig"
	"github.com/CircleCI-Public/circleci-cli/internal/configoptimize/pricing"
	"github.com/CircleCI-Public/circleci-cli/internal/configoptimize/registry"
	"github.com/CircleCI-Public/circleci-cli/internal/configoptimize/report"
	"github.com/CircleCI-Public/circleci-cli/internal/configoptimize/usage"
	"github.com/CircleCI-Public/circleci-cli/internal/configoptimize/validation"
)

type fixedCompiler struct{ compiled []byte }

func (f fixedCompiler) Compile(context.Context, pipelineconfig.CompileInput) (pipelineconfig.CompileResult, error) {
	return pipelineconfig.CompileResult{CompiledYAML: f.compiled, Compiler: "circleci (fixture)"}, nil
}

func read(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", name)) //#nosec:G304 // test fixture
	assert.NilError(t, err)
	return b
}

// ladder is an in-test pricing.Provider with the public Docker gen1 rates
// and sizes; modules and their tests may not import the table itself.
type ladder struct{ pricing.None }

var classes = []struct {
	name         string
	credits      float64
	vcpus, ramGB float64
}{
	{"small", 5, 1, 2}, {"medium", 10, 2, 4}, {"medium+", 15, 3, 6}, {"large", 20, 4, 8},
	{"xlarge", 40, 8, 16}, {"2xlarge", 80, 16, 32}, {"2xlarge+", 100, 20, 40},
}

func (ladder) CreditsPerMinute(platform, class, generation string) (float64, bool) {
	for _, c := range classes {
		if platform == "docker" && generation == "gen1" && c.name == class {
			return c.credits, true
		}
	}
	return 0, false
}

func (ladder) LadderBelow(platform, class, generation string) (string, bool) {
	for i, c := range classes {
		if platform == "docker" && generation == "gen1" && c.name == class && i > 0 {
			return classes[i-1].name, true
		}
	}
	return "", false
}

func (ladder) Size(platform, class, generation string) (float64, float64, bool) {
	for _, c := range classes {
		if platform == "docker" && generation == "gen1" && c.name == class {
			return c.vcpus, c.ramGB, true
		}
	}
	return 0, 0, false
}

func table(*testing.T) pricing.Provider { return ladder{} }

// findingFor is byJob's finding on job; the test stops when there is none.
func findingFor(t *testing.T, byJob map[string]report.Finding, job string) report.Finding {
	t.Helper()
	f, ok := byJob[job]
	assert.Assert(t, ok, "no finding for %s", job)
	return f
}

// anyMateriality turns off R1's materiality threshold for tests of the
// rule's other conditions and of the edit, whose fixture has many equal jobs.
func anyMateriality(t *testing.T) {
	t.Helper()
	old := resourceclass.MinPipelineSaving
	resourceclass.MinPipelineSaving = 0
	t.Cleanup(func() { resourceclass.MinPipelineSaving = old })
}

func TestDownsize(t *testing.T) {
	anyMateriality(t)
	u, err := usage.Parse(read(t, "usage.json"))
	assert.NilError(t, err)
	doc, err := engine.Analyze(context.Background(), engine.AnalyzeInput{
		Path: "jobs.yml", Config: read(t, "jobs.yml"),
		Modules:  []module.Analyzer{resourceclass.New(table(t), u)},
		Compiler: fixedCompiler{compiled: read(t, "jobs.compiled.yml")},
		Policy:   registry.Policy(),
	})
	assert.NilError(t, err)
	byJob := map[string]report.Finding{}
	for _, f := range doc.Findings {
		byJob[f.Target.Job] = f
	}

	t.Run("an oversized job steps down one class", func(t *testing.T) {
		f := findingFor(t, byJob, "oversized")
		assert.Check(t, cmp.Equal(f.Verdict, "RC_DOWNSIZE"))
		assert.Check(t, cmp.Equal(f.Disposition, "actionable"))
		assert.Check(t, cmp.Equal(f.Suggestion.Value, "large"))
		assert.Check(t, cmp.DeepEqual(f.AuthoredPaths, []string{"jobs.oversized.resource_class"}))
		assert.Check(t, cmp.Contains(f.Impact.Basis, "1.00× the duration on large at 0.50× the rate"))
		assert.Check(t, cmp.Contains(f.Confidence.Reason, "paired real runs decide"))
	})
	unchanged := []struct {
		name string // the job
		code string
	}{
		{name: "few-runs", code: resourceclass.CodeFewRuns},
		{name: "short", code: resourceclass.CodeShortJob},
		{name: "memory-bound", code: resourceclass.CodeMemory},
		{name: "small-class", code: resourceclass.CodeSmallClass},
		{name: "class-mismatch", code: resourceclass.CodeClassMismatch},
		{name: "machine", code: resourceclass.CodeNotDocker},
	}
	for _, tc := range unchanged {
		t.Run("no change: "+tc.name, func(t *testing.T) {
			f := findingFor(t, byJob, tc.name)
			assert.Check(t, cmp.Equal(f.Verdict, "RC_NO_CHANGE"))
			assert.Check(t, cmp.DeepEqual(f.ReasonCodes, []string{tc.code}))
		})
	}
	// A class the job inherits is set on the job itself, and the shared
	// source is left alone.
	inherited := []struct {
		name string // the job
		why  string
	}{
		{name: "shared-a", why: "the executor is shared with shared-b"},
		{name: "merged", why: "the class comes from a merge key"},
		{name: "merged-own", why: "the job's own class sits beside a merge key"},
	}
	for _, tc := range inherited {
		t.Run("an inherited class is set on the job: "+tc.name, func(t *testing.T) {
			f := findingFor(t, byJob, tc.name)
			assert.Check(t, cmp.Equal(f.Verdict, "RC_DOWNSIZE"))
			assert.Check(t, cmp.Equal(f.Disposition, "actionable"), tc.why)
			assert.Check(t, cmp.DeepEqual(f.AuthoredPaths, []string{"jobs." + tc.name + ".resource_class"}), tc.why)
		})
	}
	// Saturation is a caveat, not a gate. One heavy run in twelve
	// raises the expected slowdown a little; it cannot cost more credits.
	t.Run("a saturated run is a caveat, not a gate", func(t *testing.T) {
		f := findingFor(t, byJob, "saturated")
		assert.Check(t, cmp.Equal(f.Verdict, "RC_DOWNSIZE"))
		var caveat bool
		for _, e := range f.Evidence {
			caveat = caveat || e.Claim == "caveat"
		}
		assert.Check(t, caveat, "the saturated run is named as a caveat")
	})
	t.Run("a job with no usage data is reported, not skipped", func(t *testing.T) {
		f := findingFor(t, byJob, "no-data")
		assert.Check(t, cmp.DeepEqual(f.ReasonCodes, []string{resourceclass.CodeNoUsage}))
	})
}

// A usage file in another shape (such as a Usage API export) is refused.
func TestUsageShape(t *testing.T) {
	_, err := usage.Parse([]byte(`{"jobs": {"a": {"runs": [{"duration_seconds": 60, "MAX_CPU_UTILIZATION_PCT": 100}]}}}`))
	assert.Check(t, cmp.ErrorContains(err, "unknown field"))
}

// Without every job priced, the pipeline's credits are unknown, so no
// share, and no materiality, can be claimed.
func TestIncompletePipeline(t *testing.T) {
	u, err := usage.Parse(read(t, "usage.json"))
	assert.NilError(t, err)
	doc, err := engine.Analyze(context.Background(), engine.AnalyzeInput{
		Path: "jobs.yml", Config: read(t, "jobs.yml"),
		Modules:  []module.Analyzer{resourceclass.New(table(t), u)},
		Compiler: fixedCompiler{compiled: read(t, "jobs.compiled.yml")},
		Policy:   registry.Policy(),
	})
	assert.NilError(t, err)
	i := slices.IndexFunc(doc.Findings, func(f report.Finding) bool { return f.Target.Job == "oversized" })
	assert.Assert(t, i >= 0, "no finding for %s", "oversized")
	f := doc.Findings[i]
	assert.Check(t, cmp.DeepEqual(f.ReasonCodes, []string{resourceclass.CodeShareUnknown}))
	assert.Check(t, cmp.Contains(f.Suggestion.Note, "no-data"), "the note names the unpriced jobs")
}

// Materiality on a pipeline with four 2xlarge jobs whose CPU demand
// matches the measured one. The saving must be material for the
// pipeline, not just for the job.
func TestMateriality(t *testing.T) {
	const cfg = `version: 2
jobs:
  test: {docker: [{image: cimg/go:1.26}], resource_class: 2xlarge, parallelism: 2, steps: [checkout]}
  cover: {docker: [{image: cimg/go:1.26}], resource_class: 2xlarge, steps: [checkout]}
  build: {docker: [{image: cimg/go:1.26}], resource_class: 2xlarge, steps: [checkout]}
  lint: {docker: [{image: cimg/go:1.26}], resource_class: 2xlarge, steps: [checkout]}
workflows: {w: {jobs: [test, cover, build, lint]}}
`
	// Each job: its duration, and how many of 20 samples are busy at what
	// CPU (the rest are idle at 10%).
	shape := map[string]struct {
		seconds  float64
		busy     int
		busyPct  float64
		expected string
	}{
		"test":  {369, 7, 90, "RC_DOWNSIZE"},                  // 35% of samples at 14.4 cores
		"cover": {344, 4, 80, "RC_DOWNSIZE"},                  // 20% at 12.8 cores
		"build": {184, 15, 100, resourceclass.CodeRightSized}, // 75% at 16 cores
		"lint":  {80, 10, 80, resourceclass.CodeImmaterial},   // 50% at 12.8, but a small job
	}
	jobs := map[string]usage.Job{}
	for name, s := range shape {
		// A leading idle sample, which the per-container format drops as
		// usually skewed, then the 20 that count.
		cpu := make([]float64, 21)
		for i := range cpu {
			cpu[i] = 10
			if i > 0 && i <= s.busy {
				cpu[i] = s.busyPct
			}
		}
		containers := 1
		if name == "test" {
			containers = 2
		}
		runs := make([]usage.Run, 0, 12)
		for range 12 {
			ex := make([]usage.Execution, 0, containers)
			for i := range containers {
				ex = append(ex, usage.Execution{Index: i, CPUPct: cpu, MemoryPct: []float64{10, 10}})
			}
			runs = append(runs, usage.Run{DurationSeconds: s.seconds, Outcome: "succeeded", Executions: ex})
		}
		jobs[name] = usage.Job{ResourceClass: "2xlarge", Runs: runs}
	}
	doc, err := engine.Analyze(context.Background(), engine.AnalyzeInput{
		Path: "output.yml", Config: []byte(cfg),
		Modules:  []module.Analyzer{resourceclass.New(table(t), &usage.Data{SchemaVersion: usage.SchemaPerContainer, Jobs: jobs})},
		Compiler: fixedCompiler{compiled: []byte(cfg)},
		Policy:   registry.Policy(),
	})
	assert.NilError(t, err)
	for _, f := range doc.Findings {
		t.Run(f.Target.Job, func(t *testing.T) {
			s, ok := shape[f.Target.Job]
			assert.Assert(t, ok, "a finding on unknown job %s", f.Target.Job)
			got := f.Verdict
			if got == "RC_NO_CHANGE" {
				assert.Assert(t, len(f.ReasonCodes) > 0, "no reason code: %s", f.Suggestion.Note)
				got = f.ReasonCodes[0]
			}
			assert.Check(t, cmp.Equal(got, s.expected), "note: %s", f.Suggestion.Note)
			var predicted bool
			for _, e := range f.Evidence {
				predicted = predicted || e.Claim == "predicted"
			}
			assert.Check(t, predicted, "every R1 verdict states its prediction")
		})
	}
	assert.Check(t, cmp.Len(doc.Findings, 4))
}

// Runs without memory samples cannot pass the memory condition.
func TestNoMemorySamples(t *testing.T) {
	anyMateriality(t)
	const cfg = "version: 2\njobs:\n  j: {docker: [{image: cimg/base:current}], resource_class: xlarge, steps: [checkout]}\n" +
		"workflows: {w: {jobs: [j]}}\n"
	runs := make([]usage.Run, 0, 12)
	for range 12 {
		runs = append(runs, usage.Run{DurationSeconds: 120, CPUPct: []float64{8, 8, 8, 8}})
	}
	doc, err := engine.Analyze(context.Background(), engine.AnalyzeInput{
		Path: "j.yml", Config: []byte(cfg),
		Modules:  []module.Analyzer{resourceclass.New(table(t), &usage.Data{Jobs: map[string]usage.Job{"j": {ResourceClass: "xlarge", Runs: runs}}})},
		Compiler: fixedCompiler{compiled: []byte(cfg)},
		Policy:   registry.Policy(),
	})
	assert.NilError(t, err)
	assert.Assert(t, cmp.Len(doc.Findings, 1))
	assert.Check(t, cmp.DeepEqual(doc.Findings[0].ReasonCodes, []string{resourceclass.CodeNoMemory}))
}

// A candidate that paired real runs rejected is not proposed again,
// only while the record matches it exactly.
func TestValidatedResults(t *testing.T) {
	anyMateriality(t)
	u, err := usage.Parse(read(t, "usage.json"))
	assert.NilError(t, err)
	analyze := func(t *testing.T, rs ...validation.Record) map[string]report.Finding {
		t.Helper()
		doc, err := engine.Analyze(context.Background(), engine.AnalyzeInput{
			Path: "jobs.yml", Config: read(t, "jobs.yml"),
			Modules:  []module.Analyzer{resourceclass.New(table(t), u).WithResults(rs)},
			Compiler: fixedCompiler{compiled: read(t, "jobs.compiled.yml")},
			Policy:   registry.Policy(),
		})
		assert.NilError(t, err)
		byJob := map[string]report.Finding{}
		for _, f := range doc.Findings {
			byJob[f.Target.Job] = f
		}
		return byJob
	}
	before := analyze(t)
	var downsized []string
	for job, f := range before {
		if f.Verdict == "RC_DOWNSIZE" {
			downsized = append(downsized, job)
			assert.Check(t, f.Keys["fingerprint"] != "", job)
		}
	}
	assert.Assert(t, cmp.Len(downsized, 5))
	edit := func(job string) validation.Edit {
		f := before[job]
		to, _ := f.Suggestion.Value.(string)
		return validation.Edit{From: f.Keys["class"], To: to, Fingerprint: f.Keys["fingerprint"], FingerprintVersion: f.Keys["fingerprint_version"]}
	}
	record := func(decision string, jobs ...string) validation.Record {
		r := validation.Record{ID: "exp", Status: validation.StatusMeasured, Rule: resourceclass.Rule, Decision: decision,
			AlgorithmVersion: resourceclass.Version, Candidate: map[string]validation.Edit{}}
		for _, j := range jobs {
			r.Candidate[j] = edit(j)
		}
		return r
	}
	count := func(byJob map[string]report.Finding) (n int) {
		for _, f := range byJob {
			if f.Verdict == "RC_DOWNSIZE" {
				n++
			}
		}
		return n
	}

	t.Run("the exact rejected candidate is not proposed", func(t *testing.T) {
		after := analyze(t, record(validation.DecisionMoreExpensive, downsized...))
		assert.Check(t, cmp.Equal(count(after), 0))
		f := findingFor(t, after, "oversized")
		assert.Check(t, cmp.DeepEqual(f.ReasonCodes, []string{resourceclass.CodeValidatedRejected}))
		assert.Check(t, cmp.Contains(f.Suggestion.Note, "not proposed again"))
		assert.Check(t, cmp.Equal(f.Impact.Referent, "rejected_option"))
	})
	t.Run("a rejected subset of a larger candidate suppresses nothing", func(t *testing.T) {
		after := analyze(t, record(validation.DecisionRejected, "oversized", "saturated"))
		assert.Check(t, cmp.Equal(count(after), 5))
	})
	t.Run("a single-edit rejection suppresses that job only", func(t *testing.T) {
		after := analyze(t, record(validation.DecisionRejected, "oversized"))
		assert.Check(t, cmp.Equal(count(after), 4))
		assert.Check(t, cmp.Equal(findingFor(t, after, "oversized").Verdict, "RC_NO_CHANGE"))
	})
	t.Run("an attributable failure suppresses its own job", func(t *testing.T) {
		r := record(validation.DecisionRejected, "oversized", "saturated")
		r.Attributable = map[string]string{"saturated": "oom"}
		after := analyze(t, r)
		saturated := findingFor(t, after, "saturated")
		assert.Check(t, cmp.Equal(saturated.Verdict, "RC_NO_CHANGE"))
		assert.Check(t, cmp.Contains(saturated.Suggestion.Note, "own oom"))
		assert.Check(t, cmp.Equal(findingFor(t, after, "oversized").Verdict, "RC_DOWNSIZE"))
	})
	t.Run("a cheaper candidate is still proposed", func(t *testing.T) {
		assert.Check(t, cmp.Equal(count(analyze(t, record(validation.DecisionCheaper, downsized...))), 5))
	})
	t.Run("the order of the records does not matter", func(t *testing.T) {
		one, all := record(validation.DecisionRejected, "oversized"), record(validation.DecisionRejected, downsized...)
		all.ID = "all"
		assert.Check(t, cmp.Equal(count(analyze(t, one, all)), 0))
		assert.Check(t, cmp.Equal(count(analyze(t, all, one)), 0))
	})
	t.Run("an attributable failure does not stop the whole-candidate match", func(t *testing.T) {
		r := record(validation.DecisionRejected, downsized...)
		r.Attributable = map[string]string{"saturated": "oom"}
		assert.Check(t, cmp.Equal(count(analyze(t, r)), 0))
	})
	t.Run("a cheaper result for the same edits cancels a rejection", func(t *testing.T) {
		lost, won := record(validation.DecisionMoreExpensive, downsized...), record(validation.DecisionCheaper, downsized...)
		won.ID = "won"
		assert.Check(t, cmp.Equal(count(analyze(t, lost, won)), 5))
	})
	t.Run("another pricing table keeps only a job's own failure", func(t *testing.T) {
		r := record(validation.DecisionRejected, "oversized", "saturated")
		r.PricedElsewhere = true
		assert.Check(t, cmp.Equal(count(analyze(t, r)), 5))
		r.Attributable = map[string]string{"saturated": "timeout"}
		r.Rule = "RC-R0-v1"
		after := analyze(t, r)
		assert.Check(t, cmp.Equal(count(after), 4), "a job's own timeout holds across rules and pricing")
		assert.Check(t, cmp.Equal(findingFor(t, after, "saturated").Verdict, "RC_NO_CHANGE"))
	})
	predicted := func(r *validation.Record, job string, saving float64) {
		r.Predicted.Jobs = map[string]struct {
			JobSaving *float64 `json:"job_saving"`
		}{job: {JobSaving: &saving}}
	}
	t.Run("a predicted saving that improved reopens a cost rejection only", func(t *testing.T) {
		now, ok := findingFor(t, before, "oversized").Metrics["job_saving"]
		assert.Assert(t, ok, "no job_saving metric on oversized")
		improved := record(validation.DecisionRejected, "oversized")
		predicted(&improved, "oversized", now-2*resourceclass.DriftSavingPoints)
		assert.Check(t, cmp.Equal(count(analyze(t, improved)), 5), "the saving is now much better than when rejected")
		worse := record(validation.DecisionRejected, "oversized")
		predicted(&worse, "oversized", now+2*resourceclass.DriftSavingPoints)
		assert.Check(t, cmp.Equal(count(analyze(t, worse)), 4), "a worse prediction keeps the rejection")
		oom := record(validation.DecisionRejected, "oversized", "saturated")
		oom.Attributable = map[string]string{"oversized": "oom"}
		predicted(&oom, "oversized", now-2*resourceclass.DriftSavingPoints)
		assert.Check(t, cmp.Equal(findingFor(t, analyze(t, oom), "oversized").Verdict, "RC_NO_CHANGE"), "drift never clears an OOM")
	})
	t.Run("across versions", func(t *testing.T) {
		now, ok := findingFor(t, before, "oversized").Metrics["job_saving"]
		assert.Assert(t, ok, "no job_saving metric on oversized")
		otherMinor := record(validation.DecisionRejected, "oversized")
		otherMinor.AlgorithmVersion = "2.0.0"
		predicted(&otherMinor, "oversized", now-2*resourceclass.DriftSavingPoints)
		assert.Check(t, cmp.Equal(count(analyze(t, otherMinor)), 4), "another MINOR's rejection holds, with no drift reopening")
		majorOnly := record(validation.DecisionRejected, "oversized")
		majorOnly.AlgorithmVersion, majorOnly.Rule = "", "RC-R1-v1"
		assert.Check(t, cmp.Equal(count(analyze(t, majorOnly)), 4), "a rule label maps to its MAJOR")
		otherMajor := record(validation.DecisionRejected, "oversized")
		otherMajor.AlgorithmVersion = "1.1.1"
		assert.Check(t, cmp.Equal(count(analyze(t, otherMajor)), 5), "another MAJOR's cost decision suppresses nothing")
		otherMajor.Attributable = map[string]string{"oversized": "oom"}
		assert.Check(t, cmp.Equal(count(analyze(t, otherMajor)), 4), "a job's own OOM holds across MAJOR versions")
		unknown := record(validation.DecisionRejected, "oversized")
		unknown.AlgorithmVersion, unknown.Rule = "", "custom"
		assert.Check(t, cmp.Equal(count(analyze(t, unknown)), 5), "an unknown version suppresses nothing")
	})
	t.Run("a ledger measurement belongs to no contestant", func(t *testing.T) {
		now, ok := findingFor(t, before, "oversized").Metrics["job_saving"]
		assert.Assert(t, ok, "no job_saving metric on oversized")
		m := record(validation.DecisionRejected, "oversized")
		m.Rule, m.AlgorithmVersion, m.Contestantless, m.Applicability = "", "", true, validation.Applicable
		assert.Check(t, cmp.Equal(count(analyze(t, m)), 4), "any version uses a measurement")
		improved := m
		improved.Predictions = []validation.Prediction{{AlgorithmVersion: resourceclass.Version,
			JobSaving: map[string]float64{"oversized": now - 2*resourceclass.DriftSavingPoints}}}
		assert.Check(t, cmp.Equal(count(analyze(t, improved)), 5), "this version's own prediction reopens it when the saving improved")
		otherMinor := m
		otherMinor.Predictions = []validation.Prediction{{AlgorithmVersion: "2.0.0",
			JobSaving: map[string]float64{"oversized": now - 2*resourceclass.DriftSavingPoints}}}
		assert.Check(t, cmp.Equal(count(analyze(t, otherMinor)), 4), "another version's prediction is not compared")
	})
	t.Run("a cheaper result does not cancel a job's own failure", func(t *testing.T) {
		oom := record(validation.DecisionRejected, "oversized", "saturated")
		oom.Attributable = map[string]string{"oversized": "oom"}
		won := record(validation.DecisionCheaper, "oversized")
		won.ID = "won"
		assert.Check(t, cmp.Equal(findingFor(t, analyze(t, oom, won), "oversized").Verdict, "RC_NO_CHANGE"))
	})
	t.Run("a changed job or another rule matches nothing", func(t *testing.T) {
		changed := record(validation.DecisionRejected, "oversized")
		e := changed.Candidate["oversized"]
		e.Fingerprint = "0000000000000000"
		changed.Candidate["oversized"] = e
		other := record(validation.DecisionRejected, "oversized")
		other.Rule, other.AlgorithmVersion = "RC-R0-v1", ""
		unkeyed := record(validation.DecisionRejected, "oversized")
		e = unkeyed.Candidate["oversized"]
		e.Fingerprint = ""
		unkeyed.Candidate["oversized"] = e
		tests := []struct {
			name string
			r    validation.Record
		}{
			{name: "fingerprint", r: changed},
			{name: "rule", r: other},
			{name: "no fingerprint", r: unkeyed},
		}
		for _, tc := range tests {
			t.Run(tc.name, func(t *testing.T) {
				assert.Check(t, cmp.Equal(count(analyze(t, tc.r)), 5))
			})
		}
	})
}
