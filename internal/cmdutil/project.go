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

package cmdutil

import (
	"context"

	"github.com/CircleCI-Public/circleci-cli/internal/apiclient"
	"github.com/CircleCI-Public/circleci-cli/internal/gitremote"
)

// ResolveProjectSlug returns projectSlug as-is when non-empty. Otherwise it
// detects the slug from the git remote. Unlike ResolveProjectID this does not
// make an API call and does not return a UUID — use it for endpoints that
// require a slug in the path (e.g. POST /project/{vcs}/{org}/{repo}/pipeline/run).
func ResolveProjectSlug(projectSlug string) (string, error) {
	if projectSlug != "" {
		return projectSlug, nil
	}
	info, err := gitremote.Detect()
	if err != nil {
		return "", GitDetectErr(err, "Or specify the project with --project gh/org/repo")
	}
	return info.Slug, nil
}

// ResolveProjectID returns projectID as-is when non-empty. Otherwise it resolves
// the project from the slug (--project flag or git remote) to recover its UUID.
func ResolveProjectID(ctx context.Context, client *apiclient.Client, projectSlug, projectID string) (string, error) {
	if projectID != "" {
		return projectID, nil
	}
	if projectSlug == "" {
		info, err := gitremote.Detect()
		if err != nil {
			return "", GitDetectErr(err, "Or specify the project with --project gh/org/repo or --project-id <uuid>")
		}
		projectSlug = info.Slug
	}
	proj, err := client.GetProjectBySlug(ctx, projectSlug)
	if err != nil {
		return "", APIErr(err, projectSlug, "project.not_found", "No project found for %q.",
			"Run 'circleci project link' to bind this repository to a CircleCI project",
			"Check the project slug and try again",
			"Use 'circleci project list' to see followed projects")
	}
	return proj.ID.String(), nil
}
