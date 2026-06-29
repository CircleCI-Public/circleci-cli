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

// Package org provides shared logic for CircleCI organization operations.
package org

import (
	"context"
	"fmt"
	"strings"

	"github.com/CircleCI-Public/circleci-cli/internal/apiclient"
	"github.com/CircleCI-Public/circleci-cli/internal/cmdutil"
	clierrors "github.com/CircleCI-Public/circleci-cli/internal/errors"
	"github.com/CircleCI-Public/circleci-cli/internal/iostream"
)

// List fetches the organizations the authenticated user belongs to.
func List(ctx context.Context, client *apiclient.Client) ([]apiclient.Collaboration, error) {
	collabs, err := client.ListCollaborations(ctx)
	if err != nil {
		return nil, cmdutil.APIErr(err, "", "org.list_failed", "Could not fetch your organizations.",
			"Check your API token and network connection",
		)
	}
	return collabs, nil
}

// Require fetches the user's organizations and returns an actionable error
// when the account has none. In interactive mode, offers to create a new
// CircleCI organization on the spot.
func Require(ctx context.Context, client *apiclient.Client) ([]apiclient.Collaboration, error) {
	collabs, err := List(ctx, client)
	if err != nil {
		return nil, err
	}
	if len(collabs) == 0 {
		if iostream.IsInteractive(ctx) {
			return promptCreateOrg(ctx, client)
		}
		suggestions := []string{
			"Ask an admin to invite you to an existing organization",
		}
		if appURL, err := cmdutil.AppURL(ctx); err == nil {
			suggestions = []string{
				fmt.Sprintf("Create or join an organization at %s", appURL),
				"Ask an admin to invite you to an existing organization",
			}
		}
		return nil, clierrors.New("org.none_found", "No organizations found",
			"Your account is not a member of any CircleCI organizations.").
			WithSuggestions(suggestions...).
			WithExitCode(clierrors.ExitNotFound)
	}
	return collabs, nil
}

func promptCreateOrg(ctx context.Context, client *apiclient.Client) ([]apiclient.Collaboration, error) {
	iostream.ErrPrintf(ctx, "You don't belong to any CircleCI organizations yet.\n\n")

	name, err := iostream.PromptText(ctx, "Organization name", "")
	if err != nil || name == "" {
		return nil, clierrors.New("org.create_cancelled", "Cancelled",
			"No organization name entered.").
			WithExitCode(clierrors.ExitCancelled)
	}

	created, err := client.CreateOrg(ctx, name, "circleci")
	if err != nil {
		return nil, cmdutil.APIErr(err, name, "org.create_failed", "Could not create organization %q.")
	}

	iostream.Printf(ctx, "%s Created organization %s\n", iostream.SymbolOK(ctx), created.Slug)

	return []apiclient.Collaboration{
		{ID: created.ID, Name: created.Name, Slug: created.Slug, VCSType: created.VCSType},
	}, nil
}

// Select fetches the user's organizations and presents an interactive picker.
// With exactly one org it is auto-selected; with multiple the user picks.
func Select(ctx context.Context, client *apiclient.Client) (string, error) {
	collabs, err := Require(ctx, client)
	if err != nil {
		return "", err
	}

	if len(collabs) == 1 {
		return collabs[0].Slug, nil
	}

	labels := make([]string, len(collabs))
	for i, c := range collabs {
		labels[i] = c.Slug
		if c.Name != "" && c.Name != c.Slug {
			labels[i] = fmt.Sprintf("%s (%s)", c.Slug, c.Name)
		}
	}

	idx, err := iostream.PromptSelect(ctx, "Select an organization", labels)
	if err != nil || idx < 0 {
		return "", clierrors.New("org.selection_cancelled", "Cancelled",
			"No organization selected.").
			WithExitCode(clierrors.ExitCancelled)
	}
	return collabs[idx].Slug, nil
}

// ValidateSlug checks that the given org slug is among the user's organizations.
func ValidateSlug(collabs []apiclient.Collaboration, slug string) error {
	for _, c := range collabs {
		if c.Slug == slug {
			return nil
		}
	}
	slugs := make([]string, len(collabs))
	for i, c := range collabs {
		slugs[i] = c.Slug
	}
	orgList := strings.Join(slugs, ", ")
	if len(slugs) > 10 {
		orgList = strings.Join(slugs[:10], ", ") + fmt.Sprintf(" (and %d more)", len(slugs)-10)
	}
	suggestions := []string{
		"Check the --org value and try again",
		fmt.Sprintf("Your organizations: %s", orgList),
	}
	return clierrors.New("org.not_member", "Not a member of this organization",
		fmt.Sprintf("You are not a member of organization %q.", slug)).
		WithSuggestions(suggestions...).
		WithExitCode(clierrors.ExitBadArguments)
}

// ParseSlug splits "gh/myorg" into its VCS and org components.
func ParseSlug(slug string) (vcs, orgName string, err error) {
	parts := strings.SplitN(slug, "/", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", fmt.Errorf("invalid org slug")
	}
	return parts[0], parts[1], nil
}
