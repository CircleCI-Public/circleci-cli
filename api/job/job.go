package job

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/CircleCI-Public/circleci-cli/settings"
)

type StepOutput struct {
	Message   string `json:"message"`
	Time      string `json:"time"`
	Type      string `json:"type"`
	Truncated bool   `json:"truncated"`
}

type StepAction struct {
	Name      string `json:"name"`
	OutputURL string `json:"output_url"`
	Status    string `json:"status"`
	Type      string `json:"type"`
}

type Step struct {
	Name    string       `json:"name"`
	Actions []StepAction `json:"actions"`
}

type JobDetails struct {
	BuildNum int    `json:"build_num"`
	Status   string `json:"status"`
	Steps    []Step `json:"steps"`
}

type JobClient interface {
	GetJobDetails(vcs string, org string, repo string, jobNumber int) (*JobDetails, error)
	GetStepOutput(outputURL string) ([]StepOutput, error)
}

type client struct {
	host       string
	token      string
	httpClient *http.Client
}

func NewClient(config settings.Config) *client {
	httpClient := config.HTTPClient
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	return &client{
		host:       strings.TrimRight(config.Host, "/"),
		token:      config.Token,
		httpClient: httpClient,
	}
}

func (c *client) GetJobDetails(vcs string, org string, repo string, jobNumber int) (*JobDetails, error) {
	requestURL := fmt.Sprintf("%s/api/v1.1/project/%s/%s/%s/%d", c.host, vcs, org, repo, jobNumber)
	req, err := http.NewRequest(http.MethodGet, requestURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	if c.token != "" {
		req.Header.Set("Circle-Token", c.token)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode >= 400 {
		var msg struct {
			Message string `json:"message"`
		}
		if err := json.Unmarshal(body, &msg); err == nil && msg.Message != "" {
			return nil, fmt.Errorf("failed to fetch job details: %s", msg.Message)
		}
		return nil, fmt.Errorf("failed to fetch job details: %s", strings.TrimSpace(string(body)))
	}

	var details JobDetails
	if err := json.Unmarshal(body, &details); err != nil {
		return nil, err
	}
	return &details, nil
}

func (c *client) GetStepOutput(outputURL string) ([]StepOutput, error) {
	req, err := http.NewRequest(http.MethodGet, outputURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("failed to fetch step output (%s): %s", sanitizeURL(outputURL), strings.TrimSpace(string(body)))
	}

	var outputs []StepOutput
	if err := json.Unmarshal(body, &outputs); err != nil {
		return nil, err
	}
	return outputs, nil
}

func sanitizeURL(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return "<redacted>"
	}
	u.RawQuery = ""
	u.Fragment = ""
	return u.String()
}
