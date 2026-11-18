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

// Package httpcl provides a minimal HTTP client with JSON defaults and retries.
// Copied from github.com/CircleCI-Public/chunk-cli/internal/httpcl.
// TODO: extract to a shared module.
package httpcl

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptrace"
	"net/url"
	"strings"
	"sync/atomic"
	"time"

	"github.com/hashicorp/go-retryablehttp"

	"github.com/CircleCI-Public/circleci-cli/clikit/iostream"
)

const jsonContentType = "application/json; charset=utf-8"

// Config configures a Client.
type Config struct {
	// BaseURL is prepended to every request route.
	BaseURL string
	// AuthToken is sent as a Bearer token unless AuthHeader is set.
	AuthToken string
	// AuthHeader overrides the header name for AuthToken (e.g. "Circle-Token", "x-api-key").
	// When set, the token is sent as the raw header value (not "Bearer ...").
	AuthHeader string
	// UserAgent sets the User-Agent header on every request.
	UserAgent string
	// Timeout is the per-request timeout. Defaults to 30s.
	Timeout time.Duration
	// DisableRetries disables automatic retries. By default a failed request
	// is retried up to 3 times with exponential backoff, subject to
	// retryPolicy: a POST or PATCH is retried only when the server cannot have
	// acted on it.
	DisableRetries bool
	// Transport overrides the HTTP transport (useful for testing).
	Transport http.RoundTripper
	// OnWarn, when non-nil, is called with a plain-text warning message when the
	// server signals endpoint removal via Deprecation or Sunset response headers.
	// The caller controls formatting (prefix, newline, colour).
	OnWarn func(msg string)
}

// Client is a simple HTTP client with JSON defaults and automatic retries.
type Client struct {
	baseURL    string
	authToken  string
	authHeader string
	userAgent  string
	timeout    time.Duration
	onWarn     func(string)
	http       *retryablehttp.Client
}

// New creates a Client from the given config.
func New(cfg Config) *Client {
	timeout := cfg.Timeout
	if timeout == 0 {
		timeout = 30 * time.Second
	}

	rc := retryablehttp.NewClient()
	rc.RetryMax = 3
	rc.CheckRetry = retryPolicy
	rc.ErrorHandler = retryablehttp.PassthroughErrorHandler
	if cfg.DisableRetries {
		rc.RetryMax = 0
	}
	rc.RetryWaitMin = 50 * time.Millisecond
	rc.RetryWaitMax = 2 * time.Second
	rc.Logger = nil // suppress default log output

	if cfg.Transport != nil {
		rc.HTTPClient.Transport = cfg.Transport
	}

	return &Client{
		baseURL:    cfg.BaseURL,
		authToken:  cfg.AuthToken,
		authHeader: cfg.AuthHeader,
		userAgent:  cfg.UserAgent,
		timeout:    timeout,
		onWarn:     cfg.OnWarn,
		http:       rc,
	}
}

// Call executes the request and returns the HTTP status code.
// Non-2xx responses return an *HTTPError. If a decoder is set and the
// response is 2xx, the response body is decoded.
func (c *Client) Call(ctx context.Context, r Request) (status int, err error) {
	start := time.Now()

	path := r.route
	if len(r.routeParams) > 0 {
		params := make([]any, len(r.routeParams))
		for i, value := range r.routeParams {
			switch v := value.(type) {
			case int, int8, int16, int32, int64, float32, float64:
				params[i] = value
			case PathSegments:
				segments := strings.Split(string(v), "/")
				for j, segment := range segments {
					// url.PathEscape leaves dot segments literal, which would let a
					// slug like "gh/../x" change the route.
					if segment == "." || segment == ".." {
						segments[j] = strings.ReplaceAll(segment, ".", "%2E")
						continue
					}
					segments[j] = url.PathEscape(segment)
				}
				params[i] = strings.Join(segments, "/")
			default:
				params[i] = url.PathEscape(fmt.Sprint(value))
			}
		}
		path = fmt.Sprintf(r.route, params...)
	}

	u, err := url.Parse(c.baseURL + path)
	if err != nil {
		return 0, fmt.Errorf("httpcl: bad url: %w", err)
	}
	if len(r.query) > 0 {
		u.RawQuery = r.query.Encode()
	}
	defer func() {
		duration := time.Since(start)
		iostream.DebugContext(ctx, r.method+" "+r.route,
			"http.request.method", r.method,
			"http.response.status_code", status,
			"duration", duration,
			"url.full", u.String(),
			"kind", "client",
		)
	}()

	var bodyReader io.Reader
	contentType := jsonContentType
	switch {
	case r.rawBody != nil:
		bodyReader = bytes.NewReader(r.rawBody)
		contentType = r.contentType
	case r.body != nil:
		b, err := json.Marshal(r.body)
		if err != nil {
			return 0, fmt.Errorf("httpcl: marshal body: %w", err)
		}
		bodyReader = bytes.NewReader(b)
	}

	if !r.noTimeout {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, c.timeout)
		defer cancel()
	}
	ctx, at := trackAttempts(ctx, r.idempotent || idempotentMethod(r.method))

	req, err := retryablehttp.NewRequestWithContext(ctx, r.method, u.String(), bodyReader)
	if err != nil {
		return 0, fmt.Errorf("httpcl: new request: %w", err)
	}

	if bodyReader != nil {
		req.Header.Set("Content-Type", contentType)
	}
	req.Header.Set("Accept", "application/json")

	if c.authToken != "" {
		if c.authHeader != "" {
			req.Header.Set(c.authHeader, c.authToken)
		} else {
			req.Header.Set("Authorization", "Bearer "+c.authToken)
		}
	}
	if c.userAgent != "" {
		req.Header.Set("User-Agent", c.userAgent)
	}

	for k, vals := range r.headers {
		for _, v := range vals {
			req.Header.Add(k, v)
		}
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return 0, at.unanswered(err)
	}
	defer func() {
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
	}()

	status = resp.StatusCode

	if status >= 200 && status < 300 {
		if c.onWarn != nil {
			checkDeprecation(c.onWarn, resp.Header)
		}
		if r.respHeader != nil {
			*r.respHeader = resp.Header
		}
		if r.decoder != nil {
			if err := r.decoder(resp.Body); err != nil {
				return status, fmt.Errorf("httpcl: decode response: %w", err)
			}
		}
		return status, nil
	}

	body, _ := io.ReadAll(resp.Body)
	return status, &HTTPError{
		Method:     r.method,
		Route:      path,
		StatusCode: status,
		Body:       body,
	}
}

// attempt is what retryPolicy and Call know about the latest attempt at a
// request.
type attempt struct {
	// replayable is whether the request is safe to send more than once.
	replayable bool
	// wrote is whether any of the request was written to the server, so it
	// may have arrived. Nothing can have arrived while it is false.
	wrote atomic.Bool
}

type attemptKey struct{}

// trackAttempts returns ctx carrying a new attempt for retryPolicy, which sees
// only the request's context. retryablehttp sends every attempt with that same
// context, so the attempt always describes the latest one.
func trackAttempts(ctx context.Context, replayable bool) (context.Context, *attempt) {
	at := &attempt{replayable: replayable}
	ctx = context.WithValue(ctx, attemptKey{}, at)
	ctx = httptrace.WithClientTrace(ctx, &httptrace.ClientTrace{
		GetConn:      func(string) { at.wrote.Store(false) }, // a new attempt starts
		WroteHeaders: func() { at.wrote.Store(true) },
	})
	return ctx, at
}

// unanswered returns err, from a request that got no response, as an
// UnknownOutcomeError when the server may have acted on it: the request is not
// safe to repeat, and some of it was written.
func (a *attempt) unanswered(err error) error {
	if a.replayable || !a.wrote.Load() {
		return err
	}
	return &UnknownOutcomeError{Err: err}
}

// idempotentMethod reports whether sending a request with this method twice
// has the same effect as sending it once (RFC 9110, section 9.2.2).
func idempotentMethod(method string) bool {
	switch method {
	case http.MethodGet, http.MethodHead, http.MethodOptions, http.MethodTrace,
		http.MethodPut, http.MethodDelete:
		return true
	}
	return false
}

// retryPolicy decides whether a failed attempt is sent again. retryablehttp's
// default policy (connection errors, 429 and 5xx) applies to a request that is
// safe to repeat, and to an attempt that failed before any of the request was
// written, such as a refused connection or a failed TLS handshake.
//
// Otherwise the server may already have acted on the request when the
// connection drops or a 5xx comes back, and for a request such as a POST that
// triggers a pipeline, sending it again could do the same thing twice. It is
// retried only when the server refused it with 429.
func retryPolicy(ctx context.Context, resp *http.Response, err error) (bool, error) {
	at, _ := ctx.Value(attemptKey{}).(*attempt)
	if at != nil && (at.replayable || (err != nil && !at.wrote.Load())) {
		return retryablehttp.DefaultRetryPolicy(ctx, resp, err)
	}
	if ctx.Err() != nil {
		return false, ctx.Err()
	}
	return err == nil && resp.StatusCode == http.StatusTooManyRequests, nil
}

func UserAgent(goos, goarch, version, agent string) string {
	if agent != "" {
		return fmt.Sprintf("circleci-cli (%s/%s; %s; %s)", goos, goarch, version, agent)
	}
	return fmt.Sprintf("circleci-cli (%s/%s; %s)", goos, goarch, version)
}

// checkDeprecation calls onWarn when the response carries Deprecation or Sunset
// headers signalling that the endpoint will be removed. The message is plain
// text — no prefix or newline — so the caller controls formatting.
func checkDeprecation(onWarn func(string), h http.Header) {
	dep := h.Get("Deprecation")
	sunset := h.Get("Sunset")
	if dep == "" && sunset == "" {
		return
	}
	if sunset != "" {
		if t, err := http.ParseTime(sunset); err == nil {
			days := int(time.Until(t).Hours() / 24)
			if days > 0 {
				onWarn(fmt.Sprintf("this API endpoint is deprecated and will be removed in %d days — upgrade circleci CLI", days))
			} else {
				onWarn("this API endpoint is deprecated and removal is imminent — upgrade circleci CLI")
			}
		} else {
			onWarn(fmt.Sprintf("this API endpoint is deprecated and will be removed on %s — upgrade circleci CLI", sunset))
		}
	} else {
		onWarn("this API endpoint is deprecated and will be removed — upgrade circleci CLI")
	}
}
