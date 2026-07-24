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

package apiclient

import (
	"context"
	"errors"
	"net/http"
	"strconv"

	"github.com/CircleCI-Public/circleci-cli/internal/httpcl"
)

// ErrGitHubAppNotInstalled is returned by GetGitHubAppInstallation when the
// CircleCI GitHub App is not installed for the organization (the endpoint
// answers 404). Callers use it to branch into the install flow.
var ErrGitHubAppNotInstalled = errors.New("CircleCI GitHub App is not installed for this organization")

// GitHubAppInstallation describes a CircleCI GitHub App installation for an
// organization, as returned by
// GET /api/v2/github-app/organization/{orgID}/installation.
type GitHubAppInstallation struct {
	// ID is the GitHub App installation's external (GitHub) ID.
	ID int64 `json:"id"`
	// TargetType is "Organization" or "User".
	TargetType string `json:"target_type"`
	// Login is the GitHub account the app is installed on.
	Login string `json:"login"`
	// RepositorySelection is "all" or "selected" when present.
	RepositorySelection string `json:"repository_selection,omitempty"`
}

// GetGitHubAppInstallation reports the CircleCI GitHub App installation for the
// organization. orgID must be the organization UUID. It returns
// ErrGitHubAppNotInstalled when the app is not installed (HTTP 404).
func (c *Client) GetGitHubAppInstallation(ctx context.Context, orgID string) (*GitHubAppInstallation, error) {
	var resp GitHubAppInstallation
	err := c.get(ctx, "/github-app/organization/%s/installation", &resp,
		routeParams(orgID),
	)
	if err != nil {
		if httpcl.HasStatusCode(err, http.StatusNotFound) {
			return nil, ErrGitHubAppNotInstalled
		}
		return nil, err
	}
	return &resp, nil
}

// InitiateGitHubAppInstall starts a GitHub App installation for the
// organization and returns the URL the user should open to complete the
// install on GitHub. orgID must be the organization UUID; returnURL is where
// GitHub redirects after the install completes and must be an app.circleci.com
// URL.
func (c *Client) InitiateGitHubAppInstall(ctx context.Context, orgID, returnURL string) (string, error) {
	body := map[string]any{
		"org_id":     orgID,
		"return_url": returnURL,
	}
	var resp struct {
		RedirectURL string `json:"redirect_url"`
	}
	if err := c.post(ctx, "/github-app/install", body, &resp); err != nil {
		return "", err
	}
	return resp.RedirectURL, nil
}

// GitHubAppRepository is a repository the CircleCI GitHub App can access, as
// returned by GET /api/v2/github-app/organization/{orgID}/repositories.
type GitHubAppRepository struct {
	// ID is the GitHub numeric repository ID — the external ID used as the repo
	// reference in pipeline definitions and triggers.
	ID int64 `json:"id"`
	// FullName is the "owner/repo" name.
	FullName      string `json:"repo_full_name"`
	Name          string `json:"repo_name"`
	Owner         string `json:"owner"`
	DefaultBranch string `json:"default_branch"`
	Private       bool   `json:"private"`
}

// ListGitHubAppRepositories returns one page of repositories the GitHub App can
// access for the organization, along with the total repository count. orgID
// must be the organization UUID. page is 1-based; limit is the page size (max
// 100 by the server).
func (c *Client) ListGitHubAppRepositories(ctx context.Context, orgID string, page, limit int) ([]GitHubAppRepository, int, error) {
	var resp struct {
		Items      []GitHubAppRepository `json:"items"`
		TotalCount int                   `json:"total_count"`
	}
	err := c.get(ctx, "/github-app/organization/%s/repositories", &resp,
		routeParams(orgID),
		queryParam("page", strconv.Itoa(page)),
		queryParam("limit", strconv.Itoa(limit)),
	)
	if err != nil {
		return nil, 0, err
	}
	return resp.Items, resp.TotalCount, nil
}
