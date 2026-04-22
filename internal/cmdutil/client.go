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

// Package cmdutil provides shared helpers used by command packages.
// Business logic belongs in internal/<domain>/; this package is for
// command-layer plumbing that would otherwise be copy-pasted.
package cmdutil

import (
	"fmt"
	"net/http"

	"github.com/CircleCI-Public/circleci-cli-v2/internal/apiclient"
	"github.com/CircleCI-Public/circleci-cli-v2/internal/config"
	clierrors "github.com/CircleCI-Public/circleci-cli-v2/internal/errors"
	"github.com/CircleCI-Public/circleci-cli-v2/internal/httpcl"
)

// LoadClient reads the CLI config, validates that a token is present, and
// returns an authenticated API client. On failure it returns a structured
// CLIError ready to be returned directly from a RunE handler.
func LoadClient() (*apiclient.Client, *clierrors.CLIError) {
	cfg, err := config.Load()
	if err != nil {
		return nil, clierrors.New("config.load_failed", "Failed to load config", err.Error()).
			WithExitCode(clierrors.ExitGeneralError)
	}
	token := cfg.EffectiveToken()
	if token == "" {
		return nil, clierrors.New("auth.token_missing", "Authentication required",
			"No CircleCI API token found.").
			WithSuggestions(
				"Run: circleci settings set token <your-token>",
				"Or set the CIRCLECI_TOKEN environment variable",
			).
			WithRef("https://app.circleci.com/settings/user/tokens").
			WithExitCode(clierrors.ExitAuthError)
	}
	return apiclient.New(cfg.EffectiveHost(), token, nil), nil
}

// APIErr converts an apiclient error into a structured CLIError.
//
// notFoundCode and notFoundMsg customise the 404 case for the calling resource
// (e.g. "pipeline.not_found", "No pipeline found for %q"). notFoundMsg is
// passed through fmt.Sprintf with subject as the single argument.
//
// Optional notFoundSuggestions are appended to the 404 error (useful for
// pointing users toward a list command, for example).
func APIErr(err error, subject, notFoundCode, notFoundMsg string, notFoundSuggestions ...string) *clierrors.CLIError {
	if httpcl.HasStatusCode(err, http.StatusUnauthorized) {
		return clierrors.New("auth.token_invalid", "Authentication failed",
			"The API token was rejected by CircleCI.").
			WithSuggestions("Run: circleci settings set token <your-token>").
			WithRef("https://app.circleci.com/settings/user/tokens").
			WithExitCode(clierrors.ExitAuthError)
	}
	if httpcl.HasStatusCode(err, http.StatusNotFound) {
		nf := clierrors.New(notFoundCode, "Not found",
			fmt.Sprintf(notFoundMsg, subject)).
			WithExitCode(clierrors.ExitNotFound)
		if len(notFoundSuggestions) > 0 {
			nf = nf.WithSuggestions(notFoundSuggestions...)
		}
		return nf
	}
	if he, ok := err.(*httpcl.HTTPError); ok {
		return clierrors.New("api.error", "CircleCI API error",
			fmt.Sprintf("API returned %d: %s", he.StatusCode, string(he.Body))).
			WithExitCode(clierrors.ExitAPIError)
	}
	return clierrors.New("api.error", "CircleCI API error", err.Error()).
		WithExitCode(clierrors.ExitAPIError)
}
