package gitremote

import (
	"testing"

	"gotest.tools/v3/assert"
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
				assert.ErrorContains(t, err, tc.wantError)
				return
			}
			assert.NilError(t, err)
			assert.Equal(t, slug, tc.wantSlug)
		})
	}
}
