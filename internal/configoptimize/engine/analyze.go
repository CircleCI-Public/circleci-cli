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
	"fmt"
	"slices"
	"strings"

	clierrors "github.com/CircleCI-Public/circleci-cli/clikit/errors"

	"github.com/CircleCI-Public/circleci-cli/internal/configoptimize/data"
	"github.com/CircleCI-Public/circleci-cli/internal/configoptimize/finding"
	"github.com/CircleCI-Public/circleci-cli/internal/configoptimize/module"
	"github.com/CircleCI-Public/circleci-cli/internal/configoptimize/pipelineconfig"
	"github.com/CircleCI-Public/circleci-cli/internal/configoptimize/reconcile"
	"github.com/CircleCI-Public/circleci-cli/internal/configoptimize/report"
)

// AnalyzeInput is one analyze run.
type AnalyzeInput struct {
	// Path is the config path as given, or "-" for stdin.
	Path   string
	Config []byte
	Params map[string]any
	// Modules are already selected from the catalog, in catalog order.
	Modules  []module.Analyzer
	Compiler pipelineconfig.Compiler
	// Policy is module policy for feasibility, e.g. registry.Policy().
	Policy reconcile.Policy
}

// Analyze loads the config, runs every module on the effective copy, decides
// each finding's disposition, and returns the report document. It writes
// nothing.
func Analyze(ctx context.Context, in AnalyzeInput) (report.Document, error) {
	loaded, err := Load(ctx, in.Compiler, in.Config, in.Params)
	if err != nil {
		return report.Document{}, err
	}
	entries, err := evaluate(ctx, loaded, in)
	if err != nil {
		return report.Document{}, err
	}
	return report.Build(meta(report.CommandReport, in, loaded), entries), nil
}

// evaluate runs every module on the effective copy and decides each
// finding's disposition.
func evaluate(ctx context.Context, loaded *Loaded, in AnalyzeInput) ([]reconcile.Entry, error) {
	// Modules run one after another in catalog order, and each module's
	// findings are sorted by target, so output never depends on timing.
	// TODO: decide how to keep this order if modules run in parallel.
	var findings []finding.Finding
	view := data.Empty() // No telemetry in v1; fixtures come later.
	for _, m := range in.Modules {
		got, err := m.Evaluate(ctx, loaded.Effective, view)
		if err != nil {
			// A module that fails stops the run rather than leave a partial
			// report.
			return nil, clierrors.New("optimize.check_failed", "Check failed",
				fmt.Sprintf("the %s check failed: %v", m.Name(), err)).
				WithSuggestions(fmt.Sprintf("Run again without it: --only lists the checks to run (it was %s)", m.Name())).
				WithExitCode(clierrors.ExitGeneralError)
		}
		for i := range got {
			if got[i].ID == "" {
				got[i].ID = finding.NewID(got[i].Module, got[i].Target)
			}
		}
		slices.SortStableFunc(got, compareFindings)
		findings = append(findings, got...)
	}
	return reconcile.Feasibility(findings, loaded.Effective, loaded.Mutability, in.Policy), nil
}

func meta(command string, in AnalyzeInput, loaded *Loaded) report.Meta {
	names := make([]string, 0, len(in.Modules))
	var missing []string
	for _, m := range in.Modules {
		names = append(names, m.Name())
		if r, ok := m.(module.InputReporter); ok {
			for _, what := range r.MissingInputs() {
				missing = append(missing, m.Name()+": "+what)
			}
		}
	}
	return report.Meta{
		Command:            command,
		InputPath:          in.Path,
		Input:              in.Config,
		PipelineParameters: in.Params,
		Compiler:           loaded.Compiler,
		Modules:            names,
		MissingInputs:      missing,
	}
}

func compareFindings(a, b finding.Finding) int {
	if a.Rank != b.Rank {
		return a.Rank - b.Rank
	}
	if c := strings.Compare(a.Target.Job, b.Target.Job); c != 0 {
		return c
	}
	if c := slices.Compare(a.Target.Path, b.Target.Path); c != 0 {
		return c
	}
	return strings.Compare(a.ID, b.ID)
}
