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

package cache_test

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	"gotest.tools/v3/assert"
	"gotest.tools/v3/assert/cmp"

	"github.com/CircleCI-Public/circleci-cli/internal/configoptimize/engine"
	"github.com/CircleCI-Public/circleci-cli/internal/configoptimize/module"
	"github.com/CircleCI-Public/circleci-cli/internal/configoptimize/module/cache"
	"github.com/CircleCI-Public/circleci-cli/internal/configoptimize/module/dlc"
	"github.com/CircleCI-Public/circleci-cli/internal/configoptimize/pipelineconfig"
	"github.com/CircleCI-Public/circleci-cli/internal/configoptimize/pricing"
	"github.com/CircleCI-Public/circleci-cli/internal/configoptimize/registry"
	"github.com/CircleCI-Public/circleci-cli/internal/configoptimize/report"
)

// fixedCompiler returns the committed real-compiler output for a fixture.
type fixedCompiler struct{ compiled []byte }

func (f fixedCompiler) Compile(context.Context, pipelineconfig.CompileInput) (pipelineconfig.CompileResult, error) {
	return pipelineconfig.CompileResult{CompiledYAML: f.compiled, Compiler: "circleci 1.0.49536 (fixture)"}, nil
}

func fixture(t *testing.T, name string) ([]byte, []byte) {
	t.Helper()
	src, err := os.ReadFile(filepath.Join("testdata", name+".yml")) //#nosec:G304 // test fixture
	assert.NilError(t, err)
	compiled, err := os.ReadFile(filepath.Join("testdata", name+".compiled.yml")) //#nosec:G304 // test fixture
	assert.NilError(t, err)
	return src, compiled
}

// analyze runs the product pipeline (dlc and cache, product policy) on a
// fixture, as `config-optimize analyze` does.
func analyze(t *testing.T, name string) report.Document {
	t.Helper()
	src, compiled := fixture(t, name)
	doc, err := engine.Analyze(context.Background(), engine.AnalyzeInput{
		Path:     name + ".yml",
		Config:   src,
		Modules:  []module.Analyzer{dlc.New(pricing.None{}), cache.New()},
		Compiler: fixedCompiler{compiled: compiled},
		Policy:   registry.Policy(),
	})
	assert.NilError(t, err)
	return doc
}

func load(t *testing.T, name string) cache.Analysis {
	t.Helper()
	src, compiled := fixture(t, name)
	l, err := engine.Load(context.Background(), fixedCompiler{compiled: compiled}, src, nil)
	assert.NilError(t, err)
	return cache.Analyze(l.Effective)
}

func byVerdict(doc report.Document) map[string][]report.Finding {
	out := map[string][]report.Finding{}
	for _, f := range doc.Findings {
		out[f.Verdict] = append(out[f.Verdict], f)
	}
	return out
}

// policyOf is the verdict on job's restore policy; the test stops when the
// job has none.
func policyOf(t *testing.T, a cache.Analysis, job string) cache.PolicyVerdict {
	t.Helper()
	i := slices.IndexFunc(a.Policies, func(pv cache.PolicyVerdict) bool { return pv.Policy.Job == job })
	assert.Assert(t, i >= 0, "no restore policy in %s", job)
	return a.Policies[i]
}

// findingOf is the first finding on job; the test stops when there is none.
func findingOf(t *testing.T, doc report.Document, job string) report.Finding {
	t.Helper()
	i := slices.IndexFunc(doc.Findings, func(f report.Finding) bool { return f.Target.Job == job })
	assert.Assert(t, i >= 0, "no finding for %s", job)
	return doc.Findings[i]
}

// TestCacheOptimizationMatrix is the first cache acceptance test: the
// pinned cache-optimization-matrix pipelineconfig. Its baseline workflow keys every
// cache on {{ epoch }} with no fallback; its optimized workflow keys on a
// lockfile checksum.
func TestCacheOptimizationMatrix(t *testing.T) {
	doc := analyze(t, "cache-optimization-matrix")
	got := byVerdict(doc)

	volatile := got["CACHE_VOLATILE_RESTORE"]
	assert.Check(t, cmp.Len(volatile, 11), "one per baseline workflow call")
	for _, f := range volatile {
		assert.Check(t, cmp.Regexp(regexp.QuoteMeta("-baseline")+"$", f.Target.Job), "no finding on an optimized job")
		assert.Check(t, cmp.Regexp("^"+regexp.QuoteMeta("workflows.baseline.jobs."), f.Target.Site), "%s is reported at its call site", f.Target.Job)
		assert.Check(t, cmp.Regexp(regexp.QuoteMeta(".cache_key")+"$", f.Target.Site))
		assert.Check(t, cmp.Equal(f.Disposition, "report_only"))
		assert.Check(t, hasRule(f, cache.RuleX1), "the JSON evidence names the rule version")
		assert.Check(t, cmp.Equal(f.Confidence.Level, "medium"))
		// X1-static-v2: each has a single restore key with {{ epoch }} passed
		// through cache_key and no fallback, so every one is TIME_SCOPED.
		assert.Check(t, cmp.DeepEqual(f.ReasonCodes, []string{"TIME_SCOPED"}))
		assert.Check(t, cmp.Contains(f.Suggestion.Note, "embeds the time of each cache step"))
		assert.Check(t, cmp.Contains(f.Suggestion.Note, "Reuse is limited"))
		assert.Check(t, !strings.Contains(f.Suggestion.Note, "never hit"), f.Suggestion.Note)
		assert.Check(t, !strings.Contains(f.Suggestion.Note, "rerun"), "no claim about reruns until one is observed")
	}
	assert.Check(t, cmp.Len(got["CACHE_CANDIDATE"], 0))
	for v := range got {
		assert.Check(t, !strings.HasPrefix(v, "DLC_"), "no DLC findings, got %s", v)
	}
	assert.Check(t, cmp.Len(doc.Findings, 11))

	// No match edge between flask-baseline and flask-optimized, in either
	// direction: pip-flask-{{ epoch }} and pip-flask-{{ checksum … }} share a
	// raw prefix but not a typed template.
	a := load(t, "cache-optimization-matrix")
	for _, e := range a.Model.Edges {
		pair := []string{e.Policy.Job, e.Save.Job}
		assert.Check(t, !slices.Equal(pair, []string{"flask-baseline", "flask-optimized"}) &&
			!slices.Equal(pair, []string{"flask-optimized", "flask-baseline"}), "edge %v", pair)
		assert.Check(t, cmp.Equal(e.Save.Job, e.Policy.Job), "every restore here reads only its own job's save")
	}
}

func hasRule(f report.Finding, rule string) bool {
	for _, e := range f.Evidence {
		if e.Claim == "rule" && strings.HasPrefix(e.Value, rule+":") {
			return true
		}
	}
	return false
}

func TestFocusedCases(t *testing.T) {
	doc := analyze(t, "focused")
	byJob := map[string][]report.Finding{}
	for _, f := range doc.Findings {
		byJob[f.Target.Job] = append(byJob[f.Target.Job], f)
	}
	only := func(t *testing.T, job, verdict string) report.Finding {
		t.Helper()
		fs := byJob[job]
		assert.Assert(t, cmp.Len(fs, 1), "%s: %v", job, fs)
		assert.Check(t, cmp.Equal(fs[0].Verdict, verdict))
		return fs[0]
	}
	none := func(t *testing.T, job string) {
		t.Helper()
		assert.Check(t, cmp.Len(byJob[job], 0), "%s: %v", job, byJob[job])
	}

	t.Run("a stable fallback key means no finding", func(t *testing.T) { none(t, "stable-fallback") })
	t.Run("a call site that overrides a stable default with a volatile value", func(t *testing.T) {
		none(t, "keyed-default")
		f := only(t, "keyed-volatile", "CACHE_VOLATILE_RESTORE")
		assert.Check(t, cmp.Equal(f.Target.Site, "workflows.focused.jobs.2.keyed.cache_key"))
	})
	t.Run("a call site that overrides a volatile default with a stable value", func(t *testing.T) {
		f := only(t, "volatile-default-kept", "CACHE_VOLATILE_RESTORE")
		assert.Check(t, cmp.Equal(f.Target.Site, "jobs.volatile-default.parameters.cache_key.default"))
		none(t, "volatile-default-overridden")
	})
	t.Run("every variant of a matrix job", func(t *testing.T) {
		var variants int
		for job, fs := range byJob {
			if strings.HasPrefix(job, "matrixed-") {
				variants++
				assert.Assert(t, cmp.Len(fs, 1), job)
				assert.Check(t, cmp.Equal(fs[0].Verdict, "CACHE_VOLATILE_RESTORE"))
			}
		}
		assert.Check(t, cmp.Equal(variants, 2))
	})
	t.Run("a restore in another job reads the save", func(t *testing.T) {
		none(t, "cross-restore")
		a := load(t, "focused")
		var linked bool
		for _, e := range a.Model.Edges {
			if e.Policy.Job == "cross-restore" && e.Save.Job == "cross-save" && e.Exact {
				linked = true
			}
		}
		assert.Check(t, linked)
	})
	t.Run("an untraceable key is uninspectable, not a finding", func(t *testing.T) {
		none(t, "untraceable")
		pv := policyOf(t, load(t, "focused"), "untraceable")
		assert.Check(t, pv.Uninspectable)
		assert.Assert(t, len(pv.Keys) > 0, "the policy has no keys")
		assert.Check(t, cmp.Contains(pv.Keys[0].Reason, "can be overridden when the pipeline is triggered"))
		assert.Check(t, cmp.Equal(pv.Keys[0].Cause, cache.CauseValueUnavailable))
	})
	t.Run("each matrix variant is judged with its own value", func(t *testing.T) {
		only(t, "mixed-matrix-{{ epoch }}", "CACHE_VOLATILE_RESTORE")
		none(t, `mixed-matrix-{{ checksum "package-lock.json" }}`)
	})
	t.Run("an install with no cache is a candidate", func(t *testing.T) {
		f := only(t, "candidate", "CACHE_CANDIDATE")
		assert.Check(t, hasRule(f, cache.RuleF4))
		assert.Check(t, cmp.Equal(f.Confidence.Level, "low"))
	})
	t.Run("an unreviewed CircleCI function blocks the candidate", func(t *testing.T) { none(t, "function-run") })
	t.Run("a broad fallback is flagged, never merged", func(t *testing.T) {
		none(t, "broad-restore")
		a := load(t, "focused")
		var broad []string
		for _, e := range a.Model.Edges {
			if e.Policy.Job == "broad-restore" {
				assert.Assert(t, len(e.Save.Keys) > 0, "the save in %s has no key", e.Save.Job)
				assert.Check(t, e.Broad, "%s -> %s", e.Policy.Keys[e.Key].Raw, e.Save.Keys[0].Raw)
				broad = append(broad, e.Save.Job)
			}
		}
		slices.Sort(broad)
		assert.Check(t, cmp.DeepEqual(broad, []string{"broad-a", "broad-b"}))
	})
	t.Run("the JSON is deterministic", func(t *testing.T) {
		var first, second bytes.Buffer
		assert.NilError(t, report.WriteJSON(&first, analyze(t, "focused")))
		assert.NilError(t, report.WriteJSON(&second, analyze(t, "focused")))
		assert.Check(t, cmp.Equal(first.String(), second.String()))
	})
}

// pipeline.git.revision is substituted by the compiler, so it is read from
// the authored key. testdata may not hold ambient pipeline values, so the
// compiled copy is built here with a fixed SHA standing in for the checkout.
func TestPipelineRevisionKey(t *testing.T) {
	const sha = "0123456789abcdef0123456789abcdef01234567"
	authored := `version: 2.1
jobs:
  direct:
    docker: [{image: cimg/base:current}]
    steps:
      - restore_cache: {keys: ["rev-<< pipeline.git.revision >>"]}
      - save_cache: {key: "rev-<< pipeline.git.revision >>", paths: [~/.npm]}
  via-param:
    parameters:
      k: {type: string}
    docker: [{image: cimg/base:current}]
    steps:
      - restore_cache: {keys: ["p-<< parameters.k >>"]}
      - save_cache: {key: "p-<< parameters.k >>", paths: [~/.npm]}
workflows:
  w:
    jobs:
      - direct
      - via-param: {k: "<< pipeline.git.revision >>"}
`
	compiled := strings.ReplaceAll(`version: 2
jobs:
  direct:
    docker: [{image: cimg/base:current}]
    steps:
      - restore_cache: {keys: ["rev-SHA"]}
      - save_cache: {key: "rev-SHA", paths: [~/.npm]}
  via-param:
    docker: [{image: cimg/base:current}]
    steps:
      - restore_cache: {keys: ["p-SHA"]}
      - save_cache: {key: "p-SHA", paths: [~/.npm]}
workflows:
  w:
    jobs: [direct, via-param]
`, "SHA", sha)
	doc, err := engine.Analyze(context.Background(), engine.AnalyzeInput{
		Path: "revision.yml", Config: []byte(authored),
		Modules:  []module.Analyzer{cache.New()},
		Compiler: fixedCompiler{compiled: []byte(compiled)},
		Policy:   registry.Policy(),
	})
	assert.NilError(t, err)
	sites := map[string]string{}
	for _, f := range doc.Findings {
		assert.Check(t, cmp.Equal(f.Verdict, "CACHE_VOLATILE_RESTORE"))
		assert.Check(t, cmp.DeepEqual(f.ReasonCodes, []string{"REVISION_SCOPED"}))
		assert.Check(t, cmp.Contains(f.Suggestion.Note, "can't reuse caches from a different commit"))
		assert.Assert(t, len(f.Evidence) > 1, "%s: %v", f.Target.Job, f.Evidence)
		assert.Check(t, cmp.Equal(f.Evidence[1].Claim, "key 0 is REVISION_SCOPED"))
		sites[f.Target.Job] = f.Target.Site
	}
	assert.Check(t, cmp.DeepEqual(sites, map[string]string{
		"direct":    "jobs.direct.steps.0.restore_cache.keys.0",
		"via-param": "workflows.w.jobs.1.via-param.k",
	}))
}

// Regression cases found in code review.
func TestChangingKeysAndWrappers(t *testing.T) {
	doc := analyze(t, "changing-keys-and-wrappers")
	byJob := map[string][]report.Finding{}
	for _, f := range doc.Findings {
		byJob[f.Target.Job] = append(byJob[f.Target.Job], f)
	}
	a := load(t, "changing-keys-and-wrappers")
	policy := func(t *testing.T, job string) cache.PolicyVerdict {
		t.Helper()
		return policyOf(t, a, job)
	}
	none := func(t *testing.T, job string) {
		t.Helper()
		assert.Check(t, cmp.Len(byJob[job], 0), "%s: %v", job, byJob[job])
	}
	verdict := func(t *testing.T, job, want string) report.Finding {
		t.Helper()
		assert.Assert(t, cmp.Len(byJob[job], 1), "%s: %v", job, byJob[job])
		assert.Check(t, cmp.Equal(byJob[job][0].Verdict, want))
		return byJob[job][0]
	}

	t.Run("a changing key that may match a save of another shape is not a finding", func(t *testing.T) {
		none(t, "seed-restore")
		pv := policy(t, "seed-restore")
		assert.Check(t, pv.Uninspectable)
		assert.Check(t, cmp.Contains(pv.Reason, `may match "seed-42-seeded"`))
	})
	t.Run("an unknown value next to a changing one makes the key uninspectable", func(t *testing.T) {
		none(t, "adjacent-unknown")
		pv := policy(t, "adjacent-unknown")
		assert.Assert(t, len(pv.Keys) > 0, "the policy has no keys")
		assert.Check(t, cmp.Equal(pv.Keys[0].Cause, cache.CauseValueUnavailable))
	})
	t.Run("an install in an uncalled function or an on_fail step does not run", func(t *testing.T) {
		none(t, "function-body")
		none(t, "on-fail-only")
	})
	t.Run("a wrapped CircleCI function blocks the candidate", func(t *testing.T) { none(t, "wrapped-function") })
	t.Run("a wrapper option with a value does not hide the install", func(t *testing.T) {
		verdict(t, "env-unset", "CACHE_CANDIDATE")
	})
	t.Run("mvn --version is not an install", func(t *testing.T) { none(t, "mvn-version") })
	t.Run("matrix jobs whose names share a prefix are traced to their own job", func(t *testing.T) {
		verdict(t, "build-macos", "CACHE_VOLATILE_RESTORE")
		verdict(t, "build-linux-amd64", "CACHE_VOLATILE_RESTORE")
	})
	t.Run("an enum pipeline parameter whose values disagree depends on the scenario", func(t *testing.T) {
		none(t, "enum-mixed")
		pv := policy(t, "enum-mixed")
		assert.Assert(t, len(pv.Keys) > 0, "the policy has no keys")
		assert.Check(t, cmp.Equal(pv.Keys[0].Cause, cache.CauseScenario))
	})
	t.Run("a prefix fallback matching an exact save and a longer one is broad", func(t *testing.T) {
		var edges int
		for _, e := range a.Model.Edges {
			if e.Policy.Job == "broad-exact-restore" {
				edges++
				assert.Assert(t, len(e.Save.Keys) > 0, "the save in %s has no key", e.Save.Job)
				assert.Check(t, e.Broad, "%s -> %s", e.Policy.Keys[e.Key].Raw, e.Save.Keys[0].Raw)
			}
		}
		assert.Check(t, edges > 0, "no edge from broad-exact-restore")
	})
	t.Run("with no save in the config, the note claims no stored cache", func(t *testing.T) {
		f := verdict(t, "no-save", "CACHE_VOLATILE_RESTORE")
		assert.Check(t, cmp.DeepEqual(f.ReasonCodes, []string{"JOB_SCOPED"}), "a job-scoped key fires with no visible save")
	})
	t.Run("a setup (dynamic) config is uninspectable", func(t *testing.T) {
		doc := analyze(t, "setup-config")
		assert.Check(t, cmp.Len(doc.Findings, 0))
		sa := load(t, "setup-config")
		assert.Assert(t, len(sa.Policies) > 0, "the setup config has a restore policy")
		pv := sa.Policies[0]
		assert.Check(t, pv.Uninspectable)
		assert.Assert(t, len(pv.Keys) > 0, "the policy has no keys")
		assert.Check(t, cmp.Equal(pv.Keys[0].Cause, cache.CauseValueUnavailable))
		for _, jv := range sa.Jobs {
			assert.Check(t, !jv.Candidate(), "no candidate in a setup config: %s", jv.Job)
		}
	})
}

// Regression cases found in code review.
func TestSharedTokensAndWrapperForms(t *testing.T) {
	doc := analyze(t, "shared-tokens")
	byJob := map[string][]report.Finding{}
	for _, f := range doc.Findings {
		byJob[f.Target.Job] = append(byJob[f.Target.Job], f)
	}
	none := func(t *testing.T, job string) {
		t.Helper()
		assert.Check(t, cmp.Len(byJob[job], 0), "%s: %v", job, byJob[job])
	}
	verdict := func(t *testing.T, job, want string) report.Finding {
		t.Helper()
		assert.Assert(t, cmp.Len(byJob[job], 1), "%s: %v", job, byJob[job])
		assert.Check(t, cmp.Equal(byJob[job][0].Verdict, want))
		return byJob[job][0]
	}
	t.Run("a save sharing the changing token keeps the finding", func(t *testing.T) {
		verdict(t, "correlated-prefix", "CACHE_VOLATILE_RESTORE")
	})
	t.Run("an epoch key without spaces is medium confidence, with the caveat", func(t *testing.T) {
		f := verdict(t, "epoch-no-spaces", "CACHE_VOLATILE_RESTORE")
		assert.Check(t, cmp.Equal(f.Confidence.Level, "medium"))
		assert.Check(t, cmp.DeepEqual(f.ReasonCodes, []string{"TIME_SCOPED"}), "epoch is found by token type, not spelling")
	})
	t.Run("a called function's install is seen", func(t *testing.T) { verdict(t, "called-function", "CACHE_CANDIDATE") })
	t.Run("wrappers with long options and split strings", func(t *testing.T) {
		verdict(t, "sudo-long", "CACHE_CANDIDATE")
		verdict(t, "env-split", "CACHE_CANDIDATE")
	})
	t.Run("Maven: a path-qualified wrapper with a goal, not a version query", func(t *testing.T) {
		verdict(t, "mvnw-path", "CACHE_CANDIDATE")
		none(t, "mvn-file-version")
	})
	t.Run("a matrix alias does not rename the jobs", func(t *testing.T) {
		verdict(t, "aliased-linux", "CACHE_VOLATILE_RESTORE")
	})
	t.Run("an excluded tuple does not claim another invocation's job", func(t *testing.T) {
		verdict(t, "excluded-amd64-linux", "CACHE_VOLATILE_RESTORE")
	})
}

// A restore key whose change comes from a compile-time value can have the
// same compiled text as a hard-coded save: "deps-<< pipeline.number >>" is
// "deps-42" in pipeline 42, like a save of "deps-42" made by pipeline 41.
// That exact edge proves nothing, so there is no finding.
func TestExactEdgeToStableSave(t *testing.T) {
	authored := `version: 2.1
jobs:
  seed:
    docker: [{image: cimg/base:current}]
    steps:
      - save_cache: {key: deps-42, paths: [x]}
  restore:
    docker: [{image: cimg/base:current}]
    steps:
      - restore_cache: {keys: ["deps-<< pipeline.number >>"]}
workflows:
  w:
    jobs: [seed, restore]
`
	compiled := `version: 2
jobs:
  seed:
    docker: [{image: cimg/base:current}]
    steps:
      - save_cache: {key: deps-42, paths: [x]}
  restore:
    docker: [{image: cimg/base:current}]
    steps:
      - restore_cache: {keys: ["deps-42"]}
workflows:
  w:
    jobs: [seed, restore]
`
	l, err := engine.Load(context.Background(), fixedCompiler{compiled: []byte(compiled)}, []byte(authored), nil)
	assert.NilError(t, err)
	a := cache.Analyze(l.Effective)
	assert.Assert(t, cmp.Len(a.Policies, 1))
	pv := a.Policies[0]
	assert.Check(t, !pv.Volatile)
	assert.Check(t, cmp.Equal(pv.Cause, cache.CauseScenario))
	assert.Check(t, cmp.Contains(pv.Reason, "not built from the same scoped value"))
}

// analyzeInline runs the cache module on an authored config and a compiled
// copy built by hand, for cases that need pipeline values (testdata may not
// hold them).
func analyzeInline(t *testing.T, authored, compiled string) cache.Analysis {
	t.Helper()
	l, err := engine.Load(context.Background(), fixedCompiler{compiled: []byte(compiled)}, []byte(authored), nil)
	assert.NilError(t, err)
	return cache.Analyze(l.Effective)
}

// Regression cases found in code review.
func TestKeyCorrelation(t *testing.T) {
	t.Run("equal template text over different variables is not correlated", func(t *testing.T) {
		// Commit B saves deps-A (its base revision); a later pipeline on
		// commit A restores deps-A. No finding.
		a := analyzeInline(t, `version: 2.1
jobs:
  restore:
    parameters: {k: {type: string}}
    docker: [{image: cimg/base:current}]
    steps:
      - restore_cache: {keys: ["deps-<< parameters.k >>"]}
  seed:
    parameters: {k: {type: string}}
    docker: [{image: cimg/base:current}]
    steps:
      - save_cache: {key: "deps-<< parameters.k >>", paths: [x]}
workflows:
  w:
    jobs:
      - restore: {k: << pipeline.git.revision >>}
      - seed: {k: << pipeline.git.base_revision >>}
`, `version: 2
jobs:
  restore:
    docker: [{image: cimg/base:current}]
    steps:
      - restore_cache: {keys: ["deps-aaaa"]}
  seed:
    docker: [{image: cimg/base:current}]
    steps:
      - save_cache: {key: "deps-aaaa", paths: [x]}
workflows:
  w:
    jobs: [restore, seed]
`)
		assert.Assert(t, cmp.Len(a.Policies, 1))
		assert.Check(t, !a.Policies[0].Volatile)
		assert.Check(t, cmp.Equal(a.Policies[0].Cause, cache.CauseScenario))
	})
	t.Run("a prefix save built from the same changing value stays correlated", func(t *testing.T) {
		// deps-42 and deps-42-seeded both follow pipeline.number: no reuse
		// across pipelines. A finding.
		a := analyzeInline(t, `version: 2.1
jobs:
  build:
    parameters: {k: {type: string}}
    docker: [{image: cimg/base:current}]
    steps:
      - restore_cache: {keys: ["deps-<< parameters.k >>"]}
      - save_cache: {key: "deps-<< parameters.k >>-seeded", paths: [~/.npm]}
workflows:
  w:
    jobs:
      - build: {k: << pipeline.number >>}
`, `version: 2
jobs:
  build:
    docker: [{image: cimg/base:current}]
    steps:
      - restore_cache: {keys: ["deps-42"]}
      - save_cache: {key: "deps-42-seeded", paths: [~/.npm]}
workflows:
  w:
    jobs: [build]
`)
		assert.Assert(t, cmp.Len(a.Policies, 1))
		assert.Check(t, a.Policies[0].Volatile, a.Policies[0].Reason)
	})
}

func TestEnvSplitAndNarrowestToken(t *testing.T) {
	doc := analyze(t, "env-split")
	byJob := map[string][]report.Finding{}
	for _, f := range doc.Findings {
		byJob[f.Target.Job] = append(byJob[f.Target.Job], f)
	}
	verdict := func(t *testing.T, job, want string) report.Finding {
		t.Helper()
		assert.Assert(t, cmp.Len(byJob[job], 1), "%s: %v", job, byJob[job])
		assert.Check(t, cmp.Equal(byJob[job][0].Verdict, want))
		return byJob[job][0]
	}
	t.Run("env -S with quotes", func(t *testing.T) { verdict(t, "env-split-quoted", "CACHE_CANDIDATE") })
	t.Run("wrapper options that take a value", func(t *testing.T) {
		verdict(t, "env-argv0", "CACHE_CANDIDATE")
		verdict(t, "sudo-chroot", "CACHE_CANDIDATE")
	})
	t.Run("mvn --color never runs no goal", func(t *testing.T) {
		assert.Check(t, cmp.Len(byJob["mvn-color"], 0), "%v", byJob["mvn-color"])
	})
	t.Run("within one key the narrowest token wins, and confidence is medium", func(t *testing.T) {
		f := verdict(t, "epoch-and-buildnum", "CACHE_VOLATILE_RESTORE")
		assert.Check(t, cmp.DeepEqual(f.ReasonCodes, []string{"TIME_SCOPED"}), "{{ epoch }} is narrower than {{ .BuildNum }}")
		assert.Check(t, cmp.Equal(f.Confidence.Level, "medium"))
		assert.Check(t, cmp.Contains(f.Confidence.Reason, "how often the job runs"))
	})
}

// Regression cases found in code review.
func TestScenariosAndCallTracing(t *testing.T) {
	doc := analyze(t, "scenarios")
	byJob := map[string][]report.Finding{}
	for _, f := range doc.Findings {
		byJob[f.Target.Job] = append(byJob[f.Target.Job], f)
	}
	none := func(t *testing.T, job string) {
		t.Helper()
		assert.Check(t, cmp.Len(byJob[job], 0), "%s: %v", job, byJob[job])
	}
	verdict := func(t *testing.T, job, want string) report.Finding {
		t.Helper()
		assert.Assert(t, cmp.Len(byJob[job], 1), "%s: %v", job, byJob[job])
		assert.Check(t, cmp.Equal(byJob[job][0].Verdict, want))
		return byJob[job][0]
	}
	t.Run("different changing alternatives depend on the scenario", func(t *testing.T) {
		none(t, "enum-alternatives")
		pv := policyOf(t, load(t, "scenarios"), "enum-alternatives")
		assert.Assert(t, len(pv.Keys) > 0, "the policy has no keys")
		assert.Check(t, cmp.Equal(pv.Keys[0].Cause, cache.CauseScenario))
	})
	t.Run("env -S splits like env, then reads assignments", func(t *testing.T) {
		none(t, "env-split-semicolon")
		verdict(t, "env-split-assignment", "CACHE_CANDIDATE")
	})
	t.Run("mvn -itr verify runs a goal", func(t *testing.T) { verdict(t, "mvn-itr", "CACHE_CANDIDATE") })
	t.Run("a call runs the definition in force at that point", func(t *testing.T) {
		none(t, "function-redefined")
		none(t, "function-before-definition")
	})
	t.Run("a save in another job is not called the job's own", func(t *testing.T) {
		f := verdict(t, "cross-restore", "CACHE_VOLATILE_RESTORE")
		assert.Check(t, cmp.DeepEqual(f.ReasonCodes, []string{"JOB_SCOPED"}))
		assert.Check(t, cmp.Contains(f.Suggestion.Note, "normally can't reuse a cache from an earlier job"))
	})
}

func TestRevisionTracing(t *testing.T) {
	t.Run("a base revision can be shared by sibling commits", func(t *testing.T) {
		a := analyzeInline(t, `version: 2.1
jobs:
  build:
    docker: [{image: cimg/base:current}]
    steps:
      - restore_cache: {keys: ["deps-<< pipeline.git.base_revision >>"]}
      - save_cache: {key: "deps-<< pipeline.git.base_revision >>", paths: [x]}
workflows:
  w:
    jobs: [build]
`, `version: 2
jobs:
  build:
    docker: [{image: cimg/base:current}]
    steps:
      - restore_cache: {keys: ["deps-aaaa"]}
      - save_cache: {key: "deps-aaaa", paths: [x]}
workflows:
  w:
    jobs: [build]
`)
		assert.Assert(t, cmp.Len(a.Policies, 1))
		assert.Check(t, !a.Policies[0].Volatile)
		assert.Check(t, a.Policies[0].Uninspectable)
	})
	t.Run("reference spacing does not break correlation", func(t *testing.T) {
		a := analyzeInline(t, `version: 2.1
jobs:
  build:
    parameters: {k: {type: string}}
    docker: [{image: cimg/base:current}]
    steps:
      - restore_cache: {keys: ["deps-<<parameters.k>>"]}
      - save_cache: {key: "deps-<< parameters.k >>-seeded", paths: [~/.npm]}
workflows:
  w:
    jobs:
      - build: {k: << pipeline.number >>}
`, `version: 2
jobs:
  build:
    docker: [{image: cimg/base:current}]
    steps:
      - restore_cache: {keys: ["deps-42"]}
      - save_cache: {key: "deps-42-seeded", paths: [~/.npm]}
workflows:
  w:
    jobs: [build]
`)
		assert.Assert(t, cmp.Len(a.Policies, 1))
		assert.Check(t, a.Policies[0].Volatile, a.Policies[0].Reason)
	})
}

// Regression cases found in code review.
func TestPipelineAndJobScopes(t *testing.T) {
	t.Run("a pipeline-scoped key with no dependency paths is not reported", func(t *testing.T) {
		// seed saves deps-<< pipeline.id >> and consume restores it in the
		// same pipeline: sharing within a pipeline, like a workspace. Under
		// X1-static-v2 the claim "can't reuse caches from another pipeline"
		// is true, but PIPELINE_SCOPED fires only for dependency paths.
		a := analyzeInline(t, `version: 2.1
jobs:
  seed:
    docker: [{image: cimg/base:current}]
    steps:
      - save_cache: {key: "deps-<< pipeline.id >>", paths: [x]}
  consume:
    docker: [{image: cimg/base:current}]
    steps:
      - restore_cache: {keys: ["deps-<< pipeline.id >>"]}
workflows:
  w:
    jobs:
      - seed
      - consume: {requires: [seed]}
`, `version: 2
jobs:
  seed:
    docker: [{image: cimg/base:current}]
    steps:
      - save_cache: {key: "deps-0f00", paths: [x]}
  consume:
    docker: [{image: cimg/base:current}]
    steps:
      - restore_cache: {keys: ["deps-0f00"]}
workflows:
  w:
    jobs: [seed, {consume: {requires: [seed]}}]
`)
		assert.Assert(t, cmp.Len(a.Policies, 1))
		assert.Check(t, !a.Policies[0].Volatile)
		assert.Check(t, cmp.Equal(a.Policies[0].Scope, cache.ScopePipeline))
		assert.Check(t, cmp.Contains(a.Policies[0].Reason, "reported only for dependency caches"))
	})
	t.Run("a per-job key saved by another job still cannot be restored", func(t *testing.T) {
		doc := analyze(t, "scenarios")
		var found bool
		for _, f := range doc.Findings {
			if f.Target.Job == "cross-restore" {
				found = true
			}
		}
		assert.Check(t, found, "cj-{{ .BuildNum }} differs per job")
	})
	t.Run("a restore with no save does not claim this config saves the cache", func(t *testing.T) {
		f := findingOf(t, analyze(t, "changing-keys-and-wrappers"), "no-save")
		assert.Check(t, !strings.Contains(f.Suggestion.Note, "this config saves"), f.Suggestion.Note)
	})
}

// Regression cases found in code review.
func TestRevisionAliasesAndScopes(t *testing.T) {
	doc := analyze(t, "revision-aliases")
	byJob := map[string][]report.Finding{}
	for _, f := range doc.Findings {
		byJob[f.Target.Job] = append(byJob[f.Target.Job], f)
	}
	t.Run("the same revision under another name is the same key", func(t *testing.T) {
		// REVISION_SCOPED, but the save stores /tmp/deps, not a dependency
		// path, so it is not reported.
		assert.Check(t, cmp.Len(byJob["sha-consume"], 0), "%v", byJob["sha-consume"])
		pv := policyOf(t, load(t, "revision-aliases"), "sha-consume")
		assert.Check(t, cmp.Equal(pv.Scope, cache.ScopeRevision))
		assert.Check(t, pv.Saved, "the CIRCLE_SHA1 save is an exact match")
	})
	t.Run("a job-scoped key in a parallel job", func(t *testing.T) {
		// X1-static-v2: "normally can't reuse a cache from an earlier job"
		// stays true when parallel nodes of the same job share it.
		assert.Assert(t, cmp.Len(byJob["parallel"], 1))
		assert.Check(t, cmp.DeepEqual(byJob["parallel"][0].ReasonCodes, []string{"JOB_SCOPED"}))
	})
	t.Run("a pipeline id key is pipeline-scoped", func(t *testing.T) {
		a := analyzeInline(t, `version: 2.1
jobs:
  build:
    docker: [{image: cimg/base:current}]
    steps:
      - restore_cache: {keys: ["deps-<< pipeline.id >>"]}
      - save_cache: {key: "deps-<< pipeline.id >>", paths: [x]}
workflows:
  w:
    jobs: [build]
`, `version: 2
jobs:
  build:
    docker: [{image: cimg/base:current}]
    steps:
      - restore_cache: {keys: ["deps-0f00"]}
      - save_cache: {key: "deps-0f00", paths: [x]}
workflows:
  w:
    jobs: [build]
`)
		assert.Assert(t, cmp.Len(a.Policies, 1))
		assert.Assert(t, len(a.Policies[0].Keys) > 0, "the policy has no keys")
		kv := a.Policies[0].Keys[0]
		assert.Check(t, cmp.Equal(kv.Scope, cache.ScopePipeline))
	})
}

// Regression cases found in code review.
func TestEpochKeysAndSaveOrder(t *testing.T) {
	doc := analyze(t, "epoch-save-order")
	byJob := map[string][]report.Finding{}
	for _, f := range doc.Findings {
		byJob[f.Target.Job] = append(byJob[f.Target.Job], f)
	}
	t.Run("an epoch key in a parallel job still cannot be restored", func(t *testing.T) {
		assert.Assert(t, cmp.Len(byJob["parallel-epoch"], 1))
		f := byJob["parallel-epoch"][0]
		assert.Assert(t, len(f.Evidence) > 1, "%v", f.Evidence)
		assert.Check(t, cmp.Equal(f.Evidence[1].Claim, "key 0 is TIME_SCOPED"))
	})
	t.Run("a job-scoped save earlier in the same job", func(t *testing.T) {
		// X1-static-v2: the restore reuses this job's own save, which "can't
		// reuse a cache from an earlier job" allows.
		assert.Check(t, cmp.Len(byJob["save-first"], 1), "%v", byJob["save-first"])
	})
	t.Run("an earlier epoch save has another key by the time of the restore", func(t *testing.T) {
		assert.Check(t, cmp.Len(byJob["save-first-epoch"], 1), "%v", byJob["save-first-epoch"])
	})
	t.Run("the rule names the scope model", func(t *testing.T) {
		assert.Assert(t, cmp.Len(byJob["parallel-epoch"], 1))
		f := byJob["parallel-epoch"][0]
		assert.Check(t, hasRule(f, cache.RuleX1))
		i := slices.IndexFunc(f.Evidence, func(e report.Evidence) bool { return e.Claim == "rule" })
		assert.Assert(t, i >= 0, "no rule evidence: %v", f.Evidence)
		assert.Check(t, cmp.Contains(f.Evidence[i].Value, "broadest key"))
	})
}

// X1-static-v2 scopes.
func TestScopes(t *testing.T) {
	doc := analyze(t, "scopes")
	codes := map[string][]string{}
	for _, f := range doc.Findings {
		if f.Verdict == "CACHE_VOLATILE_RESTORE" {
			codes[f.Target.Job] = f.ReasonCodes
		}
	}
	scoped := func(t *testing.T, job, code string) {
		t.Helper()
		assert.Check(t, cmp.DeepEqual(codes[job], []string{code}), "%s", job)
	}
	none := func(t *testing.T, job string) {
		t.Helper()
		_, found := codes[job]
		assert.Check(t, !found, "%s: %v", job, codes[job])
	}
	t.Run("epoch then revision is REVISION_SCOPED", func(t *testing.T) { scoped(t, "epoch-then-revision", "REVISION_SCOPED") })
	t.Run("revision then epoch is REVISION_SCOPED", func(t *testing.T) { scoped(t, "revision-then-epoch", "REVISION_SCOPED") })
	t.Run("epoch with a stable fallback is no finding", func(t *testing.T) { none(t, "epoch-stable-fallback") })
	t.Run("one key with epoch and revision is TIME_SCOPED", func(t *testing.T) { scoped(t, "epoch-and-revision-one-key", "TIME_SCOPED") })
	t.Run("a pipeline parameter is not a built-in pipeline value", func(t *testing.T) {
		none(t, "pipeline-param-string")
		none(t, "pipeline-param-enum")
		a := load(t, "scopes")
		assert.Check(t, policyOf(t, a, "pipeline-param-string").Uninspectable, "a trigger can override it")
		enum := policyOf(t, a, "pipeline-param-enum")
		assert.Check(t, !enum.Uninspectable)
		assert.Check(t, cmp.Equal(enum.Scope, cache.Unscoped))
	})
	t.Run("a workflow id key is WORKFLOW_SCOPED, never pipeline-scoped", func(t *testing.T) {
		scoped(t, "workflow-id", "WORKFLOW_SCOPED")
	})
	t.Run("broad scopes need dependency paths", func(t *testing.T) {
		none(t, "revision-build-output")
		none(t, "revision-mixed-paths")
		none(t, "revision-param-path")
		none(t, "revision-no-save")
		a := load(t, "scopes")
		assert.Check(t, cmp.Equal(policyOf(t, a, "revision-build-output").PathClass, cache.BuildOutputPaths))
		assert.Check(t, cmp.Equal(policyOf(t, a, "revision-mixed-paths").PathClass, cache.MixedPaths))
		assert.Check(t, cmp.Equal(policyOf(t, a, "revision-param-path").PathClass, cache.ParameterizedPaths))
		assert.Check(t, cmp.Equal(policyOf(t, a, "revision-no-save").PathClass, cache.NoVisibleSave))
	})
	t.Run("a bare $VAR is literal text", func(t *testing.T) { none(t, "bare-env-var") })
	t.Run("a variable-length number next to another variable value", func(t *testing.T) {
		none(t, "buildnum-branch-adjacent")
		none(t, "buildnum-param-adjacent")
		scoped(t, "epoch-branch-adjacent", "TIME_SCOPED")
	})
	t.Run("findings are ranked from TIME to REVISION", func(t *testing.T) {
		var order []string
		for _, f := range doc.Findings {
			if f.Verdict == "CACHE_VOLATILE_RESTORE" {
				assert.Assert(t, len(f.ReasonCodes) > 0, "%s has no reason code", f.Target.Job)
				order = append(order, f.ReasonCodes[0])
			}
		}
		assert.Check(t, cmp.DeepEqual(order, []string{"TIME_SCOPED", "TIME_SCOPED", "WORKFLOW_SCOPED", "REVISION_SCOPED", "REVISION_SCOPED"}))
	})
}

// A parameter default of pipeline.git.revision is inherited by one call and
// overridden by another; a stable default is overridden with the revision.
func TestScopedParameterDefaults(t *testing.T) {
	const sha = "0123456789abcdef0123456789abcdef01234567"
	authored := `version: 2.1
jobs:
  rev-default:
    parameters:
      k: {type: string, default: "<< pipeline.git.revision >>"}
    docker: [{image: cimg/base:current}]
    steps:
      - restore_cache: {keys: ["rd-<< parameters.k >>"]}
      - save_cache: {key: "rd-<< parameters.k >>", paths: [~/.npm]}
  stable-default:
    parameters:
      k: {type: string, default: "v1"}
    docker: [{image: cimg/base:current}]
    steps:
      - restore_cache: {keys: ["sd-<< parameters.k >>"]}
      - save_cache: {key: "sd-<< parameters.k >>", paths: [~/.npm]}
workflows:
  w:
    jobs:
      - rev-default: {name: inherits}
      - rev-default: {name: overrides, k: "{{ checksum \"package-lock.json\" }}"}
      - stable-default: {name: stable-kept}
      - stable-default: {name: stable-overridden, k: "<< pipeline.git.revision >>"}
`
	compiled := strings.ReplaceAll(`version: 2
jobs:
  inherits:
    docker: [{image: cimg/base:current}]
    steps:
      - restore_cache: {keys: ["rd-SHA"]}
      - save_cache: {key: "rd-SHA", paths: [~/.npm]}
  overrides:
    docker: [{image: cimg/base:current}]
    steps:
      - restore_cache: {keys: ["rd-{{ checksum \"package-lock.json\" }}"]}
      - save_cache: {key: "rd-{{ checksum \"package-lock.json\" }}", paths: [~/.npm]}
  stable-kept:
    docker: [{image: cimg/base:current}]
    steps:
      - restore_cache: {keys: ["sd-v1"]}
      - save_cache: {key: "sd-v1", paths: [~/.npm]}
  stable-overridden:
    docker: [{image: cimg/base:current}]
    steps:
      - restore_cache: {keys: ["sd-SHA"]}
      - save_cache: {key: "sd-SHA", paths: [~/.npm]}
workflows:
  w:
    jobs: [inherits, overrides, stable-kept, stable-overridden]
`, "SHA", sha)
	a := analyzeInline(t, authored, compiled)
	tests := []struct {
		name     string // the job
		volatile bool
		scope    cache.Scope
	}{
		{name: "inherits", volatile: true, scope: cache.ScopeRevision},
		{name: "overrides", volatile: false, scope: cache.Unscoped},
		{name: "stable-kept", volatile: false, scope: cache.Unscoped},
		{name: "stable-overridden", volatile: true, scope: cache.ScopeRevision},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			pv := policyOf(t, a, tc.name)
			assert.Check(t, cmp.Equal(pv.Volatile, tc.volatile), pv.Reason)
			assert.Check(t, cmp.Equal(pv.Scope, tc.scope))
		})
	}
}

// The explicit dependency and build-output lists.
func TestPathClasses(t *testing.T) {
	tests := []struct {
		name string // the path
		want cache.PathClass
	}{
		{name: "~/.npm", want: cache.DependencyPaths},
		{name: "/home/circleci/.cache/pip", want: cache.DependencyPaths},
		{name: "~/go/pkg/mod/cache/download", want: cache.DependencyPaths},
		{name: "~/.m2/repository", want: cache.DependencyPaths},
		{name: "node_modules", want: cache.DependencyPaths},
		{name: "./packages/app/node_modules", want: cache.DependencyPaths},
		{name: "vendor/bundle", want: cache.DependencyPaths},
		{name: "~/.cache/go-build", want: cache.BuildOutputPaths},
		{name: "dist", want: cache.BuildOutputPaths},
		{name: "target/", want: cache.BuildOutputPaths},
		{name: "/tmp/deps", want: cache.UnknownPaths},
		{name: "$HOME/.npm", want: cache.UnknownPaths},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Check(t, cmp.Equal(cache.ClassifyPath(tc.name), tc.want))
		})
	}
}

// pipeline.number next to the branch with no
// delimiter can collide across pipelines, and through aggregation would make
// a revision claim false.
func TestPipelineNumberBoundary(t *testing.T) {
	policy := func(t *testing.T, keys, save string) cache.PolicyVerdict {
		t.Helper()
		a := analyzeInline(t, `version: 2.1
jobs:
  build:
    docker: [{image: cimg/base:current}]
    steps:
      - restore_cache: {keys: [`+keys+`]}
      - save_cache: {key: "`+save+`", paths: [~/.npm]}
workflows:
  w:
    jobs: [build]
`, strings.NewReplacer("<< pipeline.number >>", "12").Replace(`version: 2
jobs:
  build:
    docker: [{image: cimg/base:current}]
    steps:
      - restore_cache: {keys: [`+keys+`]}
      - save_cache: {key: "`+save+`", paths: [~/.npm]}
workflows:
  w:
    jobs: [build]
`))
		assert.Assert(t, cmp.Len(a.Policies, 1))
		return a.Policies[0]
	}
	t.Run("no delimiter: uninspectable", func(t *testing.T) {
		pv := policy(t, `"deps-<< pipeline.number >>{{ .Branch }}"`, "deps-<< pipeline.number >>{{ .Branch }}")
		assert.Check(t, !pv.Volatile)
		assert.Check(t, cmp.Contains(pv.Reason, "variable-length number"))
	})
	t.Run("a delimiter keeps the parts apart", func(t *testing.T) {
		pv := policy(t, `"deps-<< pipeline.number >>-{{ .Branch }}"`, "deps-<< pipeline.number >>-{{ .Branch }}")
		assert.Check(t, pv.Volatile, pv.Reason)
		assert.Check(t, cmp.Equal(pv.Scope, cache.ScopePipeline))
	})
	t.Run("a digit is not a delimiter", func(t *testing.T) {
		// Pipeline 1 on "12fix" and pipeline 11
		// on "2fix" are both "deps-1112fix".
		pv := policy(t, `"deps-<< pipeline.number >>1{{ .Branch }}"`, "deps-<< pipeline.number >>1{{ .Branch }}")
		assert.Check(t, !pv.Volatile)
		assert.Check(t, cmp.Contains(pv.Reason, "non-digit delimiter"))
	})
	t.Run("a trailing digit with nothing variable after it is safe", func(t *testing.T) {
		pv := policy(t, `"deps-<< pipeline.number >>1"`, "deps-<< pipeline.number >>1")
		assert.Check(t, pv.Volatile, pv.Reason)
		assert.Check(t, cmp.Equal(pv.Scope, cache.ScopePipeline))
	})
	t.Run("the collision cannot hide behind a broader fallback", func(t *testing.T) {
		pv := policy(t, `"deps-<< pipeline.number >>{{ .Branch }}", "deps-rev-{{ .Revision }}"`, "deps-<< pipeline.number >>{{ .Branch }}")
		assert.Check(t, !pv.Volatile, "not a REVISION_SCOPED claim")
	})
}

// A shared runtime token proves only its own
// scope. The restore's pipeline scope comes from pipeline.number, which the
// save does not share, so there is no finding.
func TestCorrelationChecksTheScopingPart(t *testing.T) {
	a := analyzeInline(t, `version: 2.1
parameters:
  cache_generation: {type: enum, enum: ["12"], default: "12"}
jobs:
  seed:
    docker: [{image: cimg/base:current}]
    steps:
      - save_cache: {key: "deps-<< pipeline.parameters.cache_generation >>-{{ .Revision }}", paths: [~/.npm]}
  consume:
    docker: [{image: cimg/base:current}]
    steps:
      - restore_cache: {keys: ["deps-<< pipeline.number >>-{{ .Revision }}"]}
workflows:
  w:
    jobs: [seed, consume]
`, `version: 2
jobs:
  seed:
    docker: [{image: cimg/base:current}]
    steps:
      - save_cache: {key: "deps-12-{{ .Revision }}", paths: [~/.npm]}
  consume:
    docker: [{image: cimg/base:current}]
    steps:
      - restore_cache: {keys: ["deps-12-{{ .Revision }}"]}
workflows:
  w:
    jobs: [seed, consume]
`)
	assert.Assert(t, cmp.Len(a.Policies, 1))
	pv := a.Policies[0]
	assert.Check(t, !pv.Volatile, "PIPELINE_SCOPED would be false: pipeline 12 matches a save from an earlier pipeline")
	assert.Check(t, cmp.Contains(pv.Reason, "not built from the same scoped value"))
}
