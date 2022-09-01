package config

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"

	"github.com/CircleCI-Public/circleci-cli/api/header"
	"github.com/CircleCI-Public/circleci-cli/pipeline"
	"github.com/CircleCI-Public/circleci-cli/version"
	"github.com/pkg/errors"
)

// transportFunc is utility type for declaring a http.RoundTripper as a function literal
type transportFunc func(*http.Request) (*http.Response, error)

func (fn transportFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return fn(req)
}

type Client struct {
	Debug      bool
	httpClient *http.Client
	Endpoint   string
	Host       string
	Token      string
}

// NewClient returns a reference to a Client.
func NewClient(httpClient *http.Client, host, endpoint, token string, debug bool) *Client {
	return &Client{
		httpClient: http.DefaultClient,
		Endpoint:   endpoint,
		Host:       host,
		Token:      token,
		Debug:      debug,
	}
}

type Options struct {
	OwnerId            string            `json:"owner_id"`
	PipelineParameters string            `json:"pipeline_parameters"`
	PipelineValues     []pipeline.KeyVal `json:"pipeline_values"`
}

// type ConfigCompileRequest struct {
// 	ConfigYml string  `json:"config_yml"`
// 	Options   Options `json:"options"`
// }

type ConfigCompileRequest struct {
	ConfigYml string  `json:"config_yml"`
	Options   Options `json:"options"`
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

type Request struct {
	ConfigYml string  `json:"config_yml"`
	Options   Options `json:"options"`
	// Header represent any request headers that will be set
	// when the request is made.
	Header http.Header `json:"-"`
}

type ConfigError struct {
	Type    string
	Message string
}

type Response struct {
	Valid       bool
	Output_yaml string
	Errors      []ConfigError
	Source_Yaml string
}

// NewRequest returns a new GraphQL request.
func NewRequest() *Request {
	request := &Request{
		Header: make(map[string][]string),
	}

	request.Header.Set("User-Agent", version.UserAgent())
	commandStr := header.GetCommandStr()
	if commandStr != "" {
		request.Header.Set("Circleci-Cli-Command", commandStr)
	}
	return request
}

// Encode will return a buffer of the JSON encoded request body
func (request *Request) Encode() (bytes.Buffer, error) {
	var body bytes.Buffer
	err := json.NewEncoder(&body).Encode(request)
	return body, err
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

// func (cl *Client) CompileConfigWithDefaults(config_yml string, options Options) (string, error) {
// 	l := log.New(os.Stderr, "", 0)
// 	ctx := context.Background()

// 	select {
// 	case <-ctx.Done():
// 		return "", ctx.Err()
// 	default:
// 	}

// 	address, err := getServerAddress(cl.Host, cl.Endpoint)
// 	if err != nil {
// 		return "", err
// 	}

// 	reqBody, err := json.Marshal(&ConfigCompileRequest{ConfigYml: config_yml, Options: options})
// 	if err != nil {
// 		return "", fmt.Errorf("failed to construct request body: %w", err)
// 	}

// 	if cl.Debug {
// 		l.Printf(">> config_string: %s", config_yml)
// 		l.Printf(">> options: %+v", options) // check %v
// 	}

// 	req, err := http.NewRequestWithContext(ctx, "POST", address, bytes.NewBuffer(reqBody))
// 	if err != nil {
// 		return "", fmt.Errorf("failed to construct request: %w", err)
// 	}

// 	req.Header.Set("Authorization", cl.Token)

// 	res, err := cl.httpClient.Do(req)
// 	if err != nil {
// 		return "", fmt.Errorf("failed to get response: %w", err)
// 	}

// 	defer func() {
// 		responseBodyCloseErr := res.Body.Close()
// 		if responseBodyCloseErr != nil {
// 			l.Printf(responseBodyCloseErr.Error())
// 		}
// 	}()

// 	if cl.Debug {
// 		l.Printf("<< request id: %s", res.Header.Get("X-Request-Id"))
// 		l.Printf("<< result status: %s", res.Status)
// 	}

// 	if res.StatusCode != http.StatusOK {
// 		return "", fmt.Errorf("failure calling compile config API: %s", res.Status)
// 	}

// 	// Request.Body is an io.ReadCloser it can only be read once
// 	if cl.Debug {
// 		var bodyBytes []byte
// 		if res.Body != nil {
// 			bodyBytes, err = ioutil.ReadAll(res.Body)
// 			if err != nil {
// 				return "", errors.Wrap(err, "reading response")
// 			}

// 			l.Printf("<< %s", string(bodyBytes))

// 			// Restore the io.ReadCloser to its original state
// 			res.Body = ioutil.NopCloser(bytes.NewBuffer(bodyBytes))
// 		}
// 	}

// 	body, _ := ioutil.ReadAll(res.Body)

// 	return string(body), nil
// }

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
		l.Printf(">> config_yml: %s", request.ConfigYml)
		l.Printf(">> options: %+v", request.Options)
	}

	res, err := cl.httpClient.Do(req)
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	req.Header.Set("Accept", "application/json; charset=utf-8")
	req.Header.Set("Authorization", cl.Token)
	req.Header.Set("User-Agent", version.UserAgent())

	commandStr := header.GetCommandStr() // TODO: check this
	if commandStr != "" {
		req.Header.Set("Circleci-Cli-Command", commandStr)
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
		return fmt.Errorf("failure calling compile config API: %s", res.Status)
	}

	// Request.Body is an io.ReadCloser it can only be read once
	if cl.Debug {
		var bodyBytes []byte
		if res.Body != nil {
			bodyBytes, err = ioutil.ReadAll(res.Body)
			if err != nil {
				return errors.Wrap(err, "reading response")
			}

			l.Printf("<< %s", string(bodyBytes))

			// Restore the io.ReadCloser to its original state
			res.Body = ioutil.NopCloser(bytes.NewBuffer(bodyBytes))
		}
	}

	if err := json.NewDecoder(res.Body).Decode(&resp); err != nil {
		if cl.Debug {
			l.Printf("<< decoded result: %+v", resp)
		}

		return errors.Wrap(err, "decoding response")
	}

	return nil
}
