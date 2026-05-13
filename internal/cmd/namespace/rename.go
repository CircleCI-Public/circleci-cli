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

	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"

	"github.com/CircleCI-Public/circleci-cli-v2/internal/apiclient"
	"github.com/CircleCI-Public/circleci-cli-v2/internal/cmdutil"
	"github.com/CircleCI-Public/circleci-cli-v2/internal/iostream"
)

func newRenameCmd() *cobra.Command {
	var jsonOut bool

	cmd := &cobra.Command{
		Use:   "rename <name> <new-name>",
		Short: "Rename a namespace",
		Long: heredoc.Doc(`
			Rename a CircleCI orb namespace.

			Any orbs already published under the old name will continue to be
			accessible — renaming creates an alias. Ensure that any configs
			and orbs still referencing the old name are updated.

			JSON fields: id, name
		`),
		Example: heredoc.Doc(`
			# Rename a namespace
			$ circleci namespace rename oldname newname

			# Rename and output the result as JSON
			$ circleci namespace rename oldname newname --json

			# Confirm the new ID after rename
			$ circleci namespace rename oldname newname --json --jq '.id'
		`),
		Args: cobra.MaximumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := cmdutil.RequireArgs(args, "name", "new-name"); err != nil {
				return err
			}
			ctx := iostream.FromCmd(cmd.Context(), cmd)
			client, err := cmdutil.LoadClient(ctx, cmd)
			if err != nil {
				return err
			}
			return runRename(ctx, client, args[0], args[1], jsonOut)
		},
	}

	cmdutil.AddJSONFlag(cmd, &jsonOut)
	cmdutil.AddJQFlag(cmd)

	return cmd
}

func runRename(ctx context.Context, client *apiclient.Client, name, newName string, jsonOut bool) error {
	ns, err := client.RenameNamespace(ctx, apiclient.RenameNamespaceRequest{Name: name, NewName: newName})
	if err != nil {
		return apiErr(err, name)
	}

	if jsonOut {
		return iostream.PrintJSON(ctx, namespaceOutput{ID: ns.ID, Name: ns.Name})
	}

	iostream.Printf(ctx, "%s Renamed namespace %q to %q (%s)\n",
		iostream.SymbolOK(ctx), name, ns.Name, ns.ID)
	return nil
}
