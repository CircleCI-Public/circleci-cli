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

package org

import (
	"context"
	"errors"

	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"

	"github.com/CircleCI-Public/circleci-cli/clikit/iostream"
	"github.com/CircleCI-Public/circleci-cli/internal/apiclient"
	"github.com/CircleCI-Public/circleci-cli/internal/cmdutil"
	"github.com/CircleCI-Public/circleci-cli/internal/httpcl"
)

// createVCSType is the only organization type the CLI creates, so it is a
// constant rather than a flag. A VCS-backed org (gh/..., bb/...) is imported by
// CircleCI when the user signs in or installs an integration; there is no
// meaningful way to create one, and offering the choice would only let a caller
// ask for something the platform does not do.
const createVCSType = "circleci"

func newCreateCmd() *cobra.Command {
	var jsonOut bool

	cmd := &cobra.Command{
		Use:   "create <name>",
		Short: "Create an organization",
		Annotations: map[string]string{
			"help:arguments": heredoc.Docf(`
				%[1]s<name>%[1]s is the name for the new organization.
			`, "`"),
		},
		Long: heredoc.Doc(`
			Create a CircleCI-native organization owned by your account.

			VCS-backed organizations (gh/..., bb/...) are imported when you sign in
			or install the CircleCI GitHub App, so only CircleCI-native ones are
			created here. The new org's slug is what --org takes elsewhere.

			JSON fields: id, name, slug, vcs_type
		`),
		Example: heredoc.Doc(`
			# Create an organization
			$ circleci org create acme

			# Create one and output as JSON for scripting
			$ circleci org create acme --json

			# Capture the slug to pass to --org
			$ circleci org create acme --json --jq '.slug'
		`),
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := cmdutil.RequireArgs(args, "name"); err != nil {
				return err
			}
			ctx := cmd.Context()
			client, err := cmdutil.LoadClient(ctx)
			if err != nil {
				return err
			}
			return runOrgCreate(ctx, client, args[0], jsonOut)
		},
	}

	cmdutil.AddJSONFlag(cmd, &jsonOut)
	cmdutil.AddJQFlag(cmd)

	return cmd
}

// createErr explains a create the API refused.
//
// APIErr diagnoses the cases it recognises — a rejected token, a gone endpoint —
// and renders anything else as a bare status line with no way forward. A refused
// create is exactly that "anything else", and it is the common failure here, so
// the two things a caller can act on are added.
//
// The suggestions are gated on the server having actually responded. APIErr
// reports an unreachable host under the same code as a refusal, and "check
// whether the name is taken" is a nonsense answer to a DNS failure. The status
// code is deliberately not interpreted beyond that: the server's own message
// already says which refusal it was, and guessing at code semantics would put
// words in its mouth.
func createErr(err error, name string) error {
	cliErr := cmdutil.APIErr(err, name, "org.create_failed", "Could not create organization %q.")
	if _, responded := errors.AsType[*httpcl.HTTPError](err); !responded || cliErr.Code != "api.error" {
		return cliErr
	}
	return cliErr.WithSuggestions(
		"Check whether the name is already taken: circleci org list",
		"Organization creation may not be enabled for your account",
	)
}

type orgCreateOutput struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Slug    string `json:"slug"`
	VCSType string `json:"vcs_type"`
}

func runOrgCreate(ctx context.Context, client *apiclient.Client, name string, jsonOut bool) error {
	created, err := client.CreateOrg(ctx, name, createVCSType)
	if err != nil {
		return createErr(err, name)
	}

	out := orgCreateOutput{
		ID:      created.ID,
		Name:    created.Name,
		Slug:    created.Slug,
		VCSType: created.VCSType,
	}

	if jsonOut {
		return iostream.PrintJSON(ctx, out)
	}

	iostream.Printf(ctx, "%s Created organization %s\n", iostream.SymbolOK(ctx), out.Name)
	iostream.Printf(ctx, "  Slug: %s\n", out.Slug)
	iostream.Printf(ctx, "\nNext: circleci project create --org %s\n", out.Slug)
	return nil
}
