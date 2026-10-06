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
	ID              string `json:"id"`
	ResourceClass   string `json:"resource_class"`
	ResourceClassID string `json:"resource_class_id"`
	Nickname        string `json:"nickname"`
	CreatedAt       string `json:"created_at"`
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
	return c.listResourceClasses(ctx, "org_id", orgID.String())
}

// ListResourceClassesByNamespace returns every resource class in a namespace via the v3
// filter[namespace] endpoint.
func (c *Client) ListResourceClassesByNamespace(ctx context.Context, namespace string) ([]ResourceClass, error) {
	return c.listResourceClasses(ctx, "namespace", namespace)
}

// ListResourceClassesByClass returns the single resource class named by its namespace/name via
// the v3 filter[resource_class] endpoint. The server answers 404 when the class does not exist
// or the caller cannot view it.
func (c *Client) ListResourceClassesByClass(ctx context.Context, resourceClass string) ([]ResourceClass, error) {
	return c.listResourceClasses(ctx, "resource_class", resourceClass)
}

// listResourceClasses calls GET /api/v3/runner/resource-classes under one filter, which the
// endpoint requires exactly one of.
func (c *Client) listResourceClasses(ctx context.Context, filter, value string) ([]ResourceClass, error) {
	var resp v3List[v3ResourceClassItem]
	_, err := c.main.Call(ctx, httpcl.NewRequest(http.MethodGet, "/api/v3/runner/resource-classes",
		filterParam(filter, value),
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
// class matches the fully qualified resource class.
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

// GetResourceClass looks up a single resource class by its fully qualified namespace/name form
// via the v3 filter[resource_class] parameter. Returns ErrResourceClassNotFound when the class
// does not exist or belongs to an organization the caller cannot view.
func (c *Client) GetResourceClass(ctx context.Context, resourceClass string) (*ResourceClass, error) {
	var resp struct {
		Data []v3ResourceClassItem `json:"data"`
	}
	_, err := c.main.Call(ctx, httpcl.NewRequest(http.MethodGet, "/api/v3/runner/resource-classes",
		httpcl.QueryParam("filter[resource_class]", resourceClass),
		httpcl.JSONDecoder(&resp),
	))
	if err != nil {
		if httpcl.HasStatusCode(err, http.StatusNotFound) {
			return nil, fmt.Errorf("%w: %q", ErrResourceClassNotFound, resourceClass)
		}
		return nil, err
	}
	if len(resp.Data) == 0 {
		return nil, fmt.Errorf("%w: %q", ErrResourceClassNotFound, resourceClass)
	}
	rc := resp.Data[0].toResourceClass()
	return &rc, nil
}

// GetResourceClassByID looks up a single resource class by its UUID via the v3
// /runner/resource-classes/{id} endpoint. Returns ErrResourceClassNotFound when
// the server answers 404.
func (c *Client) GetResourceClassByID(ctx context.Context, id uuid.UUID) (*ResourceClass, error) {
	var resp v3Entity[v3ResourceClassItem]
	_, err := c.main.Call(ctx, httpcl.NewRequest(http.MethodGet, "/api/v3/runner/resource-classes/%s",
		httpcl.RouteParams(id.String()),
		httpcl.JSONDecoder(&resp),
	))
	if err != nil {
		if httpcl.HasStatusCode(err, http.StatusNotFound) {
			return nil, fmt.Errorf("%w: %q", ErrResourceClassNotFound, id)
		}
		return nil, err
	}
	rc := resp.Data.toResourceClass()
	return &rc, nil
}

// ResourceClassByName returns the resource class with the given fully qualified
// namespace/name resource class.
func (c *Client) ResourceClassByName(ctx context.Context, resourceClass string) (*ResourceClass, error) {
	if !strings.Contains(resourceClass, "/") {
		return nil, fmt.Errorf("%w: %q is not in namespace/name form", ErrResourceClassNotFound, resourceClass)
	}
	return c.GetResourceClass(ctx, resourceClass)
}

// UpdateResourceClass updates the description of a runner resource class.
// The request body is flat ({description: ...}), not the data envelope other v3
// writes use, but the response is the usual v3 data entity.
func (c *Client) UpdateResourceClass(ctx context.Context, id uuid.UUID, description string) (*ResourceClass, error) {
	body := map[string]any{"description": description}
	var resp v3Entity[v3ResourceClassItem]
	_, err := c.main.Call(ctx, httpcl.NewRequest(http.MethodPost,
		"/api/v3/runner/resource-classes/%s/update",
		httpcl.RouteParams(id.String()),
		httpcl.Body(body),
		httpcl.JSONDecoder(&resp),
	))
	if err != nil {
		return nil, err
	}
	rc := resp.Data.toResourceClass()
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

// v3RunnerTokenItem is a token item from the V3 /runner/tokens endpoint.
// References name the resource class the token belongs to.
type v3RunnerTokenItem struct {
	ID         string `json:"id"`
	Attributes struct {
		Nickname  string `json:"nickname"`
		CreatedAt string `json:"created_at"`
		Token     string `json:"token,omitempty"`
	} `json:"attributes"`
	References struct {
		ResourceClass struct {
			ID         string `json:"id"`
			Attributes struct {
				ResourceClass string `json:"resource_class"`
			} `json:"attributes"`
		} `json:"resource_class"`
	} `json:"references"`
}

// ListRunnerTokensByResourceClass returns tokens for the resource class named by
// its fully qualified namespace/name.
func (c *Client) ListRunnerTokensByResourceClass(ctx context.Context, resourceClass string) ([]RunnerToken, error) {
	return c.listRunnerTokens(ctx, "resource_class", resourceClass)
}

// ListRunnerTokensByResourceClassID returns tokens for the resource class with the given UUID.
func (c *Client) ListRunnerTokensByResourceClassID(ctx context.Context, rcID uuid.UUID) ([]RunnerToken, error) {
	return c.listRunnerTokens(ctx, "resource_class_id", rcID.String())
}

// listRunnerTokens lists GET /api/v3/runner/tokens under one filter; the endpoint takes
// exactly one of the two. The ResourceClass and ResourceClassID fields in returned
// tokens are the ones the response references.
func (c *Client) listRunnerTokens(ctx context.Context, filter, value string) ([]RunnerToken, error) {
	var resp struct {
		Data []v3RunnerTokenItem `json:"data"`
	}
	_, err := c.main.Call(ctx, httpcl.NewRequest(http.MethodGet, "/api/v3/runner/tokens",
		httpcl.QueryParam("filter["+filter+"]", value),
		httpcl.JSONDecoder(&resp),
	))
	if err != nil {
		return nil, err
	}
	tokens := make([]RunnerToken, len(resp.Data))
	for i, t := range resp.Data {
		tokens[i] = RunnerToken{
			ID:              t.ID,
			ResourceClass:   t.References.ResourceClass.Attributes.ResourceClass,
			ResourceClassID: t.References.ResourceClass.ID,
			Nickname:        t.Attributes.Nickname,
			CreatedAt:       t.Attributes.CreatedAt,
		}
	}
	return tokens, nil
}

// CreateRunnerTokenV3 creates a new token for the given resource class UUID using
// the V3 /runner/tokens endpoint. The ResourceClass field is the namespace/name
// the response references.
func (c *Client) CreateRunnerTokenV3(ctx context.Context, rcID uuid.UUID, nickname string) (*RunnerToken, error) {
	body := map[string]any{
		"data": map[string]any{
			"attributes": map[string]any{
				"nickname": nickname,
			},
			"references": map[string]any{
				"resource_class": map[string]any{"id": rcID.String()},
			},
		},
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
		ID:              t.ID,
		ResourceClass:   t.References.ResourceClass.Attributes.ResourceClass,
		ResourceClassID: t.References.ResourceClass.ID,
		Nickname:        t.Attributes.Nickname,
		CreatedAt:       t.Attributes.CreatedAt,
		Token:           t.Attributes.Token,
	}, nil
}

// DeleteRunnerTokenV3 deletes a runner token by its ID using the V3 /runner/tokens endpoint.
func (c *Client) DeleteRunnerTokenV3(ctx context.Context, tokenID uuid.UUID) error {
	_, err := c.main.Call(ctx, httpcl.NewRequest(http.MethodDelete, "/api/v3/runner/tokens/%s",
		httpcl.RouteParams(tokenID.String()),
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
// its fully qualified namespace/name resource class.
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

// RunnerFleet is a runner fleet: the agent and task summary of one resource class. Its ID is the
// resource class's ID. RunningTasks and QueuedTasks are only populated by GetRunnerFleet; the
// list endpoint omits them.
type RunnerFleet struct {
	ID         uuid.UUID `json:"id"`
	Attributes struct {
		Name               string     `json:"name"`
		ActiveAgents       int        `json:"active_agents"`
		IdleAgents         int        `json:"idle_agents"`
		DisconnectedAgents int        `json:"disconnected_agents"`
		AgentCount         int        `json:"agent_count"`
		RunningTasks       *int       `json:"running_tasks"`
		QueuedTasks        *int       `json:"queued_tasks"`
		LastTaskClaimedAt  *time.Time `json:"last_task_claimed_at"`
	} `json:"attributes"`
	References struct {
		ResourceClass struct {
			ID uuid.UUID `json:"id"`
		} `json:"resource_class"`
		// RunnerAgents is capped by the server; AgentCount is the true total.
		RunnerAgents []struct {
			ID         uuid.UUID `json:"id"`
			Attributes struct {
				Name string `json:"name"`
			} `json:"attributes"`
		} `json:"runner_agents"`
	} `json:"references"`
}

// ListRunnerFleetsByOrg returns every fleet in an organization.
func (c *Client) ListRunnerFleetsByOrg(ctx context.Context, orgID uuid.UUID) ([]RunnerFleet, error) {
	return c.listRunnerFleets(ctx, "org_id", orgID.String())
}

// ListRunnerFleetsByNamespace returns every fleet in a namespace.
func (c *Client) ListRunnerFleetsByNamespace(ctx context.Context, namespace string) ([]RunnerFleet, error) {
	return c.listRunnerFleets(ctx, "namespace", namespace)
}

// ListRunnerFleetsByResourceClass returns the fleet of the resource class named by its fully
// qualified namespace/name.
func (c *Client) ListRunnerFleetsByResourceClass(ctx context.Context, resourceClass string) ([]RunnerFleet, error) {
	return c.listRunnerFleets(ctx, "resource_class", resourceClass)
}

// ListRunnerFleetsByResourceClassID returns the fleet of the resource class with the given ID.
func (c *Client) ListRunnerFleetsByResourceClassID(ctx context.Context, id uuid.UUID) ([]RunnerFleet, error) {
	return c.listRunnerFleets(ctx, "resource_class_id", id.String())
}

// fleetPageLimit is the largest page[limit] GET /api/v3/runner/fleets accepts.
const fleetPageLimit = 250

// listRunnerFleets pages GET /api/v3/runner/fleets under one filter, which the endpoint requires
// exactly one of.
func (c *Client) listRunnerFleets(ctx context.Context, filter, value string) ([]RunnerFleet, error) {
	var all []RunnerFleet
	cursor := ""

	for {
		var env v3List[RunnerFleet]
		_, err := c.main.Call(ctx, httpcl.NewRequest(http.MethodGet, "/api/v3/runner/fleets",
			filterParam(filter, value),
			pageLimit(fleetPageLimit),
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

// GetRunnerFleet returns one fleet by its ID, which is its resource class's ID. Unlike the list,
// it includes running and queued task counts.
func (c *Client) GetRunnerFleet(ctx context.Context, id uuid.UUID) (*RunnerFleet, error) {
	var resp v3Entity[RunnerFleet]
	_, err := c.main.Call(ctx, httpcl.NewRequest(http.MethodGet, "/api/v3/runner/fleets/%s",
		httpcl.RouteParams(id.String()),
		httpcl.JSONDecoder(&resp),
	))
	if err != nil {
		return nil, err
	}
	return &resp.Data, nil
}
