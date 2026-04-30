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
	"time"

	"github.com/google/uuid"
)

// Context is a CircleCI context — a named collection of secret environment
// variables shared across pipelines in an organization.
type Context struct {
	ID        uuid.UUID `json:"id"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
}

// ContextEnvVar is an environment variable stored in a context.
// The value is never returned by the API.
type ContextEnvVar struct {
	Variable       string    `json:"variable"`
	TruncatedValue string    `json:"truncated_value"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
	ContextID      uuid.UUID `json:"context_id"`
}

// ListContexts returns all contexts owned by the given organization slug
// (e.g. "gh/myorg"). Paginates automatically.
func (c *Client) ListContexts(ctx context.Context, ownerSlug, name string) ([]Context, error) {
	var all []Context
	pageToken := ""

	for {
		var resp struct {
			Items         []Context `json:"items"`
			NextPageToken string    `json:"next_page_token"`
		}
		err := c.get(ctx, "/context", &resp,
			optionalQueryParam("owner-slug", ownerSlug),
			optionalQueryParam("name", name),
			optionalQueryParam("page-token", pageToken),
		)
		if err != nil {
			return nil, err
		}
		all = append(all, resp.Items...)
		if resp.NextPageToken == "" {
			return all, nil
		}
		pageToken = resp.NextPageToken
	}
}

// CreateContext creates a new context for the given organization slug.
func (c *Client) CreateContext(ctx context.Context, name, ownerSlug string) (*Context, error) {
	body := map[string]any{
		"name": name,
		"owner": map[string]any{
			"slug": ownerSlug,
			"type": "organization",
		},
	}
	var ctxt Context
	err := c.post(ctx, "/context", body, &ctxt)
	if err != nil {
		return nil, err
	}
	return &ctxt, nil
}

type ContextDetail struct {
	ID                   uuid.UUID            `json:"id"`
	Name                 string               `json:"name"`
	CreatedAt            time.Time            `json:"created_at"`
	OrgID                uuid.UUID            `json:"org_id"`
	EnvironmentVariables []ContextEnvVar      `json:"environment_variables"`
	Restrictions         []ContextRestriction `json:"restrictions"`
}

type ContextRestriction struct {
	ContextID        uuid.UUID `json:"context_id"`
	ID               uuid.UUID `json:"id"`
	Name             string    `json:"name"`
	RestrictionType  string    `json:"restriction_type"`
	RestrictionValue string    `json:"restriction_value"`
}

// GetContext returns a context by its UUID.
func (c *Client) GetContext(ctx context.Context, id uuid.UUID) (*ContextDetail, error) {
	var ctxt ContextDetail
	err := c.get(ctx, "/context/%s", &ctxt,
		routeParams(id),
	)
	if err != nil {
		return nil, err
	}
	return &ctxt, nil
}

// DeleteContext deletes a context by its UUID.
func (c *Client) DeleteContext(ctx context.Context, id uuid.UUID) error {
	return c.deleteV2(ctx, "/context/%s",
		routeParams(id),
	)
}

// ListContextEnvVars returns the environment variable names stored in a context.
// Values are never returned by the API.
func (c *Client) ListContextEnvVars(ctx context.Context, contextID string) ([]ContextEnvVar, error) {
	var all []ContextEnvVar
	pageToken := ""

	for {
		var resp struct {
			Items         []ContextEnvVar `json:"items"`
			NextPageToken string          `json:"next_page_token"`
		}
		err := c.get(ctx, "/context/%s/environment-variable", &resp,
			routeParams(contextID),
			optionalQueryParam("page-token", pageToken),
		)
		if err != nil {
			return nil, err
		}
		all = append(all, resp.Items...)
		if resp.NextPageToken == "" {
			return all, nil
		}
		pageToken = resp.NextPageToken
	}
}

// SetContextEnvVar adds or updates an environment variable in a context.
func (c *Client) SetContextEnvVar(ctx context.Context, contextID, name, value string) (*ContextEnvVar, error) {
	body := map[string]any{"value": value}
	var ev ContextEnvVar
	err := c.put(ctx, "/context/%s/environment-variable/%s", body, &ev,
		routeParams(contextID, name),
	)
	if err != nil {
		return nil, err
	}
	return &ev, nil
}

// DeleteContextEnvVar removes an environment variable from a context.
func (c *Client) DeleteContextEnvVar(ctx context.Context, contextID, name string) error {
	return c.deleteV2(ctx, "/context/%s/environment-variable/%s",
		routeParams(contextID, name),
	)
}

// CreateContextRestriction adds a project, expression, or group restriction to a context.
// restrictionType must be one of "project", "expression", or "group".
// For project restrictions, restrictionValue is the project UUID.
// For expression restrictions, restrictionValue is the pipeline expression rule.
// For group restrictions, restrictionValue is the group UUID.
func (c *Client) CreateContextRestriction(ctx context.Context, contextID uuid.UUID, restrictionType, restrictionValue string) (*ContextRestriction, error) {
	body := map[string]any{
		"restriction_type":  restrictionType,
		"restriction_value": restrictionValue,
	}
	var r ContextRestriction
	err := c.post(ctx, "/context/%s/restrictions", body, &r,
		routeParams(contextID),
	)
	if err != nil {
		return nil, err
	}
	return &r, nil
}

// DeleteContextRestriction removes a restriction from a context by its restriction UUID.
func (c *Client) DeleteContextRestriction(ctx context.Context, contextID, restrictionID uuid.UUID) error {
	return c.deleteV2(ctx, "/context/%s/restrictions/%s",
		routeParams(contextID, restrictionID),
	)
}
