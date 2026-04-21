package apiclient

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/CircleCI-Public/circleci-cli-v2/internal/httpcl"
)

// Pipeline represents a CircleCI pipeline.
type Pipeline struct {
	ID          string          `json:"id"`
	State       string          `json:"state"`
	Number      int64           `json:"number"`
	CreatedAt   time.Time       `json:"created_at"`
	UpdatedAt   time.Time       `json:"updated_at"`
	ProjectSlug string          `json:"project_slug"`
	Trigger     PipelineTrigger `json:"trigger"`
	VCS         *PipelineVCS    `json:"vcs,omitempty"`
	Errors      []PipelineError `json:"errors,omitempty"`
}

// PipelineTrigger describes what triggered a pipeline.
type PipelineTrigger struct {
	Type       string    `json:"type"`
	ReceivedAt time.Time `json:"received_at"`
	Actor      Actor     `json:"actor"`
}

// Actor is a CircleCI user or token.
type Actor struct {
	Login     string `json:"login"`
	AvatarURL string `json:"avatar_url"`
}

// PipelineVCS holds version-control metadata for a pipeline.
type PipelineVCS struct {
	ProviderName        string     `json:"provider_name"`
	OriginRepositoryURL string     `json:"origin_repository_url"`
	TargetRepositoryURL string     `json:"target_repository_url"`
	Revision            string     `json:"revision"`
	Branch              string     `json:"branch,omitempty"`
	Tag                 string     `json:"tag,omitempty"`
	Commit              *VCSCommit `json:"commit,omitempty"`
}

// VCSCommit holds commit metadata.
type VCSCommit struct {
	Subject string `json:"subject"`
	Body    string `json:"body"`
}

// PipelineError is an error associated with a pipeline.
type PipelineError struct {
	Type    string `json:"type"`
	Message string `json:"message"`
}

// GetPipeline fetches a single pipeline by its UUID.
func (c *Client) GetPipeline(ctx context.Context, id string) (*Pipeline, error) {
	var p Pipeline
	if err := c.get(ctx, "/pipeline/"+id, &p); err != nil {
		return nil, err
	}
	return &p, nil
}

// GetPipelineByNumber fetches a pipeline by its project-scoped number.
func (c *Client) GetPipelineByNumber(ctx context.Context, projectSlug string, number int64) (*Pipeline, error) {
	var p Pipeline
	path := fmt.Sprintf("/project/%s/pipeline/%d", url.PathEscape(projectSlug), number)
	if err := c.get(ctx, path, &p); err != nil {
		return nil, err
	}
	return &p, nil
}

// GetLatestPipeline returns the most recent pipeline for the given project slug
// and branch. Pass an empty branch to get the latest pipeline regardless of branch.
func (c *Client) GetLatestPipeline(ctx context.Context, projectSlug, branch string) (*Pipeline, error) {
	path := fmt.Sprintf("/project/%s/pipeline", url.PathEscape(projectSlug))

	var resp struct {
		Items []Pipeline `json:"items"`
	}
	opts := []func(*httpcl.Request){}
	if branch != "" {
		opts = append(opts, httpcl.QueryParam("branch", branch))
	}
	if err := c.get(ctx, path, &resp, opts...); err != nil {
		return nil, err
	}

	if len(resp.Items) == 0 {
		return nil, &httpcl.HTTPError{Method: http.MethodGet, Route: path, StatusCode: http.StatusNotFound}
	}
	return &resp.Items[0], nil
}

// ListPipelines returns up to limit pipelines for a project, optionally filtered
// by branch. It paginates the API automatically until the limit is reached or all
// results are exhausted. Pass limit <= 0 for no limit (fetches all pages).
func (c *Client) ListPipelines(ctx context.Context, projectSlug, branch string, limit int) ([]Pipeline, error) {
	path := fmt.Sprintf("/project/%s/pipeline", url.PathEscape(projectSlug))

	var all []Pipeline
	pageToken := ""

	for {
		var resp struct {
			Items         []Pipeline `json:"items"`
			NextPageToken string     `json:"next_page_token"`
		}

		opts := []func(*httpcl.Request){}
		if branch != "" {
			opts = append(opts, httpcl.QueryParam("branch", branch))
		}
		if pageToken != "" {
			opts = append(opts, httpcl.QueryParam("page-token", pageToken))
		}

		if err := c.get(ctx, path, &resp, opts...); err != nil {
			return nil, err
		}

		all = append(all, resp.Items...)

		if limit > 0 && len(all) >= limit {
			return all[:limit], nil
		}

		if resp.NextPageToken == "" {
			return all, nil
		}
		pageToken = resp.NextPageToken
	}
}

// TriggerResponse is the response body from triggering a pipeline.
type TriggerResponse struct {
	ID        string    `json:"id"`
	State     string    `json:"state"`
	Number    int64     `json:"number"`
	CreatedAt time.Time `json:"created_at"`
}

// TriggerPipeline triggers a new pipeline for the given project and branch.
// params may be nil or empty if no pipeline parameters are needed.
func (c *Client) TriggerPipeline(ctx context.Context, projectSlug, branch string, params map[string]any) (*TriggerResponse, error) {
	path := fmt.Sprintf("/project/%s/pipeline", url.PathEscape(projectSlug))

	body := map[string]any{}
	if branch != "" {
		body["branch"] = branch
	}
	if len(params) > 0 {
		body["parameters"] = params
	}

	var resp TriggerResponse
	if err := c.post(ctx, path, body, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// PipelineWorkflowSummary holds brief workflow status for a pipeline.
type PipelineWorkflowSummary struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Status string `json:"status"`
}

// GetPipelineWorkflows returns the workflows for a pipeline.
func (c *Client) GetPipelineWorkflows(ctx context.Context, pipelineID string) ([]PipelineWorkflowSummary, error) {
	var resp struct {
		Items []PipelineWorkflowSummary `json:"items"`
	}
	if err := c.get(ctx, "/pipeline/"+pipelineID+"/workflow", &resp); err != nil {
		return nil, err
	}
	return resp.Items, nil
}
