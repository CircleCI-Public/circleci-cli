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

package namespace

import (
	"context"
	"fmt"

	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"

	"github.com/CircleCI-Public/circleci-cli/internal/apiclient"
	"github.com/CircleCI-Public/circleci-cli/internal/cmdutil"
	clierrors "github.com/CircleCI-Public/circleci-cli/internal/errors"
	"github.com/CircleCI-Public/circleci-cli/internal/iostream"
)

func newDeleteCmd() *cobra.Command {
	var force bool
	var dryRun bool

	cmd := &cobra.Command{
		Use:     "delete <name>",
		Aliases: []string{"rm"},
		Short:   "Delete a namespace and all its orbs",
		Annotations: map[string]string{
			"help:arguments": heredoc.Doc(`
				<name> is the name of the namespace to delete, e.g. "myorg".
			`),
		},
		Long: heredoc.Doc(`
			Delete a CircleCI orb namespace and all orbs published under it.

			This operation is irreversible. Any pipelines referencing orbs in
			this namespace will fail after deletion.

			In a terminal, you will be prompted to confirm before deleting.
			Use --force (-f) to skip the prompt for scripting.
			Use --dry-run (-n) to preview what would be deleted without deleting.
		`),
		Example: heredoc.Doc(`
			# Delete a namespace (with confirmation prompt)
			$ circleci namespace delete myorg

			# Preview what would be deleted without deleting
			$ circleci namespace delete myorg --dry-run

			# Delete without a confirmation prompt
			$ circleci namespace delete myorg --force

			# Delete in a CI environment (non-interactive; --force required)
			$ circleci namespace delete myorg -f
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
			return runDelete(ctx, client, args[0], force, dryRun)
		},
	}

	cmd.Flags().BoolVarP(&force, "force", "f", false, "skip confirmation prompt")
	cmd.Flags().BoolVarP(&dryRun, "dry-run", "n", false, "print what would be deleted without deleting")

	return cmd
}

func runDelete(ctx context.Context, client *apiclient.Client, name string, force, dryRun bool) error {
	if dryRun {
		iostream.Printf(ctx, "Would delete namespace %s and all its orbs.\n", name)
		return nil
	}

	if err := cmdutil.ConfirmOrForce(ctx, iostream.Get(ctx), force,
		fmt.Sprintf("Delete namespace %q and all its orbs? This cannot be undone.", name),
		clierrors.New("namespace.delete_aborted", "Deletion aborted",
			"Namespace deletion was not confirmed.").
			WithExitCode(clierrors.ExitCancelled),
		clierrors.New("namespace.delete_requires_force", "Deletion requires --force",
			fmt.Sprintf("Deleting namespace %q will remove all its orbs.", name)).
			WithExitCode(clierrors.ExitBadArguments),
	); err != nil {
		return err
	}

	if err := client.DeleteNamespace(ctx, name); err != nil {
		return apiErr(err, name)
	}

	iostream.Printf(ctx, "%s Deleted namespace %s\n", iostream.SymbolOK(ctx), name)
	return nil
}
