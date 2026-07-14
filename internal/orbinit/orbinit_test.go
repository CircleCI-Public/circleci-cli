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

package orbinit

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/CircleCI-Public/circleci-cli/internal/iostream"

	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
	"gotest.tools/v3/fs"
)

func TestSubstitute(t *testing.T) {
	in := "orb: <orb-name>\n" +
		"ns: <namespace>\n" +
		"ctx: <publishing-context>\n" +
		"project: <project-name>\n" +
		"org: <organization>\n" +
		"<!--- hidden --->\n" +
		"**Meta** should be stripped\n"

	got := Substitute(in, "my-project", "acme", "my-orb", "acme-ns")

	t.Run("replaces placeholders", func(t *testing.T) {
		assert.Check(t, is.Contains(got, "orb: my-orb"))
		assert.Check(t, is.Contains(got, "ns: acme-ns"))
		assert.Check(t, is.Contains(got, "ctx: orb-publishing"))
		assert.Check(t, is.Contains(got, "project: my-project"))
		assert.Check(t, is.Contains(got, "org: acme"))
	})

	t.Run("unwraps comment markers but keeps inner content", func(t *testing.T) {
		assert.Check(t, is.Contains(got, " hidden "))
		assert.Check(t, !bytes.Contains([]byte(got), []byte("<!---")))
		assert.Check(t, !bytes.Contains([]byte(got), []byte("--->")))
	})

	t.Run("strips meta marker line", func(t *testing.T) {
		assert.Check(t, !bytes.Contains([]byte(got), []byte("**Meta**")))
	})
}

func TestProjectNameFromRemote(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"https with .git suffix", "https://github.com/acme/my-orb.git", "my-orb"},
		{"https without suffix", "https://github.com/acme/my-orb", "my-orb"},
		{"ssh scp-style", "git@github.com:acme/my-orb.git", "my-orb"},
		{"bare name", "my-orb", "my-orb"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Check(t, is.Equal(ProjectNameFromRemote(tc.in), tc.want))
		})
	}
}

func TestRemovePrivateLicense(t *testing.T) {
	t.Run("removes an existing LICENSE", func(t *testing.T) {
		dir := fs.NewDir(t, "orbinit", fs.WithFile("LICENSE", "MIT"))

		assert.NilError(t, RemovePrivateLicense(dir.Path()))
		_, err := os.Stat(dir.Join("LICENSE"))
		assert.Check(t, os.IsNotExist(err))
	})

	t.Run("a missing LICENSE is not an error", func(t *testing.T) {
		assert.NilError(t, RemovePrivateLicense(fs.NewDir(t, "orbinit").Path()))
	})
}

func TestApplyTemplate(t *testing.T) {
	dir := fs.NewDir(t, "orbinit",
		fs.WithDir(".circleci",
			fs.WithFile("config.yml", "orb: <orb-name>"),
			fs.WithFile("test-deploy.yml", "ns: <namespace>"),
		),
		fs.WithDir("src",
			fs.WithFile("@orb.yml", "org: <organization>"),
		),
		fs.WithFile("README.md", "project: <project-name>"),
	)

	assert.NilError(t, ApplyTemplate(dir.Path(), "proj", "acme", "my-orb", "acme-ns"))

	// Every placeholder token is replaced across the whole tree.
	assert.Assert(t, fs.Equal(dir.Path(), fs.Expected(t,
		fs.MatchAnyFileMode,
		fs.WithDir(".circleci", fs.MatchAnyFileMode,
			fs.WithFile("config.yml", "orb: my-orb", fs.MatchAnyFileMode),
			fs.WithFile("test-deploy.yml", "ns: acme-ns", fs.MatchAnyFileMode)),
		fs.WithDir("src", fs.MatchAnyFileMode,
			fs.WithFile("@orb.yml", "org: acme", fs.MatchAnyFileMode)),
		fs.WithFile("README.md", "project: proj", fs.MatchAnyFileMode),
	)))
}

// zipFromEntries builds an in-memory zip archive from name→content entries.
func zipFromEntries(t *testing.T, entries map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for name, content := range entries {
		w, err := zw.Create(name)
		assert.NilError(t, err)
		_, err = w.Write([]byte(content))
		assert.NilError(t, err)
	}
	assert.NilError(t, zw.Close())
	return buf.Bytes()
}

// templateZip mimics a GitHub zipball: every entry is nested under a top-level
// wrapper directory that extraction must strip.
func templateZip(t *testing.T) []byte {
	t.Helper()
	return zipFromEntries(t, map[string]string{
		"Orb-Template-deadbeef/README.md":            "readme",
		"Orb-Template-deadbeef/LICENSE":              "MIT",
		"Orb-Template-deadbeef/src/@orb.yml":         "version: 2.1",
		"Orb-Template-deadbeef/.circleci/config.yml": "cfg",
	})
}

func TestFetchTemplate(t *testing.T) {
	zipBytes := templateZip(t)

	mux := http.NewServeMux()
	mux.HandleFunc("/zip", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(zipBytes)
	})
	var srv *httptest.Server
	mux.HandleFunc("/tags", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode([]templateRelease{
			{Name: "not-a-release", ZipURL: srv.URL + "/zip"},
			{Name: "v1.2.3", ZipURL: srv.URL + "/zip"},
		})
	})
	srv = httptest.NewServer(mux)
	defer srv.Close()

	t.Setenv(templateTagsURLEnv, srv.URL+"/tags")

	dest := t.TempDir()
	assert.NilError(t, FetchTemplate(iostream.Testing(context.Background()), dest))

	// The wrapper directory is stripped: the archive contents land directly in
	// dest with nothing extra and no leftover wrapper dir.
	assert.Assert(t, fs.Equal(dest, fs.Expected(t,
		fs.MatchAnyFileMode,
		fs.WithFile("README.md", "readme", fs.MatchAnyFileMode),
		fs.WithFile("LICENSE", "MIT", fs.MatchAnyFileMode),
		fs.WithDir("src", fs.MatchAnyFileMode,
			fs.WithFile("@orb.yml", "version: 2.1", fs.MatchAnyFileMode)),
		fs.WithDir(".circleci", fs.MatchAnyFileMode,
			fs.WithFile("config.yml", "cfg", fs.MatchAnyFileMode)),
	)))
}

func TestFetchTemplate_NoReleaseTags(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode([]templateRelease{{Name: "nightly", ZipURL: "x"}})
	}))
	defer srv.Close()

	t.Setenv(templateTagsURLEnv, srv.URL)

	err := FetchTemplate(iostream.Testing(context.Background()), t.TempDir())
	assert.ErrorContains(t, err, "no release tags")
}

func TestExtractZip_StripsWrapperAndRejectsZipSlip(t *testing.T) {
	t.Run("strips wrapper dir", func(t *testing.T) {
		raw := zipFromEntries(t, map[string]string{
			"wrapper/a.txt":     "a",
			"wrapper/dir/b.txt": "b",
		})
		zr, err := zip.NewReader(bytes.NewReader(raw), int64(len(raw)))
		assert.NilError(t, err)

		dest := t.TempDir()
		assert.NilError(t, extractZip(zr, dest))

		assert.Assert(t, fs.Equal(dest, fs.Expected(t,
			fs.MatchAnyFileMode,
			fs.WithFile("a.txt", "a", fs.MatchAnyFileMode),
			fs.WithDir("dir", fs.MatchAnyFileMode,
				fs.WithFile("b.txt", "b", fs.MatchAnyFileMode)),
		)))
	})

	t.Run("rejects zip-slip", func(t *testing.T) {
		// After the wrapper component is stripped this still escapes dest.
		raw := zipFromEntries(t, map[string]string{
			"wrapper/../../evil.txt": "pwned",
		})
		zr, err := zip.NewReader(bytes.NewReader(raw), int64(len(raw)))
		assert.NilError(t, err)

		err = extractZip(zr, t.TempDir())
		assert.ErrorContains(t, err, "illegal file path")
	})
}

func TestInitRepoAndCheckoutAlpha(t *testing.T) {
	dir := fs.NewDir(t, "orbinit", fs.WithFile("README.md", "hi"))

	repo, w, err := InitRepo(dir.Path(), "https://github.com/acme/my-orb.git", "main")
	assert.NilError(t, err)

	t.Run("creates a git repository", func(t *testing.T) {
		_, err := os.Stat(dir.Join(".git"))
		assert.NilError(t, err)
	})

	t.Run("adds the origin remote", func(t *testing.T) {
		remote, err := repo.Remote("origin")
		assert.NilError(t, err)
		assert.Check(t, is.DeepEqual(remote.Config().URLs, []string{"https://github.com/acme/my-orb.git"}))
	})

	t.Run("creates an initial commit", func(t *testing.T) {
		head, err := repo.Head()
		assert.NilError(t, err)
		_, err = repo.CommitObject(head.Hash())
		assert.NilError(t, err)
	})

	t.Run("CheckoutAlpha switches to the alpha branch", func(t *testing.T) {
		assert.NilError(t, CheckoutAlpha(w))
		head, err := repo.Head()
		assert.NilError(t, err)
		assert.Check(t, is.Equal(head.Name().Short(), "alpha"))
	})
}

func TestInitRepo_ExistingRepoRejected(t *testing.T) {
	dir := fs.NewDir(t, "orbinit", fs.WithDir(".git"))

	_, _, err := InitRepo(dir.Path(), "https://github.com/acme/my-orb.git", "main")
	assert.ErrorContains(t, err, "already a git repository")
}
