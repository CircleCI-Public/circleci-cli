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

// ProviderConnection is an organization's connection to a VCS provider, as
// returned by GET /api/v3/provider/connections.
//
// The presence of a connection is what says the provider is connected. The
// enrichment fields come from a live call out to the provider, so they are absent
// when ConnectionError is set — a connection that reports an error is still a
// connection.
type ProviderConnection struct {
	// ID is the connection's CircleCI UUID.
	ID string
	// Provider is the integration, e.g. "github_app".
	Provider string
	// ExternalID is the provider's own id for the installation.
	ExternalID string
	// Login is the provider account the connection is installed on.
	Login string
	// RepositorySelection is "all" or "selected" when present.
	RepositorySelection string
	// AuthorizedUsername is the calling user's account with the provider.
	AuthorizedUsername string
	// ConnectionError describes why the provider could not be reached, if it
	// could not be.
	ConnectionError string
}

// connectionAttrs is the attributes object of a v3 provider connection entity.
type connectionAttrs struct {
	Provider            string `json:"provider"`
	ExternalID          string `json:"external_id"`
	Login               string `json:"login,omitempty"`
	RepositorySelection string `json:"repository_selection,omitempty"`
	AuthorizedUsername  string `json:"authorized_username,omitempty"`
	ConnectionError     string `json:"connection_error,omitempty"`
}

// connectionEntity is the data entity of GET /api/v3/provider/connections.
type connectionEntity struct {
	ID         string          `json:"id"`
	Attributes connectionAttrs `json:"attributes"`
}

// ListProviderConnections returns the organization's provider connections.
// orgID must be the organization UUID. An organization with no connections yields
// an empty slice, so an error means the check itself failed rather than that
// nothing is connected.
func (c *Client) ListProviderConnections(ctx context.Context, orgID string) ([]ProviderConnection, error) {
	var resp v3List[connectionEntity]
	_, err := c.main.Call(ctx, httpcl.NewRequest(http.MethodGet, "/api/v3/provider/connections",
		filterParam("org_id", orgID),
		httpcl.JSONDecoder(&resp),
	))
	if err != nil {
		return nil, err
	}

	conns := make([]ProviderConnection, 0, len(resp.Data))
	for _, e := range resp.Data {
		conns = append(conns, ProviderConnection{
			ID:                  e.ID,
			Provider:            e.Attributes.Provider,
			ExternalID:          e.Attributes.ExternalID,
			Login:               e.Attributes.Login,
			RepositorySelection: e.Attributes.RepositorySelection,
			AuthorizedUsername:  e.Attributes.AuthorizedUsername,
			ConnectionError:     e.Attributes.ConnectionError,
		})
	}
	return conns, nil
}
