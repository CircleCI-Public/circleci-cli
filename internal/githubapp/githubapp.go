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

// Package githubapp drives the CircleCI GitHub App onboarding steps: checking
// whether the app is installed for an organization, walking the user through a
// browser-based install, and resolving a repository's external ID. It talks to
// the public v2 GitHub App endpoints exposed by public-api-service.
package githubapp

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/CircleCI-Public/circleci-cli/internal/apiclient"
	"github.com/CircleCI-Public/circleci-cli/internal/browser"
	"github.com/CircleCI-Public/circleci-cli/internal/iostream"
)

const (
	// pollInterval is how often the install poll checks the installation status.
	pollInterval = 3 * time.Second
	// pollTimeout bounds how long we wait for a browser-based install.
	pollTimeout = 3 * time.Minute
	// repoPageLimit is the page size used when listing repositories.
	repoPageLimit = 100
	// maxRepoPages bounds the repository pagination to avoid an unbounded loop.
	maxRepoPages = 50
)

// EnsureInstalled reports whether the CircleCI GitHub App is installed for the
// organization, walking the user through a browser-based install when it is
// not. orgID is the organization UUID; returnURL is where GitHub redirects
// after install (an app.circleci.com URL).
//
// It returns true once the app is installed. In non-interactive sessions (or
// with noBrowser), it prints the install URL and returns false without waiting.
// A returned error is a genuine API failure; callers typically degrade to
// manual guidance on either a false result or an error.
func EnsureInstalled(ctx context.Context, client *apiclient.Client, orgID, returnURL string, noBrowser bool) (bool, error) {
	if _, err := client.GetGitHubAppInstallation(ctx, orgID); err == nil {
		return true, nil
	} else if !errors.Is(err, apiclient.ErrGitHubAppNotInstalled) {
		return false, err
	}

	redirectURL, err := client.InitiateGitHubAppInstall(ctx, orgID, returnURL)
	if err != nil {
		return false, err
	}

	if noBrowser || !iostream.IsInteractive(ctx) {
		iostream.Printf(ctx, "\nInstall the CircleCI GitHub App to connect your repository:\n%s\n", redirectURL)
		return false, nil
	}

	iostream.Printf(ctx, "\nOpening your browser to install the CircleCI GitHub App...\n")
	if err := browser.OpenURLOrPrint(iostream.Err(ctx), redirectURL); err != nil {
		iostream.Printf(ctx, "Open this URL to install the app:\n%s\n", redirectURL)
	}

	sp := iostream.Spinner(ctx, true, "Waiting for GitHub App installation")
	installed, err := pollInstalled(ctx, client, orgID)
	sp.Stop()
	if err != nil {
		return false, err
	}
	if !installed {
		iostream.ErrPrintf(ctx, "%s Timed out waiting for the GitHub App installation.\n", iostream.SymbolWarn(ctx))
		return false, nil
	}
	iostream.Printf(ctx, "%s GitHub App installed\n", iostream.SymbolOK(ctx))
	return true, nil
}

// pollInstalled polls the installation endpoint until the app is installed, the
// timeout elapses, or the context is cancelled.
func pollInstalled(ctx context.Context, client *apiclient.Client, orgID string) (bool, error) {
	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()
	timer := time.NewTimer(pollTimeout)
	defer timer.Stop()

	for {
		select {
		case <-ctx.Done():
			return false, ctx.Err()
		case <-timer.C:
			return false, nil
		case <-ticker.C:
			_, err := client.GetGitHubAppInstallation(ctx, orgID)
			if err == nil {
				return true, nil
			}
			if errors.Is(err, apiclient.ErrGitHubAppNotInstalled) {
				continue
			}
			return false, err
		}
	}
}

// ResolveRepoID returns the external (numeric) ID of the repository named
// repoFullName ("owner/repo") among the repositories the GitHub App can access
// for the organization. It returns "" (with no error) when the repository is
// not found — e.g. it exists but was not granted to the app.
func ResolveRepoID(ctx context.Context, client *apiclient.Client, orgID, repoFullName string) (string, error) {
	if repoFullName == "" {
		return "", nil
	}

	seen := 0
	for page := 1; page <= maxRepoPages; page++ {
		repos, total, err := client.ListGitHubAppRepositories(ctx, orgID, page, repoPageLimit)
		if err != nil {
			return "", err
		}
		for _, r := range repos {
			if strings.EqualFold(r.FullName, repoFullName) {
				return strconv.FormatInt(r.ID, 10), nil
			}
		}
		seen += len(repos)
		if len(repos) == 0 || (total > 0 && seen >= total) {
			break
		}
	}
	return "", nil
}
