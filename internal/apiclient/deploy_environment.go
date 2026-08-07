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

// V3Environment represents a deploy environment returned by GET /api/v3/deploy/environments.
type V3Environment struct {
	ID         string               `json:"id"`
	Attributes V3EnvironmentAttrs   `json:"attributes"`
	References V3EnvironmentRefs    `json:"references"`
}

// V3EnvironmentAttrs holds the attributes of a deploy environment.
type V3EnvironmentAttrs struct {
	Name string `json:"name"`
}

// V3EnvironmentRefs holds reference IDs for a deploy environment.
type V3EnvironmentRefs struct {
	Organization struct {
		ID string `json:"id"`
	} `json:"organization"`
}

// ListEnvironments returns deploy environments for an org.
// Pass limit <= 0 for no limit (fetches all pages).
func (c *Client) ListEnvironments(ctx context.Context, orgID string, limit int) ([]V3Environment, error) {
	var all []V3Environment
	cursor := ""

	for {
		var resp v3List[V3Environment]
		err := c.getV3(ctx, "/deploy/environments", &resp,
			queryParam("filter[org_id]", orgID),
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

// GetEnvironment returns a single deploy environment by ID.
func (c *Client) GetEnvironment(ctx context.Context, envID string) (*V3Environment, error) {
	var resp v3Entity[V3Environment]
	err := c.getV3(ctx, "/deploy/environments/"+envID, &resp)
	if err != nil {
		return nil, err
	}
	return &resp.Data, nil
}
