// Package apiclient provides a thin HTTP client for the CircleCI REST API.
package apiclient

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/CircleCI-Public/circleci-cli-v2/internal/httpcl"
)

// Client is an authenticated CircleCI API client.
type Client struct {
	main    *httpcl.Client // circleci.com/api/v1.1, /api/v2
	runner  *httpcl.Client // runner.circleci.com/api/v3
	token   string
	baseURL string // e.g. "https://circleci.com"
	// raw is used only for requests to arbitrary full URLs (artifact downloads,
	// step output, and the api escape-hatch command).
	raw *http.Client
}

// New creates a Client. baseURL should be the CircleCI host, e.g. "https://circleci.com".
// An http.RoundTripper can be injected for testing. Set CIRCLECI_DEBUG=1 to log
// all HTTP requests and response status codes to stderr.
func New(baseURL, token string, transport http.RoundTripper) *Client {
	if os.Getenv("CIRCLECI_DEBUG") != "" {
		if transport == nil {
			transport = http.DefaultTransport
		}
		transport = &debugTransport{wrapped: transport}
	}

	cfg := httpcl.Config{
		AuthToken:  token,
		AuthHeader: "Circle-Token",
		Transport:  transport,
	}

	mainCfg := cfg
	mainCfg.BaseURL = baseURL

	runnerCfg := cfg
	runnerCfg.BaseURL = runnerBaseURL(baseURL)

	rawTransport := transport
	if rawTransport == nil {
		rawTransport = http.DefaultTransport
	}
	return &Client{
		main:    httpcl.New(mainCfg),
		runner:  httpcl.New(runnerCfg),
		token:   token,
		baseURL: baseURL,
		raw:     &http.Client{Timeout: 30 * time.Second, Transport: rawTransport},
	}
}

// runnerBaseURL derives the runner API base URL from the main API base URL.
// For circleci.com cloud the runner API is at runner.circleci.com.
// For self-hosted server the runner API is co-located at the same host.
func runnerBaseURL(baseURL string) string {
	if strings.Contains(baseURL, "circleci.com") {
		return "https://runner.circleci.com"
	}
	return baseURL
}

func (c *Client) get(ctx context.Context, path string, dst any, opts ...func(*httpcl.Request)) error {
	allOpts := make([]func(*httpcl.Request), 0, 1+len(opts))
	allOpts = append(allOpts, httpcl.JSONDecoder(dst))
	allOpts = append(allOpts, opts...)
	_, err := c.main.Call(ctx, httpcl.NewRequest(http.MethodGet, "/api/v2"+path, allOpts...))
	return err
}

func (c *Client) getV1(ctx context.Context, path string, dst any) error {
	_, err := c.main.Call(ctx, httpcl.NewRequest(http.MethodGet, "/api/v1.1"+path, httpcl.JSONDecoder(dst)))
	return err
}

func (c *Client) post(ctx context.Context, path string, body, dst any) error {
	_, err := c.main.Call(ctx, httpcl.NewRequest(http.MethodPost, "/api/v2"+path,
		httpcl.Body(body), httpcl.JSONDecoder(dst)))
	return err
}

func (c *Client) postV1(ctx context.Context, path string, body, dst any) error {
	_, err := c.main.Call(ctx, httpcl.NewRequest(http.MethodPost, "/api/v1.1"+path,
		httpcl.Body(body), httpcl.JSONDecoder(dst)))
	return err
}

func (c *Client) deleteV2(ctx context.Context, path string) error {
	_, err := c.main.Call(ctx, httpcl.NewRequest(http.MethodDelete, "/api/v2"+path))
	return err
}

func (c *Client) getRunner(ctx context.Context, path string, dst any, opts ...func(*httpcl.Request)) error {
	allOpts := make([]func(*httpcl.Request), 0, 1+len(opts))
	allOpts = append(allOpts, httpcl.JSONDecoder(dst))
	allOpts = append(allOpts, opts...)
	_, err := c.runner.Call(ctx, httpcl.NewRequest(http.MethodGet, "/api/v3"+path, allOpts...))
	return err
}

func (c *Client) postRunner(ctx context.Context, path string, body, dst any) error {
	_, err := c.runner.Call(ctx, httpcl.NewRequest(http.MethodPost, "/api/v3"+path,
		httpcl.Body(body), httpcl.JSONDecoder(dst)))
	return err
}

func (c *Client) deleteRunner(ctx context.Context, path string) error {
	_, err := c.runner.Call(ctx, httpcl.NewRequest(http.MethodDelete, "/api/v3"+path))
	return err
}

// debugTransport logs HTTP requests and response status codes to stderr.
type debugTransport struct {
	wrapped http.RoundTripper
}

func (d *debugTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	fmt.Fprintf(os.Stderr, "DEBUG: %s %s\n", req.Method, req.URL)
	resp, err := d.wrapped.RoundTrip(req)
	if err != nil {
		fmt.Fprintf(os.Stderr, "DEBUG: error: %v\n", err)
	} else {
		fmt.Fprintf(os.Stderr, "DEBUG: %s\n", resp.Status)
	}
	return resp, err
}
