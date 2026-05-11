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
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"

	"github.com/CircleCI-Public/circleci-cli-v2/internal/httpcl"
)

// gqlOpNameRe extracts the operation name from a named query or mutation.
var gqlOpNameRe = regexp.MustCompile(`(?:query|mutation)\s+(\w+)`)

// GQLError is returned when a GraphQL response contains application-level errors
// in the top-level errors[] array or in a mutation's inline errors field.
type GQLError struct {
	messages []string
}

func (e *GQLError) Error() string {
	return strings.Join(e.messages, "; ")
}

// graphQL executes a GraphQL query or mutation against /graphql-unstable.
// On a non-200 HTTP status it returns an *httpcl.HTTPError so the caller
// can inspect the status code (e.g. 401 Unauthorized) like any REST call.
func (c *Client) graphQL(ctx context.Context, query string, variables map[string]any, data any) error {
	payload := map[string]any{
		"query":     query,
		"variables": variables,
	}
	if m := gqlOpNameRe.FindStringSubmatch(query); len(m) > 1 {
		payload["operationName"] = m[1]
	}
	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(payload); err != nil {
		return fmt.Errorf("encoding GraphQL request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/graphql-unstable", &buf)
	if err != nil {
		return fmt.Errorf("building GraphQL request: %w", err)
	}
	req.Header.Set("Circle-Token", c.token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := c.raw.Do(req)
	if err != nil {
		return fmt.Errorf("GraphQL request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("reading GraphQL response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return &httpcl.HTTPError{
			Method:     http.MethodPost,
			Route:      "/graphql-unstable",
			StatusCode: resp.StatusCode,
			Body:       b,
		}
	}

	var envelope struct {
		Data   json.RawMessage `json:"data"`
		Errors []struct {
			Message string `json:"message"`
		} `json:"errors"`
	}
	if err := json.Unmarshal(b, &envelope); err != nil {
		return fmt.Errorf("decoding GraphQL response: %w", err)
	}
	if len(envelope.Errors) > 0 {
		msgs := make([]string, len(envelope.Errors))
		for i, e := range envelope.Errors {
			msgs[i] = e.Message
		}
		return &GQLError{messages: msgs}
	}
	if data != nil && len(envelope.Data) > 0 {
		return json.Unmarshal(envelope.Data, data)
	}
	return nil
}
