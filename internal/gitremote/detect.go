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

// Package gitremote detects the CircleCI project slug from a git repository's
// remote URL and the current branch.
package gitremote

import (
	"fmt"
	"os/exec"
	"regexp"
	"strings"
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

// Detect reads the git remote "origin" and current branch from the working
// directory and returns a ProjectInfo suitable for CircleCI API calls.
func Detect() (*ProjectInfo, error) {
	remoteURL, err := gitOutput("remote", "get-url", "origin")
	if err != nil {
		return nil, fmt.Errorf("could not read git remote: %w", err)
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
	out, err := exec.Command("git", args...).Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return "", fmt.Errorf("%w: %s", err, strings.TrimSpace(string(exitErr.Stderr)))
		}
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}
