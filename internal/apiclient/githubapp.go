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
	"net/http"
	"strconv"

	"github.com/CircleCI-Public/circleci-cli/internal/httpcl"
)

// Both halves of the install handshake — whether the app is installed for an
// organization, and starting an install — go through the provider-agnostic
// connections endpoints: see ListProviderConnections and
// SetupProviderConnection.

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
	_, err := c.main.Call(ctx, httpcl.NewRequest(http.MethodGet, "/api/v2/github-app/organization/%s/repositories",
		httpcl.RouteParams(orgID),
		httpcl.QueryParam("page", strconv.Itoa(page)),
		httpcl.QueryParam("limit", strconv.Itoa(limit)),
		httpcl.JSONDecoder(&resp),
	))
	if err != nil {
		return nil, 0, err
	}
	return resp.Items, resp.TotalCount, nil
}
