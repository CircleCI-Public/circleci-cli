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

	"github.com/CircleCI-Public/circleci-cli-v2/internal/cmdutil"
	clierrors "github.com/CircleCI-Public/circleci-cli-v2/internal/errors"
	"github.com/CircleCI-Public/circleci-cli-v2/internal/gitremote"
	"github.com/CircleCI-Public/circleci-cli-v2/internal/httpcl"
	"github.com/CircleCI-Public/circleci-cli-v2/internal/iostream"
	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"
)

func newTokenCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "token <command>",
		Short: "Manage runner tokens",
		Long: heredoc.Doc(`
			Manage CircleCI runner authentication tokens.

			Tokens are used by runner agents to authenticate with CircleCI.
			Each token is associated with a specific resource class.

			Token values are only shown once at creation time and cannot be retrieved afterwards.
		`),
	}

	cmd.AddCommand(newTokenListCmd())
	cmd.AddCommand(newTokenCreateCmd())
	cmd.AddCommand(newTokenDeleteCmd())

	return cmd
}

// --- token list ---

func newTokenListCmd() *cobra.Command {
	var resourceClass string
	var jsonOut bool

	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "List tokens for a resource class",
		Long: heredoc.Doc(`
			List authentication tokens for runner resource classes.

			Without --resource-class, lists tokens for all resource classes
			you have access to.

			Token values are never shown after creation. This command lists
			token metadata (ID, nickname, creation date) only.

			JSON fields: id, resource_class, nickname, created_at
		`),
		Example: heredoc.Doc(`
			# List tokens across all resource classes
			$ circleci runner token list

			# List tokens for a specific resource class
			$ circleci runner token list --resource-class my-org/my-runner

			# Output as JSON
			$ circleci runner token list --json

			# Pipe to jq to extract IDs
			$ circleci runner token list --resource-class my-org/my-runner --json | jq '.[].id'
		`),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			streams := iostream.FromCmd(cmd)
			return runTokenList(ctx, streams, resourceClass, jsonOut)
		},
	}

	cmd.Flags().StringVar(&resourceClass, "resource-class", "", "Filter by resource class (namespace/name)")
	cmd.Flags().BoolVar(&jsonOut, "json", false, "Output as JSON")
	return cmd
}

type tokenOutput struct {
	ID            string `json:"id"`
	ResourceClass string `json:"resource_class"`
	Nickname      string `json:"nickname"`
	CreatedAt     string `json:"created_at"`
}

func runTokenList(ctx context.Context, streams iostream.Streams, resourceClass string, jsonOut bool) error {
	client, cliErr := cmdutil.LoadClient()
	if cliErr != nil {
		return cliErr
	}

	var resourceClasses []string
	if resourceClass != "" {
		resourceClasses = []string{resourceClass}
	} else {
		namespace, err := gitremote.DetectNamespace()
		if err != nil {
			return clierrors.New("runner.namespace_required", "Namespace required",
				"Could not detect organization namespace from git remote.").
				WithSuggestions("Specify a resource class: circleci runner token list --resource-class <namespace/name>").
				WithExitCode(clierrors.ExitBadArguments)
		}
		classes, apiErr2 := client.ListResourceClasses(ctx, namespace)
		if apiErr2 != nil {
			if httpcl.HasStatusCode(apiErr2, http.StatusNotFound) {
				return runnerNotEnabledErr()
			}
			return apiErr(apiErr2, namespace)
		}
		for _, rc := range classes {
			resourceClasses = append(resourceClasses, rc.ResourceClass)
		}
	}

	var out []tokenOutput
	for _, rc := range resourceClasses {
		tokens, err := client.ListRunnerTokens(ctx, rc)
		if err != nil {
			return apiErr(err, rc)
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

	if jsonOut {
		if out == nil {
			out = []tokenOutput{}
		}
		enc := json.NewEncoder(streams.Out)
		enc.SetIndent("", "  ")
		return enc.Encode(out)
	}

	if len(out) == 0 {
		if resourceClass != "" {
			streams.Printf("No tokens found for %s.\n", resourceClass)
		} else {
			streams.Printf("No tokens found.\n")
		}
		return nil
	}
	for _, t := range out {
		streams.Printf("%-36s  %-20s  %s\n", t.ID, t.Nickname, t.CreatedAt)
	}
	return nil
}

// --- token create ---

func newTokenCreateCmd() *cobra.Command {
	var nickname string
	var jsonOut bool

	cmd := &cobra.Command{
		Use:   "create <resource-class>",
		Short: "Create a token for a resource class",
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
			streams := iostream.FromCmd(cmd)
			return runTokenCreate(ctx, streams, args[0], nickname, jsonOut)
		},
	}

	cmd.Flags().StringVar(&nickname, "nickname", "", "Human-readable nickname for the token")
	cmd.Flags().BoolVar(&jsonOut, "json", false, "Output as JSON")
	return cmd
}

type tokenCreateOutput struct {
	ID            string `json:"id"`
	ResourceClass string `json:"resource_class"`
	Nickname      string `json:"nickname"`
	CreatedAt     string `json:"created_at"`
	Token         string `json:"token"`
}

func runTokenCreate(ctx context.Context, streams iostream.Streams, resourceClass, nickname string, jsonOut bool) error {
	client, cliErr := cmdutil.LoadClient()
	if cliErr != nil {
		return cliErr
	}

	tok, err := client.CreateRunnerToken(ctx, resourceClass, nickname)
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
		enc := json.NewEncoder(streams.Out)
		enc.SetIndent("", "  ")
		return enc.Encode(out)
	}

	streams.Printf("Created token for resource class: %s\n", out.ResourceClass)
	if out.Nickname != "" {
		streams.Printf("Nickname:  %s\n", out.Nickname)
	}
	streams.Printf("ID:        %s\n", out.ID)
	streams.Printf("Created:   %s\n", out.CreatedAt)
	streams.Printf("\nToken (save this — it will not be shown again):\n%s\n", out.Token)
	return nil
}

// --- token delete ---

func newTokenDeleteCmd() *cobra.Command {
	var force bool

	cmd := &cobra.Command{
		Use:   "delete <token-id>",
		Short: "Delete a runner token",
		Long: heredoc.Doc(`
			Delete a CircleCI runner authentication token by its ID.

			Any runner agents using this token will immediately lose their ability
			to claim new jobs. Running jobs are not affected.

			Find token IDs with: circleci runner token list <resource-class>

			In a terminal, you will be prompted to confirm before deleting.
			Use --force (-f) to skip the prompt for scripting.
		`),
		Example: heredoc.Doc(`
			# Delete a token by ID (with confirmation prompt)
			$ circleci runner token delete abc12345-0000-0000-0000-000000000000

			# Delete without confirmation
			$ circleci runner token delete abc12345-0000-0000-0000-000000000000 --force

			# Delete in a script using JSON output
			$ ID=$(circleci runner token list --resource-class my-org/my-runner --json | jq -r '.[0].id')
			$ circleci runner token delete "$ID" --force
		`),
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if cliErr := cmdutil.RequireArgs(args, "token-id"); cliErr != nil {
				return cliErr
			}
			ctx := cmd.Context()
			streams := iostream.FromCmd(cmd)
			return runTokenDelete(ctx, streams, args[0], force)
		},
	}

	cmd.Flags().BoolVarP(&force, "force", "f", false, "skip confirmation prompt")
	return cmd
}

func runTokenDelete(ctx context.Context, streams iostream.Streams, tokenID string, force bool) error {
	if !force {
		if streams.IsInteractive() {
			prompt := fmt.Sprintf("Delete token %q? Agents using this token will lose the ability to claim new jobs.", tokenID)
			if !streams.Confirm(prompt) {
				return clierrors.New("runner.delete_aborted", "Deletion aborted",
					"Token deletion was not confirmed.").
					WithExitCode(clierrors.ExitCancelled)
			}
		} else {
			return clierrors.New("runner.delete_requires_force", "Deletion requires --force",
				fmt.Sprintf("Deleting token %q is irreversible.", tokenID)).
				WithSuggestions("Pass --force (-f) to confirm deletion in non-interactive mode").
				WithExitCode(clierrors.ExitCancelled)
		}
	}

	client, cliErr := cmdutil.LoadClient()
	if cliErr != nil {
		return cliErr
	}

	if err := client.DeleteRunnerToken(ctx, tokenID); err != nil {
		return apiErr(err, tokenID)
	}

	streams.Printf("Deleted token: %s\n", tokenID)
	return nil
}
