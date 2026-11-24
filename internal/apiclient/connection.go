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
	"fmt"
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
	// ProviderURL is the provider instance the connection talks to, e.g.
	// "https://forgejo.example.com" for a self-hosted Forgejo.
	ProviderURL string
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

// Next steps a connection setup can call for. Redirect is the case the CLI can
// drive: CircleCI already has an app registered with the provider, so the user
// only has to approve it. Register means no app exists yet and one has to be
// registered from a manifest, which is a browser flow the CLI cannot complete.
// Configure means the connection is stored and the user has to create a webhook
// at the provider.
const (
	ConnectionNextStepRedirect  = "redirect"
	ConnectionNextStepRegister  = "register"
	ConnectionNextStepConfigure = "configure"
)

// ProviderConnectionSetup is where to send a user to finish connecting a
// provider, as returned by POST /api/v3/provider/connections/setup. Nothing is
// connected when it is returned: the connection only exists once the user
// completes the flow at the provider.
type ProviderConnectionSetup struct {
	// NextStep is what has to happen next, one of the ConnectionNextStep values.
	NextStep string
	// URL is where to send the user. Present when NextStep is redirect.
	URL string
	// StateToken ties the provider's callback back to this setup. Present when
	// NextStep is register.
	StateToken string
}

// connectionSetupAttrs is the attributes object of a setup response. The manifest
// that accompanies a register step is not modelled: the CLI cannot drive an app
// registration, so it has nothing to do with it.
type connectionSetupAttrs struct {
	NextStep   string `json:"next_step"`
	URL        string `json:"url,omitempty"`
	StateToken string `json:"state_token,omitempty"`
}

// SetupProviderConnection starts connecting a provider to an organization.
// orgID must be the organization UUID; returnURL is where the provider sends the
// user once the flow completes.
//
// Every call mints a fresh, hour-long state token, so this is not idempotent:
// call it when a user is about to be sent to the provider, not to probe.
func (c *Client) SetupProviderConnection(ctx context.Context, orgID, provider, returnURL string) (*ProviderConnectionSetup, error) {
	body := map[string]any{
		"type":       "vcs",
		"vcs":        map[string]any{"provider": provider},
		"return_url": returnURL,
	}

	var resp v3Entity[struct {
		Attributes connectionSetupAttrs `json:"attributes"`
	}]
	_, err := c.main.Call(ctx, httpcl.NewRequest(http.MethodPost, "/api/v3/provider/connections/setup",
		filterParam("org_id", orgID),
		httpcl.Body(body),
		httpcl.JSONDecoder(&resp),
	))
	if err != nil {
		return nil, err
	}

	return &ProviderConnectionSetup{
		NextStep:   resp.Data.Attributes.NextStep,
		URL:        resp.Data.Attributes.URL,
		StateToken: resp.Data.Attributes.StateToken,
	}, nil
}

// ForgejoSetup is what connecting a Forgejo instance to an organization takes.
type ForgejoSetup struct {
	// URL is the instance URL, e.g. "https://forgejo.example.com".
	URL string
	// Owner is the Forgejo organization or user that owns the repositories.
	Owner string
	// ConnectionAudience and CheckoutAudience are the audiences Forgejo generated
	// for the two integrations the bot user created.
	ConnectionAudience string
	CheckoutAudience   string
	// RequestHeaders are sent on every request CircleCI makes to the instance, for
	// an instance behind a proxy or gateway. Optional.
	RequestHeaders map[string]string
}

// ForgejoConnection is a stored Forgejo connection and the webhook the user has
// to create for it, as returned by a configure step.
type ForgejoConnection struct {
	// ID is the connection's CircleCI UUID.
	ID      string
	Owner   ConnectionOwner
	Webhook ConnectionWebhook
}

// ConnectionOwner is the provider account a connection is for.
type ConnectionOwner struct {
	Login string `json:"login"`
	ID    string `json:"id"`
	// Type is "organization" or "user".
	Type string `json:"type"`
}

// ConnectionWebhook is the webhook the user creates at the provider. Secret is
// returned only by the setup call that created the connection.
type ConnectionWebhook struct {
	URL    string   `json:"url"`
	Secret string   `json:"secret"`
	Events []string `json:"events"`
}

// SetupForgejoConnection connects a Forgejo instance to an organization. orgID
// must be the organization UUID. The API validates the values against the
// instance before storing anything, and answers a 400 naming the value to fix.
//
// An organization has at most one Forgejo connection, so a repeat call is a 409.
func (c *Client) SetupForgejoConnection(ctx context.Context, orgID string, in ForgejoSetup) (*ForgejoConnection, error) {
	vcs := map[string]any{
		"provider":            "forgejo",
		"url":                 in.URL,
		"owner":               in.Owner,
		"connection_audience": in.ConnectionAudience,
		"checkout_audience":   in.CheckoutAudience,
	}
	if len(in.RequestHeaders) > 0 {
		vcs["request_headers"] = in.RequestHeaders
	}
	body := map[string]any{"type": "vcs", "vcs": vcs}

	var resp v3Entity[struct {
		ID         string `json:"id"`
		Attributes struct {
			NextStep string            `json:"next_step"`
			Owner    ConnectionOwner   `json:"owner"`
			Webhook  ConnectionWebhook `json:"webhook"`
		} `json:"attributes"`
	}]
	_, err := c.main.Call(ctx, httpcl.NewRequest(http.MethodPost, "/api/v3/provider/connections/setup",
		filterParam("org_id", orgID),
		httpcl.Body(body),
		httpcl.JSONDecoder(&resp),
	))
	if err != nil {
		return nil, err
	}
	if resp.Data.Attributes.NextStep != ConnectionNextStepConfigure {
		return nil, fmt.Errorf("unexpected connection setup step %q", resp.Data.Attributes.NextStep)
	}

	return &ForgejoConnection{
		ID:      resp.Data.ID,
		Owner:   resp.Data.Attributes.Owner,
		Webhook: resp.Data.Attributes.Webhook,
	}, nil
}

// connectionAttrs is the attributes object of a v3 provider connection entity.
type connectionAttrs struct {
	Provider            string `json:"provider"`
	ProviderURL         string `json:"provider_url,omitempty"`
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
			ProviderURL:         e.Attributes.ProviderURL,
			ExternalID:          e.Attributes.ExternalID,
			Login:               e.Attributes.Login,
			RepositorySelection: e.Attributes.RepositorySelection,
			AuthorizedUsername:  e.Attributes.AuthorizedUsername,
			ConnectionError:     e.Attributes.ConnectionError,
		})
	}
	return conns, nil
}
