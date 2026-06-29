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
	"fmt"
	"net/url"
	"strings"

	"github.com/google/uuid"

	"github.com/CircleCI-Public/circleci-cli/internal/apiclient"
)

// ParseSlug splits a project slug "vcs/org/repo" into its three components.
func ParseSlug(slug string) (vcs, org, repo string, err error) {
	parts := strings.SplitN(slug, "/", 3)
	if len(parts) != 3 || parts[0] == "" || parts[1] == "" || parts[2] == "" {
		return "", "", "", fmt.Errorf("invalid slug: %q", slug)
	}
	return parts[0], parts[1], parts[2], nil
}

func RunSlugURL(appURL string, slug string) (string, error) {
	parts := strings.SplitN(slug, "/", 3)
	if len(parts) != 3 {
		return "", fmt.Errorf("invalid slug: %q", slug)
	}
	return fmt.Sprintf("%s/pipelines/%s/%s/%s",
		appURL,
		url.PathEscape(parts[0]),
		url.PathEscape(parts[1]),
		url.PathEscape(parts[2]),
	), nil
}

// RunURL returns the CircleCI pipelines page URL for the given project slug.
func RunURL(appURL string, id uuid.UUID) string {
	return fmt.Sprintf("%s/pipeline/%s", appURL, id)
}

func WorkflowURL(appURL string, id uuid.UUID) string {
	return fmt.Sprintf("%s/workflow/%s", appURL, id)
}

func JobURL(appURL string, workflowID, jobID uuid.UUID) string {
	return fmt.Sprintf("%s/workflow/%s/job/%s", appURL, workflowID, jobID)
}

// ProjectURL returns the CircleCI project page URL for the given project slug.
func ProjectURL(appURL, slug string) (string, error) {
	parts := strings.SplitN(slug, "/", 3)
	if len(parts) != 3 {
		return "", fmt.Errorf("invalid slug: %q", slug)
	}
	return fmt.Sprintf("%s/projects/%s/%s/%s",
		appURL,
		url.PathEscape(parts[0]),
		url.PathEscape(parts[1]),
		url.PathEscape(parts[2]),
	), nil
}

// ContextsURL returns the CircleCI contexts settings page URL for the given org slug.
func ContextsURL(appURL, orgSlug string) (string, error) {
	parts := strings.SplitN(orgSlug, "/", 2)
	if len(parts) != 2 {
		return "", fmt.Errorf("invalid org slug: %q", orgSlug)
	}
	return fmt.Sprintf("%s/settings/organization/%s/%s/contexts",
		appURL,
		url.PathEscape(parts[0]),
		url.PathEscape(parts[1]),
	), nil
}

// RunnersURL returns the CircleCI runners inventory page URL for the given org slug.
func RunnersURL(appURL, orgSlug string) (string, error) {
	parts := strings.SplitN(orgSlug, "/", 2)
	if len(parts) != 2 {
		return "", fmt.Errorf("invalid org slug: %q", orgSlug)
	}
	return fmt.Sprintf("%s/runners/%s/%s/inventory",
		appURL,
		url.PathEscape(parts[0]),
		url.PathEscape(parts[1]),
	), nil
}

// DeployURL returns the CircleCI deploys page URL for the given project.
func DeployURL(appURL string, proj *apiclient.ProjectInfo) string {
	return fmt.Sprintf("%s/deploys/%s/%s/projects/%s",
		appURL,
		url.PathEscape(VCSSlug(proj.VCSInfo.Provider)),
		url.PathEscape(proj.OrganizationName),
		url.PathEscape(proj.ID),
	)
}

// VCSSlug maps API provider strings to the slug prefix used in CircleCI URLs
// (e.g. "GitHub" → "gh").
func VCSSlug(provider string) string {
	switch strings.ToLower(provider) {
	case "github":
		return "gh"
	case "bitbucket":
		return "bb"
	case "gitlab":
		return "gl"
	default:
		return provider
	}
}
