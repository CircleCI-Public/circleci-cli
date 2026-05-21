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

	"github.com/CircleCI-Public/circleci-cli/internal/apiclient"
)

// PipelinesURL returns the CircleCI pipelines page URL for the given project slug.
func PipelinesURL(appURL, slug string) (string, error) {
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
