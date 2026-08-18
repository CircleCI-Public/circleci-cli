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

	"github.com/CircleCI-Public/circleci-cli/internal/httpcl"
)

// ProviderRepository is a repository an organization's provider connection can
// reach, as returned by GET /api/v3/provider/repositories.
//
// What "can reach" means depends on the provider. Where access is granted per
// user, two members of one organization can see different lists; where it is
// granted per installation, every member who can view the org sees all of it.
type ProviderRepository struct {
	// ID is the provider's own repository id — the external ID used as the repo
	// reference in pipeline definitions and triggers. It is a string because it is
	// numeric for some providers and text for others.
	ID string
	// FullName is the "owner/repo" name.
	FullName string
	// Name is the repository name without its owner.
	Name string
	// Owner is the account the repository belongs to.
	Owner string
	// DefaultBranch is the repository's default branch.
	DefaultBranch string
	// IsPrivate reports whether the repository is private. Providers that do not
	// report visibility are reported private, which is the conservative reading.
	IsPrivate bool
	// Provider is the integration the repository was listed through.
	Provider string
}

// repositoryAttrs is the attributes object of a v3 provider repository entity.
type repositoryAttrs struct {
	RepoID        string `json:"repo_id"`
	Owner         string `json:"owner"`
	RepoName      string `json:"repo_name"`
	RepoFullName  string `json:"repo_full_name"`
	DefaultBranch string `json:"default_branch"`
	IsPrivate     bool   `json:"is_private"`
	Provider      string `json:"provider"`
}

// repositoryEntity is the data entity of GET /api/v3/provider/repositories. The
// entity carries no id of its own: a repository is identified by its provider id,
// which lives in the attributes.
type repositoryEntity struct {
	Attributes repositoryAttrs `json:"attributes"`
}

// ListProviderRepositories returns one page of the repositories an organization's
// connection to provider can reach, along with the cursor for the next page.
// An empty next cursor means the page returned was the last one.
//
// orgID must be the organization UUID. limit caps the page size; the server's
// maximum is 100. A cursor encodes the limit it was minted with, so pass either a
// cursor or a limit, not both.
func (c *Client) ListProviderRepositories(
	ctx context.Context, orgID, provider, cursor string, limit int,
) ([]ProviderRepository, string, error) {
	opts := []func(*httpcl.Request){
		filterParam("org_id", orgID),
		filterParam("provider", provider),
		pageCursor(cursor),
	}
	// The server rejects a limit sent alongside a cursor, so the limit only travels
	// on the first request.
	if cursor == "" {
		opts = append(opts, pageLimit(limit))
	}

	var resp v3List[repositoryEntity]
	opts = append(opts, httpcl.JSONDecoder(&resp))
	if _, err := c.main.Call(ctx, httpcl.NewRequest(http.MethodGet, "/api/v3/provider/repositories", opts...)); err != nil {
		return nil, "", err
	}

	repos := make([]ProviderRepository, 0, len(resp.Data))
	for _, e := range resp.Data {
		repos = append(repos, ProviderRepository{
			ID:            e.Attributes.RepoID,
			FullName:      e.Attributes.RepoFullName,
			Name:          e.Attributes.RepoName,
			Owner:         e.Attributes.Owner,
			DefaultBranch: e.Attributes.DefaultBranch,
			IsPrivate:     e.Attributes.IsPrivate,
			Provider:      e.Attributes.Provider,
		})
	}

	var next string
	if resp.Page.Next != nil {
		next = *resp.Page.Next
	}
	return repos, next, nil
}
