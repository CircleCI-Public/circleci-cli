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

package cmdfunction

import (
	"context"

	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"

	"github.com/CircleCI-Public/circleci-cli/clikit/iostream"
	"github.com/CircleCI-Public/circleci-cli/internal/apiclient"
	"github.com/CircleCI-Public/circleci-cli/internal/cmdutil"
	"github.com/CircleCI-Public/circleci-cli/internal/function"
)

type updateResult struct {
	Alias           string `json:"alias"`
	Function        string `json:"function"`
	PreviousVersion string `json:"previous_version"`
	Version         string `json:"version"`
	Config          string `json:"config"`
}

func newUpdateCmd() *cobra.Command {
	var (
		version string
		dryRun  bool
		jsonOut bool
	)

	cmd := &cobra.Command{
		Use:   "update <alias>",
		Short: "Change the version a declared function is pinned to",
		Long: heredoc.Docf(`
			Re-pin a declared function to its latest release.

			The argument is the alias the function is declared under, which
			%[1]sfunction list --pinned%[1]s reports.

			JSON fields: alias, function, previous_version, version, config.
		`, "`"),
		Example: heredoc.Doc(`
			# Move a pin to the latest release
			$ circleci function update setup-go

			# Pin an exact version
			$ circleci function update setup-go --version v0.5.1-684fd5b

			# Preview the change without writing it
			$ circleci function update setup-go --dry-run
		`),
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			client, err := cmdutil.LoadClient(ctx)
			if err != nil {
				return err
			}
			return runUpdate(ctx, client, args[0], version, configPath(cmd), dryRun, jsonOut)
		},
	}

	cmd.Flags().StringVar(&version, "version", "", "version to pin (default: the latest release)")
	cmd.Flags().BoolVarP(&dryRun, "dry-run", "n", false, "show the change without writing it")
	addConfigFlag(cmd)
	cmdutil.AddJSONFlag(cmd, &jsonOut)
	cmdutil.AddJQFlag(cmd)

	return cmd
}

func runUpdate(ctx context.Context, client *apiclient.Client, alias, version, path string, dryRun, jsonOut bool) error {
	if version != "" {
		if err := function.ValidateVersion(version); err != nil {
			return err
		}
	}

	// The declaration names the function, so the config is read before the
	// network: an alias that is not declared cannot be updated regardless.
	pin, err := function.FindPin(path, alias)
	if err != nil {
		return notDeclaredErr(alias, err)
	}
	if pin.Version == "" {
		return malformedPinErr(alias, pin.Function)
	}

	ref, err := resolveReference(ctx, client, pin.Function, version, !dryRun && !jsonOut)
	if err != nil {
		return err
	}

	current := ref.Version == pin.Version
	if !current && !dryRun {
		if err := function.UpdatePin(path, alias, ref); err != nil {
			return err
		}
	}

	if jsonOut {
		return iostream.PrintJSON(ctx, updateResult{
			Alias: alias, Function: ref.Path, PreviousVersion: pin.Version, Version: ref.Version, Config: path,
		})
	}
	if current {
		iostream.Printf(ctx, "%s %s is already pinned to %s\n", iostream.SymbolOK(ctx), alias, ref)
		return nil
	}
	if dryRun {
		iostream.Printf(ctx, "Would change the functions block in %s:\n\n  %s: %s\n", path, alias, ref)
		return nil
	}

	iostream.Printf(ctx, "%s Updated %s from %s to %s in %s\n",
		iostream.SymbolOK(ctx), alias, pin.Version, ref.Version, path)
	return nil
}
