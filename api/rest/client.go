package rest

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/CircleCI-Public/circleci-cli/api/header"
	"github.com/CircleCI-Public/circleci-cli/settings"
	"github.com/CircleCI-Public/circleci-cli/version"
)

type Client struct {
	baseURL *url.URL
	// The config api host differs for both cloud and server setups.
	// For cloud, the base domain will be https://api.circleci.com
	// for server, this should match the host as we don't have the same
	// api subdomain setup
	apiURL      *url.URL
	circleToken string
	client      *http.Client
}

func New(host string, config *settings.Config) *Client {
	// Ensure endpoint ends with a slash
	endpoint := config.RestEndpoint
	if !strings.HasSuffix(endpoint, "/") {
		endpoint += "/"
	}

	baseURL, _ := url.Parse(host)
	apiURL, _ := url.Parse(config.ConfigAPIHost)

	client := config.HTTPClient
	client.Timeout = 10 * time.Second

	return &Client{
		apiURL:      apiURL.ResolveReference(&url.URL{Path: endpoint}),
		baseURL:     baseURL.ResolveReference(&url.URL{Path: endpoint}),
		circleToken: config.Token,
		client:      client,
	}
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

	req, err = http.NewRequest(method, c.baseURL.ResolveReference(u).String(), r)
	if err != nil {
		return nil, err
	}

	c.enrichRequestHeaders(req, payload)
	return req, nil
}

// NewAPIRequest - similar to NewRequest except it uses the apiURL as the base URL.
func (c *Client) NewAPIRequest(method string, u *url.URL, payload interface{}) (req *http.Request, err error) {
	var r io.Reader
	if payload != nil {
		buf := &bytes.Buffer{}
		r = buf
		err = json.NewEncoder(buf).Encode(payload)
		if err != nil {
			fmt.Printf("failed to encode payload as json: %s\n", err.Error())
			return nil, err
		}
	}

	req, err = http.NewRequest(method, c.apiURL.ResolveReference(u).String(), r)
	if err != nil {
		fmt.Printf("failed to create new http request: %s\n", err.Error())
		return nil, err
	}

	c.enrichRequestHeaders(req, payload)
	return req, nil
}

func (c *Client) enrichRequestHeaders(req *http.Request, payload interface{}) {
	req.Header.Set("Circle-Token", c.circleToken)
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

func (c *Client) DoRequest(req *http.Request, resp interface{}) (statusCode int, err error) {
	httpResp, err := c.client.Do(req)
	if err != nil {
		fmt.Printf("failed to make http request: %s\n", err.Error())
		return 0, err
	}
	defer httpResp.Body.Close()

	if httpResp.StatusCode >= 300 {
		httpError := struct {
			Message string `json:"message"`
		}{}
		err = json.NewDecoder(httpResp.Body).Decode(&httpError)
		if err != nil {
			fmt.Printf("failed to decode body: %s", err.Error())
			return httpResp.StatusCode, err
		}
		return httpResp.StatusCode, &HTTPError{Code: httpResp.StatusCode, Message: httpError.Message}
	}

	if resp != nil {
		if !strings.Contains(httpResp.Header.Get("Content-Type"), "application/json") {
			return httpResp.StatusCode, errors.New("wrong content type received")
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
