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

package deploy

import (
	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"

	"github.com/CircleCI-Public/circleci-cli/internal/cmd/componentversion"
	"github.com/CircleCI-Public/circleci-cli/internal/cmdutil"
)

func newVersionCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "version <command>",
		Short: "Manage deploy component versions",
		Long: heredoc.Doc(`
			List and inspect CircleCI deploy component versions.

			Deploy component versions represent specific released versions
			of a component across environments.
		`),
		RunE: cmdutil.GroupRunE,
		FParseErrWhitelist: cobra.FParseErrWhitelist{
			UnknownFlags: true,
		},
	}

	cmd.AddCommand(newVersionListCmd())

	return cmd
}

func newVersionListCmd() *cobra.Command {
	var (
		envID   string
		jsonOut bool
	)

	cmd := &cobra.Command{
		Use:     "list <component-id>",
		Aliases: []string{"ls"},
		Short:   "List versions of a deploy component",
		Long: heredoc.Doc(`
			List versions of a CircleCI deploy component.

			Optionally filter by deploy environment with --environment.

			JSON fields: name, component_id, created_at

			Primary alias: circleci component-version list <component-id>
		`),
		Example: heredoc.Doc(`
			# List versions of a component
			$ circleci deploy version list a0000000-0000-4000-8000-000000c00001

			# Filter by environment
			$ circleci deploy version list a0000000-0000-4000-8000-000000c00001 --environment a0000000-0000-4000-8000-000000e00001

			# Output as JSON
			$ circleci deploy version list a0000000-0000-4000-8000-000000c00001 --json
		`),
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			client, err := cmdutil.LoadClient(ctx)
			if err != nil {
				return err
			}
			return componentversion.RunList(ctx, client, args[0], envID, jsonOut)
		},
	}

	cmd.Flags().StringVar(&envID, "environment", "", "Filter by deploy environment ID")
	cmdutil.AddJSONFlag(cmd, &jsonOut)
	cmdutil.AddJQFlag(cmd)

	return cmd
}
