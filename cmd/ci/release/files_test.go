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

package main

import (
	"testing"

	"gotest.tools/v3/assert"
	"gotest.tools/v3/assert/cmp"
)

func pr(number int, labels ...string) pullRequest {
	p := pullRequest{Number: number}
	for _, l := range labels {
		p.Labels = append(p.Labels, struct {
			Name string `json:"name"`
		}{l})
	}
	return p
}

func TestBumpFor(t *testing.T) {
	tests := []struct {
		name string
		prs  []pullRequest
		want bump
	}{
		{
			name: "unlabelled PRs are minor",
			prs:  []pullRequest{pr(1), pr(2, "bug")},
			want: bumpMinor,
		},
		{
			name: "all patch",
			prs:  []pullRequest{pr(1, "release:patch"), pr(2, "release:patch")},
			want: bumpPatch,
		},
		{
			name: "an unlabelled PR outranks patch",
			prs:  []pullRequest{pr(1, "release:patch"), pr(2)},
			want: bumpMinor,
		},
		{
			name: "any major",
			prs:  []pullRequest{pr(1, "release:patch"), pr(2, "release:major"), pr(3)},
			want: bumpMajor,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := bumpFor(tt.prs)
			assert.Check(t, cmp.Equal(got, tt.want))
		})
	}
}

func TestBump(t *testing.T) {
	v := version{1, 0, 52041}
	for b, want := range map[bump]string{bumpPatch: "1.0.52042", bumpMinor: "1.1.0", bumpMajor: "2.0.0"} {
		got := v.bump(b).String()
		assert.Check(t, cmp.Equal(got, want))
	}
}

func TestLatestVersion(t *testing.T) {
	t.Run("of the newest section", func(t *testing.T) {
		got := addRelease(changelog, version{1, 4, 0}, "2026-10-02", "notes")
		v, err := latestVersion(got)
		assert.NilError(t, err)
		assert.Check(t, cmp.Equal(v, version{1, 4, 0}))
	})

	t.Run("without a section", func(t *testing.T) {
		_, err := latestVersion("# Changelog\n\n## Unreleased\n")
		assert.Check(t, cmp.ErrorContains(err, `CHANGELOG.md has no "## [x.y.z]" release section`))
	})
}

const changelog = `# Changelog

Intro.

## [1.3.1] - 2026-01-29

### Fixed
- A fix
`

func TestAddRelease(t *testing.T) {
	notes := "<!-- Release notes generated using configuration in .github/release.yml at main -->\n\n" +
		"## What's Changed\n* Add a thing by @someone in https://github.com/o/r/pull/9\n\n\n" +
		"**Full Changelog**: https://github.com/o/r/compare/v1.3.1...v1.4.0"

	got := addRelease(changelog, version{1, 4, 0}, "2026-10-02", notes)

	t.Run("adds the section above the newest", func(t *testing.T) {
		want := `# Changelog

Intro.

## [1.4.0] - 2026-10-02

### What's Changed
* Add a thing by @someone in https://github.com/o/r/pull/9


**Full Changelog**: https://github.com/o/r/compare/v1.3.1...v1.4.0

## [1.3.1] - 2026-01-29

### Fixed
- A fix
`
		assert.Check(t, cmp.Equal(got, want))
	})

	t.Run("reads back as the release notes", func(t *testing.T) {
		notesGot, err := releaseNotes(got, version{1, 4, 0})
		assert.NilError(t, err)
		notesWant := "### What's Changed\n* Add a thing by @someone in https://github.com/o/r/pull/9\n\n\n" +
			"**Full Changelog**: https://github.com/o/r/compare/v1.3.1...v1.4.0"
		assert.Check(t, cmp.Equal(notesGot, notesWant))
	})

	t.Run("to a changelog with no releases", func(t *testing.T) {
		got := addRelease("# Changelog\n\nIntro.\n", version{1, 1, 0}, "2026-10-02", "notes")
		assert.Check(t, cmp.Equal(got, "# Changelog\n\nIntro.\n\n## [1.1.0] - 2026-10-02\n\nnotes\n\n"))
	})
}

func TestReleaseNotes(t *testing.T) {
	t.Run("of the last section", func(t *testing.T) {
		got, err := releaseNotes(changelog, version{1, 3, 1})
		assert.NilError(t, err)
		assert.Check(t, cmp.Equal(got, "### Fixed\n- A fix"))
	})

	t.Run("without the section", func(t *testing.T) {
		_, err := releaseNotes(changelog, version{1, 3, 2})
		assert.Check(t, cmp.ErrorContains(err, "no ## [1.3.2] section"))
	})
}
