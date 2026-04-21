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
	"net/url"
	"time"

	"github.com/hashicorp/go-retryablehttp"
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
	// DisableRetries disables automatic retries. By default requests are
	// retried up to 3 times with exponential backoff.
	DisableRetries bool
	// Transport overrides the HTTP transport (useful for testing).
	Transport http.RoundTripper
}

// Client is a simple HTTP client with JSON defaults and automatic retries.
type Client struct {
	baseURL    string
	authToken  string
	authHeader string
	userAgent  string
	timeout    time.Duration
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
		http:       rc,
	}
}

// Call executes the request and returns the HTTP status code.
// Non-2xx responses return an *HTTPError. If a decoder is set and the
// response is 2xx, the response body is decoded.
func (c *Client) Call(ctx context.Context, r Request) (int, error) {
	u, err := url.Parse(c.baseURL + r.route)
	if err != nil {
		return 0, fmt.Errorf("httpcl: bad url: %w", err)
	}
	if len(r.query) > 0 {
		u.RawQuery = r.query.Encode()
	}

	var bodyReader io.Reader
	if r.body != nil {
		b, err := json.Marshal(r.body)
		if err != nil {
			return 0, fmt.Errorf("httpcl: marshal body: %w", err)
		}
		bodyReader = bytes.NewReader(b)
	}

	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	req, err := retryablehttp.NewRequestWithContext(ctx, r.method, u.String(), bodyReader)
	if err != nil {
		return 0, fmt.Errorf("httpcl: new request: %w", err)
	}

	if r.body != nil {
		req.Header.Set("Content-Type", jsonContentType)
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
		return 0, err
	}
	defer func() {
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
	}()

	status := resp.StatusCode

	if status >= 200 && status < 300 {
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
		Route:      r.route,
		StatusCode: status,
		Body:       body,
	}
}
