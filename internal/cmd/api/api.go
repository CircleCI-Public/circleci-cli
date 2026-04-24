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

// Package api implements the "circleci api" escape-hatch command for making
// raw authenticated requests to the CircleCI REST API.
package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"

	"github.com/CircleCI-Public/circleci-cli-v2/internal/apiclient"
	"github.com/CircleCI-Public/circleci-cli-v2/internal/cmdutil"
	clierrors "github.com/CircleCI-Public/circleci-cli-v2/internal/errors"
	"github.com/CircleCI-Public/circleci-cli-v2/internal/iostream"
)

// NewAPICmd returns the "circleci api" command.
func NewAPICmd() *cobra.Command {
	var (
		method  string
		fields  []string
		headers []string
		jsonOut bool
	)

	cmd := &cobra.Command{
		Use:   "api <path>",
		Short: "Make an authenticated request to the CircleCI API",
		Long: heredoc.Doc(`
			Make an authenticated HTTP request to the CircleCI REST API and print
			the raw response body.

			<path> is relative to /api/v2 (e.g. /project/gh/org/repo). To target
			a different version prefix, include it explicitly:
			  circleci api /api/v1.1/me

			The Circle-Token header is added automatically from your stored token.
			Use -H to add extra headers and -f to set fields (query parameters for
			GET/DELETE, JSON body fields for POST/PUT/PATCH).

			Exit code reflects the HTTP response: 0 for 2xx, 4 for 4xx/5xx.
		`),
		Example: heredoc.Doc(`
			# Get your user profile
			$ circleci api /me

			# Get a project
			$ circleci api /project/gh/myorg/myrepo

			# List pipelines for a project (pretty-printed)
			$ circleci api /project/gh/myorg/myrepo/pipeline --json

			# Trigger a pipeline on a branch
			$ circleci api -X POST /project/gh/myorg/myrepo/pipeline -f branch=main

			# Add a custom header
			$ circleci api -H "Accept: application/json" /me

			# Access the v1.1 API
			$ circleci api /api/v1.1/me
		`),
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := iostream.FromCmd(cmd.Context(), cmd)

			client, cliErr := cmdutil.LoadClient(ctx, cmd)
			if cliErr != nil {
				return cliErr
			}
			return run(ctx, client, args[0], method, fields, headers, jsonOut)
		},
	}

	cmd.Flags().StringVarP(&method, "method", "X", "", "HTTP method (default: GET, or POST when -f is used)")
	cmd.Flags().StringArrayVarP(&fields, "field", "f", nil, "Add a field: key=value (query param for GET/DELETE, JSON body for POST/PUT/PATCH)")
	cmd.Flags().StringArrayVarP(&headers, "header", "H", nil, "Add a request header: \"Key: Value\"")
	cmd.Flags().BoolVar(&jsonOut, "json", false, "Pretty-print the response as JSON")

	return cmd
}

func run(ctx context.Context, client *apiclient.Client, path, method string, fields, headers []string, jsonOut bool) error {
	// Parse key=value fields.
	parsedFields := make(map[string]string, len(fields))
	for _, f := range fields {
		k, v, ok := strings.Cut(f, "=")
		if !ok {
			return clierrors.New("api.invalid_field", "Invalid field format",
				fmt.Sprintf("%q is not in key=value format.", f)).
				WithExitCode(clierrors.ExitBadArguments)
		}
		parsedFields[k] = v
	}

	// Default method: POST when fields are provided (mirrors gh api), otherwise GET.
	if method == "" {
		if len(parsedFields) > 0 {
			method = http.MethodPost
		} else {
			method = http.MethodGet
		}
	}
	method = strings.ToUpper(method)

	// Default to /api/v2 when no version prefix is given.
	if !strings.HasPrefix(path, "/api/") {
		path = "/api/v2" + path
	}

	// For GET/DELETE: fields become query parameters.
	// For writes: fields become a JSON body.
	extraHeaders := make(http.Header)
	var body io.Reader

	switch method {
	case http.MethodGet, http.MethodDelete:
		if len(parsedFields) > 0 {
			// Use url.Values so keys and values are percent-encoded.  Raw
			// concatenation (k+"="+v) would allow a value containing "&" or
			// "=" to inject extra query parameters.
			q := url.Values{}
			for k, v := range parsedFields {
				q.Set(k, v)
			}
			sep := "?"
			if strings.Contains(path, "?") {
				sep = "&"
			}
			path += sep + q.Encode()
		}
	default:
		if len(parsedFields) > 0 {
			b, err := json.Marshal(parsedFields)
			if err != nil {
				return clierrors.New("api.marshal_failed", "Failed to encode fields", err.Error()).
					WithExitCode(clierrors.ExitGeneralError)
			}
			body = bytes.NewReader(b)
			extraHeaders.Set("Content-Type", "application/json")
		}
	}

	// Parse -H "Key: Value" header strings.
	for _, h := range headers {
		k, v, ok := strings.Cut(h, ":")
		if !ok {
			return clierrors.New("api.invalid_header", "Invalid header format",
				fmt.Sprintf("%q is not in \"Key: Value\" format.", h)).
				WithExitCode(clierrors.ExitBadArguments)
		}
		extraHeaders.Add(strings.TrimSpace(k), strings.TrimSpace(v))
	}

	status, respBody, err := client.Do(ctx, method, path, extraHeaders, body)
	if err != nil {
		return clierrors.New("api.request_failed", "Request failed", err.Error()).
			WithExitCode(clierrors.ExitAPIError)
	}

	// Print the response body. --json pretty-prints if the body is valid JSON.
	output := strings.TrimRight(string(respBody), "\n")
	if jsonOut {
		var v any
		if jsonErr := json.Unmarshal(respBody, &v); jsonErr == nil {
			_ = cmdutil.WriteJSON(iostream.Out(ctx), v)
		} else {
			iostream.Println(ctx, output)
		}
	} else {
		iostream.Println(ctx, output)
	}

	if status >= 400 {
		return clierrors.New("api.error_status",
			fmt.Sprintf("HTTP %d", status),
			fmt.Sprintf("The API returned status %d.", status)).
			WithExitCode(clierrors.ExitAPIError)
	}
	return nil
}
