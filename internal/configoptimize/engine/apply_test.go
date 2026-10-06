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

package engine_test

import (
	"bytes"
	"context"
	"errors"
	"slices"
	"testing"

	clierrors "github.com/CircleCI-Public/circleci-cli/clikit/errors"
	"gopkg.in/yaml.v3"
	"gotest.tools/v3/assert"
	"gotest.tools/v3/assert/cmp"

	"github.com/CircleCI-Public/circleci-cli/internal/configoptimize/engine"
	"github.com/CircleCI-Public/circleci-cli/internal/configoptimize/finding"
	"github.com/CircleCI-Public/circleci-cli/internal/configoptimize/module"
	"github.com/CircleCI-Public/circleci-cli/internal/configoptimize/pipelineconfig"
	"github.com/CircleCI-Public/circleci-cli/internal/configoptimize/reconcile"
	"github.com/CircleCI-Public/circleci-cli/internal/configoptimize/report"
)

// identityCompiler "compiles" a config to itself, which is exact for configs
// with no orbs, executors, commands or parameters. From call driftFrom on it
// appends a key, simulating a compiler whose output changed underneath.
type identityCompiler struct {
	// rejectLosingDLC names a job: a candidate in which it no longer has
	// machine DLC is rejected as invalid, as a real compiler might.
	rejectLosingDLC string
	driftFrom       int
	calls           int
}

func (c *identityCompiler) Compile(_ context.Context, in pipelineconfig.CompileInput) (pipelineconfig.CompileResult, error) {
	c.calls++
	if c.rejectLosingDLC != "" && !machineDLC(in.ConfigYAML, c.rejectLosingDLC) {
		return pipelineconfig.CompileResult{}, &pipelineconfig.InvalidConfigError{Diagnostics: []string{c.rejectLosingDLC + " candidate rejected"}}
	}
	out := in.ConfigYAML
	if c.driftFrom > 0 && c.calls >= c.driftFrom {
		out = append(bytes.Clone(out), []byte("drifted: true\n")...)
	}
	return pipelineconfig.CompileResult{CompiledYAML: out, Compiler: "identity"}, nil
}

func machineDLC(src []byte, job string) bool {
	var doc struct {
		Jobs map[string]struct {
			Machine map[string]any `yaml:"machine"`
		} `yaml:"jobs"`
	}
	if yaml.Unmarshal(src, &doc) != nil {
		return false
	}
	_, ok := doc.Jobs[job].Machine["docker_layer_caching"]
	return ok
}

const twoJobs = `version: 2.1
jobs:
  unit:
    machine:
      image: ubuntu-2204:current
      docker_layer_caching: true
    steps:
      - setup_remote_docker:
          docker_layer_caching: true
      - run: echo unit
  lint:
    machine:
      image: ubuntu-2204:current
      docker_layer_caching: true
    steps:
      - run: echo lint
workflows:
  ci:
    jobs: [unit, lint]
`

// dropDLC is a stand-in check: for every job with DLC turned on, it
// proposes removing every place that turns it on, as one change.
type dropDLC struct{}

const dropDLCName = "drop-dlc"

func (dropDLC) Name() string { return dropDLCName }

func (dropDLC) Evaluate(_ context.Context, cfg *pipelineconfig.Effective) ([]finding.Finding, error) {
	var out []finding.Finding
	for _, job := range cfg.Jobs {
		paths := job.DockerLayerCachingAt()
		if len(paths) == 0 {
			continue
		}
		target := finding.Target{Path: []string{"jobs", job.Name}, Job: job.Name}
		out = append(out, finding.Finding{
			ID:         finding.NewID(dropDLCName, target),
			Module:     dropDLCName,
			Verdict:    "DLC_UNUSED",
			Target:     target,
			Impact:     finding.Impact{Cost: finding.DirectionBetter, Time: finding.DirectionNeutral},
			Confidence: finding.Confidence{Level: finding.ConfidenceHigh},
			Evidence: []finding.Evidence{{
				Kind: finding.EvidenceConfigFact, Claim: "DLC is enabled", Value: "docker_layer_caching: true",
				Source: "config:jobs." + job.Name,
			}},
			Suggestion: finding.Suggestion{Op: finding.OpRemove, Paths: paths, FalseMeansAbsent: true},
		})
	}
	return out, nil
}

// applyWith runs apply with only the dropDLC check.
func applyWith(t *testing.T, c pipelineconfig.Compiler, policy reconcile.Policy) (engine.ApplyResult, error) {
	t.Helper()
	return engine.Apply(context.Background(), engine.AnalyzeInput{
		Path:     "config.yml",
		Config:   []byte(twoJobs),
		Modules:  []module.Analyzer{dropDLC{}},
		Compiler: c,
		Policy:   policy,
	})
}

// fading wraps a check; from its second evaluation on, no finding proposes
// anything, as if the first change had made the rest unactionable.
type fading struct {
	module.Analyzer
	calls int
}

func (f *fading) Evaluate(ctx context.Context, cfg *pipelineconfig.Effective) ([]finding.Finding, error) {
	f.calls++
	fs, err := f.Analyzer.Evaluate(ctx, cfg)
	for i := range fs {
		if f.calls > 1 {
			fs[i].Suggestion = finding.Suggestion{Op: finding.OpNone, Note: "fresh analysis"}
		}
	}
	return fs, err
}

func TestApply(t *testing.T) {
	t.Run("a change that fails to compile is rejected with the compiler's message; others still apply", func(t *testing.T) {
		res, err := applyWith(t, &identityCompiler{rejectLosingDLC: "lint"}, reconcile.Policy{})
		assert.NilError(t, err)
		assert.Check(t, cmp.Equal(res.Document.Summary.Applied, 1))
		assert.Check(t, cmp.Equal(res.Rejected, 1))
		i := slices.IndexFunc(res.Document.Findings, func(f report.Finding) bool { return f.Target.Job == "lint" })
		assert.Assert(t, i >= 0, "no finding for job %s", "lint")
		f := res.Document.Findings[i]
		assert.Check(t, cmp.Equal(f.Disposition, "rejected"))
		assert.Check(t, cmp.Contains(f.Reason, "does not compile: lint candidate rejected"))
		assert.Check(t, machineDLC(res.Output, "lint"), "a rejected change leaves its job untouched")
		assert.Check(t, !machineDLC(res.Output, "unit"), "the accepted change is kept")
	})

	t.Run("a finding that becomes unactionable is reported from the fresh analysis", func(t *testing.T) {
		res, err := engine.Apply(context.Background(), engine.AnalyzeInput{
			Path:     "config.yml",
			Config:   []byte(twoJobs),
			Modules:  []module.Analyzer{&fading{Analyzer: dropDLC{}}},
			Compiler: &identityCompiler{},
		})
		assert.NilError(t, err)
		assert.Check(t, cmp.Equal(res.Rejected, 1))
		var seen bool
		for _, f := range res.Document.Findings {
			if f.Disposition == "rejected" {
				seen = true
				assert.Check(t, cmp.Contains(f.Reason, "no longer actionable"))
				assert.Check(t, cmp.Equal(f.Suggestion.Note, "fresh analysis"))
			}
		}
		assert.Check(t, seen, "one finding is rejected")
	})

	t.Run("compiles the input twice, then each candidate once", func(t *testing.T) {
		tests := []struct {
			name        string
			compiler    *identityCompiler
			policy      reconcile.Policy
			wantCalls   int
			wantApplied int
		}{
			// A report-only check plans nothing.
			{name: "nothing planned", compiler: &identityCompiler{}, policy: reconcile.Policy{ReportOnly: map[string]string{dropDLCName: "report-only"}}, wantCalls: 1},
			{name: "both applied, one at a time", compiler: &identityCompiler{}, wantCalls: 4, wantApplied: 2},
			{name: "one rejected", compiler: &identityCompiler{rejectLosingDLC: "lint"}, wantCalls: 4, wantApplied: 1},
		}
		for _, tc := range tests {
			t.Run(tc.name, func(t *testing.T) {
				res, err := applyWith(t, tc.compiler, tc.policy)
				assert.NilError(t, err)
				assert.Check(t, cmp.Equal(tc.compiler.calls, tc.wantCalls))
				assert.Check(t, cmp.Equal(res.Document.Summary.Applied, tc.wantApplied))
				if tc.wantApplied == 0 {
					assert.Check(t, cmp.Equal(string(res.Output), twoJobs), "a no-op gives identical bytes")
				}
			})
		}
	})

	t.Run("a rejected group leaves the config unchanged, so it is not analysed again", func(t *testing.T) {
		m := &fading{Analyzer: dropDLC{}}
		res, err := engine.Apply(context.Background(), engine.AnalyzeInput{
			Path:     "config.yml",
			Config:   []byte(twoJobs),
			Modules:  []module.Analyzer{m},
			Compiler: &identityCompiler{rejectLosingDLC: "lint"},
		})
		assert.NilError(t, err)
		assert.Check(t, cmp.Equal(m.calls, 1))
		assert.Check(t, cmp.Equal(res.Document.Summary.Applied, 1))
		assert.Check(t, cmp.Equal(res.Rejected, 1))
	})

	t.Run("compiler drift aborts instead of blaming a change", func(t *testing.T) {
		_, err := applyWith(t, &identityCompiler{driftFrom: 2}, reconcile.Policy{})
		cliErr, ok := errors.AsType[*clierrors.CLIError](err)
		assert.Assert(t, ok, "want a CLIError, got %v", err)
		assert.Check(t, cmp.Equal(cliErr.Code, "compile.drift"))
	})
}
