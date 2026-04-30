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

// RunnerInstance is a live runner agent connected to CircleCI.
type RunnerInstance struct {
	ResourceClass  string `json:"resource_class"`
	Hostname       string `json:"hostname"`
	Name           string `json:"name"`
	FirstConnected string `json:"first_connected"`
	LastConnected  string `json:"last_connected"`
	LastUsed       string `json:"last_used"`
	IP             string `json:"ip"`
	Version        string `json:"version"`
}

// RunnerTaskCounts holds unclaimed and running task counts for a resource class.
type RunnerTaskCounts struct {
	Unclaimed int `json:"unclaimed_task_count"`
	Running   int `json:"running_runner_tasks"`
}

// ListResourceClasses returns resource classes, optionally filtered by namespace.
// Uses the runner API at runner.circleci.com (or the configured server host).
func (c *Client) ListResourceClasses(ctx context.Context, namespace string) ([]ResourceClass, error) {
	var resp struct {
		Items []ResourceClass `json:"items"`
	}
	err := c.getRunner(ctx, "/runner", &resp,
		optionalQueryParam("namespace", namespace),
	)
	if err != nil {
		return nil, err
	}
	return resp.Items, nil
}

// CreateResourceClass creates a new runner resource class.
func (c *Client) CreateResourceClass(ctx context.Context, resourceClass, description string) (*ResourceClass, error) {
	body := map[string]any{
		"resource_class": resourceClass,
		"description":    description,
	}
	var rc ResourceClass
	err := c.postRunner(ctx, "/runner/resource", body, &rc)
	if err != nil {
		return nil, err
	}
	return &rc, nil
}

// DeleteResourceClass deletes a runner resource class by its namespace/name slug.
func (c *Client) DeleteResourceClass(ctx context.Context, resourceClass string) error {
	return c.deleteRunner(ctx, "/runner/resource/%s",
		routeParams(resourceClass),
	)
}

// ListRunnerTokens returns tokens for the given resource class.
func (c *Client) ListRunnerTokens(ctx context.Context, resourceClass string) ([]RunnerToken, error) {
	var resp struct {
		Items []RunnerToken `json:"items"`
	}
	err := c.getRunner(ctx, "/runner/token", &resp, queryParam("resource-class", resourceClass))
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
	err := c.postRunner(ctx, "/runner/token", body, &tok)
	if err != nil {
		return nil, err
	}
	return &tok, nil
}

// DeleteRunnerToken deletes a runner token by its ID.
func (c *Client) DeleteRunnerToken(ctx context.Context, tokenID string) error {
	return c.deleteRunner(ctx, "/runner/token/%s",
		routeParams(tokenID),
	)
}

// GetRunnerTaskCounts returns unclaimed and running task counts for a resource class.
func (c *Client) GetRunnerTaskCounts(ctx context.Context, resourceClass string) (*RunnerTaskCounts, error) {
	qp := queryParam("resource-class", resourceClass)
	var unclaimed struct {
		Count int `json:"unclaimed_task_count"`
	}
	var running struct {
		Count int `json:"running_runner_tasks"`
	}
	err := c.getRunner(ctx, "/runner/tasks", &unclaimed, qp)
	if err != nil {
		return nil, err
	}
	err = c.getRunner(ctx, "/runner/tasks/running", &running, qp)
	if err != nil {
		return nil, err
	}
	return &RunnerTaskCounts{
		Unclaimed: unclaimed.Count,
		Running:   running.Count,
	}, nil
}

// ListRunnerInstances returns live runner instances.
// Exactly one of resourceClass or namespace must be non-empty.
func (c *Client) ListRunnerInstances(ctx context.Context, resourceClass, namespace string) ([]RunnerInstance, error) {
	var resp struct {
		Items []RunnerInstance `json:"items"`
	}
	err := c.getRunner(ctx, "/runner", &resp,
		optionalQueryParam("resource-class", resourceClass),
		optionalQueryParam("namespace", namespace),
	)
	if err != nil {
		return nil, err
	}
	return resp.Items, nil
}
