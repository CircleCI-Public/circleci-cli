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
	"os"
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
	if envOrg := orgFromEnv(); envOrg != "" {
		return envOrg, nil
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
		ref = orgFromEnv()
	}
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

func orgFromEnv() string {
	return strings.TrimSpace(os.Getenv("CIRCLE_ORG"))
}

// InferOrgID best-effort resolves the org UUID for the current directory's
// CircleCI project. Detection follows gitremote.Detect's resolution order — a
// `circleci project link` binding (.circleci/info.yml) takes precedence over
// the git remote, so an explicit link wins when the remote is not the right
// answer (repository renames, forks, standalone projects). When the link
// recorded the org as a UUID it is used directly; otherwise the resolved
// project is looked up through the API to recover its owning org.
//
// It is the lenient counterpart to ResolveOrgSlugOrID, for commands where the
// org is an optional convenience rather than required — e.g. config compilation
// passes it so private and namespaced orbs resolve without an explicit --org.
// It returns "" (with no error) whenever the org cannot be determined — not a
// git checkout, an unrecognised remote, or a failed project lookup — so callers
// fall back to public-only behaviour instead of failing.
func InferOrgID(ctx context.Context, client *apiclient.Client) string {
	// The hint strings are unused: any error is swallowed into a "" result.
	raw, err := orgIDFromGitRemote(ctx, client, "", "")
	if err != nil {
		return ""
	}
	return raw
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
	// A `circleci project link` binding records the org ID directly. When it is a
	// UUID — the same form the project lookup below returns — use it as-is and
	// skip the API round-trip, so the org resolves even offline or with a token
	// that can't read the project. A non-UUID (compact) recorded ID falls through
	// to the lookup, which yields the canonical UUID.
	if info.OrgID != "" {
		if id, err := uuid.Parse(info.OrgID); err == nil {
			return id.String(), nil
		}
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
