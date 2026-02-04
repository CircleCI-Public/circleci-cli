package rest

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/CircleCI-Public/circleci-cli/api/header"
	"github.com/CircleCI-Public/circleci-cli/settings"
	"github.com/CircleCI-Public/circleci-cli/version"
)

type Client struct {
	BaseURL     *url.URL
	circleToken string
	client      *http.Client
}

func New(baseURL *url.URL, token string, httpClient *http.Client) *Client {
	return &Client{
		BaseURL:     baseURL,
		circleToken: token,
		client:      httpClient,
	}
}

func NewFromConfig(host string, config *settings.Config) *Client {
	// Ensure endpoint ends with a slash
	endpoint := config.RestEndpoint
	if !strings.HasSuffix(endpoint, "/") {
		endpoint += "/"
	}

	baseURL, err := url.Parse(host)
	if err != nil || baseURL.Host == "" {
		panic("Error: invalid CircleCI URL")
	}

	timeout := header.GetDefaultTimeout()
	if timeoutEnv, ok := os.LookupEnv("CIRCLECI_CLI_TIMEOUT"); ok {
		if parsedTimeout, err := time.ParseDuration(timeoutEnv); err == nil {
			timeout = parsedTimeout
		} else {
			fmt.Printf("failed to parse CIRCLECI_CLI_TIMEOUT: %s\n", err.Error())
		}
	}

	client := config.HTTPClient
	client.Timeout = timeout

	return New(
		baseURL.ResolveReference(&url.URL{Path: endpoint}),
		config.Token,
		client,
	)
}

func (c *Client) NewRequest(method string, u *url.URL, payload interface{}) (req *http.Request, err error) {
	var r io.Reader
	if payload != nil {
		buf := &bytes.Buffer{}
		r = buf
		err = json.NewEncoder(buf).Encode(payload)
		if err != nil {
			return nil, err
		}
	}

	req, err = http.NewRequest(method, c.BaseURL.ResolveReference(u).String(), r)
	if err != nil {
		return nil, err
	}

	c.enrichRequestHeaders(req, payload)
	return req, nil
}

func (c *Client) enrichRequestHeaders(req *http.Request, payload interface{}) {
	if c.circleToken != "" {
		req.Header.Set("Circle-Token", c.circleToken)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", version.UserAgent())
	commandStr := header.GetCommandStr()
	if commandStr != "" {
		req.Header.Set("Circleci-Cli-Command", commandStr)
	}
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
}

func (c *Client) DoRequest(req *http.Request, resp interface{}) (int, error) {
	httpResp, err := c.client.Do(req)
	if err != nil {
		fmt.Printf("failed to make http request: %s\n", err.Error())
		return 0, err
	}
	defer httpResp.Body.Close()

	if httpResp.StatusCode >= 400 {
		var msgErr struct {
			Message string `json:"message"`
		}
		body, err := io.ReadAll(httpResp.Body)
		if err != nil {
			return httpResp.StatusCode, err
		}
		err = json.Unmarshal(body, &msgErr)
		if err != nil {
			return httpResp.StatusCode, &HTTPError{Code: httpResp.StatusCode, Message: string(body)}
		}
		return httpResp.StatusCode, &HTTPError{Code: httpResp.StatusCode, Message: msgErr.Message}
	}

	if resp != nil {
		if !strings.Contains(httpResp.Header.Get("Content-Type"), "application/json") {
			body, _ := io.ReadAll(httpResp.Body)
			return httpResp.StatusCode, fmt.Errorf("wrong content type received. method: %s. path: %s. content-type: %s. body: %s", req.Method, req.URL.Path, httpResp.Header.Get("Content-Type"), string(body))
		}

		err = json.NewDecoder(httpResp.Body).Decode(resp)
		if err != nil {
			return httpResp.StatusCode, err
		}
	}
	return httpResp.StatusCode, nil
}

type HTTPError struct {
	Code    int
	Message string
}

func (e *HTTPError) Error() string {
	if e.Code == 0 {
		e.Code = http.StatusInternalServerError
	}
	if e.Message != "" {
		return e.Message
	}
	return fmt.Sprintf("response %d (%s)", e.Code, http.StatusText(e.Code))
}
