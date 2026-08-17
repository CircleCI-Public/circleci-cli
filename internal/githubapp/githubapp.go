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
// browser-based install, and resolving a repository's external ID.
package githubapp

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/CircleCI-Public/circleci-cli/clikit/browser"
	"github.com/CircleCI-Public/circleci-cli/clikit/iostream"
	"github.com/CircleCI-Public/circleci-cli/internal/apiclient"
)

const (
	pollInterval  = 3 * time.Second
	pollTimeout   = 3 * time.Minute
	repoPageLimit = 100
	// maxRepoPages keeps the walk in ResolveRepoID bounded.
	maxRepoPages = 50
)

// provider is the integration this package drives. It is the value the
// provider-agnostic connections endpoint reports for a CircleCI GitHub App
// installation.
const provider = "github_app"

// EnsureInstalled reports whether the CircleCI GitHub App is installed for the
// organization, walking the user through a browser-based install when it is not.
// returnURL is where GitHub redirects after install.
//
// Non-interactive sessions (or noBrowser) print the install URL and return false
// without waiting. Callers degrade to manual guidance on false or on an error.
func EnsureInstalled(ctx context.Context, client *apiclient.Client, orgID, returnURL string, noBrowser bool) (bool, error) {
	installed, err := connected(ctx, client, orgID)
	if err != nil {
		return false, err
	}
	if installed {
		return true, nil
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
	installed, err = pollInstalled(ctx, client, orgID)
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

// connected reports whether the organization has a connection for this package's
// provider. An organization with none is answered as an empty list rather than an
// error, so a non-nil error means the check itself failed — a caller must not read
// it as "not installed" and send the user into an install flow.
func connected(ctx context.Context, client *apiclient.Client, orgID string) (bool, error) {
	conns, err := client.ListProviderConnections(ctx, orgID)
	if err != nil {
		return false, err
	}
	for _, conn := range conns {
		// A connection whose provider could not be reached still counts as
		// installed: ConnectionError describes a degraded read, not a missing
		// installation.
		if conn.Provider == provider {
			return true, nil
		}
	}
	return false, nil
}

// pollInstalled polls for the connection until the app is installed, the timeout
// elapses, or the context is cancelled.
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
			installed, err := connected(ctx, client, orgID)
			if err != nil {
				return false, err
			}
			if installed {
				return true, nil
			}
		}
	}
}

// ErrTooManyRepositories reports that the search hit its page cap before examining
// every repository, so the target may well be accessible. Callers must not report it
// as "the app cannot access that repository" — the advice that follows from that,
// grant access and re-run, searches the same bounded prefix again.
var ErrTooManyRepositories = errors.New("too many repositories to search for a match")

// ResolveRepoID returns the external (numeric) ID of repoFullName ("owner/repo")
// among the repositories the GitHub App can access for the organization. The
// endpoint offers no name filter, so the list has to be walked.
//
// It returns "" with no error only when the whole list was examined and held no
// match; hitting the page cap first returns ErrTooManyRepositories.
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
			return "", nil // whole list examined, genuinely no match
		}
	}
	return "", ErrTooManyRepositories
}
