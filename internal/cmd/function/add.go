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

	clierrors "github.com/CircleCI-Public/circleci-cli/clikit/errors"
	"github.com/CircleCI-Public/circleci-cli/clikit/iostream"
	"github.com/CircleCI-Public/circleci-cli/internal/apiclient"
	"github.com/CircleCI-Public/circleci-cli/internal/cmdutil"
	"github.com/CircleCI-Public/circleci-cli/internal/function"
)

// fnSuffix is appended to an alias something else in the config already claims.
const fnSuffix = "-fn"

func newAddCmd() *cobra.Command {
	var (
		alias   string
		version string
		dryRun  bool
	)

	cmd := &cobra.Command{
		Use:   "add <name>",
		Short: "Declare a function in the pipeline config",
		Long: heredoc.Docf(`
			Resolve a function's latest version and declare it in the
			%[1]sfunctions%[1]s block of the pipeline config.

			The alias a step invokes defaults to the function's name; %[1]s-fn%[1]s is
			appended if the config already uses that name.
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
			return runAdd(ctx, client, args[0], alias, version, configPath(cmd), dryRun)
		},
	}

	cmd.Flags().StringVar(&alias, "as", "", "alias a step invokes it by (default: the function's name)")
	cmd.Flags().StringVar(&version, "version", "", "version to pin (default: the latest release)")
	cmd.Flags().BoolVarP(&dryRun, "dry-run", "n", false, "show the change without writing it")
	addConfigFlag(cmd)

	return cmd
}

func runAdd(ctx context.Context, client *apiclient.Client, arg, alias, version, path string, dryRun bool) error {
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

	requested := alias
	alias, conflict, err := resolveAlias(path, alias)
	if err != nil {
		return err
	}

	ref, err := resolveReference(ctx, client, name, version, !dryRun)
	if err != nil {
		return err
	}

	if conflict != "" {
		iostream.ErrPrintf(ctx, "%s Aliased as %q: %q already names %s %s in this config\n",
			iostream.SymbolWarn(ctx), alias, requested, article(conflict), conflict)
	}

	if dryRun {
		iostream.Printf(ctx, "Would add to the functions block in %s:\n\n  %s: %s\n", path, alias, ref)
		return nil
	}

	if err := function.AddPin(path, alias, ref); err != nil {
		return err
	}

	iostream.Printf(ctx, "%s Declared %s as %s in %s\n", iostream.SymbolOK(ctx), alias, ref, path)
	iostream.Printf(ctx, "  Invoke it as a step named %q. See: circleci help functions\n", alias)
	return nil
}

// resolveAlias returns the alias to write and what, if anything, forced it to
// change. An alias already declared is left alone: that is a re-pin, which
// AddPin reports, not a name collision.
func resolveAlias(path, alias string) (string, string, error) {
	conflict, err := function.Conflicts(path, alias)
	if err != nil {
		return "", "", err
	}
	if conflict == "" {
		return alias, "", nil
	}

	suffixed := alias + fnSuffix
	also, err := function.Conflicts(path, suffixed)
	if err != nil {
		return "", "", err
	}
	if also != "" {
		return "", "", clierrors.New("function.alias_taken", "Function alias unavailable",
			fmt.Sprintf("Both %q and %q already name something else in this config.", alias, suffixed)).
			WithSuggestions("Choose an alias: circleci function add <name> --as <alias>").
			WithExitCode(clierrors.ExitBadArguments)
	}
	return suffixed, conflict, nil
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

func article(kind string) string {
	if kind == "orb" {
		return "an"
	}
	return "a"
}
