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
	"fmt"
	"net/http"
	"runtime"

	"github.com/CircleCI-Public/circleci-cli/internal/httpcl"
)

// Client is a CircleCI API client. It is authenticated when Config.Token was
// set; see Authenticated.
type Client struct {
	// main is based at the CircleCI host, so requests pass the full path
	// including the version prefix: /api/v1.1, /api/v2 or /api/v3.
	main *httpcl.Client
	// raw has no base URL, for absolute URLs the API hands us (e.g. artifacts).
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
		AuthHeader: "Authorization",
		UserAgent:  httpcl.UserAgent(runtime.GOOS, runtime.GOARCH, cfg.Version, cfg.Agent),
		Transport:  cfg.Transport,
		OnWarn:     cfg.OnWarn,
	}
	// Leave AuthToken empty for an anonymous client so httpcl omits the header
	// entirely. Sending a valueless "Authorization: Bearer" instead makes the API
	// reject the request as malformed credentials rather than treating it as
	// unauthenticated, which breaks the endpoints that do serve anonymous callers
	// (see cmdutil.LoadClientOptionalAuth).
	if cfg.Token != "" {
		baseCfg.AuthToken = "Bearer " + cfg.Token
	}

	mainCfg := baseCfg
	mainCfg.BaseURL = cfg.BaseURL

	return &Client{
		main:  httpcl.New(mainCfg),
		raw:   httpcl.New(baseCfg),
		token: cfg.Token,
	}
}

// Authenticated reports whether the client carries an API token.
func (c *Client) Authenticated() bool { return c.token != "" }

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
	return httpcl.QueryParam("page[limit]", fmt.Sprintf("%d", n))
}

// pageCursor returns a request option that sets page[cursor].
// An empty cursor is ignored (first page).
func pageCursor(cursor string) func(*httpcl.Request) {
	return httpcl.OptionalQueryParam("page[cursor]", cursor)
}

// filterParam returns a request option that sets filter[key]=val.
// An empty val is ignored.
func filterParam(key, val string) func(*httpcl.Request) {
	return httpcl.OptionalQueryParam("filter["+key+"]", val)
}
