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
)

func newRemoveFromCategoryCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "remove-from-category <ns>/<orb> <category>",
		Short: "Remove an orb from a registry category",
		Annotations: map[string]string{
			"help:arguments": heredoc.Docf(`
				- %[1]s<ns>/<orb>%[1]s: the orb to remove, as "namespace/orb-name"
				- %[1]s<category>%[1]s: the registry category name, e.g. "Testing"
			`, "`"),
		},
		Long: heredoc.Docf(`
			Remove an orb from an orb registry category.

			Use 'circleci orb get %[1]s<ns>/<orb>%[1]s' to see the current categories
			for an orb, and 'circleci orb list-categories' for all available
			categories.
		`, "`"),
		Example: heredoc.Doc(`
			# Remove an orb from the Testing category
			$ circleci orb remove-from-category myorg/my-orb "Testing"

			# Remove an orb from the Deployment category
			$ circleci orb remove-from-category myorg/my-orb "Deployment"

			# See current categories
			$ circleci orb get myorg/my-orb
		`),
		Args: cobra.MaximumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := cmdutil.RequireArgs(args, "ns/orb", "category"); err != nil {
				return err
			}
			ctx := cmd.Context()
			client, err := cmdutil.LoadClient(ctx)
			if err != nil {
				return err
			}
			return runRemoveFromCategory(ctx, client, args[0], args[1])
		},
	}
}

func runRemoveFromCategory(ctx context.Context, client *apiclient.Client, fullName, categoryName string) error {
	pkg, err := client.GetOrbPackageByName(ctx, fullName)
	if err != nil {
		return orbAPIErr(err, fullName)
	}

	cat, err := client.GetOrbCategoryByName(ctx, categoryName)
	if err != nil {
		return orbAPIErr(err, categoryName)
	}

	if err := client.RemoveOrbFromCategory(ctx, pkg.ID, cat.ID); err != nil {
		return orbAPIErr(err, fullName)
	}

	iostream.Printf(ctx, "%s Removed %q from category %q.\n", iostream.SymbolOK(ctx), fullName, categoryName)
	return nil
}
