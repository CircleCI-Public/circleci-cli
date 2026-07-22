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
	"fmt"
	"net/http"
	"runtime"

	"github.com/CircleCI-Public/circleci-cli/internal/httpcl"
)

// Client is an authenticated CircleCI API client.
type Client struct {
	main  *httpcl.Client // circleci.com/api/v1.1, /api/v2
	raw   *httpcl.Client
	token string
}

type Config struct {
	BaseURL string
	Token   string
	Version string
	Agent   string

	Transport http.RoundTripper
	// OnWarn, when non-nil, is called with a plain-text deprecation warning.
	// See httpcl.Config.OnWarn for details.
	OnWarn func(msg string)
}

// New creates a Client. baseURL should be the CircleCI host, e.g. "https://circleci.com".
// An http.RoundTripper can be injected for testing. Set CIRCLE_DEBUG=1 to log
// all HTTP requests and response status codes to stderr.
func New(cfg Config) *Client {
	if cfg.Transport == nil {
		cfg.Transport = http.DefaultTransport
	}

	baseCfg := httpcl.Config{
		AuthToken:  "Bearer " + cfg.Token,
		AuthHeader: "Authorization",
		UserAgent:  httpcl.UserAgent(runtime.GOOS, runtime.GOARCH, cfg.Version, cfg.Agent),
		Transport:  cfg.Transport,
		OnWarn:     cfg.OnWarn,
	}

	mainCfg := baseCfg
	mainCfg.BaseURL = cfg.BaseURL

	return &Client{
		main:  httpcl.New(mainCfg),
		raw:   httpcl.New(baseCfg),
		token: cfg.Token,
	}
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

func (c *Client) postStatus(ctx context.Context, route string, body, dst any, opts ...func(*httpcl.Request)) (int, error) {
	return c.main.Call(ctx, httpcl.NewRequest(http.MethodPost, "/api/v2"+route, baseOpts(
		httpcl.Body(body),
		httpcl.JSONDecoder(dst),
	).With(opts)...))
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

func (c *Client) patch(ctx context.Context, route string, body, dst any, opts ...func(*httpcl.Request)) error {
	_, err := c.main.Call(ctx, httpcl.NewRequest(http.MethodPatch, "/api/v2"+route, baseOpts(
		httpcl.Body(body),
		httpcl.JSONDecoder(dst),
	).With(opts)...))
	return err
}

func (c *Client) deleteV2(ctx context.Context, route string, opts ...func(*httpcl.Request)) error {
	_, err := c.main.Call(ctx, httpcl.NewRequest(http.MethodDelete, "/api/v2"+route, opts...))
	return err
}

type v3Entity[T any] struct {
	Data T `json:"data"`
}

type v3List[T any] struct {
	Data []T `json:"data"`
	Page struct {
		Next *string `json:"next"`
		Prev *string `json:"prev"`
	} `json:"page"`
}

// pageLimit returns a request option that sets page[limit].
// A limit of 0 is ignored (server default is used).
func pageLimit(n int) func(*httpcl.Request) {
	if n <= 0 {
		return func(*httpcl.Request) {}
	}
	return queryParam("page[limit]", fmt.Sprintf("%d", n))
}

// pageCursor returns a request option that sets page[cursor].
// An empty cursor is ignored (first page).
func pageCursor(cursor string) func(*httpcl.Request) {
	return optionalQueryParam("page[cursor]", cursor)
}

// filterParam returns a request option that sets filter[key]=val.
// An empty val is ignored.
func filterParam(key, val string) func(*httpcl.Request) {
	return optionalQueryParam("filter["+key+"]", val)
}

func (c *Client) getV3(ctx context.Context, route string, dst any, opts ...func(*httpcl.Request)) error {
	_, err := c.main.Call(ctx, httpcl.NewRequest(http.MethodGet, "/api/v3"+route, baseOpts(
		httpcl.JSONDecoder(dst),
	).With(opts)...))
	return err
}

func (c *Client) getV3String(ctx context.Context, route string, dst *string, opts ...func(*httpcl.Request)) error {
	_, err := c.main.Call(ctx, httpcl.NewRequest(http.MethodGet, "/api/v3"+route, baseOpts(
		httpcl.StringDecoder(dst),
	).With(opts)...))
	return err
}

func (c *Client) postV3(ctx context.Context, route string, body, dst any, opts ...func(*httpcl.Request)) error {
	_, err := c.main.Call(ctx, httpcl.NewRequest(http.MethodPost, "/api/v3"+route, baseOpts(
		httpcl.Body(body),
		httpcl.JSONDecoder(dst),
	).With(opts)...))
	return err
}

func (c *Client) deleteV3(ctx context.Context, route string, opts ...func(*httpcl.Request)) error {
	_, err := c.main.Call(ctx, httpcl.NewRequest(http.MethodDelete, "/api/v3"+route, opts...))
	return err
}

type baseOptions []func(*httpcl.Request)

func baseOpts(opts ...func(*httpcl.Request)) baseOptions {
	return opts
}

func (o baseOptions) With(opts []func(*httpcl.Request)) []func(*httpcl.Request) {
	return append(o, opts...)
}
