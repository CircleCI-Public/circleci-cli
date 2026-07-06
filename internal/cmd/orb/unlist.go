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

func newUnlistCmd() *cobra.Command {
	var restore bool

	cmd := &cobra.Command{
		Use:   "unlist <namespace>/<orb>",
		Short: "Hide or restore an orb in the registry",
		Annotations: map[string]string{
			"help:arguments": heredoc.Docf(`
				- %[1]s<namespace>/<orb>%[1]s is the orb to update, for example, %[1]snamespace/orb-name%[1]s
			`, "`"),
		},
		Long: heredoc.Doc(`
			Control whether an orb is visible in the CircleCI orb registry.

			By default, hides the orb (unlists it from search results).
			Pass --restore to make the orb visible again.

			Unlisted orbs can still be used if you know the exact orb reference.
		`),
		Example: heredoc.Doc(`
			# Hide an orb from the registry
			$ circleci orb unlist myorg/my-orb

			# Restore an orb's visibility
			$ circleci orb unlist myorg/my-orb --restore
		`),
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := cmdutil.RequireArgs(args, "ns/orb"); err != nil {
				return err
			}
			ctx := cmd.Context()
			client, err := cmdutil.LoadClient(ctx)
			if err != nil {
				return err
			}
			// listed is the desired visibility: --restore makes it listed,
			// the default (unlist) makes it not listed.
			return runOrbUnlist(ctx, client, args[0], restore)
		},
	}

	cmd.Flags().BoolVar(&restore, "restore", false, "restore the orb's visibility instead of hiding it")

	return cmd
}

func runOrbUnlist(ctx context.Context, client *apiclient.Client, fullName string, listed bool) error {
	pkg, err := client.GetOrbPackageByName(ctx, fullName)
	if err != nil {
		return orbAPIErr(err, fullName)
	}

	if err := client.SetOrbListed(ctx, pkg.ID, listed); err != nil {
		return orbAPIErr(err, fullName)
	}

	if listed {
		iostream.Printf(ctx, "%s Orb %q is now listed in the registry.\n", iostream.SymbolOK(ctx), fullName)
	} else {
		iostream.Printf(ctx, "%s Orb %q has been unlisted from the registry.\n", iostream.SymbolOK(ctx), fullName)
	}
	return nil
}
