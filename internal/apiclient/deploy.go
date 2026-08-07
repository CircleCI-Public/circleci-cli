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

// V3Deployment represents a deployment returned by GET /api/v3/deploy/deployments.
type V3Deployment struct {
	ID         string                `json:"id"`
	Attributes v3DeployAttributes    `json:"attributes"`
	References v3DeployReferences    `json:"references"`
}

type v3DeployAttributes struct {
	Type          string     `json:"type"`
	Status        string     `json:"status"`
	TargetVersion struct {
		Name string `json:"name"`
	} `json:"target_version"`
	IsRollback bool       `json:"is_rollback"`
	CreatedAt  time.Time  `json:"created_at"`
	EndedAt    *time.Time `json:"ended_at,omitempty"`
}

type v3DeployReferences struct {
	DeployComponent struct {
		ID         string `json:"id"`
		Attributes struct {
			Name string `json:"name"`
		} `json:"attributes"`
	} `json:"deploy_component"`
	Pipeline *v3RefID `json:"pipeline,omitempty"`
	Workflow *v3RefID `json:"workflow,omitempty"`
}

type v3RefID struct {
	ID string `json:"id"`
}

// ListDeployments returns up to limit deployments for an org, optionally filtered
// by project. Pass limit <= 0 for no limit (fetches all pages).
func (c *Client) ListDeployments(ctx context.Context, orgID, projectID string, limit int) ([]V3Deployment, error) {
	var all []V3Deployment
	cursor := ""

	for {
		var resp v3List[V3Deployment]
		err := c.getV3(ctx, "/deploy/deployments", &resp,
			queryParam("filter[org_id]", orgID),
			filterParam("project_id", projectID),
			pageLimit(limit),
			pageCursor(cursor),
		)
		if err != nil {
			return nil, err
		}

		all = append(all, resp.Data...)

		if limit > 0 && len(all) >= limit {
			return all[:limit], nil
		}

		if resp.Page.Next == nil || *resp.Page.Next == "" {
			return all, nil
		}
		cursor = *resp.Page.Next
	}
}
