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
	"slices"
	"strings"
	"testing"

	"gotest.tools/v3/assert"
	"gotest.tools/v3/assert/cmp"

	"github.com/CircleCI-Public/circleci-cli/internal/cmdutil"
	"github.com/CircleCI-Public/circleci-cli/internal/configoptimize/engine"
	"github.com/CircleCI-Public/circleci-cli/internal/configoptimize/module"
	"github.com/CircleCI-Public/circleci-cli/internal/configoptimize/module/cache"
	"github.com/CircleCI-Public/circleci-cli/internal/configoptimize/pipelineconfig"
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

// analyze runs the module through the engine on a fixture, with the repo at
// root ("" for none).
func analyze(t *testing.T, name, root string) report.Document {
	t.Helper()
	src, compiled := fixture(t, name)
	doc, err := engine.Analyze(context.Background(), engine.AnalyzeInput{
		Path: name + ".yml", Config: src,
		Modules:  []module.Analyzer{cache.NewWithRepo(root)},
		Compiler: fixedCompiler{compiled: compiled},
		Policy:   registry.Policy(),
	})
	assert.NilError(t, err)
	return doc
}

// analyzeInline runs the analysis on an authored config and a compiled copy.
func analyzeInline(t *testing.T, authored, compiled string) cache.Analysis {
	t.Helper()
	l, err := engine.Load(context.Background(), fixedCompiler{compiled: []byte(compiled)}, []byte(authored), nil)
	assert.NilError(t, err)
	return cache.Analyze(l.Effective)
}

func load(t *testing.T, name string) cache.Analysis {
	t.Helper()
	src, compiled := fixture(t, name)
	return analyzeInline(t, string(src), string(compiled))
}

// policyOf is the verdict on job's restore policy; the test stops when the
// job has none.
func policyOf(t *testing.T, a cache.Analysis, job string) cache.PolicyVerdict {
	t.Helper()
	i := slices.IndexFunc(a.Policies, func(pv cache.PolicyVerdict) bool { return pv.Policy.Job == job })
	assert.Assert(t, i >= 0, "no restore policy in %s", job)
	return a.Policies[i]
}

func byJob(doc report.Document) map[string][]report.Finding {
	out := map[string][]report.Finding{}
	for _, f := range doc.Findings {
		out[f.Target.Job] = append(out[f.Target.Job], f)
	}
	return out
}

// only is job's one finding; the test stops when there is not exactly one.
func only(t *testing.T, findings map[string][]report.Finding, job string) report.Finding {
	t.Helper()
	assert.Assert(t, cmp.Len(findings[job], 1), "%s: %v", job, findings[job])
	return findings[job][0]
}

func ruleEvidence(f report.Finding, rule string) (string, bool) {
	for _, e := range f.Evidence {
		if e.Claim == "rule" && strings.HasPrefix(e.Value, rule+":") {
			return e.Value, true
		}
	}
	return "", false
}

func TestVolatileRestore(t *testing.T) {
	doc := analyze(t, "keys", "")
	findings := byJob(doc)
	a := load(t, "keys")

	tests := []struct {
		job  string
		code string // the reason code, or "" for no finding
		why  string
	}{
		{job: "keyed-default", why: "the default is a checksum"},
		{job: "keyed-volatile", code: "TIME_SCOPED", why: "the call site overrides the default with {{ epoch }}"},
		{job: "volatile-default-kept", code: "TIME_SCOPED"},
		{job: "volatile-default-overridden", why: "the call site overrides the {{ epoch }} default with a checksum"},
		{job: "matrixed-{{ epoch }}", code: "TIME_SCOPED", why: "each matrix variant is judged with its own value"},
		{job: "matrixed-{{ .BuildNum }}", code: "JOB_SCOPED"},
		{job: `matrixed-{{ checksum "package-lock.json" }}`},
		{job: "build-macos", code: "JOB_SCOPED", why: "matrix jobs whose names share a prefix are traced to their own job"},
		{job: "build-linux-amd64", code: "JOB_SCOPED"},
		{job: "aliased-linux", code: "JOB_SCOPED", why: "a matrix alias does not rename the jobs"},
		{job: "excluded-amd64-linux", code: "JOB_SCOPED", why: "an excluded tuple does not claim another invocation's job"},
		{job: "stable-fallback", why: "a stable fallback key"},
		{job: "cross-restore", why: "a stable key saved by another job"},
		{job: "broad-restore", why: "a broad fallback is flagged, not merged"},
		{job: "seed-restore", why: "the changing key may match a save of another shape"},
		{job: "correlated-prefix", code: "JOB_SCOPED", why: "the save shares the changing token"},
		{job: "cj-restore", code: "JOB_SCOPED", why: "a per-job key saved by another job still cannot be restored"},
		{job: "no-save", code: "JOB_SCOPED", why: "a restore with no save in the config"},
		{job: "save-first", code: "JOB_SCOPED", why: "the job's own earlier save is no reuse across jobs"},
		{job: "save-first-epoch", code: "TIME_SCOPED", why: "an earlier epoch save has another key by the restore"},
		{job: "sha-consume", why: "REVISION_SCOPED, but /tmp/deps is not a dependency path"},
		{job: "epoch-no-spaces", code: "TIME_SCOPED", why: "epoch is found by token type, not spelling"},
		{job: "epoch-and-buildnum", code: "TIME_SCOPED", why: "within one key the narrowest token wins"},
		{job: "parallel", code: "JOB_SCOPED", why: "parallel nodes of one job share it, but no earlier job can"},
		{job: "parallel-epoch", code: "TIME_SCOPED", why: "parallel nodes do not share {{ epoch }}"},
		{job: "workflow-id", code: "WORKFLOW_SCOPED", why: "never pipeline-scoped"},
		{job: "epoch-then-revision", code: "REVISION_SCOPED", why: "across the restore list the broadest scope wins"},
		{job: "revision-then-epoch", code: "REVISION_SCOPED"},
		{job: "revision-build-output", why: "broad scopes need dependency paths"},
		{job: "revision-mixed-paths"},
		{job: "revision-param-path"},
		{job: "revision-no-save"},
		{job: "bare-env-var", why: "a bare $VAR is literal text"},
		{job: "buildnum-branch-adjacent", why: "a variable-length number next to another variable value"},
		{job: "buildnum-param-adjacent"},
		{job: "epoch-branch-adjacent", code: "TIME_SCOPED", why: "{{ epoch }} has a fixed width"},
		{job: "untraceable", why: "a trigger can override the pipeline parameter"},
		{job: "enum-stable"},
		{job: "enum-mixed", why: "the enum's values disagree, so it depends on the scenario"},
		{job: "enum-alternatives"},
		{job: "adjacent-unknown", why: "an unknown value next to a changing one"},
	}
	for _, tc := range tests {
		t.Run(tc.job, func(t *testing.T) {
			if tc.code == "" {
				policyOf(t, a, tc.job) // the case is in the fixture
				assert.Check(t, cmp.Len(findings[tc.job], 0), "%s: %v", tc.why, findings[tc.job])
				return
			}
			f := only(t, findings, tc.job)
			assert.Check(t, cmp.Equal(f.Verdict, "CACHE_VOLATILE_RESTORE"))
			assert.Check(t, cmp.DeepEqual(f.ReasonCodes, []string{tc.code}), tc.why)
		})
	}

	t.Run("a finding names its call site, rule and limit", func(t *testing.T) {
		f := only(t, findings, "keyed-volatile")
		assert.Check(t, cmp.Equal(f.Target.Site, "workflows.keys.jobs.1.keyed.cache_key"))
		assert.Check(t, cmp.Equal(only(t, findings, "volatile-default-kept").Target.Site, "jobs.volatile-default.parameters.cache_key.default"))
		assert.Check(t, cmp.Equal(f.Disposition, "report_only"))
		assert.Check(t, cmp.Equal(f.Confidence.Level, "medium"))
		assert.Check(t, cmp.Contains(f.Suggestion.Note, "embeds the time of each cache step"))
		assert.Check(t, cmp.Contains(f.Suggestion.Note, "Reuse is limited"))
		assert.Check(t, !strings.Contains(f.Suggestion.Note, "never hit"), f.Suggestion.Note)
		assert.Check(t, !strings.Contains(f.Suggestion.Note, "rerun"), "no claim about reruns until one is observed")
		rule, ok := ruleEvidence(f, cache.RuleX1)
		assert.Check(t, ok, "the evidence names the rule version")
		assert.Check(t, cmp.Contains(rule, "broadest key"), "the rule names the scope model")
	})
	t.Run("the evidence names the key's scope", func(t *testing.T) {
		f := only(t, findings, "parallel-epoch")
		assert.Assert(t, len(f.Evidence) > 1, "%v", f.Evidence)
		assert.Check(t, cmp.Equal(f.Evidence[1].Claim, "key 0 is TIME_SCOPED"))
	})
	t.Run("an epoch key is medium confidence, with the caveat", func(t *testing.T) {
		for _, job := range []string{"epoch-no-spaces", "epoch-and-buildnum"} {
			f := only(t, findings, job)
			assert.Check(t, cmp.Equal(f.Confidence.Level, "medium"), job)
			assert.Check(t, cmp.Contains(f.Confidence.Reason, "how often the job runs"), job)
		}
	})
	t.Run("a job-scoped note claims no save it cannot see", func(t *testing.T) {
		assert.Check(t, cmp.Contains(only(t, findings, "cj-restore").Suggestion.Note, "normally can't reuse a cache from an earlier job"))
		note := only(t, findings, "no-save").Suggestion.Note
		assert.Check(t, !strings.Contains(note, "this config saves"), note)
	})
	t.Run("findings are ranked from TIME to REVISION", func(t *testing.T) {
		rank := []string{"TIME_SCOPED", "JOB_SCOPED", "WORKFLOW_SCOPED", "PIPELINE_SCOPED", "REVISION_SCOPED"}
		var order []int
		for _, f := range doc.Findings {
			if f.Verdict == "CACHE_VOLATILE_RESTORE" {
				assert.Assert(t, cmp.Len(f.ReasonCodes, 1), f.Target.Job)
				order = append(order, slices.Index(rank, f.ReasonCodes[0]))
			}
		}
		assert.Check(t, slices.IsSorted(order), "%v", order)
	})
	t.Run("an install with no cache is a candidate", func(t *testing.T) {
		f := only(t, findings, "candidate")
		assert.Check(t, cmp.Equal(f.Verdict, "CACHE_CANDIDATE"))
		_, ok := ruleEvidence(f, cache.RuleF4)
		assert.Check(t, ok, "the evidence names the rule version")
		assert.Check(t, cmp.Equal(f.Confidence.Level, "low"))
	})
	t.Run("the JSON is deterministic", func(t *testing.T) {
		var first, second bytes.Buffer
		assert.NilError(t, cmdutil.WriteJSON(&first, doc))
		assert.NilError(t, cmdutil.WriteJSON(&second, analyze(t, "keys", "")))
		assert.Check(t, cmp.Equal(first.String(), second.String()))
	})
}

func TestRestorePolicies(t *testing.T) {
	a := load(t, "keys")

	uninspectable := []struct {
		job    string
		reason string
	}{
		{job: "untraceable", reason: "can be overridden when the pipeline is triggered"},
		{job: "seed-restore", reason: `may match "seed-42-seeded"`},
		{job: "adjacent-unknown", reason: "{{ .Environment.PAD }}"},
		{job: "enum-mixed", reason: `is "{{ epoch }}" or "stable"`},
		{job: "enum-alternatives", reason: `is "{{ .BuildNum }}" or "1{{ .BuildNum }}"`},
	}
	for _, tc := range uninspectable {
		t.Run("uninspectable: "+tc.job, func(t *testing.T) {
			pv := policyOf(t, a, tc.job)
			assert.Check(t, pv.Uninspectable)
			assert.Check(t, cmp.Contains(pv.Reason, tc.reason))
		})
	}
	t.Run("a stable enum pipeline parameter is unscoped", func(t *testing.T) {
		pv := policyOf(t, a, "enum-stable")
		assert.Check(t, !pv.Uninspectable)
		assert.Check(t, cmp.Equal(pv.Scope, cache.Unscoped))
	})
	t.Run("the same revision under another name is the same key", func(t *testing.T) {
		pv := policyOf(t, a, "sha-consume")
		assert.Check(t, cmp.Equal(pv.Scope, cache.ScopeRevision))
		assert.Check(t, pv.Saved, "the CIRCLE_SHA1 save is an exact match")
	})
	t.Run("the saved paths are classified", func(t *testing.T) {
		for job, want := range map[string]cache.PathClass{
			"revision-build-output": cache.BuildOutputPaths, "revision-mixed-paths": cache.MixedPaths,
			"revision-param-path": cache.ParameterizedPaths, "revision-no-save": cache.NoVisibleSave,
		} {
			assert.Check(t, cmp.Equal(policyOf(t, a, job).PathClass, want), job)
		}
	})
	t.Run("edges", func(t *testing.T) {
		var broad []string
		var cross, exactBroad bool
		for _, e := range a.Model.Edges {
			assert.Assert(t, len(e.Save.Keys) > 0, "the save in %s has no key", e.Save.Job)
			pair := e.Policy.Job + " <- " + e.Save.Job
			switch e.Policy.Job {
			case "cross-restore":
				cross = cross || (e.Save.Job == "cross-save" && e.Exact)
			case "broad-restore":
				assert.Check(t, e.Broad, "%s is flagged broad, never merged", pair)
				broad = append(broad, e.Save.Job)
			case "broad-exact-restore":
				assert.Check(t, e.Broad, "a prefix matching an exact save and a longer one: %s", pair)
				exactBroad = true
			}
			assert.Check(t, !strings.HasPrefix(e.Policy.Job, "keyed-") || e.Policy.Job == e.Save.Job,
				"keyed-{{ epoch }} and keyed-{{ checksum … }} share a raw prefix but not a typed template: %s", pair)
		}
		assert.Check(t, cross, "a restore in another job reads the save")
		slices.Sort(broad)
		assert.Check(t, cmp.DeepEqual(broad, []string{"broad-a", "broad-b"}))
		assert.Check(t, exactBroad, "no edge from broad-exact-restore")
	})
}

func TestSetupConfig(t *testing.T) {
	assert.Check(t, cmp.Len(analyze(t, "setup-config", "").Findings, 0))
	a := load(t, "setup-config")
	assert.Assert(t, len(a.Policies) > 0, "the setup config has a restore policy")
	assert.Check(t, a.Policies[0].Uninspectable)
	assert.Check(t, cmp.Contains(a.Policies[0].Reason, "setup (dynamic) config"))
	for _, jv := range a.Jobs {
		assert.Check(t, !jv.Candidate(), "no candidate in a setup config: %s", jv.Job)
	}
}

// pipeline.git.revision is substituted by the compiler, so it is read from
// the authored key: directly, through a call site, or through a default.
// testdata may not hold ambient pipeline values, so the compiled copy is
// built here with a fixed SHA standing in for the checkout.
func TestPipelineRevisionKey(t *testing.T) {
	const authored = `version: 2.1
jobs:
  direct:
    docker: [{image: x}]
    steps:
      - restore_cache: {keys: ["rev-<< pipeline.git.revision >>"]}
      - save_cache: {key: "rev-<< pipeline.git.revision >>", paths: [~/.npm]}
  via-param:
    parameters: {k: {type: string}}
    docker: [{image: x}]
    steps:
      - restore_cache: {keys: ["p-<< parameters.k >>"]}
      - save_cache: {key: "p-<< parameters.k >>", paths: [~/.npm]}
  rev-default:
    parameters: {k: {type: string, default: "<< pipeline.git.revision >>"}}
    docker: [{image: x}]
    steps:
      - restore_cache: {keys: ["rd-<< parameters.k >>"]}
      - save_cache: {key: "rd-<< parameters.k >>", paths: [~/.npm]}
workflows:
  w:
    jobs:
      - direct
      - via-param: {k: "<< pipeline.git.revision >>"}
      - rev-default: {name: inherits}
      - rev-default: {name: overrides, k: '{{ checksum "package-lock.json" }}'}
`
	compiled := strings.ReplaceAll(`version: 2
jobs:
  direct: {docker: [{image: x}], steps: [{restore_cache: {keys: [rev-SHA]}}, {save_cache: {key: rev-SHA, paths: [~/.npm]}}]}
  via-param: {docker: [{image: x}], steps: [{restore_cache: {keys: [p-SHA]}}, {save_cache: {key: p-SHA, paths: [~/.npm]}}]}
  inherits: {docker: [{image: x}], steps: [{restore_cache: {keys: [rd-SHA]}}, {save_cache: {key: rd-SHA, paths: [~/.npm]}}]}
  overrides:
    docker: [{image: x}]
    steps: [{restore_cache: {keys: ['rd-{{ checksum "package-lock.json" }}']}}, {save_cache: {key: 'rd-{{ checksum "package-lock.json" }}', paths: [~/.npm]}}]
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
		assert.Check(t, cmp.DeepEqual(f.ReasonCodes, []string{"REVISION_SCOPED"}))
		assert.Check(t, cmp.Contains(f.Suggestion.Note, "can't reuse caches from a different commit"))
		assert.Assert(t, len(f.Evidence) > 1, "%s: %v", f.Target.Job, f.Evidence)
		assert.Check(t, cmp.Equal(f.Evidence[1].Claim, "key 0 is REVISION_SCOPED"))
		sites[f.Target.Job] = f.Target.Site
	}
	assert.Check(t, cmp.DeepEqual(sites, map[string]string{
		"direct":    "jobs.direct.steps.0.restore_cache.keys.0",
		"via-param": "workflows.w.jobs.1.via-param.k",
		"inherits":  "jobs.rev-default.parameters.k.default",
	}), "overrides replaces the revision default with a checksum")
}

const sha = "0123456789abcdef0123456789abcdef01234567"

// pipelineValues compiles a config without parameters: the compiler
// substitutes pipeline values, which testdata may not hold.
var pipelineValues = strings.NewReplacer("version: 2.1", "version: 2", "<< pipeline.number >>", "12",
	"<< pipeline.id >>", "0f00", "<< pipeline.git.base_revision >>", "aaaa", "<< pipeline.git.revision >>", sha)

// oneJob is a config whose one job restores keys and saves save.
func oneJob(keys, save string) string {
	return `version: 2.1
jobs:
  build:
    docker: [{image: x}]
    steps:
      - restore_cache: {keys: [` + keys + `]}
      - save_cache: {key: "` + save + `", paths: [~/.npm]}
`
}

// Keys built from values the compiler substitutes. A scope claim holds only
// when the save is provably built from the same scoped value, and when no
// variable-length number can run into another value.
func TestCompileTimeValues(t *testing.T) {
	tests := []struct {
		name     string
		authored string
		compiled string // "" for pipelineValues applied to authored
		volatile bool
		scope    cache.Scope // checked when volatile
		reason   string      // in the policy's reason, when set
	}{
		{
			name: "an exact edge to a hard-coded save proves nothing",
			authored: `version: 2.1
jobs:
  seed: {docker: [{image: x}], steps: [{save_cache: {key: deps-12, paths: [x]}}]}
  restore: {docker: [{image: x}], steps: [{restore_cache: {keys: ["deps-<< pipeline.number >>"]}}]}
workflows: {w: {jobs: [seed, restore]}}
`,
			reason: "not built from the same scoped value",
		},
		{
			name: "equal template text over different variables is not correlated",
			authored: `version: 2.1
jobs:
  restore: {parameters: {k: {type: string}}, docker: [{image: x}], steps: [{restore_cache: {keys: ["deps-<< parameters.k >>"]}}]}
  seed: {parameters: {k: {type: string}}, docker: [{image: x}], steps: [{save_cache: {key: "deps-<< parameters.k >>", paths: [x]}}]}
workflows: {w: {jobs: [{restore: {k: "<< pipeline.git.revision >>"}}, {seed: {k: "<< pipeline.git.base_revision >>"}}]}}
`,
			compiled: `version: 2
jobs:
  restore: {docker: [{image: x}], steps: [{restore_cache: {keys: [deps-aaaa]}}]}
  seed: {docker: [{image: x}], steps: [{save_cache: {key: deps-aaaa, paths: [x]}}]}
`,
			reason: "not built from the same scoped value",
		},
		{
			name: "a shared runtime token does not prove the scoping part is shared",
			authored: `version: 2.1
parameters: {cache_generation: {type: enum, enum: ["12"], default: "12"}}
jobs:
  seed: {docker: [{image: x}], steps: [{save_cache: {key: "deps-<< pipeline.parameters.cache_generation >>-{{ .Revision }}", paths: [~/.npm]}}]}
  consume: {docker: [{image: x}], steps: [{restore_cache: {keys: ["deps-<< pipeline.number >>-{{ .Revision }}"]}}]}
workflows: {w: {jobs: [seed, consume]}}
`,
			compiled: `version: 2
jobs:
  seed: {docker: [{image: x}], steps: [{save_cache: {key: "deps-12-{{ .Revision }}", paths: [~/.npm]}}]}
  consume: {docker: [{image: x}], steps: [{restore_cache: {keys: ["deps-12-{{ .Revision }}"]}}]}
`,
			reason: "not built from the same scoped value",
		},
		{
			name: "a prefix save from the same value stays correlated, however the reference is spaced",
			authored: `version: 2.1
jobs:
  build:
    parameters: {k: {type: string}}
    docker: [{image: x}]
    steps:
      - restore_cache: {keys: ["deps-<<parameters.k>>"]}
      - save_cache: {key: "deps-<< parameters.k >>-seeded", paths: [~/.npm]}
workflows: {w: {jobs: [{build: {k: "<< pipeline.number >>"}}]}}
`,
			compiled: `version: 2
jobs:
  build: {docker: [{image: x}], steps: [{restore_cache: {keys: [deps-12]}}, {save_cache: {key: deps-12-seeded, paths: [~/.npm]}}]}
`,
			volatile: true, scope: cache.ScopePipeline,
		},
		{
			name:     "a base revision can be shared by sibling commits",
			authored: oneJob(`"deps-<< pipeline.git.base_revision >>"`, "deps-<< pipeline.git.base_revision >>"),
		},
		{
			name: "a pipeline-scoped key is reported only for dependency paths",
			authored: `version: 2.1
jobs:
  seed: {docker: [{image: x}], steps: [{save_cache: {key: "deps-<< pipeline.id >>", paths: [x]}}]}
  consume: {docker: [{image: x}], steps: [{restore_cache: {keys: ["deps-<< pipeline.id >>"]}}]}
workflows: {w: {jobs: [seed, {consume: {requires: [seed]}}]}}
`,
			reason: "reported only for dependency caches",
		},
		{
			name:     "a number next to the branch with no delimiter is uninspectable",
			authored: oneJob(`"deps-<< pipeline.number >>{{ .Branch }}"`, "deps-<< pipeline.number >>{{ .Branch }}"),
			reason:   "variable-length number",
		},
		{
			name:     "a delimiter keeps the parts apart",
			authored: oneJob(`"deps-<< pipeline.number >>-{{ .Branch }}"`, "deps-<< pipeline.number >>-{{ .Branch }}"),
			volatile: true, scope: cache.ScopePipeline,
		},
		{
			// Pipeline 1 on "12fix" and pipeline 11 on "2fix" are both "deps-1112fix".
			name:     "a digit is not a delimiter",
			authored: oneJob(`"deps-<< pipeline.number >>1{{ .Branch }}"`, "deps-<< pipeline.number >>1{{ .Branch }}"),
			reason:   "non-digit delimiter",
		},
		{
			name:     "a trailing digit with nothing variable after it is safe",
			authored: oneJob(`"deps-<< pipeline.number >>1"`, "deps-<< pipeline.number >>1"),
			volatile: true, scope: cache.ScopePipeline,
		},
		{
			name:     "the collision cannot hide behind a broader fallback",
			authored: oneJob(`"deps-<< pipeline.number >>{{ .Branch }}", "deps-rev-{{ .Revision }}"`, "deps-<< pipeline.number >>{{ .Branch }}"),
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			compiled := tc.compiled
			if compiled == "" {
				compiled = pipelineValues.Replace(tc.authored)
			}
			a := analyzeInline(t, tc.authored, compiled)
			assert.Assert(t, cmp.Len(a.Policies, 1))
			pv := a.Policies[0]
			assert.Check(t, cmp.Equal(pv.Volatile, tc.volatile), pv.Reason)
			if tc.volatile {
				assert.Check(t, cmp.Equal(pv.Scope, tc.scope))
			}
			if tc.reason != "" {
				assert.Check(t, cmp.Contains(pv.Reason, tc.reason))
			}
		})
	}
}

// The explicit dependency and build-output lists, and how a path is
// normalized before it is matched.
func TestPathClasses(t *testing.T) {
	tests := []struct {
		name string // the path
		want cache.PathClass
	}{
		{name: "~/.npm", want: cache.DependencyPaths},
		{name: "/home/circleci/.cache/pip", want: cache.DependencyPaths},
		{name: "~/go/pkg/mod/cache/download", want: cache.DependencyPaths},
		{name: "node_modules", want: cache.DependencyPaths},
		{name: "./packages/app/node_modules", want: cache.DependencyPaths},
		{name: "~/.cache/go-build", want: cache.BuildOutputPaths},
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
