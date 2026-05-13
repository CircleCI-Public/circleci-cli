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
	"github.com/CircleCI-Public/circleci-cli/internal/gitremote"
	"github.com/CircleCI-Public/circleci-cli/internal/iostream"
	"github.com/CircleCI-Public/circleci-cli/internal/mdtable"
)

func newGetCmd() *cobra.Command {
	var (
		jsonOut bool
		orgSlug string
	)

	cmd := &cobra.Command{
		Use:   "get <context-id|context-name>",
		Short: "Get details of a context",
		Long: heredoc.Doc(`
			Display details of a CircleCI context, including its environment
			variable names and metadata.

			Pass a UUID to look up by ID, or a name to look up by name. When
			looking up by name, pass --org or run from a git repository so the
			organization can be inferred from the remote.

			Variable values are never returned by the API — CircleCI does not
			expose secret values after they are set.

			JSON fields: id, name, org_id, created_at, environment_variables, restrictions
		`),
		Example: heredoc.Doc(`
			# Get a context by UUID
			$ circleci context get ctx-uuid-here

			# Get a context by name (org inferred from git remote)
			$ circleci context get my-context

			# Get a context by name in a specific org
			$ circleci context get my-context --org gh/myorg

			# Output as JSON
			$ circleci context get my-context --json

			# Get just the org ID
			$ circleci context get ctx-uuid-here --json --jq '.org_id'
		`),
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := cmdutil.RequireArgs(args, "context-id"); err != nil {
				return err
			}
			ctx := iostream.FromCmd(cmd.Context(), cmd)
			client, err := cmdutil.LoadClient(ctx, cmd)
			if err != nil {
				return err
			}
			return runGet(ctx, client, args[0], orgSlug, jsonOut)
		},
	}

	cmd.Flags().StringVar(&orgSlug, "org", "", "Organization slug (e.g. gh/myorg); used when resolving name to ID")
	cmdutil.AddJSONFlag(cmd, &jsonOut)
	cmdutil.AddJQFlag(cmd)

	return cmd
}

type contextGetOutput struct {
	ID                   string                    `json:"id"`
	Name                 string                    `json:"name"`
	OrgID                string                    `json:"org_id"`
	CreatedAt            time.Time                 `json:"created_at"`
	EnvironmentVariables []contextEnvVarEntry      `json:"environment_variables"`
	Restrictions         []contextRestrictionEntry `json:"restrictions"`
}

type contextEnvVarEntry struct {
	Variable       string    `json:"variable"`
	TruncatedValue string    `json:"truncated_value,omitempty"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

type contextRestrictionEntry struct {
	ID               string `json:"id"`
	Name             string `json:"name"`
	RestrictionType  string `json:"restriction_type"`
	RestrictionValue string `json:"restriction_value"`
}

func runGet(ctx context.Context, client *apiclient.Client, contextName, orgSlug string, jsonOut bool) error {
	contextID, err := uuid.Parse(contextName)
	if err != nil {
		if orgSlug == "" {
			info, err := gitremote.Detect()
			if err != nil {
				return cmdutil.GitDetectErr(err, "Or specify the organization: circleci context get --org gh/myorg")
			}
			orgSlug = orgFromSlug(info.Slug)
		}
		id, err := resolveContextID(ctx, client, contextName, orgSlug)
		if err != nil {
			return err
		}
		contextID = id
	}

	ctxt, err := client.GetContext(ctx, contextID)
	if err != nil {
		return apiErr(err, contextID.String())
	}

	envVars := make([]contextEnvVarEntry, len(ctxt.EnvironmentVariables))
	for i, v := range ctxt.EnvironmentVariables {
		envVars[i] = contextEnvVarEntry{
			Variable:       v.Variable,
			TruncatedValue: v.TruncatedValue,
			CreatedAt:      v.CreatedAt,
			UpdatedAt:      v.UpdatedAt,
		}
	}

	restrictions := make([]contextRestrictionEntry, len(ctxt.Restrictions))
	for i, r := range ctxt.Restrictions {
		restrictions[i] = contextRestrictionEntry{
			ID:               r.ID.String(),
			Name:             r.Name,
			RestrictionType:  r.RestrictionType,
			RestrictionValue: r.RestrictionValue,
		}
	}

	out := contextGetOutput{
		ID:                   ctxt.ID.String(),
		Name:                 ctxt.Name,
		OrgID:                ctxt.OrgID.String(),
		CreatedAt:            ctxt.CreatedAt,
		EnvironmentVariables: envVars,
		Restrictions:         restrictions,
	}

	if jsonOut {
		return iostream.PrintJSON(ctx, out)
	}

	lines := fmt.Sprintf("# Context\n- ID: `%s`\n- Name: %s\n- Org ID: `%s`\n- Created: %s\n",
		out.ID, out.Name, out.OrgID, out.CreatedAt.Format(time.RFC3339))

	if len(envVars) > 0 {
		tbl := mdtable.New("Variable", "Value", "Updated")
		for _, v := range envVars {
			tbl.Row(v.Variable, "`****"+v.TruncatedValue+"`", v.UpdatedAt.Format(time.RFC3339))
		}
		lines += "\n## Environment Variables\n" + tbl.Render()
	}

	if len(restrictions) > 0 {
		tbl := mdtable.New("Type", "Value", "Name", "ID")
		for _, r := range restrictions {
			tbl.Row(r.RestrictionType, r.RestrictionValue, r.Name, "`"+r.ID+"`")
		}
		lines += "\n## Restrictions\n" + tbl.Render()
	}

	iostream.PrintMarkdown(ctx, lines)
	return nil
}
