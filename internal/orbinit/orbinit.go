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

// Package orbinit scaffolds a new orb project for "circleci orb init". It
// downloads the CircleCI Orb-Template repository, extracts it, rewrites the
// template placeholders, and initializes a local git repository. It performs
// no CircleCI API calls; the orb command layer owns those.
package orbinit

import (
	"archive/zip"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/go-git/go-git/v6"
	"github.com/go-git/go-git/v6/config"
	"github.com/go-git/go-git/v6/plumbing"
	"github.com/go-git/go-git/v6/plumbing/object"

	"github.com/CircleCI-Public/circleci-cli/clikit/closer"
	"github.com/CircleCI-Public/circleci-cli/internal/httpcl"
)

// defaultTemplateTagsURL is the GitHub API endpoint listing tags of the orb
// template repository.
const defaultTemplateTagsURL = "https://api.github.com/repos/CircleCI-Public/Orb-Template/tags"

// templateTagsURLEnv, when set, overrides the template tags endpoint. It lets
// acceptance tests point the download at a local fake server.
const templateTagsURLEnv = "CIRCLE_ORB_TEMPLATE_URL"

// tagsURL returns the endpoint to list orb template releases, honoring the
// override env var when present.
func tagsURL() string {
	if v := os.Getenv(templateTagsURLEnv); v != "" {
		return v
	}
	return defaultTemplateTagsURL
}

// releaseTagRegex matches valid semver release tags (e.g. "v1.2.3").
var releaseTagRegex = regexp.MustCompile(`^v\d+\.\d+\.\d+$`)

// templateFiles are the files whose placeholders are rewritten during setup.
var templateFiles = []string{
	filepath.Join(".circleci", "config.yml"),
	filepath.Join(".circleci", "test-deploy.yml"),
	"README.md",
	filepath.Join("src", "@orb.yml"),
}

// templateRelease is a single tag entry from the GitHub tags API.
type templateRelease struct {
	ZipURL string `json:"zipball_url"`
	Name   string `json:"name"`
}

// FetchTemplate downloads the latest release of the orb template repository and
// extracts it into orbPath. The GitHub zipball nests everything under a
// top-level directory; that wrapper directory is stripped during extraction so
// the template lands directly in orbPath.
func FetchTemplate(ctx context.Context, orbPath string) (err error) {
	client := httpcl.New(httpcl.Config{UserAgent: "circleci-cli orb-init"})

	zipURL, err := latestTemplateZipURL(ctx, client)
	if err != nil {
		return err
	}

	// zip requires random access (io.ReaderAt), so the archive can't be
	// extracted straight off the response stream. Rather than buffer the whole
	// (potentially large) archive in memory, stream it to a temp file and let
	// zip.OpenReader seek within it on disk.
	tmp, err := os.CreateTemp("", "orb-template-*.zip")
	if err != nil {
		return err
	}
	defer func() { _ = os.Remove(tmp.Name()) }()

	_, callErr := client.Call(ctx, httpcl.NewRequest(http.MethodGet, zipURL, httpcl.CopyDecoder(tmp)))
	if closeErr := tmp.Close(); callErr == nil {
		callErr = closeErr
	}
	if callErr != nil {
		return fmt.Errorf("downloading orb template: %w", callErr)
	}

	zrc, err := zip.OpenReader(tmp.Name())
	if err != nil {
		return fmt.Errorf("reading orb template archive: %w", err)
	}
	defer closer.ErrorHandler(zrc, &err)

	return extractZip(&zrc.Reader, orbPath)
}

// latestTemplateZipURL queries the tags API and returns the zipball URL of the
// newest release tag (tags are returned newest-first by the GitHub API).
func latestTemplateZipURL(ctx context.Context, client *httpcl.Client) (string, error) {
	var tags []templateRelease
	if _, err := client.Call(ctx, httpcl.NewRequest(http.MethodGet, tagsURL(), httpcl.JSONDecoder(&tags))); err != nil {
		return "", fmt.Errorf("listing orb template releases: %w", err)
	}

	for _, tag := range tags {
		if releaseTagRegex.MatchString(tag.Name) {
			return tag.ZipURL, nil
		}
	}
	return "", fmt.Errorf("no release tags found for the orb template")
}

// extractZip unzips src into dest, stripping the leading path component of each
// entry (the GitHub-generated wrapper directory).
func extractZip(zr *zip.Reader, dest string) error {
	if err := os.MkdirAll(dest, 0o750); err != nil {
		return err
	}

	for _, f := range zr.File {
		if err := extractZipEntry(f, dest); err != nil {
			return err
		}
	}
	return nil
}

func extractZipEntry(f *zip.File, dest string) (err error) {
	// Strip the leading wrapper directory GitHub adds to zipballs.
	pathParts := strings.Split(f.Name, "/")
	if len(pathParts) <= 1 {
		return nil
	}
	target := filepath.Join(append([]string{dest}, pathParts[1:]...)...)

	// Guard against zip-slip: the resolved path must stay within dest.
	if rel, rerr := filepath.Rel(dest, target); rerr != nil || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) || rel == ".." {
		return fmt.Errorf("illegal file path in template archive: %s", f.Name)
	}

	if f.FileInfo().IsDir() {
		return os.MkdirAll(target, 0o750)
	}

	if err := os.MkdirAll(filepath.Dir(target), 0o750); err != nil {
		return err
	}

	rc, err := f.Open()
	if err != nil {
		return err
	}
	defer closer.ErrorHandler(rc, &err)

	out, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode()) //nolint:gosec // target is validated against zip-slip above
	if err != nil {
		return err
	}
	defer closer.ErrorHandler(out, &err)

	_, err = io.Copy(out, rc) //nolint:gosec // template archive is from a trusted source and paths are validated above
	return err
}

// RemovePrivateLicense deletes the MIT LICENSE file from a private orb project.
// A missing file is not an error.
func RemovePrivateLicense(orbPath string) error {
	err := os.Remove(filepath.Join(orbPath, "LICENSE"))
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// ApplyTemplate rewrites the placeholder tokens in every template file under
// orbPath with the given project details.
func ApplyTemplate(orbPath, projectName, org, orbName, namespace string) error {
	for _, rel := range templateFiles {
		p := filepath.Join(orbPath, rel)
		contents, err := os.ReadFile(p) //nolint:gosec // p is a fixed template file under orbPath
		if err != nil {
			return err
		}
		rewritten := Substitute(string(contents), projectName, org, orbName, namespace)
		if err := os.WriteFile(p, []byte(rewritten), 0o644); err != nil { //nolint:gosec // template files are not secrets
			return err
		}
	}
	return nil
}

var metaLineRegex = regexp.MustCompile(`\*\*Meta\*\*.*`)

// Substitute replaces the orb template placeholder tokens in a file's contents.
func Substitute(contents, projectName, org, orbName, namespace string) string {
	replacer := strings.NewReplacer(
		"<orb-name>", orbName,
		"<namespace>", namespace,
		"<publishing-context>", "orb-publishing",
		"<project-name>", projectName,
		"<organization>", org,
		"<!---", "",
		"--->", "",
	)
	return metaLineRegex.ReplaceAllString(replacer.Replace(contents), "")
}

// IsRepo reports whether orbPath already holds a git repository.
func IsRepo(orbPath string) bool {
	_, err := os.Stat(filepath.Join(orbPath, ".git"))
	return err == nil
}

// ExistingRepo describes a git repository that is already present at the orb
// path. Both fields are best-effort: a freshly cloned empty repository has an
// origin but no commits, so no branch can be resolved from HEAD.
type ExistingRepo struct {
	// RemoteURL is origin's URL, empty if there is no origin remote.
	RemoteURL string
	// Branch is the checked-out branch, empty if HEAD cannot be resolved.
	Branch string
}

// InspectRepo reads what is already configured in the repository at orbPath, so
// orb init can adopt it instead of asking for details that are on disk.
//
// Cloning an empty repository and running `orb init .` inside it is the obvious
// way to use this command when only admins may create repositories — and it is
// the case that used to dead-end.
func InspectRepo(orbPath string) (ExistingRepo, error) {
	repo, err := git.PlainOpen(orbPath)
	if err != nil {
		return ExistingRepo{}, fmt.Errorf("opening git repository at %s: %w", orbPath, err)
	}

	var found ExistingRepo
	if remote, err := repo.Remote("origin"); err == nil {
		if urls := remote.Config().URLs; len(urls) > 0 {
			found.RemoteURL = urls[0]
		}
	}
	// A repository with no commits has an unresolvable HEAD. That is normal for a
	// fresh clone of an empty repository, so it is not an error here.
	if head, err := repo.Head(); err == nil {
		found.Branch = head.Name().Short()
	}
	return found, nil
}

// InitRepo prepares the git repository at orbPath for the orb's initial commit:
// it stages every file and commits. It returns the repository and its worktree.
//
// An existing repository is adopted rather than refused. remoteURL and branch are
// only applied to what is missing — a repository that already has an origin keeps
// it, and one that already has commits keeps its current branch. Cloning an empty
// repository and initialising the orb into it is a reasonable way to work, and is
// the only way when repository creation is restricted to admins.
func InitRepo(orbPath, remoteURL, branch string) (*git.Repository, *git.Worktree, error) {
	repo, err := openOrInitRepo(orbPath)
	if err != nil {
		return nil, nil, err
	}

	// A repository with no commits yet — freshly initialised here, or a clone of
	// an empty repository — has an unborn HEAD that go-git points at "master".
	// Point it at the branch the orb is meant to track so the initial commit
	// lands there; without this a brand-new orb starts on "master" however the
	// author set --branch (default "main"). A repository that already has commits
	// keeps whatever branch it is on.
	if _, err := repo.Head(); err != nil {
		ref := plumbing.NewSymbolicReference(plumbing.HEAD, plumbing.NewBranchReferenceName(branch))
		if err := repo.Storer.SetReference(ref); err != nil {
			return nil, nil, fmt.Errorf("setting initial branch to %q: %w", branch, err)
		}
	}

	// Only add origin if the repository does not already have one. A clone's
	// origin is authoritative: it is where the user actually intends to push.
	if _, err := repo.Remote("origin"); err != nil {
		if _, err := repo.CreateRemote(&config.RemoteConfig{
			Name: "origin",
			URLs: []string{remoteURL},
		}); err != nil {
			return nil, nil, err
		}
	}

	// CreateBranch records tracking config, and errors if the branch is already
	// configured — which it is when the repository came from a clone. Nothing to
	// do in that case.
	if err := repo.CreateBranch(&config.Branch{Name: branch, Remote: "origin"}); err != nil &&
		!errors.Is(err, git.ErrBranchExists) {
		return nil, nil, fmt.Errorf("creating branch %q: %w", branch, err)
	}

	w, err := repo.Worktree()
	if err != nil {
		return nil, nil, err
	}
	if _, err := w.Add("."); err != nil {
		return nil, nil, err
	}
	if err := commitInitial(w); err != nil {
		return nil, nil, err
	}
	return repo, w, nil
}

// openOrInitRepo opens the repository at orbPath, creating one if there is none.
func openOrInitRepo(orbPath string) (*git.Repository, error) {
	if IsRepo(orbPath) {
		repo, err := git.PlainOpen(orbPath)
		if err != nil {
			return nil, fmt.Errorf("opening git repository at %s: %w", orbPath, err)
		}
		return repo, nil
	}
	return git.PlainInit(orbPath, false)
}

// commitInitial creates the initial commit. It first lets go-git read the
// author from the local/global git config; if no identity is configured it
// falls back to a generic CircleCI identity so init still succeeds.
func commitInitial(w *git.Worktree) error {
	msg := "feat: Initial commit."
	if _, err := w.Commit(msg, &git.CommitOptions{}); err != nil {
		_, ferr := w.Commit(msg, &git.CommitOptions{
			Author: &object.Signature{
				Name:  "CircleCI",
				Email: "community-partner@circleci.com",
				When:  time.Now(),
			},
		})
		return ferr
	}
	return nil
}

// CheckoutAlpha creates and switches to the "alpha" branch.
func CheckoutAlpha(w *git.Worktree) error {
	return w.Checkout(&git.CheckoutOptions{
		Branch: plumbing.NewBranchReferenceName("alpha"),
		Create: true,
	})
}

// ProjectNameFromRemote derives a project name from a git remote URL by taking
// the last path segment and trimming any ".git" suffix.
func ProjectNameFromRemote(remoteURL string) string {
	trimmed := strings.TrimSuffix(remoteURL, ".git")
	parts := strings.Split(trimmed, "/")
	return parts[len(parts)-1]
}
