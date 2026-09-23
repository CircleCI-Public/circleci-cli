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
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/CircleCI-Public/circleci-cli/internal/httpcl"
)

// ResourceClass is a CircleCI runner resource class.
type ResourceClass struct {
	ID            string `json:"id"`
	ResourceClass string `json:"resource_class"`
	Description   string `json:"description"`
}

// RunnerToken is an authentication token for a resource class.
type RunnerToken struct {
	ID            string `json:"id"`
	ResourceClass string `json:"resource_class"`
	Nickname      string `json:"nickname"`
	CreatedAt     string `json:"created_at"`
	// Token is only populated on creation.
	Token string `json:"token,omitempty"`
}

// RunnerAgent is a connected runner agent returned by GET /api/v3/runner/agents.
type RunnerAgent struct {
	ID         uuid.UUID `json:"id"`
	Attributes struct {
		Name             string    `json:"name"`
		IsBusy           bool      `json:"is_busy"`
		Version          string    `json:"version"`
		FirstConnectedAt time.Time `json:"first_connected_at"`
		LastConnectedAt  time.Time `json:"last_connected_at"`
	} `json:"attributes"`
	References struct {
		ResourceClass struct {
			ID         uuid.UUID `json:"id"`
			Attributes struct {
				ResourceClass string `json:"resource_class"`
			} `json:"attributes"`
		} `json:"resource_class"`
	} `json:"references"`
}

// ListResourceClassesByOrg returns the resource classes for an organization,
// identified by its UUID, via the v3 filter[org_id] endpoint.
func (c *Client) ListResourceClassesByOrg(ctx context.Context, orgID uuid.UUID) ([]ResourceClass, error) {
	var resp v3List[v3ResourceClassItem]
	_, err := c.main.Call(ctx, httpcl.NewRequest(http.MethodGet, "/api/v3/runner/resource-classes",
		filterParam("org_id", orgID.String()),
		httpcl.JSONDecoder(&resp),
	))
	if err != nil {
		return nil, err
	}
	classes := make([]ResourceClass, len(resp.Data))
	for i, item := range resp.Data {
		classes[i] = item.toResourceClass()
	}
	return classes, nil
}

// ErrResourceClassNotFound is returned by ResourceClassByName when no resource
// class matches the slug.
var ErrResourceClassNotFound = errors.New("resource class not found")

// v3ResourceClassItem is the per-item shape returned by the v3 resource-classes endpoints.
type v3ResourceClassItem struct {
	ID         string `json:"id"`
	Attributes struct {
		ResourceClass string `json:"resource_class"`
		Description   string `json:"description"`
	} `json:"attributes"`
}

func (item v3ResourceClassItem) toResourceClass() ResourceClass {
	return ResourceClass{
		ID:            item.ID,
		ResourceClass: item.Attributes.ResourceClass,
		Description:   item.Attributes.Description,
	}
}

// CreateResourceClass creates a new runner resource class owned by orgID.
func (c *Client) CreateResourceClass(ctx context.Context, orgID uuid.UUID, resourceClass, description string) (*ResourceClass, error) {
	body := map[string]any{
		"data": map[string]any{
			"attributes": map[string]any{
				"resource_class": resourceClass,
				"description":    description,
			},
			"references": map[string]any{
				"org": map[string]any{"id": orgID.String()},
			},
		},
	}
	var resp v3Entity[v3ResourceClassItem]
	_, err := c.main.Call(ctx, httpcl.NewRequest(http.MethodPost, "/api/v3/runner/resource-classes",
		httpcl.Body(body),
		httpcl.JSONDecoder(&resp),
	))
	if err != nil {
		return nil, err
	}
	rc := resp.Data.toResourceClass()
	return &rc, nil
}

// GetResourceClassBySlug looks up a single resource class by its namespace/name slug via the
// v3 filter[slug] endpoint. Returns ErrResourceClassNotFound when the class does not exist or
// the caller is not authorized.
func (c *Client) GetResourceClassBySlug(ctx context.Context, slug string) (*ResourceClass, error) {
	var resp struct {
		Data []v3ResourceClassItem `json:"data"`
	}
	_, err := c.main.Call(ctx, httpcl.NewRequest(http.MethodGet, "/api/v3/runner/resource-classes",
		httpcl.QueryParam("filter[slug]", slug),
		httpcl.JSONDecoder(&resp),
	))
	if err != nil {
		return nil, err
	}
	if len(resp.Data) == 0 {
		return nil, fmt.Errorf("%w: %q", ErrResourceClassNotFound, slug)
	}
	rc := resp.Data[0].toResourceClass()
	return &rc, nil
}

// ResourceClassByName returns the resource class with the given namespace/name slug.
func (c *Client) ResourceClassByName(ctx context.Context, resourceClass string) (*ResourceClass, error) {
	if !strings.Contains(resourceClass, "/") {
		return nil, fmt.Errorf("%w: %q is not in namespace/name form", ErrResourceClassNotFound, resourceClass)
	}
	return c.GetResourceClassBySlug(ctx, resourceClass)
}

// UpdateResourceClass updates the description of a runner resource class.
// The body is flat ({description: ...}), not the data envelope other v3 writes use.
func (c *Client) UpdateResourceClass(ctx context.Context, id uuid.UUID, description string) (*ResourceClass, error) {
	body := map[string]any{"description": description}
	var rc ResourceClass
	_, err := c.main.Call(ctx, httpcl.NewRequest(http.MethodPost,
		"/api/v3/runner/resource-classes/%s/update",
		httpcl.RouteParams(id.String()),
		httpcl.Body(body),
		httpcl.JSONDecoder(&rc),
	))
	if err != nil {
		return nil, err
	}
	return &rc, nil
}

// DeleteResourceClass deletes a runner resource class by its id. With force,
// any tokens issued for it are deleted too; without it, the server rejects
// the delete (409) if tokens still exist.
func (c *Client) DeleteResourceClass(ctx context.Context, id uuid.UUID, force bool) error {
	_, err := c.main.Call(ctx, httpcl.NewRequest(http.MethodDelete, "/api/v3/runner/resource-classes/%s",
		httpcl.RouteParams(id.String()),
		httpcl.OptionalQueryParam("force", forceParam(force)),
	))
	return err
}

// forceParam renders force as the query value DeleteResourceClass sends, or ""
// (omitted by OptionalQueryParam) when force is false.
func forceParam(force bool) string {
	if !force {
		return ""
	}
	return "true"
}

// ListRunnerTokens returns tokens for the given resource class.
func (c *Client) ListRunnerTokens(ctx context.Context, resourceClass string) ([]RunnerToken, error) {
	var resp struct {
		Items []RunnerToken `json:"items"`
	}
	_, err := c.main.Call(ctx, httpcl.NewRequest(http.MethodGet, "/api/v3/runner/token",
		httpcl.QueryParam("resource-class", resourceClass),
		httpcl.JSONDecoder(&resp),
	))
	if err != nil {
		return nil, err
	}
	return resp.Items, nil
}

// CreateRunnerToken creates a new token for the given resource class.
// The token value is only returned once and is not retrievable afterwards.
func (c *Client) CreateRunnerToken(ctx context.Context, resourceClass, nickname string) (*RunnerToken, error) {
	body := map[string]any{
		"resource_class": resourceClass,
		"nickname":       nickname,
	}
	var tok RunnerToken
	_, err := c.main.Call(ctx, httpcl.NewRequest(http.MethodPost, "/api/v3/runner/token",
		httpcl.Body(body),
		httpcl.JSONDecoder(&tok),
	))
	if err != nil {
		return nil, err
	}
	return &tok, nil
}

// DeleteRunnerToken deletes a runner token by its ID.
func (c *Client) DeleteRunnerToken(ctx context.Context, tokenID string) error {
	_, err := c.main.Call(ctx, httpcl.NewRequest(http.MethodDelete, "/api/v3/runner/token/%s",
		httpcl.RouteParams(tokenID),
	))
	return err
}

// v3RunnerTokenItem is a token item from the V3 /runner/tokens endpoint.
// List items carry only attributes; the resource class is not included.
type v3RunnerTokenItem struct {
	ID         string `json:"id"`
	Attributes struct {
		Nickname  string `json:"nickname"`
		CreatedAt string `json:"created_at"`
		Token     string `json:"token,omitempty"`
	} `json:"attributes"`
}

// ListRunnerTokensV3 returns tokens for the given resource class UUID, using the
// V3 /runner/tokens endpoint. The ResourceClass field in returned tokens is
// filled from rcSlug since the V3 list response does not carry a resource class.
func (c *Client) ListRunnerTokensV3(ctx context.Context, rcID uuid.UUID, rcSlug string) ([]RunnerToken, error) {
	var resp struct {
		Data []v3RunnerTokenItem `json:"data"`
	}
	_, err := c.main.Call(ctx, httpcl.NewRequest(http.MethodGet, "/api/v3/runner/tokens",
		httpcl.QueryParam("filter[resource_class_id]", rcID.String()),
		httpcl.JSONDecoder(&resp),
	))
	if err != nil {
		return nil, err
	}
	tokens := make([]RunnerToken, len(resp.Data))
	for i, t := range resp.Data {
		tokens[i] = RunnerToken{
			ID:            t.ID,
			ResourceClass: rcSlug,
			Nickname:      t.Attributes.Nickname,
			CreatedAt:     t.Attributes.CreatedAt,
		}
	}
	return tokens, nil
}

// CreateRunnerTokenV3 creates a new token for the given resource class UUID using
// the V3 /runner/tokens endpoint. The ResourceClass field is filled from rcSlug.
func (c *Client) CreateRunnerTokenV3(ctx context.Context, rcID uuid.UUID, rcSlug, nickname string) (*RunnerToken, error) {
	body := map[string]any{
		"references": map[string]any{
			"resource_class": map[string]any{"id": rcID.String()},
		},
		"nickname": nickname,
	}
	var resp struct {
		Data v3RunnerTokenItem `json:"data"`
	}
	_, err := c.main.Call(ctx, httpcl.NewRequest(http.MethodPost, "/api/v3/runner/tokens",
		httpcl.Body(body),
		httpcl.JSONDecoder(&resp),
	))
	if err != nil {
		return nil, err
	}
	t := resp.Data
	return &RunnerToken{
		ID:            t.ID,
		ResourceClass: rcSlug,
		Nickname:      t.Attributes.Nickname,
		CreatedAt:     t.Attributes.CreatedAt,
		Token:         t.Attributes.Token,
	}, nil
}

// DeleteRunnerTokenV3 deletes a runner token by its ID using the V3 /runner/tokens endpoint.
func (c *Client) DeleteRunnerTokenV3(ctx context.Context, tokenID string) error {
	_, err := c.main.Call(ctx, httpcl.NewRequest(http.MethodDelete, "/api/v3/runner/tokens/%s",
		httpcl.RouteParams(tokenID),
	))
	return err
}

// agentPageLimit is the largest page[limit] GET /api/v3/runner/agents accepts; a bigger value is
// rejected with a 400 rather than clamped.
const agentPageLimit = 250

// ListRunnerAgentsByOrg returns every connected agent in an organization.
func (c *Client) ListRunnerAgentsByOrg(ctx context.Context, orgID uuid.UUID) ([]RunnerAgent, error) {
	return c.listRunnerAgents(ctx, "org_id", orgID.String())
}

// ListRunnerAgentsByResourceClass returns every connected agent of one resource class, named by
// its namespace/name slug.
func (c *Client) ListRunnerAgentsByResourceClass(ctx context.Context, resourceClass string) ([]RunnerAgent, error) {
	return c.listRunnerAgents(ctx, "resource_class", resourceClass)
}

// ListRunnerAgentsByNamespace returns every connected agent across a namespace's resource classes.
func (c *Client) ListRunnerAgentsByNamespace(ctx context.Context, namespace string) ([]RunnerAgent, error) {
	return c.listRunnerAgents(ctx, "namespace", namespace)
}

// listRunnerAgents pages GET /api/v3/runner/agents under one filter. The endpoint takes exactly one
// and serves 20 per page by default, so every scope has to follow the cursor to return them all.
func (c *Client) listRunnerAgents(ctx context.Context, filter, value string) ([]RunnerAgent, error) {
	var all []RunnerAgent
	cursor := ""

	for {
		var env v3List[RunnerAgent]
		_, err := c.main.Call(ctx, httpcl.NewRequest(http.MethodGet, "/api/v3/runner/agents",
			filterParam(filter, value),
			pageLimit(agentPageLimit),
			pageCursor(cursor),
			httpcl.JSONDecoder(&env),
		))
		if err != nil {
			return nil, err
		}
		all = append(all, env.Data...)
		if env.Page.Next == nil || *env.Page.Next == "" {
			return all, nil
		}
		cursor = *env.Page.Next
	}
}
