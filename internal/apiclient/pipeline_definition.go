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
