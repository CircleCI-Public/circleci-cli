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

// Package gitremote resolves the CircleCI project slug for the current
// working directory. Resolution prefers the per-checkout .circleci/info.yml
// recorded by `circleci project link` (so repository renames and standalone
// projects stay addressable), falling back to parsing the git remote URL.
package gitremote

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"

	"github.com/CircleCI-Public/circleci-cli/internal/projectref"
)

// ProjectInfo holds the information needed to identify a CircleCI project.
type ProjectInfo struct {
	// Slug is the CircleCI project slug, e.g. "gh/myorg/myrepo".
	Slug string
	// Branch is the current git branch name.
	Branch string
}

var (
	// matches git@github.com:org/repo.git (SCP-style)
	sshRemote = regexp.MustCompile(`^git@([^:]+):([^/]+)/(.+?)(?:\.git)?$`)
	// matches ssh://git@github.com/org/repo.git (protocol-style)
	sshProtoRemote = regexp.MustCompile(`^ssh://git@([^/]+)/([^/]+)/(.+?)(?:\.git)?$`)
	// matches https://github.com/org/repo.git
	httpsRemote = regexp.MustCompile(`^https?://([^/]+)/([^/]+)/(.+?)(?:\.git)?$`)
)

// DetectNamespace returns the organization name (namespace) from the git remote.
// For a slug like "gh/myorg/myrepo" it returns "myorg".
func DetectNamespace() (string, error) {
	info, err := Detect()
	if err != nil {
		return "", err
	}
	parts := strings.Split(info.Slug, "/")
	if len(parts) != 3 {
		return "", fmt.Errorf("unexpected slug format: %q", info.Slug)
	}
	return parts[1], nil
}

// InsideWorkTree reports whether the current working directory is inside a
// git working tree. Use this when a command only needs to confirm it is in a
// repository, without requiring a parseable CircleCI remote.
func InsideWorkTree() bool {
	out, err := gitOutput("rev-parse", "--is-inside-work-tree")
	return err == nil && out == "true"
}

// OriginURL returns the raw git remote URL for the origin remote.
func OriginURL() (string, error) {
	remoteURL, err := gitOutput("remote", "get-url", "origin")
	if err != nil {
		return "", fmt.Errorf("could not read git remote: %w", err)
	}
	return remoteURL, nil
}

// Detect resolves the CircleCI project for the current working directory.
//
// Resolution priority:
//  1. .circleci/info.yml in the working directory (written by `circleci project link`).
//     When this file carries both project_id and organization_id, the canonical
//     "circleci/<orgID>/<projectID>" slug is returned so lookups survive VCS-side
//     renames; otherwise the file's stored slug is returned verbatim.
//  2. The git remote "origin" URL.
//
// The branch is always read from git (best-effort when info.yml supplied the slug,
// since the branch is per-checkout and never persisted in info.yml).
func Detect() (*ProjectInfo, error) {
	if cwd, err := os.Getwd(); err == nil {
		if ref, err := projectref.Read(cwd); err == nil {
			branch, _ := gitOutput("rev-parse", "--abbrev-ref", "HEAD")
			return &ProjectInfo{Slug: ref.EffectiveSlug(), Branch: branch}, nil
		}
	}
	return DetectFromRemote()
}

// DetectFromRemote resolves the project from the git "origin" remote without
// consulting .circleci/info.yml. Use this from the `project link` command itself
// — reading info.yml there would short-circuit the very write that link is
// about to perform.
func DetectFromRemote() (*ProjectInfo, error) {
	remoteURL, err := OriginURL()
	if err != nil {
		return nil, err
	}

	slug, err := slugFromRemote(remoteURL)
	if err != nil {
		return nil, err
	}

	branch, err := gitOutput("rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return nil, fmt.Errorf("could not determine current branch: %w", err)
	}

	return &ProjectInfo{Slug: slug, Branch: branch}, nil
}

// SlugFromRemote is exported for testing.
func SlugFromRemote(remoteURL string) (string, error) {
	return slugFromRemote(remoteURL)
}

func slugFromRemote(remoteURL string) (string, error) {
	remoteURL = strings.TrimSpace(remoteURL)

	if m := sshRemote.FindStringSubmatch(remoteURL); m != nil {
		host, org, repo := m[1], m[2], m[3]
		return buildSlug(host, org, repo)
	}

	if m := sshProtoRemote.FindStringSubmatch(remoteURL); m != nil {
		host, org, repo := m[1], m[2], m[3]
		return buildSlug(host, org, repo)
	}

	if m := httpsRemote.FindStringSubmatch(remoteURL); m != nil {
		host, org, repo := m[1], m[2], m[3]
		return buildSlug(host, org, repo)
	}

	return "", fmt.Errorf("unrecognised git remote URL format: %q", remoteURL)
}

func buildSlug(host, org, repo string) (string, error) {
	var vcs string
	switch {
	case strings.Contains(host, "github"):
		vcs = "gh"
	case strings.Contains(host, "bitbucket"):
		vcs = "bb"
	case strings.Contains(host, "gitlab"):
		vcs = "gl"
	default:
		return "", fmt.Errorf("unsupported VCS host %q (expected github.com, bitbucket.org, or gitlab.com)", host)
	}
	return fmt.Sprintf("%s/%s/%s", vcs, org, repo), nil
}

func gitOutput(args ...string) (string, error) {
	out, err := exec.Command("git", args...).Output() //#nosec:G204 // args are controlled caller-supplied git subcommand names, not user input
	if err != nil {
		if exitErr, ok := errors.AsType[*exec.ExitError](err); ok {
			return "", fmt.Errorf("%w: %s", err, strings.TrimSpace(string(exitErr.Stderr)))
		}
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}
