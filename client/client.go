package client

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"

	"github.com/CircleCI-Public/circleci-cli/logger"
	"github.com/CircleCI-Public/circleci-cli/version"
	"github.com/pkg/errors"
)

// A Client is an HTTP client for our GraphQL endpoint.
type Client struct {
	endpoint   string
	httpClient *http.Client
	logger     *logger.Logger
}

// NewClient returns a reference to a Client.
func NewClient(endpoint string, logger *logger.Logger) *Client {
	return &Client{
		httpClient: http.DefaultClient,
		endpoint:   endpoint,
		logger:     logger,
	}
}

// NewAuthorizedRequest returns a new GraphQL request with the
// authorization headers set for CircleCI auth.
func NewAuthorizedRequest(token, query string) *Request {
	req := NewRequest(query)
	req.Header.Set("Authorization", token)
	req.Header.Set("User-Agent", version.UserAgent())
	return req
}

// Request is a GraphQL request.
type Request struct {
	Query     string                 `json:"query"`
	Variables map[string]interface{} `json:"variables"`

	// Header represent any request headers that will be set
	// when the request is made.
	Header http.Header `json:"-"`
}

// NewRequest makes a new Request with the specified query string.
func NewRequest(query string) *Request {
	request := &Request{
		Query:     query,
		Variables: make(map[string]interface{}),
		Header:    make(map[string][]string),
	}
	return request
}

// Var sets a variable.
func (request *Request) Var(key string, value interface{}) {
	request.Variables[key] = value
}

func (c *Client) prepareRequest(ctx context.Context, request *Request) (*http.Request, error) {
	var requestBody bytes.Buffer
	if err := json.NewEncoder(&requestBody).Encode(request); err != nil {
		return nil, errors.Wrap(err, "encode body")
	}
	c.logger.Debug(">> variables: %v", request.Variables)
	c.logger.Debug(">> query: %s", request.Query)
	r, err := http.NewRequest(http.MethodPost, c.endpoint, &requestBody)
	if err != nil {
		return nil, err
	}
	r.Header.Set("Content-Type", "application/json; charset=utf-8")
	r.Header.Set("Accept", "application/json; charset=utf-8")
	for key, values := range request.Header {
		for _, value := range values {
			r.Header.Add(key, value)
		}
	}

	r = r.WithContext(ctx)
	return r, nil
}

// Run sends an HTTP request to the GraphQL server and deserializes the response or returns an error.
func (c *Client) Run(ctx context.Context, request *Request, resp interface{}) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	req, err := c.prepareRequest(ctx, request)
	if err != nil {
		return err
	}

	res, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer func() {
		err := res.Body.Close()
		if err != nil {
			c.logger.Debug(err.Error())
		}
	}()
	var buf bytes.Buffer
	if _, err := io.Copy(&buf, res.Body); err != nil {
		return errors.Wrap(err, "reading body")
	}

	c.logger.Debug("<< %s", buf.String())
	if err := json.NewDecoder(&buf).Decode(&resp); err != nil {
		return errors.Wrap(err, "decoding response")
	}

	return nil
}
