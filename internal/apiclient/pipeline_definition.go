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
	"strings"
	"time"
)

// PipelineDefinitionRepo holds repository info for a pipeline definition source.
type PipelineDefinitionRepo struct {
	FullName   string `json:"full_name,omitempty"`
	ExternalID string `json:"external_id,omitempty"`
}

// PipelineDefinitionSource describes a config or checkout source.
type PipelineDefinitionSource struct {
	Provider string                  `json:"provider,omitempty"`
	Repo     *PipelineDefinitionRepo `json:"repo,omitempty"`
	FilePath string                  `json:"file_path,omitempty"`
}

// PipelineDefinition represents a CircleCI pipeline definition.
type PipelineDefinition struct {
	ID             string                    `json:"id"`
	Name           string                    `json:"name"`
	Description    string                    `json:"description,omitempty"`
	CreatedAt      time.Time                 `json:"created_at"`
	ConfigSource   *PipelineDefinitionSource `json:"config_source,omitempty"`
	CheckoutSource *PipelineDefinitionSource `json:"checkout_source,omitempty"`
}

// ListPipelineDefinitions returns all pipeline definitions for a project.
func (c *Client) ListPipelineDefinitions(ctx context.Context, projectID string) ([]PipelineDefinition, error) {
	var resp struct {
		Items []PipelineDefinition `json:"items"`
	}
	err := c.get(ctx, "/projects/%s/pipeline-definitions", &resp,
		routeParams(projectID),
	)
	if err != nil {
		return nil, err
	}
	return resp.Items, nil
}

// CreatePipelineDefinitionInput contains all fields for creating a pipeline definition.
type CreatePipelineDefinitionInput struct {
	Name             string
	Description      string
	ConfigProvider   string
	ConfigRepoID     string
	ConfigFilePath   string
	CheckoutProvider string
	CheckoutRepoID   string
}

// TriggerPipelineRunInput contains the options for triggering a pipeline run.
type TriggerPipelineRunInput struct {
	DefinitionID   string
	ConfigBranch   string
	ConfigTag      string
	CheckoutBranch string
	CheckoutTag    string
	Parameters     map[string]any
}

// TriggerPipelineRunResult holds the response from triggering a pipeline run.
// When Triggered is false the pipeline was skipped (e.g. due to a CI skip
// commit message) and Message describes why.
type TriggerPipelineRunResult struct {
	Triggered bool
	ID        string
	State     string
	Number    int
	CreatedAt time.Time
	Message   string
}

// TriggerPipelineRun triggers a pipeline run via the recommended v2 endpoint.
// projectSlug must be in "vcs/org/repo" form (e.g. "gh/myorg/myrepo").
func (c *Client) TriggerPipelineRun(ctx context.Context, projectSlug string, input TriggerPipelineRunInput) (*TriggerPipelineRunResult, error) {
	// The endpoint path is /project/{provider}/{organization}/{project}/pipeline/run.
	// We split the slug into three separate route params so each segment is
	// individually percent-encoded. Passing the full slug as one param would
	// encode the slashes as %2F, producing a single path segment that the server
	// cannot route — resulting in a 404.
	parts := strings.SplitN(projectSlug, "/", 3)
	if len(parts) != 3 {
		return nil, fmt.Errorf("invalid project slug %q: expected provider/organization/project", projectSlug)
	}

	body := map[string]any{}
	if input.DefinitionID != "" {
		body["definition_id"] = input.DefinitionID
	}

	cfg := map[string]any{}
	if input.ConfigBranch != "" {
		cfg["branch"] = input.ConfigBranch
	} else if input.ConfigTag != "" {
		cfg["tag"] = input.ConfigTag
	}
	if len(cfg) > 0 {
		body["config"] = cfg
	}

	checkout := map[string]any{}
	if input.CheckoutBranch != "" {
		checkout["branch"] = input.CheckoutBranch
	} else if input.CheckoutTag != "" {
		checkout["tag"] = input.CheckoutTag
	}
	if len(checkout) > 0 {
		body["checkout"] = checkout
	}

	if len(input.Parameters) > 0 {
		body["parameters"] = input.Parameters
	}

	var raw struct {
		ID        string    `json:"id"`
		State     string    `json:"state"`
		Number    int       `json:"number"`
		CreatedAt time.Time `json:"created_at"`
		Message   string    `json:"message"`
	}
	status, err := c.postStatus(ctx, "/project/%s/%s/%s/pipeline/run", body, &raw,
		routeParams(parts[0], parts[1], parts[2]),
	)
	if err != nil {
		return nil, err
	}
	return &TriggerPipelineRunResult{
		Triggered: status == http.StatusCreated,
		ID:        raw.ID,
		State:     raw.State,
		Number:    raw.Number,
		CreatedAt: raw.CreatedAt,
		Message:   raw.Message,
	}, nil
}

// CreatePipelineDefinition creates a new pipeline definition for a project.
func (c *Client) CreatePipelineDefinition(ctx context.Context, projectID string, input CreatePipelineDefinitionInput) (*PipelineDefinition, error) {
	configSource := map[string]any{
		"provider":  input.ConfigProvider,
		"file_path": input.ConfigFilePath,
	}
	if input.ConfigRepoID != "" {
		configSource["repo"] = map[string]any{
			"external_id": input.ConfigRepoID,
		}
	}

	checkoutSource := map[string]any{
		"provider": input.CheckoutProvider,
	}
	if input.CheckoutRepoID != "" {
		checkoutSource["repo"] = map[string]any{
			"external_id": input.CheckoutRepoID,
		}
	}

	body := map[string]any{
		"name":            input.Name,
		"config_source":   configSource,
		"checkout_source": checkoutSource,
	}
	if input.Description != "" {
		body["description"] = input.Description
	}

	var resp PipelineDefinition
	err := c.post(ctx, "/projects/%s/pipeline-definitions", body, &resp,
		routeParams(projectID),
	)
	if err != nil {
		return nil, err
	}
	return &resp, nil
}
