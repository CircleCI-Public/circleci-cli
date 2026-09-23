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
	"fmt"

	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"

	"github.com/CircleCI-Public/circleci-cli/clikit/iostream"
	"github.com/CircleCI-Public/circleci-cli/internal/apiclient"
	"github.com/CircleCI-Public/circleci-cli/internal/cmdutil"
	"github.com/CircleCI-Public/circleci-cli/internal/function"
)

type addResult struct {
	Alias    string `json:"alias"`
	Function string `json:"function"`
	Version  string `json:"version"`
	Config   string `json:"config"`
}

func newAddCmd() *cobra.Command {
	var (
		alias   string
		version string
		dryRun  bool
		jsonOut bool
	)

	cmd := &cobra.Command{
		Use:   "add <name>",
		Short: "Declare a function in the pipeline config",
		Long: heredoc.Docf(`
			Resolve a function's latest version and declare it in the
			%[1]sfunctions%[1]s block of the pipeline config.

			JSON fields: alias, function, version, config.
		`, "`"),
		Example: heredoc.Doc(`
			# Declare a function at its latest version
			$ circleci function add setup-go

			# Declare a specific version under a chosen alias
			$ circleci function add setup-go --version v0.5.1-684fd5b --as go

			# Preview the change without writing it
			$ circleci function add setup-go --dry-run
		`),
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			client, err := cmdutil.LoadClient(ctx)
			if err != nil {
				return err
			}
			return runAdd(ctx, client, args[0], alias, version, configPath(cmd), dryRun, jsonOut)
		},
	}

	cmd.Flags().StringVar(&alias, "as", "", "alias a step invokes it by (default: the function's name)")
	cmd.Flags().StringVar(&version, "version", "", "version to pin (default: the latest release)")
	cmd.Flags().BoolVarP(&dryRun, "dry-run", "n", false, "show the change without writing it")
	addConfigFlag(cmd)
	cmdutil.AddJSONFlag(cmd, &jsonOut)
	cmdutil.AddJQFlag(cmd)

	return cmd
}

func runAdd(ctx context.Context, client *apiclient.Client, arg, alias, version, path string, dryRun, jsonOut bool) error {
	// Everything decidable from the arguments and the file happens before the
	// network call: no point resolving a version for a pin that cannot be written.
	name, err := function.ExpandName(arg)
	if err != nil {
		return err
	}
	if version != "" {
		if err := function.ValidateVersion(version); err != nil {
			return err
		}
	}
	if alias == "" {
		alias = function.AliasFor(name)
	}
	if err := function.ValidateAlias(alias); err != nil {
		return err
	}
	if err := function.CheckAdd(path, alias); err != nil {
		return err
	}

	ref, err := resolveReference(ctx, client, name, version, !dryRun && !jsonOut)
	if err != nil {
		return err
	}

	if !dryRun {
		if err := function.AddPin(path, alias, ref); err != nil {
			return err
		}
	}

	if jsonOut {
		return iostream.PrintJSON(ctx, addResult{Alias: alias, Function: ref.Path, Version: ref.Version, Config: path})
	}
	if dryRun {
		iostream.Printf(ctx, "Would add to the functions block in %s:\n\n  %s: %s\n", path, alias, ref)
		return nil
	}

	iostream.Printf(ctx, "%s Declared %s as %s in %s\n", iostream.SymbolOK(ctx), alias, ref, path)
	iostream.Printf(ctx, "  Invoke it as a step named %q.\n", alias)
	return nil
}

// resolveReference turns a name and an optional version into a pinnable
// reference, defaulting to the function's latest release.
func resolveReference(ctx context.Context, client *apiclient.Client, name, version string, spin bool) (function.Reference, error) {
	sp := iostream.Spinner(ctx, spin, fmt.Sprintf("Resolving %s", name))
	fn, err := client.GetFunctionByName(ctx, name)
	sp.Stop()
	if err != nil {
		return function.Reference{}, apiErr(err, name)
	}

	if version == "" {
		if fn.LatestVersion == "" {
			return function.Reference{}, noVersionsErr(name)
		}
		return function.Reference{Path: fn.Name, Version: fn.LatestVersion}, nil
	}

	if _, ok := fn.Find(version); !ok {
		versions := make([]string, 0, len(fn.Versions))
		for _, v := range fn.Versions {
			versions = append(versions, v.Version)
		}
		return function.Reference{}, versionNotPublishedErr(name, version, versions)
	}
	return function.Reference{Path: fn.Name, Version: version}, nil
}
