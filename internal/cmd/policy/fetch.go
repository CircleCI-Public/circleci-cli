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

package policy

import (
	"context"
	"encoding/json"

	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"

	"github.com/CircleCI-Public/circleci-cli/internal/apiclient"
	"github.com/CircleCI-Public/circleci-cli/internal/cmdutil"
	"github.com/CircleCI-Public/circleci-cli/internal/iostream"
)

func newFetchCmd() *cobra.Command {
	var (
		org       string
		policyCtx string
		jsonOut   bool
	)

	cmd := &cobra.Command{
		Use:   "fetch [policy-name]",
		Short: "Download the remote policy bundle",
		Annotations: map[string]string{
			"help:arguments": heredoc.Docf(`
				%[1]s<policy-name>%[1]s is optional and fetches a single policy.
				When omitted, the full policy bundle is fetched.
			`, "`"),
		},
		Long: heredoc.Doc(`
			Download the remote policy bundle for the given owner and context.
			Pass a policy name to fetch a single policy.

			Output is always JSON (the bundle is structured data).

			JSON fields: policies (map of name → Rego source)
		`),
		Example: heredoc.Doc(`
			# Fetch the full policy bundle
			$ circleci policy fetch --org gh/acme

			# Fetch a single policy by name
			$ circleci policy fetch my-policy --org gh/acme

			# Output as JSON with jq filtering
			$ circleci policy fetch --org gh/acme --json --jq 'keys'
		`),
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			client, err := cmdutil.LoadClient(ctx)
			if err != nil {
				return err
			}
			policyName := ""
			if len(args) == 1 {
				policyName = args[0]
			}
			orgID, err := cmdutil.ResolveOrgSlugOrID(ctx, client, org, "circleci policy fetch")
			if err != nil {
				return err
			}
			return runFetch(ctx, client, orgID.String(), policyCtx, policyName, jsonOut)
		},
	}

	cmdutil.AddOrgFlag(cmd, &org, cmdutil.OrgFlag{Required: true})
	cmd.Flags().StringVar(&policyCtx, "policy-context", "config", "Policy context")
	cmdutil.AddJSONFlag(cmd, &jsonOut)
	cmdutil.AddJQFlag(cmd)

	return cmd
}

func runFetch(ctx context.Context, client *apiclient.Client, ownerID, policyCtx, policyName string, jsonOut bool) (err error) {
	var bundle json.RawMessage
	if policyName == "" {
		bundle, err = client.FetchPolicyBundle(ctx, ownerID, policyCtx)
	} else {
		bundle, err = client.FetchPolicyBundleWithName(ctx, ownerID, policyCtx, policyName)
	}

	if err != nil {
		return policyAPIErr(err, ownerID)
	}

	if jsonOut {
		return iostream.PrintJSON(ctx, bundle)
	}
	return cmdutil.WriteJSON(iostream.Out(ctx), bundle)
}
