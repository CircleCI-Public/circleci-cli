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
	"strings"

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

	cmdutil.AddGroup(cmd, "General commands",
		newResourceClassListCmd(),
		newResourceClassCreateCmd(),
	)
	cmdutil.AddGroup(cmd, "Targeted commands",
		newResourceClassDeleteCmd(),
	)

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

	cmdutil.AddOrgFlag(cmd, &org, cmdutil.OrgFlag{DefaultsToGitRemote: true})
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

// defaultTokenNickname matches the nickname the legacy CLI used, so runner
// install instructions referring to the "default" token stay accurate.
const defaultTokenNickname = "default"

func newResourceClassCreateCmd() *cobra.Command {
	var description string
	var generateToken bool
	var jsonOut bool

	cmd := &cobra.Command{
		Use:   "create <namespace>/<name>",
		Short: "Create a runner resource class",
		Annotations: map[string]string{
			"help:arguments": heredoc.Docf(`
				The resource class name must be given in the form %[1]snamespace/name%[1]s,
				where namespace is your organization name (for example, %[1]smy-org/my-runner%[1]s).
			`, "`"),
		},
		Long: heredoc.Doc(`
			Create a new CircleCI runner resource class.

			JSON fields: id, resource_class, description (token_id, token with --generate-token)
		`),
		Example: heredoc.Doc(`
			# Create a resource class
			$ circleci runner resource-class create my-org/my-runner

			# Create with a description
			$ circleci runner resource-class create my-org/my-runner --description "Linux amd64 runner"

			# Create a resource class and generate a token nicknamed "default"
			$ circleci runner resource-class create my-org/my-runner --generate-token

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
			return runResourceClassCreate(ctx, client, args[0], description, generateToken, jsonOut)
		},
	}

	cmd.Flags().StringVar(&description, "description", "", "Human-readable description of the resource class")
	cmd.Flags().BoolVar(&generateToken, "generate-token", false,
		`also create a token for the resource class, nicknamed "default"`)
	cmdutil.AddJSONFlag(cmd, &jsonOut)
	cmdutil.AddJQFlag(cmd)
	return cmd
}

type resourceClassCreateOutput struct {
	ID            string `json:"id"`
	ResourceClass string `json:"resource_class"`
	Description   string `json:"description"`
	TokenID       string `json:"token_id,omitempty"`
	Token         string `json:"token,omitempty"`
}

func runResourceClassCreate(ctx context.Context, client *apiclient.Client, resourceClass, description string, generateToken, jsonOut bool) error {
	rc, err := client.CreateResourceClass(ctx, resourceClass, description)
	if err != nil {
		return apiErr(err, resourceClass)
	}

	out := resourceClassCreateOutput{
		ID:            rc.ID,
		ResourceClass: rc.ResourceClass,
		Description:   rc.Description,
	}

	if generateToken {
		// Use the slug the API echoed back rather than the argument, so a server-side
		// normalisation of the name cannot send the token request somewhere else.
		tok, err := client.CreateRunnerToken(ctx, out.ResourceClass, defaultTokenNickname)
		if err != nil {
			return tokenGenerationErr(err, out.ResourceClass)
		}
		if tok.Token == "" {
			return tokenValueMissingErr(out.ResourceClass, tok.ID)
		}
		out.TokenID = tok.ID
		out.Token = tok.Token
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

	// Printed outside the markdown block because PrintMarkdown wraps to the
	// terminal width when colour is on, which would break a long token value.
	if out.Token != "" {
		iostream.Printf(ctx, "\nToken (save this — it will not be shown again):\n%s\n", out.Token)
	}
	return nil
}

// tokenGenerationErr reports a token failure that left the resource class behind.
// It borrows apiErr's status classification so a 401 still exits ExitAuthError and
// the server's own explanation still reaches the user.
func tokenGenerationErr(err error, resourceClass string) *clierrors.CLIError {
	classified := apiErr(err, resourceClass)
	suggestions := append([]string{
		fmt.Sprintf("Create the token separately: circleci runner token create %s --nickname %s",
			resourceClass, defaultTokenNickname),
	}, classified.Suggestions...)

	return clierrors.New("runner.token_generation_failed", "Token generation failed",
		fmt.Sprintf("Resource class %q was created, but generating its token failed.\n%s",
			resourceClass, strings.TrimSpace(classified.Message))).
		WithSuggestions(suggestions...).
		WithRef(classified.Ref).
		WithExitCode(classified.ExitCode)
}

// tokenValueMissingErr covers a token created without a value in the response.
// Exiting 0 would leave the user owning a live credential they can never see.
func tokenValueMissingErr(resourceClass, tokenID string) *clierrors.CLIError {
	return clierrors.New("runner.token_value_missing", "Token value not returned",
		fmt.Sprintf("Resource class %q was created and token %s was generated, but the API "+
			"did not return the token value. It cannot be retrieved later.", resourceClass, tokenID)).
		WithSuggestions(
			fmt.Sprintf("Delete the unusable token: circleci runner token delete %s --force", tokenID),
			fmt.Sprintf("Then create a replacement: circleci runner token create %s --nickname %s",
				resourceClass, defaultTokenNickname),
		).
		WithExitCode(clierrors.ExitAPIError)
}

// --- resource-class delete ---

func newResourceClassDeleteCmd() *cobra.Command {
	var force bool
	var jsonOut bool

	cmd := &cobra.Command{
		Use:     "delete <namespace>/<name>",
		Aliases: []string{"rm"},
		Short:   "Delete a runner resource class",
		Annotations: map[string]string{
			"help:arguments": heredoc.Docf(`
				The resource class to delete, given in the form %[1]snamespace/name%[1]s,
				where namespace is your organization name (for example, %[1]smy-org/my-runner%[1]s).
			`, "`"),
			"destructiveHint": "true",
		},
		Long: heredoc.Doc(`
			Delete a CircleCI runner resource class.

			All tokens associated with the resource class will also be deleted.
			Connected runner instances will no longer be able to claim jobs.

			JSON fields: id, resource_class
		`),
		Example: heredoc.Doc(`
			# Delete a resource class (with confirmation prompt)
			$ circleci runner resource-class delete my-org/my-runner

			# Delete without confirmation
			$ circleci runner resource-class delete my-org/my-runner --force

			# Delete in a script, and report what was deleted
			$ circleci runner resource-class delete my-org/my-runner --force --json
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
			return runResourceClassDelete(ctx, client, args[0], force, jsonOut)
		},
	}

	cmd.Flags().BoolVarP(&force, "force", "f", false, "skip confirmation prompt")
	cmdutil.AddJSONFlag(cmd, &jsonOut)
	cmdutil.AddJQFlag(cmd)
	return cmd
}

type resourceClassDeleteOutput struct {
	ID            string `json:"id"`
	ResourceClass string `json:"resource_class"`
}

func runResourceClassDelete(ctx context.Context, client *apiclient.Client,
	resourceClass string, force, jsonOut bool) error {
	if namespace, _, ok := strings.Cut(resourceClass, "/"); !ok || namespace == "" {
		return clierrors.New("runner.malformed_resource_class", "Malformed resource class",
			fmt.Sprintf("%q is not in namespace/name form.", resourceClass)).
			WithSuggestions("Give the resource class as <namespace>/<name>, for example my-org/my-runner").
			WithExitCode(clierrors.ExitBadArguments)
	}

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

	id, err := uuid.Parse(rc.ID)
	if err != nil {
		return apiErr(err, resourceClass)
	}

	if err := client.DeleteResourceClass(ctx, id); err != nil {
		return apiErr(err, resourceClass)
	}

	if jsonOut {
		return iostream.PrintJSON(ctx, resourceClassDeleteOutput{
			ID:            rc.ID,
			ResourceClass: resourceClass,
		})
	}

	iostream.ErrPrintf(ctx, "%s Deleted resource class %s\n", iostream.SymbolOK(ctx), resourceClass)
	return nil
}
