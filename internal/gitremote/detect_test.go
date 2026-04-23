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
	"testing"

	"gotest.tools/v3/assert"
	"gotest.tools/v3/assert/cmp"
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
