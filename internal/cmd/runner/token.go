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
	"errors"
	"fmt"
	"net/http"

	"github.com/MakeNowJust/heredoc"
	"github.com/google/uuid"
	"github.com/spf13/cobra"

	clierrors "github.com/CircleCI-Public/circleci-cli/clikit/errors"
	"github.com/CircleCI-Public/circleci-cli/clikit/iostream"
	"github.com/CircleCI-Public/circleci-cli/clikit/mdtable"
	"github.com/CircleCI-Public/circleci-cli/internal/apiclient"
	"github.com/CircleCI-Public/circleci-cli/internal/cmdutil"
	"github.com/CircleCI-Public/circleci-cli/internal/httpcl"
)

func newTokenCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "token <command>",
		Short: "Manage runner tokens",
		Long: heredoc.Doc(`
			Manage runner authentication tokens.

			Tokens are used by runner agents to authenticate with CircleCI.
			Each token is associated with a specific resource class.

			Token values are only shown once at creation time and cannot be retrieved afterwards.
		`),
	}

	cmdutil.AddGroup(cmd, "General commands",
		newTokenListCmd(),
		newTokenCreateCmd(),
	)
	cmdutil.AddGroup(cmd, "Targeted commands",
		newTokenDeleteCmd(),
	)

	return cmd
}

// --- token list ---

func newTokenListCmd() *cobra.Command {
	var resourceClass string
	var org string
	var jsonOut bool

	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "List tokens for a resource class",
		Long: heredoc.Doc(`
			List authentication tokens for runner resource classes.

			Without --resource-class, lists tokens for all resource classes
			in the organization (--org or inferred from the git remote).

			JSON fields: id, resource_class, nickname, created_at
		`),
		Example: heredoc.Doc(`
			# List tokens for all resource classes in the org inferred from the git remote
			$ circleci runner token list

			# List tokens for all resource classes in a specific org
			$ circleci runner token list --org gh/my-org

			# List tokens for a specific resource class
			$ circleci runner token list --resource-class my-org/my-runner

			# Output as JSON
			$ circleci runner token list --json

			# Extract IDs with --jq
			$ circleci runner token list --resource-class my-org/my-runner --json --jq '.[].id'
		`),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			client, err := cmdutil.LoadClient(ctx)
			if err != nil {
				return err
			}
			return runTokenList(ctx, client, org, resourceClass, jsonOut)
		},
	}

	cmdutil.AddOrgFlag(cmd, &org, cmdutil.OrgFlag{DefaultsToGitRemote: true})
	cmd.Flags().StringVar(&resourceClass, "resource-class", "", "Filter by resource class (namespace/name)")
	cmdutil.AddJSONFlag(cmd, &jsonOut)
	cmdutil.AddJQFlag(cmd)
	return cmd
}

type tokenOutput struct {
	ID            string `json:"id"`
	ResourceClass string `json:"resource_class"`
	Nickname      string `json:"nickname"`
	CreatedAt     string `json:"created_at"`
}

func runTokenList(ctx context.Context, client *apiclient.Client, org, resourceClass string, jsonOut bool) error {
	// rcs is the set of resource classes whose tokens we list, each carrying
	// both ID (for the V3 filter) and ResourceClass slug (for output reconstruction).
	var rcs []apiclient.ResourceClass

	if resourceClass != "" {
		rc, err := client.ResourceClassByName(ctx, resourceClass)
		if err != nil {
			if errors.Is(err, apiclient.ErrResourceClassNotFound) {
				// Deleted or nonexistent — treat as no tokens.
				return printTokenList(ctx, nil, resourceClass, jsonOut)
			}
			if httpcl.HasStatusCode(err, http.StatusNotFound) {
				return runnerNotEnabledErr()
			}
			return apiErr(err, resourceClass)
		}
		rcs = []apiclient.ResourceClass{*rc}
	} else {
		orgID, err := cmdutil.ResolveOrgSlugOrID(ctx, client, org, "circleci runner token list")
		if err != nil {
			return err
		}
		classes, err := client.ListResourceClassesByOrg(ctx, orgID)
		if err != nil {
			if httpcl.HasStatusCode(err, http.StatusNotFound) {
				return runnerNotEnabledErr()
			}
			return apiErr(err, orgID.String())
		}
		rcs = classes
	}

	var out []tokenOutput
	for _, rc := range rcs {
		rcID, err := uuid.Parse(rc.ID)
		if err != nil {
			return apiErr(err, rc.ResourceClass)
		}
		tokens, err := client.ListRunnerTokensV3(ctx, rcID, rc.ResourceClass)
		if err != nil {
			return apiErr(err, rc.ResourceClass)
		}
		for _, t := range tokens {
			out = append(out, tokenOutput{
				ID:            t.ID,
				ResourceClass: t.ResourceClass,
				Nickname:      t.Nickname,
				CreatedAt:     t.CreatedAt,
			})
		}
	}

	return printTokenList(ctx, out, resourceClass, jsonOut)
}

func printTokenList(ctx context.Context, out []tokenOutput, resourceClass string, jsonOut bool) error {
	if jsonOut {
		if out == nil {
			out = []tokenOutput{}
		}
		return iostream.PrintJSON(ctx, out)
	}

	if len(out) == 0 {
		if resourceClass != "" {
			iostream.Printf(ctx, "No tokens found for %s.\n", resourceClass)
		} else {
			iostream.Printf(ctx, "No tokens found.\n")
		}
		return nil
	}
	table := mdtable.New("ID", "Nickname", "Created")
	for _, t := range out {
		table.Row(t.ID, t.Nickname, t.CreatedAt)
	}
	iostream.PrintMarkdown(ctx, "# Runner Tokens\n"+table.Render())
	return nil
}

// --- token create ---

func newTokenCreateCmd() *cobra.Command {
	var nickname string
	var jsonOut bool

	cmd := &cobra.Command{
		Use:   "create <resource-class>",
		Short: "Create a token for a resource class",
		Annotations: map[string]string{
			"help:arguments": heredoc.Docf(`
				%[1]s<resource-class>%[1]s is the runner resource class to create a token for,
				in the form %[1]snamespace/name%[1]s (for example, %[1]smy-org/my-runner%[1]s).
			`, "`"),
		},
		Long: heredoc.Doc(`
			Create a new authentication token for a runner resource class.

			The token value is shown only once at creation time. Store it securely —
			it cannot be retrieved afterwards. If lost, delete this token and create
			a new one.

			JSON fields: id, resource_class, nickname, created_at, token
		`),
		Example: heredoc.Doc(`
			# Create a token for a resource class
			$ circleci runner token create my-org/my-runner

			# Create a token with a nickname
			$ circleci runner token create my-org/my-runner --nickname "prod-server-1"

			# Output as JSON (includes the token value)
			$ circleci runner token create my-org/my-runner --json
		`),
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if cliErr := cmdutil.RequireArgs(args, "resource-class"); cliErr != nil {
				return cliErr
			}
			ctx := cmd.Context()
			client, err := cmdutil.LoadClient(ctx)
			if err != nil {
				return err
			}
			return runTokenCreate(ctx, client, args[0], nickname, jsonOut)
		},
	}

	cmd.Flags().StringVar(&nickname, "nickname", "", "Human-readable nickname for the token")
	cmdutil.AddJSONFlag(cmd, &jsonOut)
	cmdutil.AddJQFlag(cmd)
	return cmd
}

type tokenCreateOutput struct {
	ID            string `json:"id"`
	ResourceClass string `json:"resource_class"`
	Nickname      string `json:"nickname"`
	CreatedAt     string `json:"created_at"`
	Token         string `json:"token"`
}

func runTokenCreate(ctx context.Context, client *apiclient.Client, resourceClass, nickname string, jsonOut bool) error {
	rc, err := client.ResourceClassByName(ctx, resourceClass)
	if err != nil {
		if errors.Is(err, apiclient.ErrResourceClassNotFound) {
			return clierrors.New("runner.not_found", "Not found",
				fmt.Sprintf("No runner resource class named %q.", resourceClass)).
				WithSuggestions("List available resource classes with: circleci runner resource-class list").
				WithExitCode(clierrors.ExitNotFound)
		}
		if httpcl.HasStatusCode(err, http.StatusNotFound) {
			return runnerNotEnabledErr()
		}
		return apiErr(err, resourceClass)
	}

	rcID, err := uuid.Parse(rc.ID)
	if err != nil {
		return apiErr(err, resourceClass)
	}

	tok, err := client.CreateRunnerTokenV3(ctx, rcID, resourceClass, nickname)
	if err != nil {
		return apiErr(err, resourceClass)
	}

	out := tokenCreateOutput{
		ID:            tok.ID,
		ResourceClass: tok.ResourceClass,
		Nickname:      tok.Nickname,
		CreatedAt:     tok.CreatedAt,
		Token:         tok.Token,
	}

	if jsonOut {
		return iostream.PrintJSON(ctx, out)
	}

	iostream.Printf(ctx, "Created token for resource class: %s\n", out.ResourceClass)
	if out.Nickname != "" {
		iostream.Printf(ctx, "Nickname:  %s\n", out.Nickname)
	}
	iostream.Printf(ctx, "ID:        %s\n", out.ID)
	iostream.Printf(ctx, "Created:   %s\n", out.CreatedAt)
	iostream.Printf(ctx, "\nToken (save this — it will not be shown again):\n%s\n", out.Token)
	return nil
}

// --- token delete ---

func newTokenDeleteCmd() *cobra.Command {
	var force bool
	var jsonOut bool

	cmd := &cobra.Command{
		Use:     "delete <token-id>",
		Aliases: []string{"rm"},
		Short:   "Delete a runner token",
		Annotations: map[string]string{
			"help:arguments": heredoc.Docf(`
				%[1]s<token-id>%[1]s is the ID of the token to delete (a UUID). Find token IDs
				with: %[1]scircleci runner token list --resource-class <namespace/name>%[1]s
			`, "`"),
			"destructiveHint": "true",
		},
		Long: heredoc.Doc(`
			Delete a CircleCI runner authentication token by its ID.

			Agents using this token can no longer claim jobs; running jobs continue.

			JSON fields: id
		`),
		Example: heredoc.Doc(`
			# Delete a token by ID (with confirmation prompt)
			$ circleci runner token delete abc12345-0000-0000-0000-000000000000

			# Delete without confirmation
			$ circleci runner token delete abc12345-0000-0000-0000-000000000000 --force

			# Delete in a script using JSON output
			$ ID=$(circleci runner token list --resource-class my-org/my-runner --json --jq '.[0].id')
			$ circleci runner token delete "$ID" --force --json
		`),
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if cliErr := cmdutil.RequireArgs(args, "token-id"); cliErr != nil {
				return cliErr
			}
			ctx := cmd.Context()
			client, err := cmdutil.LoadClient(ctx)
			if err != nil {
				return err
			}
			return runTokenDelete(ctx, client, args[0], force, jsonOut)
		},
	}

	cmd.Flags().BoolVarP(&force, "force", "f", false, "skip confirmation prompt")
	cmdutil.AddJSONFlag(cmd, &jsonOut)
	cmdutil.AddJQFlag(cmd)
	return cmd
}

type tokenDeleteOutput struct {
	ID string `json:"id"`
}

func runTokenDelete(ctx context.Context, client *apiclient.Client,
	tokenID string, force, jsonOut bool) error {
	if err := cmdutil.ConfirmOrForce(ctx, iostream.Get(ctx), force,
		fmt.Sprintf("Delete token %q? Agents using this token will lose the ability to claim new jobs.", tokenID),
		clierrors.New("runner.delete_aborted", "Deletion aborted",
			"Token deletion was not confirmed.").
			WithExitCode(clierrors.ExitCancelled),
		clierrors.New("runner.delete_requires_force", "Deletion requires --force",
			fmt.Sprintf("Deleting token %q is irreversible.", tokenID)).
			WithExitCode(clierrors.ExitCancelled),
	); err != nil {
		return err
	}

	if err := client.DeleteRunnerTokenV3(ctx, tokenID); err != nil {
		return apiErr(err, tokenID)
	}

	if jsonOut {
		return iostream.PrintJSON(ctx, tokenDeleteOutput{ID: tokenID})
	}

	iostream.Printf(ctx, "Deleted token: %s\n", tokenID)
	return nil
}
