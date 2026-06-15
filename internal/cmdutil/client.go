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
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"

	"github.com/spf13/cobra"

	"github.com/CircleCI-Public/circleci-cli/internal/apiclient"
	"github.com/CircleCI-Public/circleci-cli/internal/config"
	clierrors "github.com/CircleCI-Public/circleci-cli/internal/errors"
	"github.com/CircleCI-Public/circleci-cli/internal/httpcl"
	"github.com/CircleCI-Public/circleci-cli/internal/telemetry"
)

type versionKey struct{}

func WithVersion(ctx context.Context, version string) context.Context {
	return context.WithValue(ctx, versionKey{}, version)
}

func GetVersion(ctx context.Context) string {
	v := ctx.Value(versionKey{})
	if v == nil {
		panic("no version")
	}
	return v.(string)
}

type agentNameKey struct{}

func WithAgentName(ctx context.Context, name string) context.Context {
	return context.WithValue(ctx, agentNameKey{}, name)
}

func GetAgentName(ctx context.Context) string {
	v := ctx.Value(agentNameKey{})
	if v == nil {
		panic("no agent name")
	}
	return v.(string)
}

func IsSecureStorage(cmd *cobra.Command) bool {
	insecureStorage, _ := cmd.Root().Flags().GetBool("insecure-storage")
	secureStorage := !insecureStorage
	return secureStorage
}

type configKey struct{}

// WithConfig returns a copy of ctx carrying the given config file path.
// The path is read by LoadClient to locate the config file.
func WithConfig(ctx context.Context, cfg *config.Config) context.Context {
	return context.WithValue(ctx, configKey{}, cfg)
}

type telemetryKey struct{}

func WithTelemetry(ctx context.Context, tc *telemetry.Sender) context.Context {
	return context.WithValue(ctx, telemetryKey{}, tc)
}

func GetTelemetry(ctx context.Context) *telemetry.Sender {
	v := ctx.Value(telemetryKey{})
	if v == nil {
		return nil
	}
	return v.(*telemetry.Sender)
}

func CheckTelemetry(ctx context.Context) bool {
	v := ctx.Value(telemetryKey{})
	return v != nil
}

func GetConfig(ctx context.Context) *config.Config {
	v := ctx.Value(configKey{})
	if v == nil {
		panic("no config")
	}
	return v.(*config.Config)
}

// LoadClient reads the CLI config, validates that a token is present, and
// returns an authenticated API client. On failure it returns a structured
// CLIError ready to be returned directly from a RunE handler.
//
// Honors a --config path set by the root PersistentPreRunE via WithConfigPath.
func LoadClient(ctx context.Context) (*apiclient.Client, error) {
	cfg := GetConfig(ctx)

	token := cfg.EffectiveToken()
	if token == "" {
		return nil, clierrors.New("auth.token_missing", "Authentication required",
			"No CircleCI API token found.").
			WithSuggestions(
				"Run: circleci setting set token <your-token>",
				"Or set the CIRCLE_TOKEN environment variable",
			).
			WithRef("https://app.circleci.com/settings/user/tokens").
			WithExitCode(clierrors.ExitAuthError)
	}
	return apiclient.New(apiclient.Config{
		BaseURL: cfg.EffectiveHost(),
		Token:   token,
		Version: GetVersion(ctx),
		Agent:   GetAgentName(ctx),
	}), nil
}

func AppURL(ctx context.Context) (string, error) {
	cfg := GetConfig(ctx)
	u, err := url.Parse(cfg.EffectiveHost())
	if err != nil {
		return "", err
	}

	u.Host = "app." + u.Host
	return u.String(), nil
}

// APIErr converts an apiclient error into a structured CLIError.
//
// notFoundCode and notFoundMsg customise the 404 case for the calling resource
// (e.g. "run.not_found", "No run found for %q"). notFoundMsg is
// passed through fmt.Sprintf with subject as the single argument.
//
// Optional notFoundSuggestions are appended to the 404 error (useful for
// pointing users toward a list command, for example).
func APIErr(err error, subject, notFoundCode, notFoundMsg string, notFoundSuggestions ...string) *clierrors.CLIError {
	if httpcl.HasStatusCode(err, http.StatusUnauthorized) {
		return clierrors.New("auth.token_invalid", "Authentication failed",
			"The API token was rejected by CircleCI.").
			WithSuggestions("Run: circleci setting set token <your-token>").
			WithRef("https://app.circleci.com/settings/user/tokens").
			WithExitCode(clierrors.ExitAuthError)
	}
	if httpcl.HasStatusCode(err, http.StatusNotFound) {
		msg := fmt.Sprintf(notFoundMsg, subject)
		if apiErr, ok := apiclient.ParseError(err); ok {
			// The API detail just restates "not found" — keep only the error id.
			if apiErr.ID != "" {
				msg += "\nerror id: " + apiErr.ID
			}
		} else if he, ok := errors.AsType[*httpcl.HTTPError](err); ok && len(he.Body) > 0 {
			msg += "\nAPI: " + string(he.Body)
		}
		nf := clierrors.New(notFoundCode, "Not found", msg).
			WithExitCode(clierrors.ExitNotFound)
		if len(notFoundSuggestions) > 0 {
			nf = nf.WithSuggestions(notFoundSuggestions...)
		}
		return nf
	}
	if he, ok := errors.AsType[*httpcl.HTTPError](err); ok {
		if apiErr, ok := apiclient.ParseError(err); ok {
			title := apiErr.Title
			if title == "" {
				title = "CircleCI API error"
			}
			return clierrors.New("api.error", title,
				fmt.Sprintf("API returned %d: %s", he.StatusCode, apiErr.Message())).
				WithExitCode(clierrors.ExitAPIError)
		}
		return clierrors.New("api.error", "CircleCI API error",
			fmt.Sprintf("API returned %d: %s", he.StatusCode, string(he.Body))).
			WithExitCode(clierrors.ExitAPIError)
	}
	return clierrors.New("api.error", "CircleCI API error", err.Error()).
		WithExitCode(clierrors.ExitAPIError)
}
