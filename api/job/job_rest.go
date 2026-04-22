package job

import (
	"fmt"
	"net/url"

	"github.com/CircleCI-Public/circleci-cli/api/rest"
	"github.com/CircleCI-Public/circleci-cli/settings"
)

type jobRestClient struct {
	v2      *rest.Client
	v1      *rest.Client
	private *rest.Client
}

var _ JobClient = &jobRestClient{}

// NewJobRestClient returns a JobClient backed by the CircleCI REST APIs.
// It creates three REST clients for the v2, v1.1, and private API bases.
func NewJobRestClient(config settings.Config) (JobClient, error) {
	v2Client := rest.NewFromConfig(config.Host, &config)

	v1Config := config
	v1Config.RestEndpoint = "api/v1.1"
	v1Client := rest.NewFromConfig(config.Host, &v1Config)

	privateConfig := config
	privateConfig.RestEndpoint = "api/private"
	privateClient := rest.NewFromConfig(config.Host, &privateConfig)

	return &jobRestClient{v2: v2Client, v1: v1Client, private: privateClient}, nil
}

// GetTestResults fetches all test results for a job, paginating through all pages.
func (c *jobRestClient) GetTestResults(projectSlug string, jobNumber int) ([]TestResult, error) {
	var results []TestResult
	pageToken := ""

	for {
		path := fmt.Sprintf("project/%s/%d/tests", projectSlug, jobNumber)
		u := &url.URL{Path: path}
		if pageToken != "" {
			q := url.Values{}
			q.Set("page-token", pageToken)
			u.RawQuery = q.Encode()
		}

		req, err := c.v2.NewRequest("GET", u, nil)
		if err != nil {
			return nil, err
		}

		var resp struct {
			Items         []TestResult `json:"items"`
			NextPageToken string       `json:"next_page_token"`
		}
		_, err = c.v2.DoRequest(req, &resp)
		if err != nil {
			return nil, err
		}

		results = append(results, resp.Items...)

		if resp.NextPageToken == "" {
			break
		}
		pageToken = resp.NextPageToken
	}

	return results, nil
}

// GetJobSteps fetches detailed job information including steps via the v1.1 API.
func (c *jobRestClient) GetJobSteps(projectSlug string, jobNumber int) (*JobDetails, error) {
	path := fmt.Sprintf("project/%s/%d", projectSlug, jobNumber)
	req, err := c.v1.NewRequest("GET", &url.URL{Path: path}, nil)
	if err != nil {
		return nil, err
	}

	var resp JobDetails
	_, err = c.v1.DoRequest(req, &resp)
	if err != nil {
		return nil, err
	}
	return &resp, nil
}

// GetStepLog fetches raw log output for a step via the private API.
// logType should be "output" or "error".
func (c *jobRestClient) GetStepLog(projectSlug string, jobNumber int, taskIndex int, stepID int, logType string) (string, error) {
	path := fmt.Sprintf("output/raw/%s/%d/%s/%d/%d",
		projectSlug, jobNumber, logType, taskIndex, stepID)
	req, err := c.private.NewRequest("GET", &url.URL{Path: path}, nil)
	if err != nil {
		return "", err
	}

	body, _, err := c.private.DoRawRequest(req)
	if err != nil {
		return "", err
	}
	return string(body), nil
}
