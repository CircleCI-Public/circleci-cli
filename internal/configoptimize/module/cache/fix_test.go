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
	"context"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"gotest.tools/v3/assert"
	"gotest.tools/v3/assert/cmp"

	"github.com/CircleCI-Public/circleci-cli/internal/configoptimize/engine"
	"github.com/CircleCI-Public/circleci-cli/internal/configoptimize/module"
	"github.com/CircleCI-Public/circleci-cli/internal/configoptimize/module/cache"
	"github.com/CircleCI-Public/circleci-cli/internal/configoptimize/registry"
	"github.com/CircleCI-Public/circleci-cli/internal/configoptimize/report"
)

// matrixRepo lays out the files the pinned cache-optimization-matrix
// config's projects have at revision 1b22c21.
func matrixRepo(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	for _, f := range []string{
		"flask/requirements.txt", "pydantic/requirements.txt", "black/requirements.txt",
		"serde/Cargo.toml", "rayon/Cargo.toml", "gson/pom.xml", "guava/pom.xml",
		"zod/package.json", "zod/yarn.lock", "rxjs/package.json", "rxjs/package-lock.json",
		"lo/go.mod", "lo/go.sum", "hugo/go.mod", "hugo/go.sum",
	} {
		p := filepath.Join(root, "projects", f)
		assert.NilError(t, os.MkdirAll(filepath.Dir(p), 0o755))
		assert.NilError(t, os.WriteFile(p, nil, 0o600))
	}
	return root
}

// The key fix on the acceptance repo: six edits keyed on the lockfile
// the install reads, matching the team's hand-written optimized workflow,
// and five reports that say why there is no edit.
func TestKeyFixOnTheMatrixRepo(t *testing.T) {
	src, compiled := fixture(t, "cache-optimization-matrix")
	doc, err := engine.Analyze(context.Background(), engine.AnalyzeInput{
		Path: "config.yml", Config: src,
		Modules:  []module.Analyzer{cache.NewWithRepo(matrixRepo(t))},
		Compiler: fixedCompiler{compiled: compiled},
		Policy:   registry.Policy(),
	})
	assert.NilError(t, err)
	byJob := map[string]report.Finding{}
	for _, f := range doc.Findings {
		byJob[f.Target.Job] = f
	}
	edited := []struct {
		name string // the job
		lock string
	}{
		{name: "flask-baseline", lock: "projects/flask/requirements.txt"},
		{name: "pydantic-baseline", lock: "projects/pydantic/requirements.txt"},
		{name: "black-baseline", lock: "projects/black/requirements.txt"},
		{name: "rxjs-baseline", lock: "projects/rxjs/package-lock.json"},
		{name: "lo-baseline", lock: "projects/lo/go.sum"},
		{name: "hugo-baseline", lock: "projects/hugo/go.sum"},
	}
	for _, tc := range edited {
		t.Run(tc.name, func(t *testing.T) {
			f, ok := byJob[tc.name]
			assert.Assert(t, ok, "no finding for %s", tc.name)
			assert.Check(t, cmp.Equal(f.Disposition, "actionable"))
			assert.Check(t, cmp.Equal(f.Suggestion.Op, "set"))
			assert.Check(t, cmp.Equal(f.Suggestion.Value, `{{ checksum "`+tc.lock+`" }}`))
			assert.Assert(t, len(f.AuthoredPaths) > 0, "no authored path for %s", tc.name)
			assert.Check(t, cmp.Equal(f.AuthoredPaths[0], f.Target.Site), "%s is edited at its call site", tc.name)
		})
	}
	reported := []struct {
		name string // the job
		why  string
	}{
		{name: "gson-baseline", why: "Maven has no lockfile"},
		{name: "guava-baseline", why: "Maven has no lockfile"},
		{name: "serde-baseline", why: "cargo without --locked"},
		{name: "rayon-baseline", why: "cargo without --locked"},
		{name: "zod-baseline", why: "yarn install without --frozen-lockfile"},
	}
	for _, tc := range reported {
		t.Run(tc.name, func(t *testing.T) {
			f, ok := byJob[tc.name]
			assert.Assert(t, ok, "no finding for %s", tc.name)
			assert.Check(t, cmp.Equal(f.Disposition, "report_only"))
			assert.Check(t, cmp.Contains(f.Suggestion.Note, "No edit: needs a lockfile: "+tc.why))
			assert.Check(t, cmp.Contains(f.Suggestion.Note, "Suggested key:"))
		})
	}
}

// Without the repo, nothing is edited: a lockfile cannot be confirmed.
func TestKeyFixNeedsTheRepo(t *testing.T) {
	doc := analyze(t, "cache-optimization-matrix")
	assert.Assert(t, len(doc.Findings) > 0, "the matrix repo has cache findings")
	for _, f := range doc.Findings {
		assert.Check(t, cmp.Equal(f.Disposition, "report_only"), f.Target.Job)
		assert.Check(t, cmp.Contains(f.Suggestion.Note, "the repo is not available"), f.Target.Job)
	}
}

// A lockfile the install reads but the repo lacks is not keyed on.
func TestKeyFixNeedsTheFile(t *testing.T) {
	root := matrixRepo(t)
	assert.NilError(t, os.Remove(filepath.Join(root, "projects", "lo", "go.sum")))
	src, compiled := fixture(t, "cache-optimization-matrix")
	doc, err := engine.Analyze(context.Background(), engine.AnalyzeInput{
		Path: "config.yml", Config: src,
		Modules:  []module.Analyzer{cache.NewWithRepo(root)},
		Compiler: fixedCompiler{compiled: compiled},
		Policy:   registry.Policy(),
	})
	assert.NilError(t, err)
	i := slices.IndexFunc(doc.Findings, func(f report.Finding) bool { return f.Target.Job == "lo-baseline" })
	assert.Assert(t, i >= 0, "no finding for %s", "lo-baseline")
	f := doc.Findings[i]
	assert.Check(t, cmp.Equal(f.Disposition, "report_only"))
	assert.Check(t, cmp.Contains(f.Suggestion.Note, "projects/lo/go.sum is not in the repo"))
}
