package client

import (
	"bytes"
	"context"
	"encoding/json"
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
	token      string
}

// NewClient returns a reference to a Client.
func NewClient(endpoint string, token string, logger *logger.Logger) *Client {
	return &Client{
		httpClient: http.DefaultClient,
		endpoint:   endpoint,
		token:      token,
		logger:     logger,
	}
}

// NewAuthorizedRequest returns a new GraphQL request with the
// authorization headers set for CircleCI auth.
func (cl *Client) NewAuthorizedRequest(query string) (*Request, error) {
	if cl.token == "" {
		return nil, errors.New(`please set a token with 'circleci setup'
You can create a new personal API token here:
https://circleci.com/account/api`)
	}

	request := &Request{
		Query:     query,
		Variables: make(map[string]interface{}),
		Header:    make(map[string][]string),
	}

	request.Header.Set("Authorization", cl.token)
	request.Header.Set("User-Agent", version.UserAgent())
	return request, nil
}

// NewUnauthorizedRequest returns a new GraphQL request without any authorization header.
func (cl *Client) NewUnauthorizedRequest(query string) (*Request, error) {
	request := &Request{
		Query:     query,
		Variables: make(map[string]interface{}),
		Header:    make(map[string][]string),
	}

	request.Header.Set("User-Agent", version.UserAgent())
	return request, nil
}

// Request is a GraphQL request.
type Request struct {
	Query     string                 `json:"query"`
	Variables map[string]interface{} `json:"variables"`

	// Header represent any request headers that will be set
	// when the request is made.
	Header http.Header `json:"-"`
}

// Var sets a variable.
func (request *Request) Var(key string, value interface{}) {
	request.Variables[key] = value
}

func (cl *Client) prepareRequest(ctx context.Context, request *Request) (*http.Request, error) {
	var requestBody bytes.Buffer
	if err := json.NewEncoder(&requestBody).Encode(request); err != nil {
		return nil, errors.Wrap(err, "encode body")
	}
	cl.logger.Debug(">> variables: %v", request.Variables)
	cl.logger.Debug(">> query: %s", request.Query)
	r, err := http.NewRequest(http.MethodPost, cl.endpoint, &requestBody)
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
func (cl *Client) Run(ctx context.Context, request *Request, resp interface{}) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	req, err := cl.prepareRequest(ctx, request)
	if err != nil {
		return err
	}

	res, err := cl.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer func() {
		err := res.Body.Close()
		if err != nil {
			cl.logger.Debug(err.Error())
		}
	}()

	if err := json.NewDecoder(res.Body).Decode(&resp); err != nil {
		return errors.Wrap(err, "decoding response")
	}
	cl.logger.Debug("<< %+v", resp)

	return nil
}
