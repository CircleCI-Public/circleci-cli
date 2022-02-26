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
	"github.com/CircleCI-Public/circleci-cli/version"
)

type Client struct {
	baseURL     *url.URL
	circleToken string
	client      *http.Client
}

func New(host, endpoint, circleToken string) *Client {
	// Ensure endpoint ends with a slash
	if !strings.HasSuffix(endpoint, "/") {
		endpoint += "/"
	}

	u, _ := url.Parse(host)
	return &Client{
		baseURL:     u.ResolveReference(&url.URL{Path: endpoint}),
		circleToken: circleToken,
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
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

	req.Header.Set("Circle-Token", c.circleToken)
	req.Header.Set("Accept-Type", "application/json")
	req.Header.Set("User-Agent", version.UserAgent())
	commandStr := header.GetCommandStr()
	if commandStr != "" {
		req.Header.Set("Circleci-Cli-Command", commandStr)
	}
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	return req, nil
}

func (c *Client) DoRequest(req *http.Request, resp interface{}) (statusCode int, err error) {
	httpResp, err := c.client.Do(req)
	if err != nil {
		return 0, err
	}
	defer httpResp.Body.Close()

	if httpResp.StatusCode >= 300 {
		httpError := struct {
			Message string `json:"message"`
		}{}
		err = json.NewDecoder(httpResp.Body).Decode(&httpError)
		if err != nil {
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
