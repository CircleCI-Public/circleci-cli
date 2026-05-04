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

// Release represents a release returned by the CircleCI Deploy API.
type Release struct {
	ID              string    `json:"id"`
	ProjectID       string    `json:"project_id"`
	ComponentID     string    `json:"component_id"`
	ComponentName   string    `json:"component_name"`
	Type            string    `json:"type"`
	Status          string    `json:"status"`
	TargetVersion   *Version  `json:"target_version"`
	PipelineID      string    `json:"pipeline_id,omitempty"`
	WorkflowID      string    `json:"workflow_id,omitempty"`
	PlanIsRollback  bool      `json:"plan_is_rollback"`
	IsRerelease     bool      `json:"is_rerelease"`
	FailureReason   string    `json:"failure_reason,omitempty"`
	CreatedAt       time.Time `json:"created_at"`
	StartedAt       time.Time `json:"started_at"`
	EndedAt         time.Time `json:"ended_at"`
}

// Version holds a version name.
type Version struct {
	Name string `json:"name"`
}

// ListReleases returns releases for a project. It paginates automatically.
func (c *Client) ListReleases(ctx context.Context, projectID, orgID string) ([]Release, error) {
	var all []Release
	pageToken := ""

	for {
		var resp struct {
			Items         []Release `json:"items"`
			NextPageToken string    `json:"next_page_token"`
		}

		err := c.get(ctx, "/deploy/projects/%s/releases", &resp,
			routeParams(projectID),
			queryParam("org-id", orgID),
			queryParam("page-size", "50"),
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
