package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"

	"github.com/pkg/errors"

	"github.com/CircleCI-Public/circleci-cli/api/header"
	"github.com/CircleCI-Public/circleci-cli/settings"
	"github.com/CircleCI-Public/circleci-cli/version"
)

// PolicyRestClient communicates with the CircleCI policy-service to ask questions
// about policies. It satisfies api.PolicyInterface.
type PolicyRestClient struct {
	token  string
	server string
	client *http.Client
}

func (c *PolicyRestClient) ListPolicies(ownerID, activeFilter string) (string, error) {
	req, err := c.newListPoliciesRequest(ownerID, activeFilter)
	if err != nil {
		return "", err
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return "", err
	}

	bodyBytes, err := ioutil.ReadAll(resp.Body)
	defer resp.Body.Close()
	if err != nil {
		return "", err
	}
	if resp.StatusCode != http.StatusOK {
		var dest errorResponse
		if err = json.Unmarshal(bodyBytes, &dest); err != nil {
			return "", err
		}
		return "", errors.New(*dest.Message)
	}

	var prettyJSON bytes.Buffer
	if err = json.Indent(&prettyJSON, bodyBytes, "", "\t"); err != nil {
		return "", err
	}
	return prettyJSON.String(), nil
}

func (c *PolicyRestClient) newListPoliciesRequest(ownerID, activeFilter string) (*http.Request, error) {
	var err error

	queryURL, err := url.Parse(c.server)
	if err != nil {
		return nil, err
	}
	queryURL, err = queryURL.Parse(fmt.Sprintf("owner/%s/policy", ownerID))
	if err != nil {
		return nil, err
	}

	urlParams := url.Values{}
	if activeFilter != "" {
		urlParams.Add("active", activeFilter)
	}

	queryURL.RawQuery = urlParams.Encode()

	return c.newHTTPRequest("GET", queryURL.String(), nil)
}

func (c *PolicyRestClient) newHTTPRequest(method, url string, body io.Reader) (*http.Request, error) {
	req, err := http.NewRequest(method, url, body)
	if err != nil {
		return nil, err
	}
	req.Header.Add("circle-token", c.token)
	req.Header.Add("Accept", "application/json")
	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("User-Agent", version.UserAgent())
	commandStr := header.GetCommandStr()
	if commandStr != "" {
		req.Header.Add("Circleci-Cli-Command", commandStr)
	}
	return req, nil
}

// NewPolicyRestClient returns a new client satisfying the api.PolicyInterface interface via the REST API.
func NewPolicyRestClient(config settings.Config) (*PolicyRestClient, error) {
	// Ensure server ends with a slash
	if !strings.HasSuffix(config.RestEndpoint, "/") {
		config.RestEndpoint += "/"
	}
	serverURL, err := url.Parse(config.Host)
	if err != nil {
		return nil, err
	}

	serverURL, err = serverURL.Parse(config.RestEndpoint)
	if err != nil {
		return nil, err
	}

	client := &PolicyRestClient{
		token:  config.Token,
		server: serverURL.String(),
		client: config.HTTPClient,
	}

	return client, nil
}
