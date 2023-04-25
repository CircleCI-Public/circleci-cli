package graphql

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/CircleCI-Public/circleci-cli/api/header"
	"github.com/CircleCI-Public/circleci-cli/version"
	"github.com/pkg/errors"
)

// A Client is an HTTP client for our GraphQL endpoint.
type Client struct {
	Debug      bool
	Endpoint   string
	Host       string
	Token      string
	httpClient *http.Client
}

// NewClient returns a reference to a Client.
func NewClient(httpClient *http.Client, host, endpoint, token string, debug bool) *Client {
	return &Client{
		httpClient: httpClient,
		Endpoint:   endpoint,
		Host:       host,
		Token:      token,
		Debug:      debug,
	}
}

// Reset replaces the existing fields with out creating a new client instance
func (cl *Client) Reset(host, endpoint, token string, debug bool) {
	cl.Endpoint = endpoint
	cl.Host = host
	cl.Token = token
	cl.Debug = debug
}

// NewRequest returns a new GraphQL request.
func NewRequest(query string) *Request {
	request := &Request{
		Query:     query,
		Variables: make(map[string]interface{}),
		Header:    make(map[string][]string),
	}

	request.Header.Set("User-Agent", version.UserAgent())
	commandStr := header.GetCommandStr()
	if commandStr != "" {
		request.Header.Set("Circleci-Cli-Command", commandStr)
	}
	return request
}

// Request is a GraphQL request.
type Request struct {
	Query     string                 `json:"query"`
	Variables map[string]interface{} `json:"variables"`

	// Header represent any request headers that will be set
	// when the request is made.
	Header http.Header `json:"-"`
}

// SetToken sets the Authorization header for the request with the given token.
func (request *Request) SetToken(token string) {
	request.Header.Set("Authorization", token)
}

// Var sets a variable.
func (request *Request) Var(key string, value interface{}) {
	request.Variables[key] = value
}

// Encode will return a buffer of the JSON encoded request body
func (request *Request) Encode() (bytes.Buffer, error) {
	var body bytes.Buffer
	err := json.NewEncoder(&body).Encode(request)
	return body, err
}

// Response wraps the result from our GraphQL server response including out-of-band errors and the data itself.
type Response struct {
	Data   interface{}
	Errors ResponseErrorsCollection
}

// ResponseErrorsCollection represents a slice of errors returned by the GraphQL server out-of-band from the actual data.
type ResponseErrorsCollection []ResponseError

/*
An Example Error for an enum error looks like this:

{
  "errors": [
    {
      "message": "Provided argument value `GRUBHUB' is not member of enum type.",
      "locations": [
        {
          "line": 3,
          "column": 3
        }
      ],
      "extensions": {
        "field": "organization",
        "argument": "vcsType",
        "value": "GRUBHUB",
        "allowed-values": [
          "GITHUB",
          "BITBUCKET"
        ],
        "enum-type": "VCSType"
      }
    }
  ]
}
*/

// ResponseError represents the key-value pair of data returned by the GraphQL server to represent errors.
type ResponseError struct {
	Message   string
	Locations []struct {
		Line   int
		Column int
	}
	Extensions struct {
		Field         string
		Argument      string
		Value         string
		AllowedValues []string `json:"allowed-values"`
		EnumType      string   `json:"enum-type"`
	}
}

// Error turns a ResponseErrorsCollection into an acceptable error string that can be printed to the user.
func (errs ResponseErrorsCollection) Error() string {
	messages := []string{}

	for i := range errs {
		messages = append(messages, errs[i].Message)
	}

	return strings.Join(messages, "\n")
}

// getServerAddress returns the full address to the server
func getServerAddress(host, endpoint string) (string, error) {
	// 1. Parse the endpoint
	e, err := url.Parse(endpoint)
	if err != nil {
		return "", errors.Wrapf(err, "Parsing endpoint '%s'", endpoint)
	}

	// 2. Parse the host
	h, err := url.Parse(host)
	if err != nil {
		return "", errors.Wrapf(err, "Parsing host '%s'", host)
	}
	if !h.IsAbs() {
		return h.String(), fmt.Errorf("Host (%s) must be absolute URL, including scheme", host)
	}

	// 3. Resolve the two URLs using host as the base
	// We use ResolveReference which has specific behavior we can rely for
	// older configurations which included the absolute path for the endpoint flag.
	//
	// https://golang.org/pkg/net/url/#URL.ResolveReference
	//
	// Specifically this function always returns the reference (endpoint) if provided an absolute URL.
	// This way we can safely introduce --host and merge the two.
	return h.ResolveReference(e).String(), err
}

func prepareRequest(ctx context.Context, address string, request *Request) (*http.Request, error) {
	requestBody, err := request.Encode()
	if err != nil {
		return nil, err
	}
	r, err := http.NewRequest(http.MethodPost, address, &requestBody)
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
// TODO(zzak): This function is fairly complex, we should refactor it
// nolint: gocyclo
func (cl *Client) Run(request *Request, resp interface{}) error {
	l := log.New(os.Stderr, "", 0)
	ctx := context.Background()

	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	address, err := getServerAddress(cl.Host, cl.Endpoint)
	if err != nil {
		return err
	}

	req, err := prepareRequest(ctx, address, request)
	if err != nil {
		return err
	}

	if cl.Debug {
		l.Printf(">> variables: %v", request.Variables)
		l.Printf(">> query: %s", request.Query)
	}

	res, err := cl.httpClient.Do(req)

	if err != nil {
		return err
	}
	defer func() {
		responseBodyCloseErr := res.Body.Close()
		if responseBodyCloseErr != nil {
			l.Printf(responseBodyCloseErr.Error())
		}
	}()

	if cl.Debug {
		l.Printf("<< request id: %s", res.Header.Get("X-Request-Id"))
		l.Printf("<< result status: %s", res.Status)
	}

	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("failure calling GraphQL API: %s", res.Status)
	}

	// Request.Body is an io.ReadCloser it can only be read once
	if cl.Debug {
		var bodyBytes []byte
		if res.Body != nil {
			bodyBytes, err = io.ReadAll(res.Body)
			if err != nil {
				return errors.Wrap(err, "reading response")
			}

			l.Printf("<< %s", string(bodyBytes))

			// Restore the io.ReadCloser to its original state
			res.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
		}
	}

	wrappedResponse := &Response{
		Data: resp,
	}

	if err := json.NewDecoder(res.Body).Decode(&wrappedResponse); err != nil {
		return errors.Wrap(err, "decoding response")
	}

	if len(wrappedResponse.Errors) > 0 {
		return wrappedResponse.Errors
	}

	return nil
}
