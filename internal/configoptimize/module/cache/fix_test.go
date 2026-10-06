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
	"os"
	"path/filepath"
	"testing"

	"gotest.tools/v3/assert"
	"gotest.tools/v3/assert/cmp"
)

// fixRepo lays out the lockfiles fix.yml's installs read; hugo's go.sum is
// missing.
func fixRepo(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	for _, f := range []string{"flask/requirements.txt", "rxjs/package-lock.json", "lo/go.sum", "hugo/go.mod"} {
		p := filepath.Join(root, "projects", f)
		assert.NilError(t, os.MkdirAll(filepath.Dir(p), 0o755))
		assert.NilError(t, os.WriteFile(p, nil, 0o600))
	}
	return root
}

// The key fix keys each install on the lockfile it reads, at the call site
// that sets the key, and says why when there is no lockfile to key on.
func TestKeyFix(t *testing.T) {
	findings := byJob(analyze(t, "fix", fixRepo(t)))
	edited := []struct {
		name string // the job
		lock string
	}{
		{name: "flask", lock: "projects/flask/requirements.txt"},
		{name: "rxjs", lock: "projects/rxjs/package-lock.json"},
		{name: "lo", lock: "projects/lo/go.sum"},
	}
	for _, tc := range edited {
		t.Run(tc.name, func(t *testing.T) {
			f := only(t, findings, tc.name)
			assert.Check(t, cmp.Equal(f.Disposition, "actionable"))
			assert.Check(t, cmp.Equal(f.Suggestion.Op, "set"))
			assert.Check(t, cmp.Equal(f.Suggestion.Value, `{{ checksum "`+tc.lock+`" }}`))
			assert.Check(t, cmp.DeepEqual(f.AuthoredPaths, []string{f.Target.Site}), "edited at its call site")
		})
	}
	reported := []struct {
		name string // the job
		why  string
	}{
		{name: "hugo", why: "projects/hugo/go.sum is not in the repo"},
		{name: "gson", why: "Maven has no lockfile"},
		{name: "serde", why: "cargo without --locked"},
		{name: "zod", why: "yarn install without --frozen-lockfile"},
	}
	for _, tc := range reported {
		t.Run(tc.name, func(t *testing.T) {
			f := only(t, findings, tc.name)
			assert.Check(t, cmp.Equal(f.Disposition, "report_only"))
			assert.Check(t, cmp.Contains(f.Suggestion.Note, "No edit: needs a lockfile: "+tc.why))
			assert.Check(t, cmp.Contains(f.Suggestion.Note, "Suggested key:"))
		})
	}
}

// Without the repo, nothing is edited: a lockfile cannot be confirmed.
func TestKeyFixNeedsTheRepo(t *testing.T) {
	doc := analyze(t, "fix", "")
	assert.Check(t, cmp.Len(doc.Findings, 7))
	for _, f := range doc.Findings {
		assert.Check(t, cmp.Equal(f.Disposition, "report_only"), f.Target.Job)
		assert.Check(t, cmp.Contains(f.Suggestion.Note, "the repo is not available"), f.Target.Job)
	}
}
