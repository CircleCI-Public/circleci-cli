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
	"fmt"

	"github.com/MakeNowJust/heredoc"
	"github.com/google/uuid"
	"github.com/spf13/cobra"

	"github.com/CircleCI-Public/circleci-cli-v2/internal/apiclient"
	"github.com/CircleCI-Public/circleci-cli-v2/internal/cmdutil"
	clierrors "github.com/CircleCI-Public/circleci-cli-v2/internal/errors"
	"github.com/CircleCI-Public/circleci-cli-v2/internal/gitremote"
	"github.com/CircleCI-Public/circleci-cli-v2/internal/iostream"
)

func newDeleteCmd() *cobra.Command {
	var (
		orgSlug string
		force   bool
	)

	cmd := &cobra.Command{
		Use:   "delete <context-id>",
		Short: "Delete a context",
		Long: heredoc.Doc(`
			Delete a CircleCI context by its UUID.

			Deleting a context removes all environment variables stored in it.
			Jobs that reference this context will fail until they are updated.

			Pass --org to look up a context by name instead of UUID; the org slug
			is otherwise inferred from the git remote.

			In a terminal, you will be prompted to confirm before deleting.
			Use --force (-f) to skip the prompt for scripting.
		`),
		Example: heredoc.Doc(`
			# Delete a context by UUID (with confirmation)
			$ circleci context delete ctx-uuid-here

			# Delete without confirmation
			$ circleci context delete ctx-uuid-here --force

			# Look up a context by name and delete it
			$ circleci context list --org gh/myorg --json | jq -r '.[] | select(.name=="my-ctx") | .id' | xargs circleci context delete --force
		`),
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := cmdutil.RequireArgs(args, "context-id"); err != nil {
				return err
			}
			ctx := iostream.FromCmd(cmd.Context(), cmd)
			client, err := cmdutil.LoadClient(ctx, cmd)
			if err != nil {
				return err
			}
			return runDelete(ctx, client, args[0], orgSlug, force)
		},
	}

	cmd.Flags().StringVar(&orgSlug, "org", "", "Organization slug (e.g. gh/myorg); used when resolving name to ID")
	cmd.Flags().BoolVarP(&force, "force", "f", false, "skip confirmation prompt")

	return cmd
}

func runDelete(ctx context.Context, client *apiclient.Client, contextName, orgSlug string, force bool) error {
	// If the arg doesn't look like a UUID, resolve by name.
	displayName := contextName
	contextID, err := uuid.Parse(contextName)
	if err != nil {
		if orgSlug == "" {
			info, err := gitremote.Detect()
			if err != nil {
				return cmdutil.GitDetectErr(err, "Or specify the organization: circleci context delete --org gh/myorg")
			}
			orgSlug = orgFromSlug(info.Slug)
		}
		id, err := resolveContextID(ctx, client, contextName, orgSlug)
		if err != nil {
			return err
		}
		contextID = id
	}

	if err := cmdutil.ConfirmOrForce(ctx, iostream.Get(ctx), force,
		fmt.Sprintf("Delete context %s? All environment variables in it will be removed.", displayName),
		clierrors.New("context.delete_aborted", "Deletion aborted",
			"Context deletion was not confirmed.").
			WithExitCode(clierrors.ExitCancelled),
		clierrors.New("context.delete_requires_force", "Deletion requires --force",
			fmt.Sprintf("Deleting context %s will remove all its environment variables.", displayName)).
			WithExitCode(clierrors.ExitCancelled),
	); err != nil {
		return err
	}

	if err := client.DeleteContext(ctx, contextID); err != nil {
		return apiErr(err, displayName)
	}

	iostream.Printf(ctx, "%s Deleted context %s\n", iostream.Symbol(ctx, "✓", "OK:"), displayName)
	return nil
}

// resolveContextID looks up a context by name and returns its UUID.
func resolveContextID(ctx context.Context, client *apiclient.Client, name, orgSlug string) (uuid.UUID, error) {
	contexts, err := client.ListContexts(ctx, orgSlug)
	if err != nil {
		return uuid.Nil, apiErr(err, orgSlug)
	}
	for _, c := range contexts {
		if c.Name == name {
			return c.ID, nil
		}
	}
	return uuid.Nil, clierrors.New("context.not_found", "Context not found",
		fmt.Sprintf("No context named %q found in organization %q.", name, orgSlug)).
		WithSuggestions("Run: circleci context list --org " + orgSlug).
		WithExitCode(clierrors.ExitNotFound)
}
