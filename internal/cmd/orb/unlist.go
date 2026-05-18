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
	clierrors "github.com/CircleCI-Public/circleci-cli/internal/errors"
	"github.com/CircleCI-Public/circleci-cli/internal/iostream"
)

func newUnlistCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "unlist <ns>/<orb> true|false",
		Short: "Hide or restore an orb in the registry",
		Long: heredoc.Doc(`
			Control whether an orb is visible in the CircleCI orb registry.

			Pass 'true' to hide the orb (unlist it from search results).
			Pass 'false' to restore the orb's visibility.

			Unlisted orbs can still be used if you know the exact orb reference.
		`),
		Example: heredoc.Doc(`
			# Hide an orb from the registry
			$ circleci orb unlist myorg/my-orb true

			# Restore an orb's visibility
			$ circleci orb unlist myorg/my-orb false
		`),
		Args: cobra.MaximumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := cmdutil.RequireArgs(args, "ns/orb", "true|false"); err != nil {
				return err
			}
			var listed bool
			switch args[1] {
			case "true":
				listed = false // unlist = hide = not listed
			case "false":
				listed = true // relist = show = listed
			default:
				return clierrors.New("args.invalid_unlist", "Invalid value for listed flag",
					"Expected 'true' (to unlist) or 'false' (to relist), got: "+args[1]).
					WithExitCode(clierrors.ExitBadArguments)
			}
			ctx := iostream.FromCmd(cmd.Context(), cmd)
			client, err := cmdutil.LoadClient(ctx, cmd)
			if err != nil {
				return err
			}
			return runOrbUnlist(ctx, client, args[0], listed)
		},
	}
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
