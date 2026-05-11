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

	"github.com/CircleCI-Public/circleci-cli-v2/internal/apiclient"
	clierrors "github.com/CircleCI-Public/circleci-cli-v2/internal/errors"
	"github.com/CircleCI-Public/circleci-cli-v2/internal/gitremote"
)

// ResolveOrgID returns orgID as-is when non-empty. Otherwise it detects the
// project from the current git remote and resolves it through the API to
// recover the org UUID.
//
// cmdName is included in the GitDetectErr suggestion text so users see the
// exact override flag for the command they invoked, e.g.
// "circleci certificate list".
func ResolveOrgID(ctx context.Context, client *apiclient.Client, orgID, cmdName string) (string, error) {
	if orgID != "" {
		return orgID, nil
	}
	info, err := gitremote.Detect()
	if err != nil {
		return "", GitDetectErr(err, "Or specify the organization: "+cmdName+" --org-id <org-uuid>")
	}
	proj, err := client.GetProjectInfo(ctx, info.Slug)
	if err != nil {
		return "", clierrors.New("org.resolve_failed", "Could not resolve organization",
			fmt.Sprintf("Failed to look up project %q to recover its organization: %s", info.Slug, err.Error())).
			WithSuggestions("Pass --org-id <org-uuid> explicitly").
			WithExitCode(clierrors.ExitBadArguments)
	}
	return proj.OrganizationID, nil
}
