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
	"os"
	"path"
	"strings"

	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"

	"github.com/CircleCI-Public/circleci-cli/internal/apiclient"
	"github.com/CircleCI-Public/circleci-cli/internal/cmdutil"
	clierrors "github.com/CircleCI-Public/circleci-cli/internal/errors"
	"github.com/CircleCI-Public/circleci-cli/internal/gitremote"
	"github.com/CircleCI-Public/circleci-cli/internal/httpcl"
	"github.com/CircleCI-Public/circleci-cli/internal/iostream"
)

// NewAPICmd returns the "circleci api" command.
func NewAPICmd() *cobra.Command {
	var (
		method  string
		fields  []string
		headers []string
		data    string
	)

	cmd := &cobra.Command{
		Use:   "api <path>",
		Short: "Call the CircleCI REST API directly",
		Annotations: map[string]string{
			"help:arguments": heredoc.Docf(`
				%[1]s<path>%[1]s is the request path. It is relative to /api/v3 by default
				(for example, "projects/{project-id}"). To target
				a different version prefix, include it explicitly, for example, "api/v2/me".
			`, "`"),
		},
		Long: heredoc.Docf(`
			Make an authenticated HTTP request to the REST APIs and print
			the raw response body.

			%[1]s<path>%[1]s is relative to /api/v3 (e.g. %[1]sprojects/{project-id}%[1]s). To target
			a different version prefix, include it explicitly:
			  circleci api api/v2/me

			The Authorization: Bearer header is added automatically from your stored token.
			Use -H to add extra headers and -f to set fields (query parameters for
			GET/DELETE, JSON body fields for POST/PUT/PATCH).

			To send a raw request body instead of building one from -f fields, use
			-d/--data. The value is sent verbatim; @file reads the body from a file and
			@- reads it from stdin. -d and -f cannot be combined. When -d is given the
			default method is POST.

			Exit code reflects the HTTP response: 0 for 2xx, 4 for 4xx/5xx.
		`, "`"),
		Example: heredoc.Doc(`
			# Get your user profile
			$ circleci api api/v2/me

			# Get a project
			$ circleci api projects/{project-id}

			# List runs for a project
			$ circleci api 'runs?filter[project_id]={project-id}'

			# List my runs
			$ circleci api 'runs?filter[user_id]=me'

			# Trigger a pipeline on a branch
			$ circleci api -X POST project/gh/myorg/myrepo/pipeline -f branch=main

			# Send a raw JSON body
			$ circleci api -X POST project/gh/myorg/myrepo/pipeline -d '{"branch":"main"}'

			# Send a body read from a file (@- reads from stdin)
			$ circleci api -X POST project/gh/myorg/myrepo/pipeline -d @body.json

			# Add a custom header
			$ circleci api -H "Accept: application/json" api/v2/me

			# Access the v1.1 API
			$ circleci api api/v1.1/me
		`),
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()

			client, cliErr := cmdutil.LoadClient(ctx)
			if cliErr != nil {
				return cliErr
			}
			return run(ctx, client, args[0], method, fields, headers, data)
		},
	}

	cmd.Flags().StringVarP(&method, "method", "X", "", "HTTP method (default: GET, or POST when -f or -d is used)")
	cmd.Flags().StringArrayVarP(&fields, "field", "f", nil, "Add a field: key=value (query param for GET/DELETE, JSON body for POST/PUT/PATCH)")
	cmd.Flags().StringArrayVarP(&headers, "header", "H", nil, "Add a request header: \"Key: Value\"")
	cmd.Flags().StringVarP(&data, "data", "d", "", "Raw request body sent verbatim; @file reads from a file, @- from stdin")
	cmdutil.AddJQFlag(cmd)

	return cmd
}

func run(ctx context.Context, client *apiclient.Client, thepath, method string, fields, headers []string, data string) error {
	hasData := data != ""

	// -d and -f build the body in incompatible ways, so reject the combination.
	if hasData && len(fields) > 0 {
		return clierrors.New("api.data_and_fields", "Cannot combine -d and -f",
			"Use -d to send a raw body or -f to build one from fields, not both.").
			WithExitCode(clierrors.ExitBadArguments)
	}

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

	// Default method: POST when a body is provided (mirrors gh api), otherwise GET.
	if method == "" {
		if len(parsedFields) > 0 || hasData {
			method = http.MethodPost
		} else {
			method = http.MethodGet
		}
	}
	method = strings.ToUpper(method)

	thepath = strings.TrimPrefix(thepath, "/")

	// Default to /api/v3 when no version prefix is given.
	if !strings.HasPrefix(thepath, "api") {
		thepath = path.Join("api/v3", thepath)
	}

	thepath = "/" + thepath

	// Resolve {project-id}/{org-id} placeholders from the current git
	// repository, but only when the path actually uses them — otherwise every
	// api call would pay for a git detect and an extra API round-trip.
	if strings.Contains(thepath, "{project-id}") || strings.Contains(thepath, "{org-id}") {
		info, err := gitremote.Detect()
		if err != nil {
			return cmdutil.GitDetectErr(err, "Or pass the IDs explicitly in the path instead of {project-id}/{org-id}.")
		}
		proj, err := client.GetProjectBySlug(ctx, info.Slug)
		if err != nil {
			return clierrors.New("api.project_lookup_failed", "Could not resolve project",
				fmt.Sprintf("Failed to look up project %q: %v", info.Slug, err)).
				WithExitCode(clierrors.ExitAPIError)
		}
		r := strings.NewReplacer(
			"{project-id}", proj.ID.String(),
			"{org-id}", proj.OrgID.String(),
		)
		thepath = r.Replace(thepath)
	}

	var respBody []byte
	opts := []func(*httpcl.Request){
		httpcl.Header("Accept", "application/json"),
		httpcl.BytesDecoder(&respBody),
	}

	// A raw -d body is sent verbatim regardless of method. Otherwise:
	//   GET/DELETE: fields become query parameters.
	//   writes:     fields become a JSON body.
	switch {
	case hasData:
		body, err := resolveBody(ctx, data)
		if err != nil {
			return err
		}
		// httpcl sets Content-Type: application/json automatically when a body
		// is present. json.RawMessage marshals to itself, so the body is sent
		// verbatim rather than re-encoded as a JSON string.
		opts = append(opts, httpcl.Body(json.RawMessage(body)))
	case method == http.MethodGet || method == http.MethodDelete:
		if len(parsedFields) > 0 {
			// Use url.Values so keys and values are percent-encoded.  Raw
			// concatenation (k+"="+v) would allow a value containing "&" or
			// "=" to inject extra query parameters.
			q := url.Values{}
			for k, v := range parsedFields {
				q.Set(k, v)
			}
			sep := "?"
			if strings.Contains(thepath, "?") {
				sep = "&"
			}
			thepath += sep + q.Encode()
		}
	default:
		if len(parsedFields) > 0 {
			b, err := json.Marshal(parsedFields)
			if err != nil {
				return clierrors.New("api.marshal_failed", "Failed to encode fields", err.Error()).
					WithExitCode(clierrors.ExitGeneralError)
			}
			opts = append(opts, httpcl.Body(json.RawMessage(b)))
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
		opts = append(opts,
			httpcl.Header(strings.TrimSpace(k), strings.TrimSpace(v)),
		)
	}

	status, err := client.Do(ctx, method, thepath, opts...)
	if err != nil {
		return clierrors.New("api.request_failed", "Request failed", err.Error()).
			WithExitCode(clierrors.ExitAPIError)
	}

	// At the moment we are assuming all API calls return plain JSON.
	err = iostream.PrintJSONFromReader(ctx, bytes.NewReader(respBody))
	if err != nil {
		return err
	}

	if status >= 400 {
		return clierrors.New("api.error_status",
			fmt.Sprintf("HTTP %d", status),
			fmt.Sprintf("The API returned status %d.", status)).
			WithExitCode(clierrors.ExitAPIError)
	}
	return nil
}

// resolveBody returns the raw request body for a -d/--data value. A literal
// string is used verbatim; @file reads from a file and @- reads from stdin.
// The result must be valid JSON, since it is sent with Content-Type: application/json.
func resolveBody(ctx context.Context, data string) ([]byte, error) {
	var body []byte

	switch {
	case data == "@-":
		b, err := io.ReadAll(iostream.In(ctx))
		if err != nil {
			return nil, clierrors.New("api.data_read_failed", "Failed to read body from stdin", err.Error()).
				WithExitCode(clierrors.ExitGeneralError)
		}
		body = b
	case strings.HasPrefix(data, "@"):
		file := data[1:]
		b, err := os.ReadFile(file) //#nosec:G304 // file is a user-supplied -d @file flag value, not arbitrary external input
		if err != nil {
			return nil, clierrors.New("api.data_read_failed", "Failed to read body file",
				fmt.Sprintf("Could not read %q: %v", file, err)).
				WithExitCode(clierrors.ExitBadArguments)
		}
		body = b
	default:
		body = []byte(data)
	}

	if !json.Valid(body) {
		return nil, clierrors.New("api.invalid_json_body", "Invalid JSON body",
			"The request body is not valid JSON. Use -d to pass a JSON document or @file/@- to read one.").
			WithExitCode(clierrors.ExitBadArguments)
	}
	return body, nil
}
