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

package project

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"

	"github.com/CircleCI-Public/circleci-cli-v2/internal/apiclient"
	"github.com/CircleCI-Public/circleci-cli-v2/internal/cmdutil"
	clierrors "github.com/CircleCI-Public/circleci-cli-v2/internal/errors"
	"github.com/CircleCI-Public/circleci-cli-v2/internal/gitremote"
	"github.com/CircleCI-Public/circleci-cli-v2/internal/httpcl"
	"github.com/CircleCI-Public/circleci-cli-v2/internal/iostream"
)

func newEnvCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "envvar <command>",
		Short: "Manage project environment variables",
		Long: heredoc.Doc(`
			List, set, and delete environment variables for a CircleCI project.

			Environment variable values are masked in list output (shown as "xxxx").
			The full value is never retrievable after it has been set.

			For quick access, use the top-level alias:
			  circleci envvar list --project gh/org/repo
		`),
	}

	cmd.AddCommand(NewEnvListCmd())
	cmd.AddCommand(NewEnvSetCmd())
	cmd.AddCommand(NewEnvDeleteCmd())

	return cmd
}

// NewEnvListCmd returns the env list command. Exported so the top-level alias
// in internal/cmd/env/ can reuse it directly.
func NewEnvListCmd() *cobra.Command {
	var projectSlug string
	var jsonOut bool

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List project environment variables",
		Long: heredoc.Doc(`
			List the environment variables defined for a CircleCI project.

			Values are always masked in the response (shown as "xxxx") — CircleCI
			does not expose secret values after they are set.

			The project is inferred from the current git repository's remote
			unless overridden with --project.

			JSON fields: name, value
		`),
		Example: heredoc.Doc(`
			# List env vars for the current project
			$ circleci envvar list

			# List env vars for a specific project
			$ circleci envvar list --project gh/myorg/myrepo

			# Output as JSON
			$ circleci envvar list --json
		`),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := iostream.FromCmd(cmd.Context(), cmd)

			client, err := cmdutil.LoadClient(ctx, cmd)
			if err != nil {
				return err
			}

			return RunEnvList(ctx, client, projectSlug, jsonOut)
		},
	}

	cmd.Flags().StringVar(&projectSlug, "project", "", "Project slug (e.g. gh/org/repo); defaults to git remote")
	cmd.Flags().BoolVar(&jsonOut, "json", false, "Output as JSON")

	return cmd
}

// RunEnvList is the business logic for listing env vars. Exported for reuse by
// the top-level circleci env alias.
func RunEnvList(ctx context.Context, client *apiclient.Client, projectSlug string, jsonOut bool) error {
	if projectSlug == "" {
		info, err := gitremote.Detect()
		if err != nil {
			return cmdutil.GitDetectErr(err, "Or specify the project: circleci envvar list --project gh/org/repo")
		}
		projectSlug = info.Slug
	}

	vars, err := client.ListEnvVars(ctx, projectSlug)
	if err != nil {
		if httpcl.HasStatusCode(err, http.StatusNotFound) {
			return clierrors.New("project.not_found", "Project not found",
				fmt.Sprintf("No project found for %q.", projectSlug)).
				WithExitCode(clierrors.ExitNotFound)
		}
		return cmdutil.APIErr(err, projectSlug, "project.not_found", "No project found for %q.")
	}

	if jsonOut {
		return cmdutil.WriteJSON(iostream.Out(ctx), vars)
	}

	if len(vars) == 0 {
		iostream.ErrPrintln(ctx, "No environment variables found.")
		return nil
	}

	const createdHeader = "Created At"
	nameW := len("Name")
	valW := len("Value")
	createdW := len(createdHeader)
	for _, v := range vars {
		if len(v.Name) > nameW {
			nameW = len(v.Name)
		}
		if len(v.Value) > valW {
			valW = len(v.Value)
		}
		if v.CreatedAt != nil {
			if n := len(v.CreatedAt.Format(time.RFC3339)); n > createdW {
				createdW = n
			}
		}
	}

	var md strings.Builder
	md.WriteString("# Environment Variables\n")
	_, _ = fmt.Fprintf(&md, "| %-*s | %-*s | %-*s |\n", nameW, "Name", valW, "Value", createdW, createdHeader)
	_, _ = fmt.Fprintf(&md, "| %s | %s | %s |\n", strings.Repeat("-", nameW), strings.Repeat("-", valW), strings.Repeat("-", createdW))
	for _, v := range vars {
		created := ""
		if v.CreatedAt != nil {
			created = v.CreatedAt.Format(time.RFC3339)
		}
		_, _ = fmt.Fprintf(&md, "| %-*s | %-*s | %-*s |\n", nameW, v.Name, valW, v.Value, createdW, created)
	}
	iostream.PrintMarkdown(ctx, md.String())
	return nil
}

// NewEnvSetCmd returns the env set command. Exported so the top-level alias
// in internal/cmd/env/ can reuse it.
func NewEnvSetCmd() *cobra.Command {
	var projectSlug string

	cmd := &cobra.Command{
		Use:   "set <name> <value>",
		Short: "Set a project environment variable",
		Long: heredoc.Doc(`
			Create or update an environment variable for a CircleCI project.

			If the variable already exists it will be overwritten. The value
			is never retrievable after being set — CircleCI masks it in all
			subsequent list responses.

			The project is inferred from the current git repository's remote
			unless overridden with --project.
		`),
		Example: heredoc.Doc(`
			# Set an env var for the current project
			$ circleci envvar set MY_SECRET s3cr3t

			# Set an env var for a specific project
			$ circleci envvar set MY_SECRET s3cr3t --project gh/myorg/myrepo

			# Read a value from a file
			$ circleci envvar set MY_SECRET "$(cat secret.txt)"
		`),
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := iostream.FromCmd(cmd.Context(), cmd)

			client, err := cmdutil.LoadClient(ctx, cmd)
			if err != nil {
				return err
			}
			return RunEnvSet(ctx, client, projectSlug, args[0], args[1])
		},
	}

	cmd.Flags().StringVar(&projectSlug, "project", "", "Project slug (e.g. gh/org/repo); defaults to git remote")

	return cmd
}

// RunEnvSet is the business logic for setting an env var.
func RunEnvSet(ctx context.Context, client *apiclient.Client, projectSlug, name, value string) error {
	if projectSlug == "" {
		info, err := gitremote.Detect()
		if err != nil {
			return cmdutil.GitDetectErr(err, "Or specify the project: circleci envvar set --project gh/org/repo")
		}
		projectSlug = info.Slug
	}

	if _, err := client.SetEnvVar(ctx, projectSlug, name, value); err != nil {
		if httpcl.HasStatusCode(err, http.StatusNotFound) {
			return clierrors.New("project.not_found", "Project not found",
				fmt.Sprintf("No project found for %q.", projectSlug)).
				WithExitCode(clierrors.ExitNotFound)
		}
		return cmdutil.APIErr(err, projectSlug, "project.not_found", "No project found for %q.")
	}

	iostream.Printf(ctx, "%s Set %s\n", iostream.Symbol(ctx, "✓", "OK:"), name)
	return nil
}

// NewEnvDeleteCmd returns the env delete command. Exported so the top-level
// alias in internal/cmd/env/ can reuse it.
func NewEnvDeleteCmd() *cobra.Command {
	var projectSlug string
	var force bool

	cmd := &cobra.Command{
		Use:   "delete <name>",
		Short: "Delete a project environment variable",
		Long: heredoc.Doc(`
			Delete an environment variable from a CircleCI project.

			This action is irreversible. The variable will be removed and any
			jobs that reference it will fail until a new value is set.

			The project is inferred from the current git repository's remote
			unless overridden with --project.

			In a terminal, you will be prompted to confirm before deleting.
			Use --force (-f) to skip the prompt for scripting.
		`),
		Example: heredoc.Doc(`
			# Delete an env var from the current project (with confirmation)
			$ circleci envvar delete MY_SECRET

			# Delete without confirmation
			$ circleci envvar delete MY_SECRET --force

			# Delete an env var from a specific project
			$ circleci envvar delete MY_SECRET --project gh/myorg/myrepo --force
		`),
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := iostream.FromCmd(cmd.Context(), cmd)

			client, err := cmdutil.LoadClient(ctx, cmd)
			if err != nil {
				return err
			}

			return RunEnvDelete(ctx, client, projectSlug, args[0], force)
		},
	}

	cmd.Flags().StringVar(&projectSlug, "project", "", "Project slug (e.g. gh/org/repo); defaults to git remote")
	cmd.Flags().BoolVarP(&force, "force", "f", false, "skip confirmation prompt")

	return cmd
}

// RunEnvDelete is the business logic for deleting an env var.
func RunEnvDelete(ctx context.Context, client *apiclient.Client, projectSlug, name string, force bool) error {
	if err := cmdutil.ConfirmOrForce(ctx, iostream.Get(ctx), force,
		fmt.Sprintf("Delete environment variable %q? This cannot be undone.", name),
		clierrors.New("envvar.delete_aborted", "Deletion aborted",
			"Environment variable deletion was not confirmed.").
			WithExitCode(clierrors.ExitCancelled),
		clierrors.New("envvar.delete_requires_force", "Deletion requires --force",
			fmt.Sprintf("Deleting environment variable %q is irreversible.", name)).
			WithExitCode(clierrors.ExitCancelled),
	); err != nil {
		return err
	}

	if projectSlug == "" {
		info, err := gitremote.Detect()
		if err != nil {
			return cmdutil.GitDetectErr(err, "Or specify the project: circleci envvar delete --project gh/org/repo")
		}
		projectSlug = info.Slug
	}

	if err := client.DeleteEnvVar(ctx, projectSlug, name); err != nil {
		if httpcl.HasStatusCode(err, http.StatusNotFound) {
			return clierrors.New("envvar.not_found", "Environment variable not found",
				fmt.Sprintf("No environment variable %q found in project %q.", name, projectSlug)).
				WithExitCode(clierrors.ExitNotFound)
		}
		return cmdutil.APIErr(err, name, "envvar.not_found", "No environment variable %q found.")
	}

	iostream.Printf(ctx, "%s Deleted %s\n", iostream.Symbol(ctx, "✓", "OK:"), name)
	return nil
}
