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

package namespace

import (
	"context"
	"fmt"

	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"

	"github.com/CircleCI-Public/circleci-cli/internal/apiclient"
	"github.com/CircleCI-Public/circleci-cli/internal/cmdutil"
	"github.com/CircleCI-Public/circleci-cli/internal/iostream"
)

func newGetCmd() *cobra.Command {
	var jsonOut bool

	cmd := &cobra.Command{
		Use:   "get <name>",
		Short: "Get details of a namespace",
		Annotations: map[string]string{
			"help:arguments": heredoc.Doc(`
				name is the name of the namespace to look up, e.g. "myorg".
			`),
		},
		Long: heredoc.Doc(`
			Display details of a CircleCI orb namespace.

			JSON fields: id, name
		`),
		Example: heredoc.Doc(`
			# Get a namespace by name
			$ circleci namespace get myorg

			# Output as JSON
			$ circleci namespace get myorg --json

			# Extract just the namespace ID
			$ circleci namespace get myorg --json --jq '.id'
		`),
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := cmdutil.RequireArgs(args, "name"); err != nil {
				return err
			}
			ctx := cmd.Context()
			client, err := cmdutil.LoadClient(ctx)
			if err != nil {
				return err
			}
			return runGet(ctx, client, args[0], jsonOut)
		},
	}

	cmdutil.AddJSONFlag(cmd, &jsonOut)
	cmdutil.AddJQFlag(cmd)

	return cmd
}

func runGet(ctx context.Context, client *apiclient.Client, name string, jsonOut bool) error {
	ns, err := client.GetNamespace(ctx, name)
	if err != nil {
		return apiErr(err, name)
	}

	if jsonOut {
		return iostream.PrintJSON(ctx, namespaceOutput{ID: ns.ID, Name: ns.Name})
	}

	iostream.PrintMarkdown(ctx, fmt.Sprintf("# Namespace\n- Name: %s\n- ID: `%s`\n", ns.Name, ns.ID))
	return nil
}
