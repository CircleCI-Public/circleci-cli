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

// Project is a followed CircleCI project.
type Project struct {
	Slug     string `json:"slug"`
	Name     string `json:"name"`
	VCSType  string `json:"vcs_type"`
	Username string `json:"username"`
	RepoName string `json:"reponame"`
}

// EnvVar is a project environment variable.
// The value is masked in list responses; it is only returned on set.
type EnvVar struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

type envVarWire struct {
	Attributes struct {
		Name  string `json:"name"`
		Value string `json:"value"`
	} `json:"attributes"`
}

// ListProjects returns all followed projects for the authenticated user.
// Uses the v1.1 API.
func (c *Client) ListProjects(ctx context.Context) ([]Project, error) {
	var projects []Project
	err := c.getV1(ctx, "/projects", &projects)
	if err != nil {
		return nil, err
	}
	return projects, nil
}

// FollowProject follows a project identified by its VCS type, org, and repo.
func (c *Client) FollowProject(ctx context.Context, vcsType, org, repo string) error {
	var resp struct {
		Following bool `json:"following"`
	}
	return c.postV1(ctx, "/project/%s/%s/%s/follow", nil, &resp,
		routeParams(vcsType, org, repo),
	)
}

// ListEnvVars returns the environment variables for a project.
// projectID is the project UUID. Values are masked in the response.
func (c *Client) ListEnvVars(ctx context.Context, projectID string) ([]EnvVar, error) {
	var resp v3List[envVarWire]
	if err := c.getV3(ctx, "/projects/%s/environment-variables", &resp, routeParams(projectID)); err != nil {
		return nil, err
	}
	vars := make([]EnvVar, len(resp.Data))
	for i, w := range resp.Data {
		vars[i] = EnvVar{Name: w.Attributes.Name, Value: w.Attributes.Value}
	}
	return vars, nil
}

// SetEnvVar creates or updates a project environment variable.
// projectID is the project UUID.
func (c *Client) SetEnvVar(ctx context.Context, projectID, name, value string) (*EnvVar, error) {
	body := map[string]any{"name": name, "value": value}
	var resp v3Entity[envVarWire]
	if err := c.postV3(ctx, "/projects/%s/environment-variables", body, &resp, routeParams(projectID)); err != nil {
		return nil, err
	}
	ev := EnvVar{Name: resp.Data.Attributes.Name, Value: resp.Data.Attributes.Value}
	return &ev, nil
}

// DeleteEnvVar deletes a project environment variable by name.
// projectID is the project UUID.
func (c *Client) DeleteEnvVar(ctx context.Context, projectID, name string) error {
	return c.deleteV3(ctx, "/projects/%s/environment-variables/%s", routeParams(projectID, name))
}

// ProjectInfo contains detailed information about a CircleCI project.
type ProjectInfo struct {
	ID               string   `json:"id"`
	Slug             string   `json:"slug"`
	Name             string   `json:"name"`
	OrganizationName string   `json:"organization_name"`
	OrganizationSlug string   `json:"organization_slug"`
	OrganizationID   string   `json:"organization_id"`
	VCSInfo          *VCSInfo `json:"vcs_info"`
}

// VCSInfo contains version control information for a project.
type VCSInfo struct {
	Provider      string `json:"provider"`
	DefaultBranch string `json:"default_branch"`
	VCSURL        string `json:"vcs_url"`
}

// GetProjectInfo returns detailed information about a project by slug.
func (c *Client) GetProjectInfo(ctx context.Context, projectSlug string) (*ProjectInfo, error) {
	var info ProjectInfo
	err := c.get(ctx, "/project/%s", &info,
		routeParams(projectSlug),
	)
	if err != nil {
		return nil, err
	}
	return &info, nil
}

// CreateProject creates a new project in the given organization.
// vcs is the VCS provider (e.g. "github", "circleci").
// org is the organization slug or UUID.
// name is the project name.
func (c *Client) CreateProject(ctx context.Context, vcs, org, name string) (*ProjectInfo, error) {
	body := map[string]any{"name": name}
	var proj ProjectInfo
	err := c.post(ctx, "/organization/%s/%s/project", body, &proj,
		routeParams(vcs, org),
	)
	if err != nil {
		return nil, err
	}
	return &proj, nil
}
