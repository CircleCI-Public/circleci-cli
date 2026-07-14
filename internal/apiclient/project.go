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
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// ErrProjectNotFound is returned by GetProjectBySlug when no project matches the slug.
var ErrProjectNotFound = errors.New("project not found")

// ProjectRef is a project resolved from its slug, carrying the UUIDs that the
// v3 API needs. Returned by GetProjectBySlug.
type ProjectRef struct {
	ID    uuid.UUID
	Name  string
	OrgID uuid.UUID
}

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
	Name      string     `json:"name"`
	Value     string     `json:"value"`
	CreatedAt *time.Time `json:"created_at"`
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
// Values are masked in the response.
func (c *Client) ListEnvVars(ctx context.Context, projectSlug string) ([]EnvVar, error) {
	var resp struct {
		Items []EnvVar `json:"items"`
	}
	err := c.get(ctx, "/project/%s/envvar", &resp,
		routeParams(projectSlug),
	)
	if err != nil {
		return nil, err
	}
	return resp.Items, nil
}

// SetEnvVar creates or updates a project environment variable.
func (c *Client) SetEnvVar(ctx context.Context, projectSlug, name, value string) (*EnvVar, error) {
	body := map[string]any{"name": name, "value": value}
	var ev EnvVar
	err := c.post(ctx, "/project/%s/envvar", body, &ev,
		routeParams(projectSlug),
	)
	if err != nil {
		return nil, err
	}
	return &ev, nil
}

// DeleteEnvVar deletes a project environment variable by name.
func (c *Client) DeleteEnvVar(ctx context.Context, projectSlug, name string) error {
	return c.deleteV2(ctx, "/project/%s/envvar/%s",
		routeParams(projectSlug, name),
	)
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

// projectEntity is the v3 response envelope for GET /api/v3/projects.
type projectEntity struct {
	ID         uuid.UUID `json:"id"`
	Attributes struct {
		Name string `json:"name"`
	} `json:"attributes"`
	References struct {
		Org struct {
			ID uuid.UUID `json:"id"`
		} `json:"org"`
	} `json:"references"`
}

// GetProjectBySlug resolves a project slug (vcs/org/repo) to its UUID, name, and
// owning org UUID via GET /api/v3/projects?filter[slug]=. Use this for the
// slug-to-UUID lookup; GetProjectInfo (v2) returns the fuller settings payload.
//
// The endpoint is a collection: a slug matching no project returns an empty list
// (not a 404), which is surfaced as ErrProjectNotFound.
func (c *Client) GetProjectBySlug(ctx context.Context, slug string) (*ProjectRef, error) {
	var env v3List[projectEntity]
	err := c.getV3(ctx, "/projects", &env,
		filterParam("slug", slug),
	)
	if err != nil {
		return nil, err
	}
	if len(env.Data) == 0 {
		return nil, fmt.Errorf("%w: %q", ErrProjectNotFound, slug)
	}
	p := env.Data[0]
	return &ProjectRef{
		ID:    p.ID,
		Name:  p.Attributes.Name,
		OrgID: p.References.Org.ID,
	}, nil
}

// GetProjectByID resolves a project UUID to its name (and owning org UUID) via
// GET /api/v3/projects/:id. Use this to label runs when only the project UUID is
// known — e.g. the cross-project "my runs" listing, where runs span projects
// whose slugs were never resolved.
func (c *Client) GetProjectByID(ctx context.Context, id uuid.UUID) (*ProjectRef, error) {
	var env v3Entity[projectEntity]
	if err := c.getV3(ctx, "/projects/%s", &env, routeParams(id)); err != nil {
		return nil, err
	}
	p := env.Data
	return &ProjectRef{
		ID:    p.ID,
		Name:  p.Attributes.Name,
		OrgID: p.References.Org.ID,
	}, nil
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

// ProjectSettingsAttributes is the "attributes" object returned by
// GET /api/v3/projects/:id/settings and POST /api/v3/projects/:id/update-settings.
type ProjectSettingsAttributes struct {
	AIErrorSummarization      bool     `json:"enable_ai_error_summarization"`
	AutoCancelBuilds          bool     `json:"enable_auto_cancel_redundant_workflows"`
	BuildForkPRs              bool     `json:"enable_building_fork_prs"`
	BuildPRsOnly              bool     `json:"is_build_prs_only"`
	CanPassSecretsToForkPR    bool     `json:"can_pass_secrets_to_fork_pr_jobs"`
	CanSetGitHubStatus        bool     `json:"can_set_github_status"`
	DisableRunning            bool     `json:"is_running_disabled"`
	DisableSSH                bool     `json:"is_ssh_disabled"`
	DynamicConfig             bool     `json:"enable_dynamic_config"`
	IsAdminRequiredForWriting bool     `json:"is_admin_required_for_writing_settings"`
	IsOSS                     bool     `json:"is_oss"`
	PROnlyBranchOverrides     []string `json:"pr_only_branch_overrides"`
	UnversionedConfig         bool     `json:"enable_unversioned_config"`
}

// ProjectSettingsUpdate is the body for POST /api/v3/projects/:id/update-settings.
// Only non-nil fields are sent; omitting a field leaves that setting unchanged.
type ProjectSettingsUpdate struct {
	AIErrorSummarization      *bool     `json:"enable_ai_error_summarization,omitempty"`
	AutoCancelBuilds          *bool     `json:"enable_auto_cancel_redundant_workflows,omitempty"`
	BuildForkPRs              *bool     `json:"enable_building_fork_prs,omitempty"`
	BuildPRsOnly              *bool     `json:"is_build_prs_only,omitempty"`
	CanPassSecretsToForkPR    *bool     `json:"can_pass_secrets_to_fork_pr_jobs,omitempty"`
	CanSetGitHubStatus        *bool     `json:"can_set_github_status,omitempty"`
	DisableRunning            *bool     `json:"is_running_disabled,omitempty"`
	DisableSSH                *bool     `json:"is_ssh_disabled,omitempty"`
	DynamicConfig             *bool     `json:"enable_dynamic_config,omitempty"`
	IsAdminRequiredForWriting *bool     `json:"is_admin_required_for_writing_settings,omitempty"`
	IsOSS                     *bool     `json:"is_oss,omitempty"`
	PROnlyBranchOverrides     *[]string `json:"pr_only_branch_overrides,omitempty"`
	UnversionedConfig         *bool     `json:"enable_unversioned_config,omitempty"`
}

type projectSettingsEnvelope struct {
	Data struct {
		Attributes ProjectSettingsAttributes `json:"attributes"`
	} `json:"data"`
}

// GetProjectSettings returns settings for a project via GET /api/v3/projects/:id/settings.
func (c *Client) GetProjectSettings(ctx context.Context, projectID uuid.UUID) (*ProjectSettingsAttributes, error) {
	var env projectSettingsEnvelope
	if err := c.getV3(ctx, "/projects/%s/settings", &env, routeParams(projectID)); err != nil {
		return nil, err
	}
	return &env.Data.Attributes, nil
}

// UpdateProjectSettings updates project settings via POST /api/v3/projects/:id/update-settings.
// Only the fields set in update are changed; omitted fields are left as-is.
func (c *Client) UpdateProjectSettings(ctx context.Context, projectID uuid.UUID, update ProjectSettingsUpdate) (*ProjectSettingsAttributes, error) {
	var env projectSettingsEnvelope
	if err := c.postV3(ctx, "/projects/%s/update-settings", update, &env, routeParams(projectID)); err != nil {
		return nil, err
	}
	return &env.Data.Attributes, nil
}
