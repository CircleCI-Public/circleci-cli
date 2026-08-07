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

// V3DeploySettings represents the deploy settings for a project.
type V3DeploySettings struct {
	ID         string                `json:"id"`
	Attributes V3DeploySettingsAttrs `json:"attributes"`
	References V3DeploySettingsRefs  `json:"references"`
}

// V3DeploySettingsAttrs holds the attributes of deploy settings.
type V3DeploySettingsAttrs struct {
	AutoCancelRedundantDeploys bool `json:"auto_cancel_redundant_deploys"`
}

// V3DeploySettingsRefs holds reference IDs for deploy settings.
type V3DeploySettingsRefs struct {
	Project struct {
		ID string `json:"id"`
	} `json:"project"`
}

// GetDeploySettings returns deploy settings for a project.
func (c *Client) GetDeploySettings(ctx context.Context, projectID string) (*V3DeploySettings, error) {
	var resp v3Entity[V3DeploySettings]
	err := c.getV3(ctx, "/deploy/settings", &resp,
		queryParam("filter[project_id]", projectID),
	)
	if err != nil {
		return nil, err
	}
	return &resp.Data, nil
}
