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
	"strings"

	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"

	"github.com/CircleCI-Public/circleci-cli/internal/apiclient"
	"github.com/CircleCI-Public/circleci-cli/internal/cmdutil"
	clierrors "github.com/CircleCI-Public/circleci-cli/internal/errors"
	"github.com/CircleCI-Public/circleci-cli/internal/iostream"
)

func newCreateCmd() *cobra.Command {
	var (
		private bool
		jsonOut bool
	)

	cmd := &cobra.Command{
		Use:   "create <namespace>/<orb>",
		Short: "Reserve an orb name in a namespace",
		Long: heredoc.Doc(`
			Reserve an orb name in the given namespace.

			This registers the orb name without publishing any versions.
			After creating the orb, publish versions with 'circleci orb publish'.

			The namespace must already exist. Use 'circleci namespace create'
			to create a namespace if needed.

			JSON fields: id, name, namespace, is_private
		`),
		Example: heredoc.Doc(`
			# Create a public orb
			$ circleci orb create myorg/my-orb

			# Create a private orb
			$ circleci orb create myorg/my-orb --private

			# Create and output as JSON
			$ circleci orb create myorg/my-orb --json
		`),
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := cmdutil.RequireArgs(args, "namespace/orb"); err != nil {
				return err
			}
			parts := strings.SplitN(args[0], "/", 2)
			if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
				return clierrors.New("args.invalid_orb_ref", "Invalid orb reference",
					"Expected <namespace>/<orb>, got: "+args[0]).
					WithExitCode(clierrors.ExitBadArguments)
			}
			ctx := iostream.FromCmd(cmd.Context(), cmd)
			client, err := cmdutil.LoadClient(ctx, cmd)
			if err != nil {
				return err
			}
			return runOrbCreate(ctx, client, parts[0], parts[1], private, jsonOut)
		},
	}

	cmd.Flags().BoolVar(&private, "private", false, "create as a private orb")
	cmdutil.AddJSONFlag(cmd, &jsonOut)
	cmdutil.AddJQFlag(cmd)

	return cmd
}

type orbCreateOutput struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
	IsPrivate bool   `json:"is_private"`
}

func runOrbCreate(ctx context.Context, client *apiclient.Client, namespace, orbName string, private, jsonOut bool) error {
	ns, err := client.GetNamespace(ctx, namespace)
	if err != nil {
		return orbAPIErr(err, namespace)
	}

	pkg, err := client.CreateOrbPackage(ctx, apiclient.CreateOrbPackageRequest{
		Name:        namespace + "/" + orbName,
		NamespaceID: ns.ID,
		IsPrivate:   private,
	})
	if err != nil {
		return orbAPIErr(err, namespace+"/"+orbName)
	}

	if jsonOut {
		return iostream.PrintJSON(ctx, orbCreateOutput{
			ID:        pkg.ID,
			Name:      pkg.Name,
			Namespace: pkg.Namespace,
			IsPrivate: pkg.IsPrivate,
		})
	}

	iostream.Printf(ctx, "%s Created orb %q (%s)\n", iostream.SymbolOK(ctx), pkg.Name, pkg.ID)
	if private {
		iostream.Printf(ctx, "This orb is private and visible only to your organization.\n")
	} else {
		iostream.Printf(ctx, "Publish versions with: circleci orb publish <path> %s@<version>\n", pkg.Name)
	}
	return nil
}
