package workflow

import (
	"fmt"
	"net/url"

	"github.com/CircleCI-Public/circleci-cli/api/rest"
	"github.com/CircleCI-Public/circleci-cli/settings"
)

type workflowRestClient struct {
	client *rest.Client
}

var _ WorkflowClient = &workflowRestClient{}

func NewWorkflowRestClient(config settings.Config) (*workflowRestClient, error) {
	client := &workflowRestClient{
		client: rest.NewFromConfig(config.Host, &config),
	}
	return client, nil
}

func (c *workflowRestClient) ListWorkflowJobs(workflowID string) ([]Job, error) {
	items := []Job{}
	pageToken := ""

	for {
		path := fmt.Sprintf("workflow/%s/job", workflowID)

		params := url.Values{}
		if pageToken != "" {
			params.Set("page-token", pageToken)
		}

		req, err := c.client.NewRequest("GET", &url.URL{Path: path, RawQuery: params.Encode()}, nil)
		if err != nil {
			return nil, err
		}

		var resp JobsResponse
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
