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

func newListCmd() *cobra.Command {
	var (
		uncertified bool
		private     bool
		jsonOut     bool
	)

	cmd := &cobra.Command{
		Use:     "list [<namespace>]",
		Aliases: []string{"ls"},
		Short:   "List orbs in the registry",
		Annotations: map[string]string{
			"help:arguments": heredoc.Doc(`
				- namespace: optional. When given, lists all orbs in that namespace.
				  When omitted, lists certified orbs globally.
			`),
		},
		Long: heredoc.Doc(`
			List orbs in the CircleCI orb registry.

			Without a namespace argument, lists certified orbs globally.
			Pass a namespace to list all orbs in that namespace.

			Use --uncertified to include orbs that are not certified by CircleCI.
			Use --private to list only private orbs (requires namespace).

			JSON fields: id, name, is_private, is_listed, latest_version
		`),
		Example: heredoc.Doc(`
			# List certified orbs globally
			$ circleci orb list

			# List all orbs in a namespace
			$ circleci orb list myorg

			# Include uncertified orbs
			$ circleci orb list --uncertified

			# List orbs as JSON
			$ circleci orb list --json

			# List private orbs in a namespace
			$ circleci orb list myorg --private
		`),
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			client, err := cmdutil.LoadClient(ctx)
			if err != nil {
				return err
			}
			ns := ""
			if len(args) > 0 {
				ns = args[0]
			}
			return runOrbList(ctx, client, ns, uncertified, private, jsonOut)
		},
	}

	cmd.Flags().BoolVar(&uncertified, "uncertified", false, "include uncertified orbs")
	cmd.Flags().BoolVar(&private, "private", false, "only list private orbs (requires namespace)")
	cmdutil.AddJSONFlag(cmd, &jsonOut)
	cmdutil.AddJQFlag(cmd)

	return cmd
}

type orbListOutput struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	IsPrivate     bool   `json:"is_private"`
	IsListed      bool   `json:"is_listed"`
	LatestVersion string `json:"latest_version"`
}

func runOrbList(ctx context.Context, client *apiclient.Client, namespace string, uncertified, private, jsonOut bool) error {
	var nsID string
	if namespace != "" {
		ns, err := client.GetNamespace(ctx, namespace)
		if err != nil {
			return orbAPIErr(err, namespace)
		}
		nsID = ns.ID
	}

	orbs, err := client.ListOrbPackages(ctx, nsID, uncertified, private)
	if err != nil {
		return orbAPIErr(err, namespace)
	}

	var out []orbListOutput
	for _, o := range orbs {
		out = append(out, orbListOutput{
			ID:            o.ID,
			Name:          o.Name,
			IsPrivate:     o.IsPrivate,
			IsListed:      o.IsListed,
			LatestVersion: o.LatestVersion,
		})
	}

	if jsonOut {
		if out == nil {
			out = []orbListOutput{}
		}
		return iostream.PrintJSON(ctx, out)
	}

	if len(out) == 0 {
		if namespace != "" {
			iostream.Printf(ctx, "No orbs found in namespace %q.\n", namespace)
		} else {
			iostream.Printf(ctx, "No orbs found.\n")
		}
		return nil
	}

	table := mdtable.New("ID", "Name", "Version", "Private", "Listed")
	for _, o := range orbs {
		table.Row("`"+o.ID+"`", o.Name, o.LatestVersion, fmtBool(o.IsPrivate), fmtBool(o.IsListed))
	}
	iostream.PrintMarkdown(ctx, "# Orbs\n\n"+table.Render())
	return nil
}
