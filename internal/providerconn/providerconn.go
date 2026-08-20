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

// Package providerconn drives the onboarding steps that depend on an
// integration: checking whether an organization is connected to one, walking the
// user through a browser-based install, and resolving a repository's id.
//
// Every function takes the integration as a provider.Provider, so supporting
// another one is a registry entry rather than a change here.
package providerconn

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/CircleCI-Public/circleci-cli/clikit/browser"
	"github.com/CircleCI-Public/circleci-cli/clikit/iostream"
	"github.com/CircleCI-Public/circleci-cli/internal/apiclient"
	"github.com/CircleCI-Public/circleci-cli/internal/provider"
)

const (
	pollInterval  = 3 * time.Second
	pollTimeout   = 3 * time.Minute
	repoPageLimit = 100
	// maxRepoPages keeps the walk in ResolveRepoID bounded.
	maxRepoPages = 50
)

// EnsureConnected reports whether the organization is connected to p, walking the
// user through a browser-based install when it is not. returnURL is where the
// provider returns the browser once the install completes.
//
// Non-interactive sessions (or noBrowser) print the install URL and return false
// without waiting. Callers degrade to manual guidance on false or on an error.
func EnsureConnected(
	ctx context.Context,
	client *apiclient.Client,
	p provider.Provider,
	orgID, returnURL string,
	noBrowser bool,
) (bool, error) {
	installed, err := connected(ctx, client, p, orgID)
	if err != nil {
		return false, err
	}
	if installed {
		return true, nil
	}

	redirectURL, err := installURL(ctx, client, p, orgID, returnURL)
	if err != nil {
		return false, err
	}

	if noBrowser || !iostream.IsInteractive(ctx) {
		iostream.Printf(ctx, "\nInstall %s to connect your repository:\n%s\n", p.Install, redirectURL)
		return false, nil
	}

	iostream.Printf(ctx, "\nOpening your browser to install %s...\n", p.Install)
	if err := browser.OpenURLOrPrint(iostream.Err(ctx), redirectURL); err != nil {
		iostream.Printf(ctx, "Open this URL to install the app:\n%s\n", redirectURL)
	}

	sp := iostream.Spinner(ctx, true, "Waiting for "+p.Short+" installation")
	installed, err = pollInstalled(ctx, client, p, orgID)
	sp.Stop()
	if err != nil {
		return false, err
	}
	if !installed {
		iostream.ErrPrintf(ctx, "%s Timed out waiting for the %s installation.\n", iostream.SymbolWarn(ctx), p.Short)
		return false, nil
	}
	iostream.Printf(ctx, "%s %s installed\n", iostream.SymbolOK(ctx), p.Short)
	return true, nil
}

// installURL starts a connection to p and returns the URL the user has to open to
// finish it.
//
// The setup call answers with what has to happen next. A redirect is the case
// this flow handles: CircleCI's app is registered with the provider already, so
// the user only approves it. Anything else — today, registering an app from a
// manifest — is a browser flow the CLI cannot complete, so it is reported rather
// than silently opening nothing.
func installURL(
	ctx context.Context, client *apiclient.Client, p provider.Provider, orgID, returnURL string,
) (string, error) {
	setup, err := client.SetupProviderConnection(ctx, orgID, p.Name, returnURL)
	if err != nil {
		return "", err
	}
	if setup.NextStep != apiclient.ConnectionNextStepRedirect || setup.URL == "" {
		return "", fmt.Errorf("cannot start the install from here: the connection needs %q", setup.NextStep)
	}
	return setup.URL, nil
}

// connected reports whether the organization has a connection to p. An
// organization with none is answered as an empty list rather than an error, so a
// non-nil error means the check itself failed — a caller must not read it as "not
// installed" and send the user into an install flow.
func connected(ctx context.Context, client *apiclient.Client, p provider.Provider, orgID string) (bool, error) {
	conns, err := client.ListProviderConnections(ctx, orgID)
	if err != nil {
		return false, err
	}
	for _, conn := range conns {
		// A connection whose provider could not be reached still counts as
		// installed: ConnectionError describes a degraded read, not a missing
		// installation.
		if conn.Provider == p.Name {
			return true, nil
		}
	}
	return false, nil
}

// pollInstalled polls for the connection until the app is installed, the timeout
// elapses, or the context is cancelled.
func pollInstalled(ctx context.Context, client *apiclient.Client, p provider.Provider, orgID string) (bool, error) {
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
			installed, err := connected(ctx, client, p, orgID)
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
// as "the integration cannot access that repository" — the advice that follows from
// that, grant access and re-run, searches the same bounded prefix again.
var ErrTooManyRepositories = errors.New("too many repositories to search for a match")

// ResolveRepoID returns p's ID for repoFullName ("owner/repo") among the
// repositories the organization's connection to p can reach. The endpoint offers no
// name filter, so the list has to be walked.
//
// It returns "" with no error only when the whole list was examined and held no
// match; hitting the page cap first returns ErrTooManyRepositories.
func ResolveRepoID(
	ctx context.Context, client *apiclient.Client, p provider.Provider, orgID, repoFullName string,
) (string, error) {
	if repoFullName == "" {
		return "", nil
	}

	cursor := ""
	for range maxRepoPages {
		repos, next, err := client.ListProviderRepositories(ctx, orgID, p.Name, cursor, repoPageLimit)
		if err != nil {
			return "", err
		}
		for _, r := range repos {
			if strings.EqualFold(r.FullName, repoFullName) {
				return r.ID, nil
			}
		}
		// An absent next cursor is the end of the list, so the walk was complete and
		// the repository genuinely is not there.
		if next == "" {
			return "", nil
		}
		cursor = next
	}
	return "", ErrTooManyRepositories
}
