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

package gitremote

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"gotest.tools/v3/assert"
	"gotest.tools/v3/assert/cmp"

	"github.com/CircleCI-Public/circleci-cli-v2/internal/projectref"
)

func TestSlugFromRemote(t *testing.T) {
	tests := []struct {
		name      string
		url       string
		wantSlug  string
		wantError string
	}{
		// SSH — GitHub
		{
			name:     "ssh github with .git suffix",
			url:      "git@github.com:myorg/myrepo.git",
			wantSlug: "gh/myorg/myrepo",
		},
		{
			name:     "ssh github without .git suffix",
			url:      "git@github.com:myorg/myrepo",
			wantSlug: "gh/myorg/myrepo",
		},
		// SSH protocol-style — GitHub
		{
			name:     "ssh protocol github with .git suffix",
			url:      "ssh://git@github.com/myorg/myrepo.git",
			wantSlug: "gh/myorg/myrepo",
		},
		{
			name:     "ssh protocol github without .git suffix",
			url:      "ssh://git@github.com/myorg/myrepo",
			wantSlug: "gh/myorg/myrepo",
		},
		// SSH — Bitbucket
		{
			name:     "ssh bitbucket",
			url:      "git@bitbucket.org:myorg/myrepo.git",
			wantSlug: "bb/myorg/myrepo",
		},
		// SSH — GitLab
		{
			name:     "ssh gitlab",
			url:      "git@gitlab.com:myorg/myrepo.git",
			wantSlug: "gl/myorg/myrepo",
		},
		// SSH — self-hosted GitLab
		{
			name:     "ssh self-hosted gitlab",
			url:      "git@gitlab.mycompany.com:myorg/myrepo.git",
			wantSlug: "gl/myorg/myrepo",
		},
		// HTTPS — GitHub
		{
			name:     "https github with .git suffix",
			url:      "https://github.com/myorg/myrepo.git",
			wantSlug: "gh/myorg/myrepo",
		},
		{
			name:     "https github without .git suffix",
			url:      "https://github.com/myorg/myrepo",
			wantSlug: "gh/myorg/myrepo",
		},
		{
			name:     "http (not https) github",
			url:      "http://github.com/myorg/myrepo.git",
			wantSlug: "gh/myorg/myrepo",
		},
		// HTTPS — Bitbucket
		{
			name:     "https bitbucket",
			url:      "https://bitbucket.org/myorg/myrepo.git",
			wantSlug: "bb/myorg/myrepo",
		},
		// HTTPS — GitLab
		{
			name:     "https gitlab",
			url:      "https://gitlab.com/myorg/myrepo.git",
			wantSlug: "gl/myorg/myrepo",
		},
		// Whitespace trimming
		{
			name:     "trailing newline trimmed",
			url:      "git@github.com:myorg/myrepo.git\n",
			wantSlug: "gh/myorg/myrepo",
		},
		{
			name:     "leading and trailing whitespace trimmed",
			url:      "  https://github.com/myorg/myrepo.git  ",
			wantSlug: "gh/myorg/myrepo",
		},
		// Error cases
		{
			name:      "unsupported host",
			url:       "git@codeberg.org:myorg/myrepo.git",
			wantError: `unsupported VCS host "codeberg.org"`,
		},
		{
			name:      "unrecognised format",
			url:       "not-a-url",
			wantError: "unrecognised git remote URL format",
		},
		{
			name:      "empty string",
			url:       "",
			wantError: "unrecognised git remote URL format",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			slug, err := SlugFromRemote(tc.url)
			if tc.wantError != "" {
				assert.Check(t, cmp.ErrorContains(err, tc.wantError))
				return
			}
			assert.NilError(t, err)
			assert.Check(t, cmp.Equal(slug, tc.wantSlug))
		})
	}
}

// TestDetect_PrefersInfoYml verifies that an existing .circleci/info.yml takes
// priority over the git remote, and that a UUID-bearing record is normalised
// to the canonical "circleci/<orgID>/<projectID>" slug.
func TestDetect_PrefersInfoYml(t *testing.T) {
	tests := []struct {
		name     string
		info     projectref.Info
		wantSlug string
	}{
		{
			name: "uuids present yields canonical slug",
			info: projectref.Info{
				Organization: projectref.Organization{ID: "E6i3yYZeWZhcf8UNqcKfjN"},
				Project: projectref.Project{
					Slug: "gh/myorg/myrepo",
					ID:   "13c8F7nusayivoSxC6GMsw",
				},
			},
			wantSlug: "circleci/E6i3yYZeWZhcf8UNqcKfjN/13c8F7nusayivoSxC6GMsw",
		},
		{
			name:     "slug-only falls through verbatim",
			info:     projectref.Info{Project: projectref.Project{Slug: "gh/myorg/legacy"}},
			wantSlug: "gh/myorg/legacy",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			assert.NilError(t, projectref.Write(dir, &tc.info))

			cwd, err := os.Getwd()
			assert.NilError(t, err)
			t.Cleanup(func() { _ = os.Chdir(cwd) })
			assert.NilError(t, os.Chdir(dir))

			info, err := Detect()
			assert.NilError(t, err)
			assert.Check(t, cmp.Equal(info.Slug, tc.wantSlug))
		})
	}
}

// Sanity check that DetectFromRemote does not consult info.yml — used by
// `project link` to avoid short-circuiting against an existing entry.
func TestDetectFromRemote_IgnoresInfoYml(t *testing.T) {
	dir := t.TempDir()
	assert.NilError(t, projectref.Write(dir, &projectref.Info{
		Organization: projectref.Organization{ID: "OID"},
		Project: projectref.Project{
			Slug: "gh/myorg/myrepo",
			ID:   "PID",
		},
	}))

	cwd, err := os.Getwd()
	assert.NilError(t, err)
	t.Cleanup(func() { _ = os.Chdir(cwd) })
	assert.NilError(t, os.Chdir(dir))

	_, err = DetectFromRemote()
	assert.Check(t, err != nil, "expected git-remote detection to fail in a non-git temp dir")

	// And info.yml is still present — DetectFromRemote did not consume it.
	_, statErr := os.Stat(filepath.Join(dir, projectref.FilePath))
	assert.NilError(t, statErr)
}

func TestWorkTreeRoot_FromNestedDirectory(t *testing.T) {
	dir := t.TempDir()
	out, err := exec.Command("git", "init", dir).CombinedOutput()
	assert.NilError(t, err, "git init failed: %s", out)

	nested := filepath.Join(dir, "one", "two")
	assert.NilError(t, os.MkdirAll(nested, 0o755))

	cwd, err := os.Getwd()
	assert.NilError(t, err)
	t.Cleanup(func() { _ = os.Chdir(cwd) })
	assert.NilError(t, os.Chdir(nested))

	root, err := WorkTreeRoot()
	assert.NilError(t, err)
	realRoot, err := filepath.EvalSymlinks(root)
	assert.NilError(t, err)
	realDir, err := filepath.EvalSymlinks(dir)
	assert.NilError(t, err)
	assert.Check(t, cmp.Equal(realRoot, realDir))
	assert.Check(t, InsideWorkTree(), "expected nested directory to be inside work tree")
}
