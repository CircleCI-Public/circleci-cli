package git

import (
	"os"
	"os/exec"
	"strings"
	"testing"

	"gotest.tools/v3/assert"
)

func TestCommandOutputOrDefaultHappyPath(t *testing.T) {
	result := commandOutputOrDefault(
		exec.Command("echo", "hello"), "goodbye",
	)
	assert.Equal(t, result, "hello")
}

func TestCommandOutputOrDefaultFailure(t *testing.T) {
	result := commandOutputOrDefault(
		exec.Command("git", "this", "it", "not", "a", "command"), "goodbye",
	)
	assert.Equal(t, result, "goodbye")
}

func TestCommandOutputOrDefaultInvalidProgram(t *testing.T) {
	result := commandOutputOrDefault(
		exec.Command("this/is/not/a/command"), "morning",
	)
	assert.Equal(t, result, "morning")
}

func TestBranchTagRevisionOnCI(t *testing.T) {
	if os.Getenv("CIRCLECI") != "true" {
		t.Skip("only runs on CircleCI")
	}
	assert.Equal(t, Branch(), os.Getenv("CIRCLE_BRANCH"))
	assert.Equal(t, Revision(), os.Getenv("CIRCLE_SHA1"))
	assert.Equal(t, Tag(), os.Getenv("CIRCLE_TAG"))
}

func TestGetRemoteUrlFailsGracefully(t *testing.T) {
	_, err := getRemoteUrl("peristeronic")
	assert.Assert(t, err != nil)
	assert.Assert(t, strings.Contains(err.Error(), "Error finding the peristeronic git remote"))
}

func TestGetRemoteUrlOrigin(t *testing.T) {
	url, err := getRemoteUrl("origin")
	assert.NilError(t, err)
	assert.Assert(t, strings.Contains(url, "github"))
}

func TestFindRemoteValidURLs(t *testing.T) {
	tests := []struct {
		name     string
		url      string
		expected *Remote
	}{
		{
			name: "github ssh",
			url:  "git@github.com:foobar/foo-service.git",
			expected: &Remote{
				VcsType:      GitHub,
				Organization: "foobar",
				Project:      "foo-service",
			},
		},
		{
			name: "bitbucket ssh",
			url:  "git@bitbucket.org:example/makefile_sh.git",
			expected: &Remote{
				VcsType:      Bitbucket,
				Organization: "example",
				Project:      "makefile_sh",
			},
		},
		{
			name: "github https",
			url:  "https://github.com/apple/pear.git",
			expected: &Remote{
				VcsType:      GitHub,
				Organization: "apple",
				Project:      "pear",
			},
		},
		{
			name: "bitbucket ssh no .git",
			url:  "git@bitbucket.org:example/makefile_sh",
			expected: &Remote{
				VcsType:      Bitbucket,
				Organization: "example",
				Project:      "makefile_sh",
			},
		},
		{
			name: "bitbucket https with user",
			url:  "https://example@bitbucket.org/kiwi/fruit.git",
			expected: &Remote{
				VcsType:      Bitbucket,
				Organization: "kiwi",
				Project:      "fruit",
			},
		},
		{
			name: "bitbucket https with user no .git",
			url:  "https://example@bitbucket.org/kiwi/fruit",
			expected: &Remote{
				VcsType:      Bitbucket,
				Organization: "kiwi",
				Project:      "fruit",
			},
		},
		{
			name: "github ssh protocol",
			url:  "ssh://git@github.com/cloud/rain",
			expected: &Remote{
				VcsType:      GitHub,
				Organization: "cloud",
				Project:      "rain",
			},
		},
		{
			name: "bitbucket ssh protocol",
			url:  "ssh://git@bitbucket.org/snow/ice",
			expected: &Remote{
				VcsType:      Bitbucket,
				Organization: "snow",
				Project:      "ice",
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result, err := findRemote(tc.url)
			assert.NilError(t, err)
			assert.DeepEqual(t, result, tc.expected)
		})
	}
}

func TestFindRemoteInvalidURLs(t *testing.T) {
	tests := []struct {
		name    string
		url     string
		wantErr string
	}{
		{
			name:    "unknown remote",
			url:     "asd/asd/asd",
			wantErr: "Unknown git remote: asd/asd/asd",
		},
		{
			name:    "too many path segments",
			url:     "git@github.com:foo/bar/baz",
			wantErr: "Splitting 'foo/bar/baz' into organization and project failed",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := findRemote(tc.url)
			assert.Error(t, err, tc.wantErr)
		})
	}
}
