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

// Package open implements the "circleci open" command.
package open

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/MakeNowJust/heredoc"
	"github.com/pkg/browser"
	"github.com/spf13/cobra"

	clierrors "github.com/CircleCI-Public/circleci-cli-v2/internal/errors"
	"github.com/CircleCI-Public/circleci-cli-v2/internal/gitremote"
)

// ProjectURL builds the CircleCI pipelines URL for the given project slug.
func ProjectURL(slug string) (string, error) {
	parts := strings.SplitN(slug, "/", 3)
	if len(parts) != 3 {
		return "", fmt.Errorf("invalid slug: %q", slug)
	}
	return fmt.Sprintf("https://app.circleci.com/pipelines/%s/%s/%s",
		url.PathEscape(parts[0]),
		url.PathEscape(parts[1]),
		url.PathEscape(parts[2]),
	), nil
}

// NewOpenCmd returns the "circleci open" command.
func NewOpenCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "open",
		Short: "Open the current project in the browser",
		Long: heredoc.Doc(`
			Open the CircleCI pipelines page for the current project in your
			default web browser.

			The project is inferred from the current git repository's remote.
			Supports GitHub, Bitbucket, and GitLab remotes.
		`),
		Example: heredoc.Doc(`
			# Open pipelines for the current repo
			$ circleci open
		`),
		RunE: func(_ *cobra.Command, _ []string) error {
			info, err := gitremote.Detect()
			if err != nil {
				return clierrors.New("git.detect_failed",
					"Could not detect project from git remote", err.Error()).
					WithSuggestions(
						"Run from inside a git repository with a GitHub, Bitbucket, or GitLab remote",
					).
					WithExitCode(clierrors.ExitBadArguments)
			}

			projectURL, err := ProjectURL(info.Slug)
			if err != nil {
				return err
			}

			return browser.OpenURL(projectURL)
		},
	}
}
