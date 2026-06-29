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

import "context"

// OrgInfo is returned by GET /api/v2/organization/{slug-or-id}.
type OrgInfo struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Slug    string `json:"slug"`
	VCSType string `json:"vcs_type"`
}

// GetOrg fetches an organization by its slug or UUID.
func (c *Client) GetOrg(ctx context.Context, slugOrID string) (*OrgInfo, error) {
	var org OrgInfo
	if err := c.get(ctx, "/organization/"+slugOrID, &org); err != nil {
		return nil, err
	}
	return &org, nil
}

// CreateOrg creates a new organization. vcsType must be one of "github",
// "bitbucket", or "circleci".
func (c *Client) CreateOrg(ctx context.Context, name, vcsType string) (*OrgInfo, error) {
	body := map[string]string{"name": name, "vcs_type": vcsType}
	var org OrgInfo
	if err := c.post(ctx, "/organization", body, &org); err != nil {
		return nil, err
	}
	return &org, nil
}
