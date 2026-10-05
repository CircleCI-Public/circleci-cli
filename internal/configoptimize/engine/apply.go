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

package engine

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"

	clierrors "github.com/CircleCI-Public/circleci-cli/clikit/errors"

	"github.com/CircleCI-Public/circleci-cli/internal/configoptimize/finding"
	"github.com/CircleCI-Public/circleci-cli/internal/configoptimize/patch"
	"github.com/CircleCI-Public/circleci-cli/internal/configoptimize/pipelineconfig"
	"github.com/CircleCI-Public/circleci-cli/internal/configoptimize/plan"
	"github.com/CircleCI-Public/circleci-cli/internal/configoptimize/reconcile"
	"github.com/CircleCI-Public/circleci-cli/internal/configoptimize/report"
)

// ApplyResult is the outcome of an apply run.
type ApplyResult struct {
	// Output is the optimized pipelineconfig. With nothing applied it is the input,
	// byte for byte.
	Output   []byte
	Document report.Document
	// Rejected counts groups that were planned but not applied.
	Rejected int
}

// Apply analyses the config, then applies each planned group one at a time:
// edit the current bytes, recompile with the same inputs, and keep the
// change only if the compiled config changed exactly as the group said. It writes nothing; publishing is the caller's job.
func Apply(ctx context.Context, in AnalyzeInput) (ApplyResult, error) {
	loaded, err := Load(ctx, in.Compiler, in.Config, in.Params)
	if err != nil {
		return ApplyResult{}, err
	}
	entries, err := evaluate(ctx, loaded, in)
	if err != nil {
		return ApplyResult{}, err
	}
	if err := uniqueIDs(entries); err != nil {
		return ApplyResult{}, err
	}
	planned := reconcile.Plan(entries)

	current, compiled := in.Config, loaded.Compiled
	outcome := map[string]result{}
	// latest holds each re-found finding's entry from the analysis it was
	// applied from, so the report describes what was actually applied.
	latest := map[string]reconcile.Entry{}
	for i, g := range planned.Groups {
		// Re-analyse the current bytes, which also recompiles them: the
		// drift check before every candidate. Groups after the first are
		// looked up again by ID, since earlier edits moved their offsets.
		fresh, err := Load(ctx, in.Compiler, current, in.Params)
		if err != nil {
			return ApplyResult{}, err
		}
		same, err := patch.SameCompile(compiled, fresh.Compiled)
		if err != nil {
			return ApplyResult{}, clierrors.New("compile.unreadable", "Compiled config unreadable",
				"the compiled config could not be compared with the previous compile: "+err.Error()).
				WithSuggestions("Run `circleci config process` to check the compiled output").
				WithExitCode(clierrors.ExitGeneralError)
		}
		if !same {
			return ApplyResult{}, clierrors.New("compile.drift", "Compiler output drifted",
				"recompiling the unchanged config gave a different result, so no change can be judged").
				WithSuggestions("Pin orb versions, and keep the checkout unchanged while the command runs").
				WithExitCode(clierrors.ExitGeneralError)
		}
		if i > 0 {
			freshEntries, err := evaluate(ctx, fresh, in)
			if err != nil {
				return ApplyResult{}, err
			}
			if err := uniqueIDs(freshEntries); err != nil {
				return ApplyResult{}, err
			}
			// Record the fresh entry first, so a finding that is now
			// report-only is reported as it is now.
			for _, e := range freshEntries {
				if e.Finding.ID == g.FindingID {
					latest[g.FindingID] = e
				}
			}
			again, ok := findGroup(reconcile.Plan(freshEntries), g.FindingID)
			if !ok {
				outcome[g.FindingID] = rejected("no longer actionable after earlier changes")
				continue
			}
			if !reflect.DeepEqual(again.Fingerprint, g.Fingerprint) {
				return ApplyResult{}, internalError(fmt.Sprintf("finding ID %s now names a different target", g.FindingID))
			}
			g = again
		}
		if blocker := unmet(g, outcome); blocker != "" {
			outcome[g.FindingID] = rejected("blocked: prerequisite " + blocker + " was not applied")
			continue
		}
		r, next, nextCompiled, err := try(ctx, in, current, compiled, g)
		if err != nil {
			return ApplyResult{}, err
		}
		outcome[g.FindingID] = r
		if r.applied {
			current, compiled = next, nextCompiled
		}
	}

	doc := report.Build(meta(report.CommandWrite, in, loaded), withOutcomes(entries, latest, outcome))
	res := ApplyResult{Output: current, Document: doc}
	for _, r := range outcome {
		if !r.applied {
			res.Rejected++
		}
	}
	return res, nil
}

type result struct {
	applied bool
	reason  string
}

func rejected(reason string) result { return result{reason: reason} }

// try applies one group to current and judges it. A refused edit, a config
// that no longer compiles, or a compiled change other than the expected one
// rejects the group; only a failure to compile at all is an error.
func try(ctx context.Context, in AnalyzeInput, current, compiled []byte, g plan.Group) (result, []byte, []byte, error) {
	candidate, err := patch.Candidate(current, g)
	if err != nil {
		return rejected("edit refused: " + err.Error()), nil, nil, nil
	}
	out, err := in.Compiler.Compile(ctx, pipelineconfig.CompileInput{ConfigYAML: candidate, PipelineParameters: in.Params})
	if err != nil {
		if invalid, ok := errors.AsType[*pipelineconfig.InvalidConfigError](err); ok {
			return rejected("the edited config does not compile: " + strings.Join(invalid.Diagnostics, "; ")), nil, nil, nil
		}
		return result{}, nil, nil, compileError(err)
	}
	if err := patch.CheckDelta(compiled, out.CompiledYAML, g.Expected); err != nil {
		return rejected("unexpected compiled change: " + err.Error()), nil, nil, nil
	}
	return result{applied: true}, candidate, out.CompiledYAML, nil
}

// uniqueIDs requires every finding ID to be unique, since outcomes and
// re-finding are keyed by it. A duplicate is an internal error.
func uniqueIDs(entries []reconcile.Entry) error {
	seen := map[string]bool{}
	for _, e := range entries {
		if seen[e.Finding.ID] {
			return internalError(fmt.Sprintf("finding ID %s appears twice", e.Finding.ID))
		}
		seen[e.Finding.ID] = true
	}
	return nil
}

// internalError is a broken invariant: a bug, not a user error.
func internalError(msg string) error {
	return clierrors.New("optimize.internal_error", "Internal error", msg).
		WithSuggestions("Please report this at https://github.com/CircleCI-Public/circleci-cli/issues").
		WithExitCode(clierrors.ExitGeneralError)
}

func findGroup(p plan.Plan, id string) (plan.Group, bool) {
	for _, g := range p.Groups {
		if g.FindingID == id {
			return g, true
		}
	}
	return plan.Group{}, false
}

func unmet(g plan.Group, outcome map[string]result) string {
	for _, req := range g.Requires {
		if !outcome[req].applied {
			return req
		}
	}
	return ""
}

// withOutcomes records each planned finding's result on its entry.
func withOutcomes(entries []reconcile.Entry, latest map[string]reconcile.Entry, outcome map[string]result) []reconcile.Entry {
	out := make([]reconcile.Entry, len(entries))
	for i, e := range entries {
		if fresh, ok := latest[e.Finding.ID]; ok {
			e = fresh
		}
		if r, planned := outcome[e.Finding.ID]; planned {
			if r.applied {
				e.Disposition = finding.DispositionApplied
			} else {
				e.Disposition = finding.DispositionRejected
				e.Reason = r.reason
			}
		}
		out[i] = e
	}
	return out
}
