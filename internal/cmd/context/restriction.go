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

	"github.com/CircleCI-Public/circleci-cli/internal/apiclient"
	"github.com/CircleCI-Public/circleci-cli/internal/cmdutil"
	clierrors "github.com/CircleCI-Public/circleci-cli/internal/errors"
	"github.com/CircleCI-Public/circleci-cli/internal/iostream"
)

func newRestrictionCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "restriction <command>",
		Short: "Manage context restrictions",
		Long: heredoc.Doc(`
			Add and remove restrictions that control which projects and groups
			can use a CircleCI context.

			Restrictions scope context access to specific projects, pipeline
			expressions, or VCS groups. A context with no restrictions is
			accessible to all members of the organization.
		`),
	}

	cmd.AddCommand(newRestrictionCreateCmd())
	cmd.AddCommand(newRestrictionDeleteCmd())

	return cmd
}

// --- restriction create ---

type restrictionCreateOutput struct {
	ID               string `json:"id"`
	Name             string `json:"name"`
	RestrictionType  string `json:"restriction_type"`
	RestrictionValue string `json:"restriction_value"`
}

func newRestrictionCreateCmd() *cobra.Command {
	var (
		restrictionType  string
		restrictionValue string
		orgSlug          string
		jsonOut          bool
	)

	cmd := &cobra.Command{
		Use:   "create <context-id|context-name>",
		Short: "Add a restriction to a context",
		Annotations: map[string]string{
			"help:arguments": heredoc.Doc(`
				A context can be specified in the form:
				- "context-name"
				- by ID, e.g. 849e7902-802f-4082-8a70-da77dcd084e3
			`),
		},
		Long: heredoc.Doc(`
			Add a restriction to a CircleCI context.

			Use --type to specify the restriction type: project, expression, or group.
			Use --value to provide the restriction value.

			Pass a UUID to identify the context by ID, or a name to look up by name.
			When looking up by name, pass --org or run from a git repository so the
			organization can be inferred from the remote.

			JSON fields: id, name, restriction_type, restriction_value
		`),
		Example: heredoc.Doc(`
			# Restrict context to a specific project
			$ circleci context restriction create ctx-uuid --type project --value proj-uuid

			# Restrict context to a specific group
			$ circleci context restriction create ctx-uuid --type group --value group-uuid

			# Restrict context using a pipeline expression
			$ circleci context restriction create ctx-uuid --type expression --value 'pipeline.git.branch == "main"'

			# Capture the restriction ID
			$ circleci context restriction create ctx-uuid --type project --value proj-uuid --json --jq '.id'
		`),
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := cmdutil.RequireArgs(args, "context-id"); err != nil {
				return err
			}
			if restrictionType == "" {
				return cmdutil.RequireArgs(nil, "type")
			}
			if restrictionValue == "" {
				return cmdutil.RequireArgs(nil, "value")
			}
			switch restrictionType {
			case "project", "expression", "group":
			default:
				return clierrors.New("args.invalid_restriction_type", "Invalid restriction type",
					fmt.Sprintf("%q is not a valid restriction type. Must be one of: project, expression, group.", restrictionType)).
					WithExitCode(clierrors.ExitBadArguments)
			}
			ctx := cmd.Context()
			client, err := cmdutil.LoadClient(ctx)
			if err != nil {
				return err
			}
			contextID, err := uuid.Parse(args[0])
			if err != nil {
				orgSlug, err = cmdutil.ResolveOrgSlug(orgSlug, "circleci context restriction create")
				if err != nil {
					return err
				}
				id, err := resolveContextID(ctx, client, args[0], orgSlug)
				if err != nil {
					return err
				}
				contextID = id
			}
			return runRestrictionCreate(ctx, client, contextID, restrictionType, restrictionValue, jsonOut)
		},
	}

	cmd.Flags().StringVar(&restrictionType, "type", "", "Restriction type: project, expression, or group")
	cmd.Flags().StringVar(&restrictionValue, "value", "", "Value of the restriction")
	cmd.Flags().StringVar(&orgSlug, "org", "", "Organization slug (e.g. gh/myorg); used when resolving name to ID")
	cmdutil.AddJSONFlag(cmd, &jsonOut)
	cmdutil.AddJQFlag(cmd)

	return cmd
}

func runRestrictionCreate(ctx context.Context, client *apiclient.Client, contextID uuid.UUID, restrictionType, value string, jsonOut bool) error {
	r, err := client.CreateContextRestriction(ctx, contextID, restrictionType, value)
	if err != nil {
		return restrictionAPIErr(err, contextID.String())
	}

	out := restrictionCreateOutput{
		ID:               r.ID.String(),
		Name:             r.Name,
		RestrictionType:  r.RestrictionType,
		RestrictionValue: r.RestrictionValue,
	}

	if jsonOut {
		return iostream.PrintJSON(ctx, out)
	}

	iostream.Printf(ctx, "%s Created %s restriction %s\n",
		iostream.SymbolOK(ctx), out.RestrictionType, out.ID)
	return nil
}

// --- restriction delete ---

func newRestrictionDeleteCmd() *cobra.Command {
	var (
		restrictionID string
		orgSlug       string
		force         bool
	)

	cmd := &cobra.Command{
		Use:     "delete <context-id|context-name>",
		Aliases: []string{"rm"},
		Short:   "Delete a restriction from a context",
		Annotations: map[string]string{
			"help:arguments": heredoc.Doc(`
				A context can be specified in the form:
				- "context-name"
				- by ID, e.g. 849e7902-802f-4082-8a70-da77dcd084e3
			`),
		},
		Long: heredoc.Doc(`
			Remove a restriction from a CircleCI context.

			This action is irreversible. Once removed, the context will be
			accessible to any project or group that was previously blocked.

			Pass a UUID to identify the context by ID, or a name to look up by name.
			When looking up by name, pass --org or run from a git repository so the
			organization can be inferred from the remote.

			In a terminal, you will be prompted to confirm before deleting.
			Use --force (-f) to skip the prompt for scripting.
		`),
		Example: heredoc.Doc(`
			# Delete a restriction (with confirmation)
			$ circleci context restriction delete ctx-uuid --restriction-id r-uuid

			# Delete without confirmation
			$ circleci context restriction delete ctx-uuid --restriction-id r-uuid --force
		`),
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := cmdutil.RequireArgs(args, "context-id"); err != nil {
				return err
			}
			rID, err := uuid.Parse(restrictionID)
			if err != nil {
				return clierrors.New("args.invalid_restriction_id", "Invalid restriction ID",
					fmt.Sprintf("%q is not a valid UUID.", restrictionID)).
					WithExitCode(clierrors.ExitBadArguments)
			}
			ctx := cmd.Context()
			if err := cmdutil.ConfirmOrForce(ctx, iostream.Get(ctx), force,
				fmt.Sprintf("Delete restriction %s from context? This cannot be undone.", restrictionID),
				clierrors.New("context.restriction_delete_aborted", "Deletion aborted",
					"Restriction deletion was not confirmed.").
					WithExitCode(clierrors.ExitCancelled),
				clierrors.New("context.restriction_delete_requires_force", "Deletion requires --force",
					fmt.Sprintf("Deleting restriction %s is irreversible.", restrictionID)).
					WithExitCode(clierrors.ExitCancelled),
			); err != nil {
				return err
			}
			client, err := cmdutil.LoadClient(ctx)
			if err != nil {
				return err
			}
			contextID, err := uuid.Parse(args[0])
			if err != nil {
				orgSlug, err = cmdutil.ResolveOrgSlug(orgSlug, "circleci context restriction delete")
				if err != nil {
					return err
				}
				id, err := resolveContextID(ctx, client, args[0], orgSlug)
				if err != nil {
					return err
				}
				contextID = id
			}
			if err := client.DeleteContextRestriction(ctx, contextID, rID); err != nil {
				return restrictionAPIErr(err, restrictionID)
			}
			iostream.Printf(ctx, "%s Deleted restriction %s\n",
				iostream.SymbolOK(ctx), restrictionID)
			return nil
		},
	}

	cmd.Flags().StringVar(&restrictionID, "restriction-id", "", "UUID of the restriction to delete")
	cmd.Flags().BoolVarP(&force, "force", "f", false, "Skip confirmation prompt")
	cmd.Flags().StringVar(&orgSlug, "org", "", "Organization slug (e.g. gh/myorg); used when resolving name to ID")
	_ = cmd.MarkFlagRequired("restriction-id")

	return cmd
}

func restrictionAPIErr(err error, subject string) *clierrors.CLIError {
	return cmdutil.APIErr(err, subject,
		"context.restriction_not_found", "No restriction found for %q.",
		"Check the context ID and try again",
		"Run: circleci context get <context-id>")
}
