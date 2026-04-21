package apiclient

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"

	"github.com/CircleCI-Public/circleci-cli-v2/internal/httpcl"
)

// Artifact is a file produced by a CircleCI job.
type Artifact struct {
	Path      string `json:"path"`
	URL       string `json:"url"`
	NodeIndex int    `json:"node_index"`
}

// GetJobArtifacts returns the artifacts produced by a specific job number
// within a project.
func (c *Client) GetJobArtifacts(ctx context.Context, projectSlug string, jobNumber int64) ([]Artifact, error) {
	path := fmt.Sprintf("/project/%s/%d/artifacts", url.PathEscape(projectSlug), jobNumber)
	var resp struct {
		Items []Artifact `json:"items"`
	}
	if err := c.get(ctx, path, &resp); err != nil {
		return nil, err
	}
	return resp.Items, nil
}

// DownloadArtifact fetches an artifact URL (authenticated) and writes its
// contents to dst. The URL is a full absolute URL, not a base-relative path.
func (c *Client) DownloadArtifact(ctx context.Context, artifactURL string, dst io.Writer) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, artifactURL, nil)
	if err != nil {
		return fmt.Errorf("building request: %w", err)
	}
	req.Header.Set("Circle-Token", c.token)

	resp, err := c.raw.Do(req)
	if err != nil {
		return fmt.Errorf("downloading artifact: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode >= 400 {
		return &httpcl.HTTPError{Method: http.MethodGet, Route: artifactURL, StatusCode: resp.StatusCode}
	}

	if _, err := io.Copy(dst, resp.Body); err != nil {
		return fmt.Errorf("writing artifact: %w", err)
	}
	return nil
}
