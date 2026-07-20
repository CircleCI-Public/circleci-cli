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

// Pipeline represents a CircleCI pipeline.
type Pipeline struct {
	ID                uuid.UUID                  `json:"id"`
	State             string                     `json:"state"`
	Number            int64                      `json:"number"`
	CreatedAt         time.Time                  `json:"created_at"`
	UpdatedAt         time.Time                  `json:"updated_at"`
	ProjectSlug       string                     `json:"project_slug"`
	Trigger           PipelineTrigger            `json:"trigger"`
	TriggerParameters *PipelineTriggerParameters `json:"trigger_parameters,omitempty"`
	VCS               *PipelineVCS               `json:"vcs,omitempty"`
	Errors            []PipelineError            `json:"errors,omitempty"`
}

// PipelineTrigger describes what triggered a pipeline.
type PipelineTrigger struct {
	Type       string    `json:"type"`
	ReceivedAt time.Time `json:"received_at"`
	Actor      Actor     `json:"actor"`
}

// PipelineTriggerParameters holds git context for pipeline-definition-triggered runs.
// It is absent on legacy VCS-triggered pipelines (which use the vcs field instead).
type PipelineTriggerParameters struct {
	Git *PipelineTriggerGit `json:"git,omitempty"`
}

// PipelineTriggerGit holds the git fields within trigger_parameters.
type PipelineTriggerGit struct {
	Branch      string `json:"branch"`
	CheckoutSHA string `json:"checkout_sha"`
}

// Actor is a CircleCI user or token.
type Actor struct {
	Login     string `json:"login"`
	AvatarURL string `json:"avatar_url"`
}

// PipelineVCS holds version-control metadata for a pipeline.
type PipelineVCS struct {
	ProviderName        string     `json:"provider_name"`
	OriginRepositoryURL string     `json:"origin_repository_url"`
	TargetRepositoryURL string     `json:"target_repository_url"`
	Revision            string     `json:"revision"`
	Branch              string     `json:"branch,omitempty"`
	Tag                 string     `json:"tag,omitempty"`
	Commit              *VCSCommit `json:"commit,omitempty"`
}

// VCSCommit holds commit metadata.
type VCSCommit struct {
	Subject string `json:"subject"`
	Body    string `json:"body"`
}

// PipelineError is an error associated with a pipeline.
type PipelineError struct {
	Type    string `json:"type"`
	Message string `json:"message"`
}

// GetPipeline fetches a single pipeline by its UUID.
func (c *Client) GetPipeline(ctx context.Context, id string) (*Pipeline, error) {
	var p Pipeline
	err := c.get(ctx, "/pipeline/%s", &p,
		routeParams(id),
	)
	if err != nil {
		return nil, err
	}
	return &p, nil
}

// GetPipelineByNumber fetches a pipeline by its project-scoped number.
func (c *Client) GetPipelineByNumber(ctx context.Context, projectSlug string, number int64) (*Pipeline, error) {
	var p Pipeline
	err := c.get(ctx, "/project/%s/pipeline/%d", &p,
		routeParams(projectSlug, number),
	)
	if err != nil {
		return nil, err
	}
	return &p, nil
}

// TriggerResponse is the response body from triggering a pipeline.
type TriggerResponse struct {
	ID        string    `json:"id"`
	State     string    `json:"state"`
	Number    int64     `json:"number"`
	CreatedAt time.Time `json:"created_at"`
}

// TriggerPipeline triggers a new pipeline for the given project and branch.
// params may be nil or empty if no pipeline parameters are needed.
func (c *Client) TriggerPipeline(ctx context.Context, projectSlug, branch string, params map[string]any) (*TriggerResponse, error) {
	body := map[string]any{}
	if branch != "" {
		body["branch"] = branch
	}
	if len(params) > 0 {
		body["parameters"] = params
	}

	var resp TriggerResponse
	err := c.post(ctx, "/project/%s/pipeline", body, &resp,
		routeParams(projectSlug),
	)
	if err != nil {
		return nil, err
	}
	return &resp, nil
}
