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

			JSON fields: id, resource_class, resource_class_id, nickname, created_at
		`),
		Example: heredoc.Doc(`
			# List tokens for all resource classes in the org inferred from the git remote
			$ circleci runner token list

			# List tokens for all resource classes in a specific org
			$ circleci runner token list --org gh/my-org

			# List tokens for a specific resource class
			$ circleci runner token list --resource-class my-org/my-runner

			# List tokens for a resource class given by its ID
			$ circleci runner token list --resource-class 01234567-89ab-4cde-8f01-23456789abcd

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
	cmd.Flags().StringVar(&resourceClass, "resource-class", "", "Filter by resource class (namespace/name or ID)")
	cmdutil.AddJSONFlag(cmd, &jsonOut)
	cmdutil.AddJQFlag(cmd)
	return cmd
}

type tokenOutput struct {
	ID              string `json:"id"`
	ResourceClass   string `json:"resource_class"`
	ResourceClassID string `json:"resource_class_id"`
	Nickname        string `json:"nickname"`
	CreatedAt       string `json:"created_at"`
}

func runTokenList(ctx context.Context, client *apiclient.Client, org, resourceClass string, jsonOut bool) error {
	var out []tokenOutput
	var err error
	if resourceClass != "" {
		out, resourceClass, err = listTokensForResourceClass(ctx, client, resourceClass)
	} else {
		out, err = listTokensForOrg(ctx, client, org)
	}
	if err != nil {
		return err
	}
	return printTokenList(ctx, out, resourceClass, jsonOut)
}

// listTokensForResourceClass lists the tokens of one resource class given as
// namespace/name or ID, in a single request. It also returns the name to report
// the class by.
func listTokensForResourceClass(ctx context.Context, client *apiclient.Client, resourceClass string) ([]tokenOutput, string, error) {
	id, isID, err := parseResourceClassRef(resourceClass)
	if err != nil {
		return nil, "", err
	}

	var tokens []apiclient.RunnerToken
	if isID {
		tokens, err = client.ListRunnerTokensByResourceClassID(ctx, id)
	} else {
		tokens, err = client.ListRunnerTokensByResourceClass(ctx, resourceClass)
	}
	if err != nil {
		// The API answers 404 for a class that does not exist and for one the caller cannot access.
		if httpcl.HasStatusCode(err, http.StatusNotFound) {
			return nil, "", resourceClassNotFoundErr(resourceClass, isID)
		}
		return nil, "", apiErr(err, resourceClass)
	}

	// Rows carry the namespace/name, but an ID with no tokens has none to read it
	// from, and the "No tokens found" message names the class by namespace/name.
	if len(tokens) == 0 && isID {
		rc, err := resolveResourceClass(ctx, client, resourceClass)
		if err != nil {
			return nil, "", err
		}
		resourceClass = rc.ResourceClass
	}
	return tokenOutputs(tokens), resourceClass, nil
}

// listTokensForOrg lists the tokens of every resource class in the organization,
// one request per class.
func listTokensForOrg(ctx context.Context, client *apiclient.Client, org string) ([]tokenOutput, error) {
	orgID, err := cmdutil.ResolveOrgSlugOrID(ctx, client, org, "circleci runner token list")
	if err != nil {
		return nil, err
	}
	classes, err := client.ListResourceClassesByOrg(ctx, orgID)
	if err != nil {
		if httpcl.HasStatusCode(err, http.StatusNotFound) {
			return nil, orgNotAccessibleErr(orgID)
		}
		return nil, apiErr(err, orgID.String())
	}

	var out []tokenOutput
	for _, rc := range classes {
		rcID, err := resourceClassUUID(&rc)
		if err != nil {
			return nil, err
		}
		tokens, err := client.ListRunnerTokensByResourceClassID(ctx, rcID)
		if err != nil {
			return nil, apiErr(err, rc.ResourceClass)
		}
		out = append(out, tokenOutputs(tokens)...)
	}
	return out, nil
}

func tokenOutputs(tokens []apiclient.RunnerToken) []tokenOutput {
	out := make([]tokenOutput, 0, len(tokens))
	for _, t := range tokens {
		out = append(out, tokenOutput{
			ID:              t.ID,
			ResourceClass:   t.ResourceClass,
			ResourceClassID: t.ResourceClassID,
			Nickname:        t.Nickname,
			CreatedAt:       t.CreatedAt,
		})
	}
	return out
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
				in the form %[1]snamespace/name%[1]s (for example, %[1]smy-org/my-runner%[1]s) or as its ID.
			`, "`"),
		},
		Long: heredoc.Doc(`
			Create a new authentication token for a runner resource class.

			The token value is shown only once and cannot be retrieved afterwards.
			Store it securely. If lost, delete this token and create a new one.

			JSON fields: id, resource_class, resource_class_id, nickname, created_at, token
		`),
		Example: heredoc.Doc(`
			# Create a token for a resource class
			$ circleci runner token create my-org/my-runner

			# Create a token with a nickname
			$ circleci runner token create my-org/my-runner --nickname "prod-server-1"

			# Create a token for a resource class given by its ID
			$ circleci runner token create 01234567-89ab-4cde-8f01-23456789abcd

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
	ID              string `json:"id"`
	ResourceClass   string `json:"resource_class"`
	ResourceClassID string `json:"resource_class_id"`
	Nickname        string `json:"nickname"`
	CreatedAt       string `json:"created_at"`
	Token           string `json:"token"`
}

func runTokenCreate(ctx context.Context, client *apiclient.Client, resourceClass, nickname string, jsonOut bool) error {
	rc, err := resolveResourceClass(ctx, client, resourceClass)
	if err != nil {
		return err
	}

	rcID, err := resourceClassUUID(rc)
	if err != nil {
		return err
	}

	tok, err := client.CreateRunnerTokenV3(ctx, rcID, nickname)
	if err != nil {
		return apiErr(err, rc.ResourceClass)
	}

	out := tokenCreateOutput{
		ID:              tok.ID,
		ResourceClass:   tok.ResourceClass,
		ResourceClassID: tok.ResourceClassID,
		Nickname:        tok.Nickname,
		CreatedAt:       tok.CreatedAt,
		Token:           tok.Token,
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
	// Checked before the prompt: runner-admin rejects a non-UUID id with a 400, which
	// would otherwise surface as a generic API error after the user confirmed.
	id, err := uuid.Parse(tokenID)
	if err != nil {
		return invalidTokenIDErr(tokenID)
	}

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

	if err := client.DeleteRunnerTokenV3(ctx, id); err != nil {
		if httpcl.HasStatusCode(err, http.StatusNotFound) {
			return tokenNotFoundErr(tokenID)
		}
		return apiErr(err, tokenID)
	}

	if jsonOut {
		return iostream.PrintJSON(ctx, tokenDeleteOutput{ID: tokenID})
	}

	iostream.Printf(ctx, "Deleted token: %s\n", tokenID)
	return nil
}

// invalidTokenIDErr reports a token ID argument that is not a UUID.
func invalidTokenIDErr(tokenID string) *clierrors.CLIError {
	return clierrors.New("runner.invalid_token_id", "Invalid token ID",
		fmt.Sprintf("%q is not a valid token ID. Token IDs are UUIDs.", tokenID)).
		WithSuggestions("List tokens with: circleci runner token list --resource-class <namespace/name>").
		WithExitCode(clierrors.ExitBadArguments)
}

// tokenNotFoundErr reports a 404 for a token ID. The API answers 404 both for a
// token that does not exist and for one in an organization the caller is not a
// member of, so name both.
func tokenNotFoundErr(tokenID string) *clierrors.CLIError {
	return clierrors.New("runner.token_not_found", "Token not found",
		fmt.Sprintf("No runner token with ID %q, or it belongs to an organization your token cannot access.", tokenID)).
		WithSuggestions("List tokens with: circleci runner token list --resource-class <namespace/name>").
		WithExitCode(clierrors.ExitNotFound)
}
