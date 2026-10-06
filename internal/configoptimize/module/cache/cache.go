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

// Package cache finds what a config shows about caching: dependency
// installs with no cache, and restores whose keys change every pipeline.
//
// model.go is the entity model: the cache steps of the compiled config and
// a graph of which restore policies may read which saved keys. Keys are
// compared as typed templates, broad matches are flagged, never merged, and
// no two steps are ever grouped into one "cache".
package cache

import (
	"context"
	"maps"
	"slices"
	"strconv"
	"strings"

	"github.com/CircleCI-Public/circleci-cli/internal/configoptimize/finding"
	"github.com/CircleCI-Public/circleci-cli/internal/configoptimize/pipelineconfig"
)

// Module is the cache analyzer. It reads the config and, for the key fix,
// the repo checkout to confirm a lockfile exists. Its findings carry no
// credit figure.
type Module struct {
	// repo is the checkout root, "" when there is none.
	repo string
}

// New returns the module with no repo: it reports, and fixes nothing.
func New() *Module { return &Module{} }

// NewWithRepo returns the module reading lockfiles from the checkout at root.
func NewWithRepo(root string) *Module { return &Module{repo: root} }

// Name implements module.Analyzer.
func (*Module) Name() string { return Name }

// Evaluate implements module.Analyzer.
func (m *Module) Evaluate(_ context.Context, cfg *pipelineconfig.Effective) ([]finding.Finding, error) {
	a := Analyze(cfg)
	var out []finding.Finding
	for _, pv := range a.Policies {
		if pv.Volatile {
			f := volatileFinding(pv)
			withFix(&f, planFix(cfg, a.Model, pv, m.repo))
			out = append(out, f)
		}
	}
	for _, jv := range a.Jobs {
		if jv.Candidate() {
			out = append(out, candidateFinding(jv))
		}
	}
	return out, nil
}

// unknownImpact: the slice makes no time or savings claim.
var unknownImpact = finding.Impact{
	Time: finding.DirectionUnknown, Cost: finding.DirectionUnknown, Referent: finding.ReferentCurrentState,
	Basis: "config only: no step times, run counts or cache statistics",
}

func volatileFinding(pv PolicyVerdict) finding.Finding {
	p := pv.Policy
	// Report at the key that gives the policy its scope: the broadest one.
	site := pv.Keys[0].Site
	for _, kv := range pv.Keys {
		if kv.Scope == pv.Scope {
			site = kv.Site
			break
		}
	}
	target := finding.Target{Job: p.Job, Path: p.Path, Site: site}
	evidence := make([]finding.Evidence, 0, 2+len(pv.Keys))
	evidence = append(evidence, finding.Evidence{
		Kind: finding.EvidenceConfigFact, Claim: "rule",
		Value: RuleX1 + ": every key the restore tries is scoped, the policy's scope is its broadest key, and reuse is " +
			"limited to that scope; WORKFLOW_SCOPED, PIPELINE_SCOPED and REVISION_SCOPED are reported only for dependency caches",
		Source: "cache check",
	})
	for i, kv := range pv.Keys {
		evidence = append(evidence, finding.Evidence{
			Kind: finding.EvidenceConfigFact, Claim: "key " + strconv.Itoa(i) + " is " + kv.Scope.Code(),
			Value: kv.Key.Raw + ": " + kv.Reason, Source: "config:" + strings.Join(kv.Site, "."),
		})
	}
	if pv.Saved {
		evidence = append(evidence, finding.Evidence{
			Kind: finding.EvidenceConfigFact, Claim: "the matching save_cache stores " + pv.PathClass.String(),
			Value: strings.Join(pv.Paths, ", "), Source: "config:jobs." + p.Job,
		})
	}
	return finding.Finding{
		ID: finding.NewID(Name, target), Module: Name, Verdict: VerdictVolatileRestore, Target: target,
		Impact:     unknownImpact,
		Confidence: volatileConfidence(pv),
		Evidence:   evidence,
		Suggestion: finding.Suggestion{Op: finding.OpNone, Note: volatileNote(pv)},
		Codes:      []string{pv.Scope.Code()},
		// TIME (the narrowest reuse) first, REVISION (the mildest) last.
		Rank: int(pv.Scope),
	}
}

func candidateFinding(jv JobVerdict) finding.Finding {
	target := finding.Target{Job: jv.Job, Path: []string{"jobs", jv.Job}}
	evidence := make([]finding.Evidence, 0, 1+len(jv.Installs))
	evidence = append(evidence, finding.Evidence{
		Kind: finding.EvidenceConfigFact, Claim: "rule",
		Value:  RuleF4 + ": no visible cache (no restore_cache or save_cache step, no reviewed function that caches)",
		Source: "cache check",
	})
	for _, in := range jv.Installs {
		evidence = append(evidence, finding.Evidence{
			Kind: finding.EvidenceConfigFact, Claim: "a run step contains a dependency install command", Value: in,
			Source: "config:jobs." + jv.Job + ".steps",
		})
	}
	return finding.Finding{
		ID: finding.NewID(Name, target), Module: Name, Verdict: VerdictCandidate, Target: target,
		Impact: unknownImpact,
		Confidence: finding.Confidence{Level: finding.ConfidenceLow,
			Reason: "ranked low: step time, run frequency and lockfile churn need data the config does not have"},
		Evidence:   evidence,
		Suggestion: finding.Suggestion{Op: finding.OpNone, Note: "A cache may help if the install is slow and the job runs often; check step times before adding one."},
	}
}

// volatileConfidence is medium for every X1 finding: a static read shows
// how far a cache can be reused, but not how often the job runs or what each
// save costs, so the waste is unmeasured.
func volatileConfidence(PolicyVerdict) finding.Confidence {
	return finding.Confidence{Level: finding.ConfidenceMedium,
		Reason: "a static read cannot show how often the job runs or what each save costs"}
}

// volatileNote states the policy's scope in the owner's wording. It says
// that reuse is limited, never that the cache never hits, and makes no claim
// about reruns until one has been observed.
func volatileNote(pv PolicyVerdict) string {
	return "Every key this restore tries is " + pv.Scope.Code() + " at broadest: the restore " + pv.Scope.limit() +
		", and no fallback key is broader. Reuse is limited; add a stable fallback key, or key on a lockfile checksum."
}

// withFix turns a volatile finding into a proposed edit when the key fix
// applies, and otherwise says why not and what key to use.
func withFix(f *finding.Finding, fix Fix) {
	if fix.Source == nil {
		f.Suggestion.Note += " No edit: " + fix.Why + "."
		if fix.Suggested != "" {
			f.Suggestion.Note += " Suggested key: " + fix.Suggested + "."
		}
		return
	}
	var paths [][]string
	var compiled []finding.CompiledSet
	for _, k := range slices.Sorted(maps.Keys(fix.Compiled)) {
		p := strings.Split(k, "\x00")
		paths = append(paths, p)
		compiled = append(compiled, finding.CompiledSet{Path: p, Value: fix.Compiled[k]})
	}
	f.Suggestion = finding.Suggestion{
		Op:       finding.OpSet,
		Paths:    paths,
		Value:    fix.New,
		Authored: []finding.AuthoredSet{{Path: fix.Source, Expect: fix.Old, Value: fix.New}},
		Compiled: compiled,
		Note: "Key the cache on " + fix.Lockfile + ": " + strings.Join(fix.Source, ".") + " becomes " + fix.New +
			", which changes the restore and the save together, so the cache is reused until the lockfile changes. " +
			"Existing caches under the old keys age out over one retention period.",
	}
	f.Evidence = append(f.Evidence, finding.Evidence{
		Kind: finding.EvidenceConfigFact, Claim: "the install reads one lockfile, present in the repo and not rewritten",
		Value: fix.Lockfile, Source: "repo:" + fix.Lockfile,
	})
}
