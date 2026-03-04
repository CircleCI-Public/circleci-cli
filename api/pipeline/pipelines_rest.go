package pipeline

import (
	"fmt"
	"net/url"
)

func (c *pipelineRestClient) ListPipelinesForProject(projectSlug string, options ListPipelinesOptions) (*ListPipelinesResponse, error) {
	path := fmt.Sprintf("project/%s/pipeline", projectSlug)

	params := url.Values{}
	if options.Branch != "" {
		params.Set("branch", options.Branch)
	}
	if options.PageToken != "" {
		params.Set("page-token", options.PageToken)
	}

	req, err := c.client.NewRequest("GET", &url.URL{Path: path, RawQuery: params.Encode()}, nil)
	if err != nil {
		return nil, err
	}

	var resp ListPipelinesResponse
	_, err = c.client.DoRequest(req, &resp)
	if err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *pipelineRestClient) ListWorkflowsByPipelineId(pipelineID string) ([]Workflow, error) {
	items := []Workflow{}
	pageToken := ""

	for {
		path := fmt.Sprintf("pipeline/%s/workflow", pipelineID)

		params := url.Values{}
		if pageToken != "" {
			params.Set("page-token", pageToken)
		}

		req, err := c.client.NewRequest("GET", &url.URL{Path: path, RawQuery: params.Encode()}, nil)
		if err != nil {
			return nil, err
		}

		var resp ListWorkflowsResponse
		_, err = c.client.DoRequest(req, &resp)
		if err != nil {
			return nil, err
		}

		items = append(items, resp.Items...)

		if resp.NextPageToken == "" {
			break
		}
		pageToken = resp.NextPageToken
	}

	return items, nil
}
