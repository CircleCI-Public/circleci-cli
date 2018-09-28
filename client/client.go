package client

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log"
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

// RequestBody is used for serializing a request made up of a query and key/value pair of variables to send to the GraphQL server.
type RequestBody struct {
	Query     string                 `json:"query"`
	Variables map[string]interface{} `json:"variables"`
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

func (c *Client) prepareRequest(ctx context.Context, req *Request, resp interface{}) (*http.Request, *ResponseBody, error) {
	var requestBody bytes.Buffer
	requestBodyObj := RequestBody{
		Query:     req.q,
		Variables: req.vars,
	}
	if err := json.NewEncoder(&requestBody).Encode(requestBodyObj); err != nil {
		return nil, nil, errors.Wrap(err, "encode body")
	}
	c.logger.Debug(">> variables: %v", req.vars)
	c.logger.Debug(">> query: %s", req.q)
	body := &ResponseBody{
		Data: resp,
	}
	r, err := http.NewRequest(http.MethodPost, c.endpoint, &requestBody)
	if err != nil {
		return nil, nil, err
	}
	r.Header.Set("Content-Type", "application/json; charset=utf-8")
	r.Header.Set("Accept", "application/json; charset=utf-8")
	for key, values := range req.Header {
		for _, value := range values {
			r.Header.Add(key, value)
		}
	}

	r = r.WithContext(ctx)
	return r, body, nil
}

// Run sends an HTTP request to the GraphQL server and deserializes the response or returns an error.
func (c *Client) Run(ctx context.Context, req *Request, resp interface{}) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	r, body, err := c.prepareRequest(ctx, req, resp)
	if err != nil {
		return err
	}

	res, err := c.httpClient.Do(r)
	if err != nil {
		return err
	}
	defer func() {
		err := res.Body.Close()
		if err != nil {
			log.Fatal(err)
		}
	}()
	var buf bytes.Buffer
	if _, err := io.Copy(&buf, res.Body); err != nil {
		return errors.Wrap(err, "reading body")
	}
	c.logger.Debug("<< %s", buf.String())
	if err := json.NewDecoder(&buf).Decode(&body); err != nil {
		return errors.Wrap(err, "decoding response")
	}
	if len(body.Errors) > 0 {
		return body.bundleErrors()
	}
	return nil
}

// ResponseError wraps the error type returned from the GraphQL server.
type ResponseError struct {
	Message string
}

// Error returns the messages of a response error.
func (e ResponseError) Error() string {
	return "graphql: " + e.Message
}

// ResponseBody maps the data returned from the GraphQL server.
type ResponseBody struct {
	Data   interface{}
	Errors []ResponseError
}

func (resp ResponseBody) bundleErrors() error {
	var err error

	for _, e := range resp.Errors {
		if err != nil {
			err = errors.Wrap(err, e.Message)
		} else {
			err = errors.New(e.Message)
		}
	}
	return err
}

// Request is a GraphQL request.
type Request struct {
	q    string
	vars map[string]interface{}

	// Header represent any request headers that will be set
	// when the request is made.
	Header http.Header
}

// NewRequest makes a new Request with the specified string.
func NewRequest(q string) *Request {
	req := &Request{
		q:      q,
		Header: make(map[string][]string),
	}
	return req
}

// Var sets a variable.
func (req *Request) Var(key string, value interface{}) {
	if req.vars == nil {
		req.vars = make(map[string]interface{})
	}
	req.vars[key] = value
}
