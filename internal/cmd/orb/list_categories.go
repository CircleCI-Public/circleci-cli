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

package orb

import (
	"context"

	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"

	"github.com/CircleCI-Public/circleci-cli/internal/apiclient"
	"github.com/CircleCI-Public/circleci-cli/internal/cmdutil"
	"github.com/CircleCI-Public/circleci-cli/internal/iostream"
	"github.com/CircleCI-Public/circleci-cli/internal/mdtable"
)

func newListCategoriesCmd() *cobra.Command {
	var jsonOut bool

	cmd := &cobra.Command{
		Use:   "list-categories",
		Short: "List orb registry categories",
		Long: heredoc.Doc(`
			List all categories available in the CircleCI orb registry.

			Categories are used to organize and discover orbs. Use
			'circleci orb add-to-category' to categorize your orbs.

			JSON fields: id, name
		`),
		Example: heredoc.Doc(`
			# List all orb categories
			$ circleci orb list-categories

			# Output as JSON
			$ circleci orb list-categories --json

			# Capture category names only
			$ circleci orb list-categories --json --jq '.[].name'
		`),
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := iostream.FromCmd(cmd.Context(), cmd)
			client, err := cmdutil.LoadClient(ctx, cmd)
			if err != nil {
				return err
			}
			return runListCategories(ctx, client, jsonOut)
		},
	}

	cmdutil.AddJSONFlag(cmd, &jsonOut)
	cmdutil.AddJQFlag(cmd)

	return cmd
}

type orbCategoryOutput struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

func runListCategories(ctx context.Context, client *apiclient.Client, jsonOut bool) error {
	categories, err := client.ListOrbCategories(ctx)
	if err != nil {
		return orbAPIErr(err, "categories")
	}

	var out []orbCategoryOutput
	for _, c := range categories {
		out = append(out, orbCategoryOutput{ID: c.ID, Name: c.Name})
	}

	if jsonOut {
		if out == nil {
			out = []orbCategoryOutput{}
		}
		return iostream.PrintJSON(ctx, out)
	}

	if len(out) == 0 {
		iostream.Printf(ctx, "No categories found.\n")
		return nil
	}

	table := mdtable.New("ID", "Name")
	for _, c := range categories {
		table.Row("`"+c.ID+"`", c.Name)
	}
	iostream.PrintMarkdown(ctx, "# Orb Categories\n\n"+table.Render())
	return nil
}
