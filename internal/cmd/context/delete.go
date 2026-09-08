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

	clierrors "github.com/CircleCI-Public/circleci-cli/clikit/errors"
	"github.com/CircleCI-Public/circleci-cli/clikit/iostream"
	"github.com/CircleCI-Public/circleci-cli/internal/apiclient"
	"github.com/CircleCI-Public/circleci-cli/internal/cmdutil"
)

func newDeleteCmd() *cobra.Command {
	var (
		orgRef string
		force  bool
	)

	cmd := &cobra.Command{
		Use:     "delete <context-id|context-name>",
		Aliases: []string{"rm"},
		Short:   "Delete a context",
		Annotations: map[string]string{
			"help:arguments": heredoc.Docf(`
				A context can be specified by name or ID:
				- By name, for example, %[1]scontext-name%[1]s
				- By ID, for example, %[1]s849e7902-802f-4082-8a70-da77dcd084e3%[1]s
			`, "`"),
			"destructiveHint": "true",
		},
		Long: heredoc.Doc(`
			Delete a CircleCI context by UUID or name.

			Deleting a context removes all environment variables stored in it.
			Jobs that reference this context will fail until they are updated.
		`),
		Example: heredoc.Doc(`
			# Delete a context by UUID (with confirmation)
			$ circleci context delete ctx-uuid-here

			# Delete a context by name (org inferred from git remote)
			$ circleci context delete my-context

			# Delete a context by name in a specific org, without confirmation
			$ circleci context delete my-context --org gh/myorg --force
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
			return runDelete(ctx, client, args[0], orgRef, force)
		},
	}

	cmd.Flags().StringVar(&orgRef, "org", "", "Organization slug (e.g. gh/myorg) or UUID; used when resolving name to ID")
	cmd.Flags().BoolVarP(&force, "force", "f", false, "skip confirmation prompt")

	return cmd
}

func runDelete(ctx context.Context, client *apiclient.Client, contextName, orgRef string, force bool) error {
	// If the arg doesn't look like a UUID, resolve by name.
	displayName := contextName
	contextID, err := resolveContextArg(ctx, client, contextName, orgRef, "circleci context delete")
	if err != nil {
		return err
	}

	if err := cmdutil.ConfirmOrForce(ctx, iostream.Get(ctx), force,
		fmt.Sprintf("Delete context %s? All environment variables in it will be removed.", displayName),
		clierrors.New("context.delete_aborted", "Deletion aborted",
			"Context deletion was not confirmed.").
			WithExitCode(clierrors.ExitCancelled),
		clierrors.New("context.delete_requires_force", "Deletion requires --force",
			fmt.Sprintf("Deleting context %s will remove all its environment variables.", displayName)).
			WithExitCode(clierrors.ExitCancelled),
	); err != nil {
		return err
	}

	if err := client.DeleteContext(ctx, contextID); err != nil {
		return contextIDErr(err, displayName)
	}

	iostream.Printf(ctx, "%s Deleted context %s\n", iostream.SymbolOK(ctx), displayName)
	return nil
}

// resolveContextArg resolves a context UUID or name to a UUID. A valid UUID is
// returned as-is; anything else is looked up by name, which needs an org —
// orgRef if given, otherwise inferred from the git remote. cmdName appears in
// the git-detection error suggestion (e.g. "circleci context secret list").
func resolveContextArg(
	ctx context.Context, client *apiclient.Client, arg, orgRef, cmdName string,
) (uuid.UUID, error) {
	id, _, err := resolveContextRef(ctx, client, arg, orgRef, cmdName)
	return id, err
}

// resolveContextRef is resolveContextArg, additionally returning the context
// record when the arg was a name.
//
// The name lookup goes through ListContexts, whose response carries the whole
// context — including its group references — so a caller that needs the full
// context can use it directly instead of spending a round trip re-fetching by
// id. The returned record is nil when arg was already a UUID, since then
// nothing has been fetched. OrgID is filled in from the org that was resolved,
// which the list response itself omits.
func resolveContextRef(
	ctx context.Context, client *apiclient.Client, arg, orgRef, cmdName string,
) (uuid.UUID, *apiclient.Context, error) {
	if id, err := uuid.Parse(arg); err == nil {
		return id, nil, nil
	}
	orgID, err := cmdutil.ResolveOrgSlugOrID(ctx, client, orgRef, cmdName)
	if err != nil {
		return uuid.Nil, nil, err
	}
	ctxt, err := resolveContext(ctx, client, arg, orgID, orgRef)
	if err != nil {
		return uuid.Nil, nil, err
	}
	ctxt.OrgID = orgID
	return ctxt.ID, ctxt, nil
}

// resolveContext looks up a context by exact name within an org. The name is
// passed to ListContexts as a filter, which narrows the result set; the exact
// match here is what picks the one context out of it, so "build" never
// resolves to "build-secrets".
//
// orgRef is only used for error text — it is what the user typed, so the
// suggestion echoes that rather than a UUID they have never seen.
func resolveContext(
	ctx context.Context, client *apiclient.Client, name string, orgID uuid.UUID, orgRef string,
) (*apiclient.Context, error) {
	contexts, err := client.ListContexts(ctx, orgID, name)
	if err != nil {
		return nil, apiErr(err, orgDisplay(orgID, orgRef))
	}
	for i := range contexts {
		if contexts[i].Name == name {
			return &contexts[i], nil
		}
	}
	return nil, clierrors.New("context.not_found", "Context not found",
		fmt.Sprintf("No context named %q found in organization %q.", name, orgDisplay(orgID, orgRef))).
		WithSuggestions("Run: circleci context list --org " + orgDisplay(orgID, orgRef)).
		WithExitCode(clierrors.ExitNotFound)
}

// orgDisplay returns the org reference to show a user: what they typed when
// they gave one, else the resolved UUID.
func orgDisplay(orgID uuid.UUID, orgRef string) string {
	if orgRef != "" {
		return orgRef
	}
	return orgID.String()
}
