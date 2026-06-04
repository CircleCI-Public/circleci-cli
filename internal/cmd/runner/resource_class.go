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
	"fmt"
	"net/http"
	"strings"

	"github.com/MakeNowJust/heredoc"
	"github.com/google/uuid"
	"github.com/spf13/cobra"

	"github.com/CircleCI-Public/circleci-cli/internal/apiclient"
	"github.com/CircleCI-Public/circleci-cli/internal/cmdutil"
	clierrors "github.com/CircleCI-Public/circleci-cli/internal/errors"
	"github.com/CircleCI-Public/circleci-cli/internal/httpcl"
	"github.com/CircleCI-Public/circleci-cli/internal/iostream"
	"github.com/CircleCI-Public/circleci-cli/internal/mdtable"
)

func newResourceClassCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "resource-class <command>",
		Short: "Manage runner resource classes",
		Long: heredoc.Doc(`
			Manage runner resource classes.

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
	var org string
	var namespace string
	var jsonOut bool

	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "List runner resource classes",
		Long: heredoc.Doc(`
			List CircleCI runner resource classes.

			The organization is inferred from the current git repository's remote
			unless overridden with --org, which accepts an org slug (e.g. gh/myorg)
			or an org UUID. Optionally filter further by namespace.

			JSON fields: id, resource_class, description
		`),
		Example: heredoc.Doc(`
			# List resource classes for the org inferred from the git remote
			$ circleci runner resource-class list

			# List resource classes for a specific organization (slug)
			$ circleci runner resource-class list --org gh/my-org

			# List resource classes for a specific organization (UUID)
			$ circleci runner resource-class list --org f22b6566-597d-46d5-ba74-99ef5bb3d85c

			# Output as JSON
			$ circleci runner resource-class list --org gh/my-org --json
		`),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			client, err := cmdutil.LoadClient(ctx)
			if err != nil {
				return err
			}
			return runResourceClassList(ctx, client, org, namespace, jsonOut)
		},
	}

	cmd.Flags().StringVar(&org, "org", "", "Organization slug (e.g. gh/myorg) or UUID; defaults to git remote")
	cmd.Flags().StringVar(&namespace, "namespace", "", "Filter by namespace (organization)")
	cmdutil.AddJSONFlag(cmd, &jsonOut)
	cmdutil.AddJQFlag(cmd)
	return cmd
}

type resourceClassOutput struct {
	ID            string `json:"id"`
	ResourceClass string `json:"resource_class"`
	Description   string `json:"description"`
}

func runResourceClassList(ctx context.Context, client *apiclient.Client, org, namespace string, jsonOut bool) error {
	// List by org UUID when --org (slug or UUID) is given or can be inferred
	// from the git remote. When only a namespace filter is supplied, keep the
	// legacy namespace-based listing and skip org resolution.
	var (
		classes []apiclient.ResourceClass
		subject = namespace
		err     error
	)
	if org != "" || namespace == "" {
		var orgID uuid.UUID
		orgID, err = cmdutil.ResolveOrgSlugOrID(ctx, client, org, "circleci runner resource-class list")
		if err != nil {
			return err
		}
		subject = orgID.String()
		classes, err = client.ListResourceClassesByOrg(ctx, orgID)
	} else {
		classes, err = client.ListResourceClassesByNamespace(ctx, namespace)
	}
	if err != nil {
		if httpcl.HasStatusCode(err, http.StatusNotFound) {
			return runnerNotEnabledErr()
		}
		return apiErr(err, subject)
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
		return iostream.PrintJSON(ctx, out)
	}

	if len(out) == 0 {
		iostream.Printf(ctx, "No resource classes found.\n")
		return nil
	}
	table := mdtable.New("Resource Class", "Description")
	for _, rc := range out {
		table.Row(rc.ResourceClass, rc.Description)
	}
	iostream.PrintMarkdown(ctx, "# Runner Resource Classes\n"+table.Render())
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
			ctx := cmd.Context()
			client, err := cmdutil.LoadClient(ctx)
			if err != nil {
				return err
			}
			return runResourceClassCreate(ctx, client, args[0], description, jsonOut)
		},
	}

	cmd.Flags().StringVar(&description, "description", "", "Human-readable description of the resource class")
	cmdutil.AddJSONFlag(cmd, &jsonOut)
	cmdutil.AddJQFlag(cmd)
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
		return iostream.PrintJSON(ctx, out)
	}

	var md strings.Builder
	md.WriteString("# Created Resource Class\n")
	_, _ = fmt.Fprintf(&md, "- Resource Class: %s\n", out.ResourceClass)
	if out.Description != "" {
		_, _ = fmt.Fprintf(&md, "- Description: %s\n", out.Description)
	}
	_, _ = fmt.Fprintf(&md, "- ID: `%s`\n", out.ID)
	iostream.PrintMarkdown(ctx, md.String())
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
			ctx := cmd.Context()
			client, err := cmdutil.LoadClient(ctx)
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
	if err := cmdutil.ConfirmOrForce(ctx, iostream.Get(ctx), force,
		fmt.Sprintf("Delete resource class %q? All tokens and runner connections will be removed.", resourceClass),
		clierrors.New("runner.delete_aborted", "Deletion aborted",
			"Resource class deletion was not confirmed.").
			WithExitCode(clierrors.ExitCancelled),
		clierrors.New("runner.delete_requires_force", "Deletion requires --force",
			fmt.Sprintf("Deleting resource class %q is irreversible.", resourceClass)).
			WithExitCode(clierrors.ExitCancelled),
	); err != nil {
		return err
	}

	if err := client.DeleteResourceClass(ctx, resourceClass); err != nil {
		return apiErr(err, resourceClass)
	}

	iostream.ErrPrintf(ctx, "%s Deleted resource class %s\n", iostream.SymbolOK(ctx), resourceClass)
	return nil
}
