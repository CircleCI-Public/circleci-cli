package apiclient

import (
	"context"
	"fmt"
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

// ListProjects returns all followed projects for the authenticated user.
// Uses the v1.1 API.
func (c *Client) ListProjects(ctx context.Context) ([]Project, error) {
	var projects []Project
	if err := c.getV1(ctx, "/projects", &projects); err != nil {
		return nil, err
	}
	return projects, nil
}

// FollowProject follows a project identified by its VCS type, org, and repo.
func (c *Client) FollowProject(ctx context.Context, vcsType, org, repo string) error {
	var resp struct {
		Following bool `json:"following"`
	}
	path := fmt.Sprintf("/project/%s/%s/%s/follow", vcsType, org, repo)
	return c.postV1(ctx, path, nil, &resp)
}

// ListEnvVars returns the environment variables for a project.
// Values are masked in the response.
func (c *Client) ListEnvVars(ctx context.Context, projectSlug string) ([]EnvVar, error) {
	var resp struct {
		Items []EnvVar `json:"items"`
	}
	if err := c.get(ctx, fmt.Sprintf("/project/%s/envvar", projectSlug), &resp); err != nil {
		return nil, err
	}
	return resp.Items, nil
}

// SetEnvVar creates or updates a project environment variable.
func (c *Client) SetEnvVar(ctx context.Context, projectSlug, name, value string) (*EnvVar, error) {
	body := map[string]any{"name": name, "value": value}
	var ev EnvVar
	if err := c.post(ctx, fmt.Sprintf("/project/%s/envvar", projectSlug), body, &ev); err != nil {
		return nil, err
	}
	return &ev, nil
}

// DeleteEnvVar deletes a project environment variable by name.
func (c *Client) DeleteEnvVar(ctx context.Context, projectSlug, name string) error {
	return c.deleteV2(ctx, fmt.Sprintf("/project/%s/envvar/%s", projectSlug, name))
}
