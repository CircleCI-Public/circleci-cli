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

// V3Component represents a deploy component returned by GET /api/v3/deploy/components.
type V3Component struct {
	ID         string            `json:"id"`
	Attributes V3ComponentAttrs  `json:"attributes"`
	References V3ComponentRefs   `json:"references"`
}

// V3ComponentAttrs holds the attributes of a deploy component.
type V3ComponentAttrs struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

// V3ComponentRefs holds reference IDs for a deploy component.
type V3ComponentRefs struct {
	Project struct {
		ID string `json:"id"`
	} `json:"project"`
}

// V3ComponentVersion represents a version of a deploy component.
type V3ComponentVersion struct {
	ID         string                    `json:"id"`
	Attributes V3ComponentVersionAttrs   `json:"attributes"`
	References V3ComponentVersionRefs    `json:"references"`
}

// V3ComponentVersionAttrs holds the attributes of a component version.
type V3ComponentVersionAttrs struct {
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
}

// V3ComponentVersionRefs holds reference IDs for a component version.
type V3ComponentVersionRefs struct {
	Component struct {
		ID string `json:"id"`
	} `json:"component"`
}

// ListComponents returns deploy components for an org, optionally filtered by project.
// Pass limit <= 0 for no limit (fetches all pages).
func (c *Client) ListComponents(ctx context.Context, orgID, projectID string, limit int) ([]V3Component, error) {
	var all []V3Component
	cursor := ""

	for {
		var resp v3List[V3Component]
		err := c.getV3(ctx, "/deploy/components", &resp,
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

// GetComponent returns a single deploy component by ID.
func (c *Client) GetComponent(ctx context.Context, componentID string) (*V3Component, error) {
	var resp v3Entity[V3Component]
	err := c.getV3(ctx, "/deploy/components/"+componentID, &resp)
	if err != nil {
		return nil, err
	}
	return &resp.Data, nil
}

// ListComponentVersions returns versions of a component, optionally filtered by environment.
// Pass limit <= 0 for no limit (fetches all pages).
func (c *Client) ListComponentVersions(ctx context.Context, componentID, envID string, limit int) ([]V3ComponentVersion, error) {
	var all []V3ComponentVersion
	cursor := ""

	for {
		var resp v3List[V3ComponentVersion]
		err := c.getV3(ctx, "/deploy/components/"+componentID+"/versions", &resp,
			filterParam("environment_id", envID),
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
