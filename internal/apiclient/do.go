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
