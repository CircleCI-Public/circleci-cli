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
	"fmt"
	"strings"

	"github.com/google/uuid"

	"github.com/CircleCI-Public/circleci-cli/internal/apiclient"
	clierrors "github.com/CircleCI-Public/circleci-cli/internal/errors"
	"github.com/CircleCI-Public/circleci-cli/internal/gitremote"
)

// ResolveOrgSlug returns orgSlug as-is when non-empty. Otherwise it derives the
// organization slug (vcs/org, e.g. gh/myorg) from the current git remote. It
// makes no API call — use this for endpoints keyed on an owner slug rather than
// an org UUID (see ResolveOrgSlugOrID for the UUID case).
//
// cmdName is included in the GitDetectErr suggestion text so users see the
// exact override flag for the command they invoked, e.g. "circleci context list".
func ResolveOrgSlug(orgSlug, cmdName string) (string, error) {
	if orgSlug != "" {
		return orgSlug, nil
	}
	info, err := gitremote.Detect()
	if err != nil {
		return "", GitDetectErr(err, "Or specify the organization: "+cmdName+" --org <vcs>/<org>")
	}
	return orgSlugFromProjectSlug(info.Slug), nil
}

// orgSlugFromProjectSlug extracts the org portion of a project slug.
// "gh/myorg/myrepo" → "gh/myorg".
func orgSlugFromProjectSlug(projectSlug string) string {
	parts := strings.SplitN(projectSlug, "/", 3)
	if len(parts) >= 2 {
		return parts[0] + "/" + parts[1]
	}
	return projectSlug
}

// ResolveOrgSlugOrID resolves an organization reference to its UUID. The
// reference may be:
//
//   - an org UUID (e.g. f22b6566-597d-46d5-ba74-99ef5bb3d85c), used as-is;
//   - an org slug (e.g. gh/myorg), looked up through the API;
//   - empty, in which case the org is inferred from the current git remote.
//
// cmdName is included in the GitDetectErr suggestion text so users see the
// exact override flag for the command they invoked, e.g.
// "circleci runner instance list".
func ResolveOrgSlugOrID(ctx context.Context, client *apiclient.Client, ref, cmdName string) (uuid.UUID, error) {
	if ref == "" {
		raw, err := orgIDFromGitRemote(ctx, client,
			"Or specify the organization: "+cmdName+" --org <vcs>/<org>",
			"Pass --org <vcs>/<org> or the org UUID explicitly")
		if err != nil {
			return uuid.Nil, err
		}
		return parseOrgID(raw)
	}

	// A bare UUID is already an org ID; no lookup required.
	if id, err := uuid.Parse(ref); err == nil {
		return id, nil
	}

	org, err := client.GetOrg(ctx, ref)
	if err != nil {
		return uuid.Nil, clierrors.New("org.resolve_failed", "Could not resolve organization",
			fmt.Sprintf("Failed to look up organization %q: %s", ref, err.Error())).
			WithSuggestions(
				"Provide the org slug as <vcs>/<org> (e.g. gh/acme)",
				"Or pass the organization UUID directly",
			).
			WithExitCode(clierrors.ExitBadArguments)
	}
	return parseOrgID(org.ID)
}

// orgIDFromGitRemote detects the project from the current git remote and
// resolves it through the API to recover the org UUID (as the raw string the
// API returns). detectHint is passed to GitDetectErr and projectFailSuggestion
// is used when the project lookup fails, so callers can phrase the override in
// terms of their own flag.
func orgIDFromGitRemote(ctx context.Context, client *apiclient.Client, detectHint, projectFailSuggestion string) (string, error) {
	info, err := gitremote.Detect()
	if err != nil {
		return "", GitDetectErr(err, detectHint)
	}
	proj, err := client.GetProjectBySlug(ctx, info.Slug)
	if err != nil {
		return "", clierrors.New("org.resolve_failed", "Could not resolve organization",
			fmt.Sprintf("Failed to look up project %q to recover its organization: %s", info.Slug, err.Error())).
			WithSuggestions(projectFailSuggestion).
			WithExitCode(clierrors.ExitBadArguments)
	}
	return proj.OrgID.String(), nil
}

// parseOrgID converts an organization ID string returned by the API into a
// uuid.UUID, surfacing a structured error if the API returned a malformed ID.
func parseOrgID(raw string) (uuid.UUID, error) {
	id, err := uuid.Parse(raw)
	if err != nil {
		return uuid.Nil, clierrors.New("org.resolve_failed", "Could not resolve organization",
			fmt.Sprintf("The API returned an organization ID that is not a valid UUID: %q", raw)).
			WithExitCode(clierrors.ExitAPIError)
	}
	return id, nil
}
