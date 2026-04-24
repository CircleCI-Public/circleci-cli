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

package runner

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"

	"github.com/CircleCI-Public/circleci-cli-v2/internal/apiclient"
	"github.com/CircleCI-Public/circleci-cli-v2/internal/cmdutil"
	clierrors "github.com/CircleCI-Public/circleci-cli-v2/internal/errors"
	"github.com/CircleCI-Public/circleci-cli-v2/internal/gitremote"
	"github.com/CircleCI-Public/circleci-cli-v2/internal/httpcl"
	"github.com/CircleCI-Public/circleci-cli-v2/internal/iostream"
)

func newResourceClassCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "resource-class <command>",
		Short: "Manage runner resource classes",
		Long: heredoc.Doc(`
			Manage CircleCI runner resource classes.

			Resource classes define the type of runner available to your jobs.
			Each resource class belongs to a namespace (usually your organization).
		`),
	}

	cmd.AddCommand(newResourceClassListCmd())
	cmd.AddCommand(newResourceClassCreateCmd())
	cmd.AddCommand(newResourceClassDeleteCmd())

	return cmd
}

// --- resource-class list ---

func newResourceClassListCmd() *cobra.Command {
	var namespace string
	var jsonOut bool

	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "List runner resource classes",
		Long: heredoc.Doc(`
			List CircleCI runner resource classes.

			Optionally filter by namespace (organization name).

			JSON fields: id, resource_class, description
		`),
		Example: heredoc.Doc(`
			# List all resource classes you have access to
			$ circleci runner resource-class list

			# List resource classes for a specific namespace
			$ circleci runner resource-class list --namespace my-org

			# Output as JSON
			$ circleci runner resource-class list --namespace my-org --json
		`),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := iostream.FromCmd(cmd.Context(), cmd)
			client, err := cmdutil.LoadClient(ctx, cmd)
			if err != nil {
				return err
			}
			return runResourceClassList(ctx, client, namespace, jsonOut)
		},
	}

	cmd.Flags().StringVar(&namespace, "namespace", "", "Filter by namespace (organization)")
	cmd.Flags().BoolVar(&jsonOut, "json", false, "Output as JSON")
	return cmd
}

type resourceClassOutput struct {
	ID            string `json:"id"`
	ResourceClass string `json:"resource_class"`
	Description   string `json:"description"`
}

func runResourceClassList(ctx context.Context, client *apiclient.Client, namespace string, jsonOut bool) error {
	if namespace == "" {
		ns, err := gitremote.DetectNamespace()
		if err != nil {
			return clierrors.New("runner.namespace_required", "Namespace required",
				"Could not detect organization namespace from git remote.").
				WithSuggestions("Specify your organization: circleci runner resource-class list --namespace <your-org>").
				WithExitCode(clierrors.ExitBadArguments)
		}
		namespace = ns
	}

	classes, err := client.ListResourceClasses(ctx, namespace)
	if err != nil {
		if httpcl.HasStatusCode(err, http.StatusNotFound) {
			return runnerNotEnabledErr()
		}
		return apiErr(err, namespace)
	}

	out := make([]resourceClassOutput, len(classes))
	for i, rc := range classes {
		out[i] = resourceClassOutput{
			ID:            rc.ID,
			ResourceClass: rc.ResourceClass,
			Description:   rc.Description,
		}
	}

	if jsonOut {
		enc := json.NewEncoder(iostream.Out(ctx))
		enc.SetIndent("", "  ")
		return enc.Encode(out)
	}

	if len(out) == 0 {
		iostream.Printf(ctx, "No resource classes found.\n")
		return nil
	}
	for _, rc := range out {
		if rc.Description != "" {
			iostream.Printf(ctx, "%-40s  %s\n", rc.ResourceClass, rc.Description)
		} else {
			iostream.Printf(ctx, "%s\n", rc.ResourceClass)
		}
	}
	return nil
}

// --- resource-class create ---

func newResourceClassCreateCmd() *cobra.Command {
	var description string
	var jsonOut bool

	cmd := &cobra.Command{
		Use:   "create <namespace>/<name>",
		Short: "Create a runner resource class",
		Long: heredoc.Doc(`
			Create a new CircleCI runner resource class.

			The resource class name must be in the format namespace/name,
			where namespace is your organization name.

			JSON fields: id, resource_class, description
		`),
		Example: heredoc.Doc(`
			# Create a resource class
			$ circleci runner resource-class create my-org/my-runner

			# Create with a description
			$ circleci runner resource-class create my-org/my-runner --description "Linux amd64 runner"

			# Output as JSON
			$ circleci runner resource-class create my-org/my-runner --json
		`),
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if cliErr := cmdutil.RequireArgs(args, "namespace/name"); cliErr != nil {
				return cliErr
			}
			ctx := iostream.FromCmd(cmd.Context(), cmd)
			client, err := cmdutil.LoadClient(ctx, cmd)
			if err != nil {
				return err
			}
			return runResourceClassCreate(ctx, client, args[0], description, jsonOut)
		},
	}

	cmd.Flags().StringVar(&description, "description", "", "Human-readable description of the resource class")
	cmd.Flags().BoolVar(&jsonOut, "json", false, "Output as JSON")
	return cmd
}

func runResourceClassCreate(ctx context.Context, client *apiclient.Client, resourceClass, description string, jsonOut bool) error {
	rc, err := client.CreateResourceClass(ctx, resourceClass, description)
	if err != nil {
		return apiErr(err, resourceClass)
	}

	out := resourceClassOutput{
		ID:            rc.ID,
		ResourceClass: rc.ResourceClass,
		Description:   rc.Description,
	}

	if jsonOut {
		enc := json.NewEncoder(iostream.Out(ctx))
		enc.SetIndent("", "  ")
		return enc.Encode(out)
	}

	iostream.Printf(ctx, "Created resource class: %s\n", out.ResourceClass)
	if out.Description != "" {
		iostream.Printf(ctx, "Description: %s\n", out.Description)
	}
	iostream.Printf(ctx, "ID: %s\n", out.ID)
	return nil
}

// --- resource-class delete ---

func newResourceClassDeleteCmd() *cobra.Command {
	var force bool

	cmd := &cobra.Command{
		Use:   "delete <namespace>/<name>",
		Short: "Delete a runner resource class",
		Long: heredoc.Doc(`
			Delete a CircleCI runner resource class.

			All tokens associated with the resource class will also be deleted.
			Connected runner instances will no longer be able to claim jobs.

			In a terminal, you will be prompted to confirm before deleting.
			Use --force (-f) to skip the prompt for scripting.
		`),
		Example: heredoc.Doc(`
			# Delete a resource class (with confirmation prompt)
			$ circleci runner resource-class delete my-org/my-runner

			# Delete without confirmation
			$ circleci runner resource-class delete my-org/my-runner --force

			# Delete in a script
			$ circleci runner resource-class delete my-org/my-runner --force
		`),
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if cliErr := cmdutil.RequireArgs(args, "namespace/name"); cliErr != nil {
				return cliErr
			}
			ctx := iostream.FromCmd(cmd.Context(), cmd)
			client, err := cmdutil.LoadClient(ctx, cmd)
			if err != nil {
				return err
			}
			return runResourceClassDelete(ctx, client, args[0], force)
		},
	}

	cmd.Flags().BoolVarP(&force, "force", "f", false, "skip confirmation prompt")
	return cmd
}

func runResourceClassDelete(ctx context.Context, client *apiclient.Client, resourceClass string, force bool) error {
	if !force {
		if iostream.IsInteractive(ctx) {
			prompt := fmt.Sprintf("Delete resource class %q? All tokens and runner connections will be removed.", resourceClass)
			if !iostream.Confirm(ctx, prompt) {
				return clierrors.New("runner.delete_aborted", "Deletion aborted",
					"Resource class deletion was not confirmed.").
					WithExitCode(clierrors.ExitCancelled)
			}
		} else {
			return clierrors.New("runner.delete_requires_force", "Deletion requires --force",
				fmt.Sprintf("Deleting resource class %q is irreversible.", resourceClass)).
				WithSuggestions("Pass --force (-f) to confirm deletion in non-interactive mode").
				WithExitCode(clierrors.ExitCancelled)
		}
	}

	if err := client.DeleteResourceClass(ctx, resourceClass); err != nil {
		return apiErr(err, resourceClass)
	}

	iostream.ErrPrintf(ctx, "%s Deleted resource class %s\n", iostream.Symbol(ctx, "✓", "OK:"), resourceClass)
	return nil
}
