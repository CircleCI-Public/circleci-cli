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

package context

import (
	"context"
	"time"

	"github.com/MakeNowJust/heredoc"
	"github.com/google/uuid"
	"github.com/spf13/cobra"

	"github.com/CircleCI-Public/circleci-cli/internal/apiclient"
	"github.com/CircleCI-Public/circleci-cli/internal/cmdutil"
	"github.com/CircleCI-Public/circleci-cli/internal/iostream"
)

func newCreateCmd() *cobra.Command {
	var (
		orgSlug string
		jsonOut bool
	)

	cmd := &cobra.Command{
		Use:   "create <name>",
		Short: "Create a new context",
		Annotations: map[string]string{
			"help:arguments": heredoc.Doc(`
				The name for the new context, e.g. "build-secrets".
			`),
		},
		Long: heredoc.Doc(`
			Create a new CircleCI context for an organization.

			The organization is inferred from the current git repository's remote
			unless overridden with --org. Provide the org slug in the form
			gh/myorg or bitbucket/myorg.

			JSON fields: id, name, created_at
		`),
		Example: heredoc.Doc(`
			# Create a context for the org inferred from git remote
			$ circleci context create my-context

			# Create a context for a specific organization
			$ circleci context create my-context --org gh/myorg

			# Create and capture the ID
			$ circleci context create my-context --org gh/myorg --json --jq '.id'
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
			return runCreate(ctx, client, args[0], orgSlug, jsonOut)
		},
	}

	cmd.Flags().StringVar(&orgSlug, "org", "", "Organization slug (e.g. gh/myorg); defaults to git remote")
	cmdutil.AddJSONFlag(cmd, &jsonOut)
	cmdutil.AddJQFlag(cmd)

	return cmd
}

type contextCreateOutput struct {
	ID        uuid.UUID `json:"id"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
}

func runCreate(ctx context.Context, client *apiclient.Client, name, orgSlug string, jsonOut bool) error {
	orgSlug, err := cmdutil.ResolveOrgSlug(orgSlug, "circleci context create")
	if err != nil {
		return err
	}

	ctxt, err := client.CreateContext(ctx, name, orgSlug)
	if err != nil {
		return apiErr(err, name)
	}

	if jsonOut {
		return iostream.PrintJSON(ctx, contextCreateOutput{
			ID:        ctxt.ID,
			Name:      ctxt.Name,
			CreatedAt: ctxt.CreatedAt,
		})
	}

	iostream.Printf(ctx, "%s Created context %q (%s)\n",
		iostream.SymbolOK(ctx), ctxt.Name, ctxt.ID)
	return nil
}
