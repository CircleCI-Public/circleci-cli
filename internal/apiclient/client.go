// Copyright (c) 2026 Circle Internet Services, Inc.
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.
//
// SPDX-License-Identifier: MIT

// Package apiclient provides a thin HTTP client for the CircleCI REST API.
package apiclient

import (
	"context"
	"net/http"
	"runtime"
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

// DeviceID is set once at startup (in PersistentPreRunE, after --config is
// parsed) and embedded in the User-Agent header as:
//
//	circleci-cli (<os>; <device-id>)
//
// The device ID is a stable per-machine UUID persisted in the config file.
var DeviceID string

// New creates a Client. baseURL should be the CircleCI host, e.g. "https://circleci.com".
// An http.RoundTripper can be injected for testing. Set CIRCLECI_DEBUG=1 to log
// all HTTP requests and response status codes to stderr.
func New(baseURL, token string, transport http.RoundTripper) *Client {
	if transport == nil {
		transport = http.DefaultTransport
	}

	cfg := httpcl.Config{
		AuthToken:  token,
		AuthHeader: "Circle-Token",
		UserAgent:  "circleci-cli (" + runtime.GOOS + "; " + DeviceID + ")",
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

func queryParam(key, val string) func(*httpcl.Request) {
	return httpcl.QueryParam(key, val)
}

func optionalQueryParam(key, val string) func(*httpcl.Request) {
	return httpcl.OptionalQueryParam(key, val)
}

func routeParams(v ...any) func(*httpcl.Request) {
	return httpcl.RouteParams(v...)
}

func (c *Client) get(ctx context.Context, route string, dst any, opts ...func(*httpcl.Request)) error {
	_, err := c.main.Call(ctx, httpcl.NewRequest(http.MethodGet, "/api/v2"+route, baseOpts(
		httpcl.JSONDecoder(dst),
	).With(opts)...))
	return err
}

func (c *Client) getV1(ctx context.Context, route string, dst any, opts ...func(*httpcl.Request)) error {
	_, err := c.main.Call(ctx, httpcl.NewRequest(http.MethodGet, "/api/v1.1"+route, baseOpts(
		httpcl.JSONDecoder(dst),
	).With(opts)...))
	return err
}

func (c *Client) post(ctx context.Context, route string, body, dst any, opts ...func(*httpcl.Request)) error {
	_, err := c.main.Call(ctx, httpcl.NewRequest(http.MethodPost, "/api/v2"+route, baseOpts(
		httpcl.Body(body),
		httpcl.JSONDecoder(dst),
	).With(opts)...))
	return err
}

func (c *Client) postV1(ctx context.Context, route string, body, dst any, opts ...func(*httpcl.Request)) error {
	_, err := c.main.Call(ctx, httpcl.NewRequest(http.MethodPost, "/api/v1.1"+route, baseOpts(
		httpcl.Body(body),
		httpcl.JSONDecoder(dst),
	).With(opts)...))
	return err
}

func (c *Client) put(ctx context.Context, route string, body, dst any, opts ...func(*httpcl.Request)) error {
	_, err := c.main.Call(ctx, httpcl.NewRequest(http.MethodPut, "/api/v2"+route, baseOpts(
		httpcl.Body(body),
		httpcl.JSONDecoder(dst),
	).With(opts)...))
	return err
}

func (c *Client) deleteV2(ctx context.Context, route string, opts ...func(*httpcl.Request)) error {
	_, err := c.main.Call(ctx, httpcl.NewRequest(http.MethodDelete, "/api/v2"+route, opts...))
	return err
}

func (c *Client) getRunner(ctx context.Context, route string, dst any, opts ...func(*httpcl.Request)) error {
	_, err := c.runner.Call(ctx, httpcl.NewRequest(http.MethodGet, "/api/v3"+route, baseOpts(
		httpcl.JSONDecoder(dst),
	).With(opts)...))
	return err
}

func (c *Client) postRunner(ctx context.Context, path string, body, dst any, opts ...func(*httpcl.Request)) error {
	_, err := c.runner.Call(ctx, httpcl.NewRequest(http.MethodPost, "/api/v3"+path, baseOpts(
		httpcl.Body(body),
		httpcl.JSONDecoder(dst),
	).With(opts)...))
	return err
}

func (c *Client) deleteRunner(ctx context.Context, route string, opts ...func(*httpcl.Request)) error {
	_, err := c.runner.Call(ctx, httpcl.NewRequest(http.MethodDelete, "/api/v3"+route, opts...))
	return err
}

type baseOptions []func(*httpcl.Request)

func baseOpts(opts ...func(*httpcl.Request)) baseOptions {
	return opts
}

func (o baseOptions) With(opts []func(*httpcl.Request)) []func(*httpcl.Request) {
	return append(o, opts...)
}
