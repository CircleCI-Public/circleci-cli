package apiclient

import (
	"context"
	"fmt"
	"io"
	"net/http"
)

// Do makes a raw authenticated request to the CircleCI API and returns the
// HTTP status code and raw response body. It is intended for the "circleci api"
// escape-hatch command and should not be used by typed command packages.
//
// path must be an absolute path including the API version prefix
// (e.g. "/api/v2/project/..."). The Circle-Token header is added automatically;
// callers may supply additional headers via extraHeaders.
//
// Non-2xx status codes do NOT return an error — the caller is responsible for
// inspecting the status code and formatting the output accordingly.
func (c *Client) Do(ctx context.Context, method, path string, extraHeaders http.Header, body io.Reader) (int, []byte, error) {
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, body)
	if err != nil {
		return 0, nil, fmt.Errorf("building request: %w", err)
	}
	req.Header.Set("Circle-Token", c.token)
	req.Header.Set("Accept", "application/json")
	for k, vals := range extraHeaders {
		for _, v := range vals {
			req.Header.Add(k, v)
		}
	}

	resp, err := c.raw.Do(req)
	if err != nil {
		return 0, nil, fmt.Errorf("request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return resp.StatusCode, nil, fmt.Errorf("reading response: %w", err)
	}
	return resp.StatusCode, b, nil
}
