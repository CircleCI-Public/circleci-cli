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
	"time"

	"github.com/MakeNowJust/heredoc"
	"github.com/google/uuid"
	"github.com/spf13/cobra"

	"github.com/CircleCI-Public/circleci-cli/internal/apiclient"
	"github.com/CircleCI-Public/circleci-cli/internal/cmdutil"
	clierrors "github.com/CircleCI-Public/circleci-cli/internal/errors"
	"github.com/CircleCI-Public/circleci-cli/internal/iostream"
	"github.com/CircleCI-Public/circleci-cli/internal/mdtable"
)

func newSecretCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "secret <command>",
		Short: "Manage context environment variables",
		Long: heredoc.Doc(`
			List, set, and delete environment variables stored in a CircleCI context.

			Context environment variables are injected into jobs that reference the
			context. Variable values are never returned by the API after being set.
		`),
	}

	cmd.AddCommand(newSecretListCmd())
	cmd.AddCommand(newSecretSetCmd())
	cmd.AddCommand(newSecretDeleteCmd())

	return cmd
}

// --- secret list ---

func newSecretListCmd() *cobra.Command {
	var (
		jsonOut bool
		orgSlug string
	)

	cmd := &cobra.Command{
		Use:     "list <context-id|context-name>",
		Aliases: []string{"ls"},
		Short:   "List environment variables in a context",
		Annotations: map[string]string{
			"help:arguments": heredoc.Doc(`
				A context can be specified in the form:
				- "context-name"
				- by ID, e.g. 849e7902-802f-4082-8a70-da77dcd084e3
			`),
		},
		Long: heredoc.Doc(`
			List the environment variable names stored in a CircleCI context.

			Variable values are never returned by the API — CircleCI does not
			expose secret values after they are set.

			Pass a UUID to identify the context by ID, or a name to look up by name.
			When looking up by name, pass --org or run from a git repository so the
			organization can be inferred from the remote.

			JSON fields: variable, truncated_value, context_id, created_at, updated_at
		`),
		Example: heredoc.Doc(`
			# List env vars in a context by UUID
			$ circleci context secret list ctx-uuid-here

			# List env vars by context name (org inferred from git remote)
			$ circleci context secret list my-context

			# List env vars by context name in a specific org
			$ circleci context secret list my-context --org gh/myorg

			# Get variable names only
			$ circleci context secret list ctx-uuid-here --json --jq '.[].variable'
		`),
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := cmdutil.RequireArgs(args, "context-id"); err != nil {
				return err
			}
			ctx := cmd.Context()
			client, err := cmdutil.LoadClient(ctx)
			if err != nil {
				return err
			}
			contextID, err := resolveContextArg(ctx, client, args[0], orgSlug,
				"circleci context secret list")
			if err != nil {
				return err
			}
			return runSecretList(ctx, client, contextID, jsonOut)
		},
	}

	cmd.Flags().StringVar(&orgSlug, "org", "", "Organization slug (e.g. gh/myorg); used when resolving name to ID")
	cmdutil.AddJSONFlag(cmd, &jsonOut)
	cmdutil.AddJQFlag(cmd)

	return cmd
}

type secretListEntry struct {
	Variable       string    `json:"variable"`
	TruncatedValue string    `json:"truncated_value,omitempty"`
	ContextID      uuid.UUID `json:"context_id"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

func runSecretList(ctx context.Context, client *apiclient.Client, contextID string, jsonOut bool) error {
	vars, err := client.ListContextEnvVars(ctx, contextID)
	if err != nil {
		return secretAPIErr(err, contextID)
	}

	entries := make([]secretListEntry, len(vars))
	for i, v := range vars {
		entries[i] = secretListEntry{
			Variable:       v.Variable,
			TruncatedValue: v.TruncatedValue,
			ContextID:      v.ContextID,
			CreatedAt:      v.CreatedAt,
			UpdatedAt:      v.UpdatedAt,
		}
	}

	if jsonOut {
		return iostream.PrintJSON(ctx, entries)
	}

	if len(vars) == 0 {
		iostream.ErrPrintln(ctx, "No environment variables found.")
		return nil
	}

	tbl := mdtable.New("Variable", "Value", "Created", "Updated")
	for _, e := range entries {
		tbl.Row(e.Variable, "`****"+e.TruncatedValue+"`", e.CreatedAt.Format(time.RFC3339), e.UpdatedAt.Format(time.RFC3339))
	}
	iostream.PrintMarkdown(ctx, "# Environment Variables\n"+tbl.Render())
	return nil
}

// --- secret set ---

func newSecretSetCmd() *cobra.Command {
	var (
		name    string
		value   string
		orgSlug string
	)

	cmd := &cobra.Command{
		Use:   "set <context-id|context-name>",
		Short: "Set an environment variable in a context",
		Annotations: map[string]string{
			"help:arguments": heredoc.Doc(`
				A context can be specified in the form:
				- "context-name"
				- by ID, e.g. 849e7902-802f-4082-8a70-da77dcd084e3
			`),
		},
		Long: heredoc.Doc(`
			Add or update an environment variable in a CircleCI context.

			If the variable already exists its value will be overwritten. The value
			is never retrievable after being set — CircleCI masks it in all
			subsequent list responses.

			Pass a UUID to identify the context by ID, or a name to look up by name.
			When looking up by name, pass --org or run from a git repository so the
			organization can be inferred from the remote.

			In a terminal, --value may be omitted and the value will be prompted
			interactively with input masking.
		`),
		Example: heredoc.Doc(`
			# Set an environment variable by context UUID (value prompted)
			$ circleci context secret set ctx-uuid-here --name MY_SECRET

			# Set an environment variable by context name
			$ circleci context secret set my-context --org gh/myorg --name MY_SECRET --value s3cr3t

			# Read a value from a file
			$ circleci context secret set ctx-uuid-here --name MY_SECRET --value "$(cat secret.txt)"

			# Read a value from stdin
			$ circleci context secret set ctx-uuid-here --name MY_SECRET --value "$(cat)"
		`),
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := cmdutil.RequireArgs(args, "context-id"); err != nil {
				return err
			}
			if name == "" {
				return cmdutil.RequireArgs(nil, "name")
			}
			ctx := cmd.Context()
			if value == "" {
				if !iostream.IsInteractive(ctx) {
					return clierrors.New("context.secret_value_required", "Secret value required",
						"--value is required in non-interactive mode.").
						WithSuggestions("Pass --value <secret> or run in a terminal to be prompted.").
						WithExitCode(clierrors.ExitBadArguments)
				}
				var err error
				value, err = iostream.PromptSecret(ctx, "Enter value for "+name)
				if err != nil {
					return err
				}
				if value == "" {
					return clierrors.New("context.secret_aborted", "Aborted",
						"No value was entered.").
						WithExitCode(clierrors.ExitCancelled)
				}
			}
			client, err := cmdutil.LoadClient(ctx)
			if err != nil {
				return err
			}
			contextID, err := resolveContextArg(ctx, client, args[0], orgSlug,
				"circleci context secret set")
			if err != nil {
				return err
			}
			return runSecretSet(ctx, client, contextID, name, value)
		},
	}

	cmd.Flags().StringVar(&name, "name", "", "Name of the environment variable")
	cmd.Flags().StringVar(&value, "value", "", "Value of the environment variable (prompted if omitted in a terminal)")
	cmd.Flags().StringVar(&orgSlug, "org", "", "Organization slug (e.g. gh/myorg); used when resolving name to ID")

	return cmd
}

func runSecretSet(ctx context.Context, client *apiclient.Client, contextID, name, value string) error {
	if _, err := client.SetContextEnvVar(ctx, contextID, name, value); err != nil {
		return secretAPIErr(err, contextID)
	}

	iostream.Printf(ctx, "%s Set %s\n", iostream.SymbolOK(ctx), name)
	return nil
}

// --- secret delete ---

func newSecretDeleteCmd() *cobra.Command {
	var (
		name    string
		orgSlug string
		force   bool
	)

	cmd := &cobra.Command{
		Use:     "delete <context-id|context-name>",
		Aliases: []string{"rm"},
		Short:   "Delete an environment variable from a context",
		Annotations: map[string]string{
			"help:arguments": heredoc.Doc(`
				A context can be specified in the form:
				- "context-name"
				- by ID, e.g. 849e7902-802f-4082-8a70-da77dcd084e3
			`),
		},
		Long: heredoc.Doc(`
			Remove an environment variable from a CircleCI context.

			This action is irreversible. Jobs that depend on this variable will
			fail until a new value is set.

			Pass a UUID to identify the context by ID, or a name to look up by name.
			When looking up by name, pass --org or run from a git repository so the
			organization can be inferred from the remote.

			In a terminal, you will be prompted to confirm before deleting.
			Use --force (-f) to skip the prompt for scripting.
		`),
		Example: heredoc.Doc(`
			# Delete a variable (with confirmation)
			$ circleci context secret delete ctx-uuid-here --name MY_SECRET

			# Delete without confirmation
			$ circleci context secret delete ctx-uuid-here --name MY_SECRET --force
		`),
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := cmdutil.RequireArgs(args, "context-id"); err != nil {
				return err
			}
			if name == "" {
				return cmdutil.RequireArgs(nil, "name")
			}
			ctx := cmd.Context()
			client, err := cmdutil.LoadClient(ctx)
			if err != nil {
				return err
			}
			contextID, err := resolveContextArg(ctx, client, args[0], orgSlug,
				"circleci context secret delete")
			if err != nil {
				return err
			}
			return runSecretDelete(ctx, client, contextID, name, force)
		},
	}

	cmd.Flags().StringVar(&name, "name", "", "Name of the environment variable to delete")
	cmd.Flags().BoolVarP(&force, "force", "f", false, "skip confirmation prompt")
	cmd.Flags().StringVar(&orgSlug, "org", "", "Organization slug (e.g. gh/myorg); used when resolving name to ID")

	return cmd
}

func runSecretDelete(ctx context.Context, client *apiclient.Client, contextID, name string, force bool) error {
	if err := cmdutil.ConfirmOrForce(ctx, iostream.Get(ctx), force,
		fmt.Sprintf("Delete environment variable %q from context? This cannot be undone.", name),
		clierrors.New("context.secret_delete_aborted", "Deletion aborted",
			"Environment variable deletion was not confirmed.").
			WithExitCode(clierrors.ExitCancelled),
		clierrors.New("context.secret_delete_requires_force", "Deletion requires --force",
			fmt.Sprintf("Deleting environment variable %q is irreversible.", name)).
			WithExitCode(clierrors.ExitCancelled),
	); err != nil {
		return err
	}

	if err := client.DeleteContextEnvVar(ctx, contextID, name); err != nil {
		return secretAPIErr(err, name)
	}

	iostream.Printf(ctx, "%s Deleted %s\n", iostream.SymbolOK(ctx), name)
	return nil
}

func secretAPIErr(err error, subject string) *clierrors.CLIError {
	return cmdutil.APIErr(err, subject,
		"context.secret_not_found", "No environment variable found for %q.",
		"Check the context ID and variable name and try again")
}

// resolveContextArg resolves a context UUID or name to a UUID string.
// If arg is a valid UUID it is returned as-is. Otherwise it looks up by name,
// requiring orgSlug or a detectable git remote. cmdName is used in the
// git-detection error suggestion (e.g. "circleci context secret list").
func resolveContextArg(ctx context.Context, client *apiclient.Client, arg, orgSlug, cmdName string) (string, error) {
	if _, err := uuid.Parse(arg); err == nil {
		return arg, nil
	}
	orgSlug, err := cmdutil.ResolveOrgSlug(orgSlug, cmdName)
	if err != nil {
		return "", err
	}
	id, err := resolveContextID(ctx, client, arg, orgSlug)
	if err != nil {
		return "", err
	}
	return id.String(), nil
}
