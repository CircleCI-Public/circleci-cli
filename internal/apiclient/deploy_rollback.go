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

// Rollback types reported by the rollback endpoint, naming what the returned ID
// identifies: a pipeline rollback is carried out by a triggered pipeline run, an
// agent rollback by a restore-version command the release agent picks up.
const (
	RollbackTypePipeline = "pipeline"
	RollbackTypeAgent    = "agent"
)

// V3RollbackRequest is the body of POST /api/v3/projects/{id}/rollback.
//
// The component and the environment are addressed by ID, and Namespace scopes
// the component within that environment. CurrentVersion is not redundant with
// TargetVersion: the API rolls back only if the component instance really is
// running CurrentVersion, so a caller working from stale state cannot roll back
// a version that has since been superseded.
type V3RollbackRequest struct {
	ComponentID    string `json:"component_id"`
	EnvironmentID  string `json:"environment_id"`
	Namespace      string `json:"namespace,omitempty"`
	CurrentVersion string `json:"current_version"`
	TargetVersion  string `json:"target_version"`
	Reason         string `json:"reason,omitempty"`
	// Parameters are passed to the rollback pipeline.
	Parameters map[string]any `json:"parameters,omitempty"`
	// CheckoutRef and ConfigRef select the code and the config the rollback
	// pipeline runs from. Empty means the project's default branch. Both are
	// ignored when the project has no rollback pipeline configured.
	CheckoutRef string `json:"checkout_ref,omitempty"`
	ConfigRef   string `json:"config_ref,omitempty"`
}

// V3Rollback is a requested rollback returned by
// POST /api/v3/projects/{id}/rollback. ID is a handle to the work that carries
// the rollback out — the pipeline run or the agent command — not an ID minted
// for the request itself, and RollbackType says which of the two it is.
type V3Rollback struct {
	ID         string          `json:"id"`
	Attributes V3RollbackAttrs `json:"attributes"`
	References V3RollbackRefs  `json:"references"`
}

// V3RollbackAttrs holds the attributes of a requested rollback.
type V3RollbackAttrs struct {
	RollbackType string `json:"rollback_type"`
}

// V3RollbackRefs holds reference IDs for a requested rollback.
type V3RollbackRefs struct {
	Project struct {
		ID string `json:"id"`
	} `json:"project"`
	DeployComponent struct {
		ID string `json:"id"`
	} `json:"deploy_component"`
	DeployEnvironment struct {
		ID string `json:"id"`
	} `json:"deploy_environment"`
}

// RollbackProject rolls a deployed component of the given project back to an
// earlier version.
func (c *Client) RollbackProject(ctx context.Context, projectID string, req V3RollbackRequest) (*V3Rollback, error) {
	var resp v3Entity[V3Rollback]
	_, err := c.main.Call(ctx, httpcl.NewRequest(http.MethodPost, "/api/v3/projects/%s/rollback",
		httpcl.RouteParams(projectID),
		httpcl.Body(req),
		httpcl.JSONDecoder(&resp),
	))
	if err != nil {
		return nil, err
	}
	return &resp.Data, nil
}
