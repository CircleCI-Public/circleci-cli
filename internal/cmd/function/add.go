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
	"strings"

	"github.com/MakeNowJust/heredoc"
	"github.com/google/uuid"
	"github.com/spf13/cobra"

	clierrors "github.com/CircleCI-Public/circleci-cli/clikit/errors"
	"github.com/CircleCI-Public/circleci-cli/clikit/iostream"
	"github.com/CircleCI-Public/circleci-cli/internal/apiclient"
	"github.com/CircleCI-Public/circleci-cli/internal/cmdutil"
	"github.com/CircleCI-Public/circleci-cli/internal/function"
)

// fnSuffix is appended to a default alias something else in the config already
// claims.
const fnSuffix = "-fn"

type addResult struct {
	Alias    string `json:"alias"`
	Function string `json:"function"`
	Version  string `json:"version"`
	Config   string `json:"config"`
	Step     string `json:"step,omitempty"`
}

func newAddCmd() *cobra.Command {
	var (
		alias      string
		version    string
		dryRun     bool
		jsonOut    bool
		configFile string
	)

	cmd := &cobra.Command{
		Use:   "add <name>",
		Short: "Declare a function in the pipeline config",
		Long: heredoc.Docf(`
			Resolve a function's latest version and declare it in the
			%[1]sfunctions%[1]s block of the pipeline config.

			A default alias that clashes with an orb or command gets %[1]s-fn%[1]s appended.

			JSON fields: alias, function, version, config, step.
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
			return runAdd(ctx, client, args[0], alias, version, configFile, dryRun, jsonOut)
		},
	}

	cmd.Flags().StringVar(&alias, "as", "", "alias a step invokes it by (default: the function's name)")
	cmd.Flags().StringVar(&version, "version", "", "version to pin (default: the latest release)")
	cmd.Flags().BoolVarP(&dryRun, "dry-run", "n", false, "show the change without writing it")
	cmdutil.AddPipelineConfigFlag(cmd, &configFile)
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
	explicit := alias != ""
	if !explicit {
		alias = function.AliasFor(name)
	}
	if err := function.ValidateAlias(alias); err != nil {
		return err
	}

	requested := alias
	alias, conflict, err := resolveAlias(path, alias, explicit)
	if err != nil {
		return err
	}
	if err := function.CheckAdd(path, alias); err != nil {
		return err
	}

	ref, resolved, err := resolveReference(ctx, client, name, version, !dryRun && !jsonOut)
	if err != nil {
		return err
	}
	step := exampleStep(ctx, client, resolved, alias)

	if conflict != "" {
		iostream.ErrPrintf(ctx, "%s Aliased as %q: %q already names %s %s in this config\n",
			iostream.SymbolWarn(ctx), alias, requested, article(conflict), conflict)
	}

	if !dryRun {
		if err := function.AddPin(path, alias, ref); err != nil {
			return err
		}
	}

	if jsonOut {
		return iostream.PrintJSON(ctx, addResult{Alias: alias, Function: ref.Path, Version: ref.Version, Config: path, Step: step})
	}
	if dryRun {
		iostream.Printf(ctx, "Would add to the functions block in %s:\n\n  %s: %s\n", path, alias, ref)
		if step != "" {
			iostream.Printf(ctx, "\nInvoke it from a job's steps:\n\n%s", indent(step, "  "))
		}
		return nil
	}

	iostream.Printf(ctx, "%s Declared %s as %s in %s\n", iostream.SymbolOK(ctx), alias, ref, path)
	if step == "" {
		iostream.Printf(ctx, "  Invoke it as a step named %q.\n", alias)
		return nil
	}
	iostream.Printf(ctx, "  Invoke it from a job's steps:\n\n%s", indent(step, "    "))
	return nil
}

// exampleStep returns the step from the resolved version's example config,
// keyed by alias. It is a hint: a version with no example, or one that cannot
// be fetched, yields "" rather than failing an add.
func exampleStep(ctx context.Context, client *apiclient.Client, version apiclient.FunctionVersion, alias string) string {
	if version.ID == uuid.Nil {
		return ""
	}
	descriptor, err := client.GetFunctionVersion(ctx, version.ID)
	if err != nil {
		return ""
	}
	return function.ExampleStep(descriptor.Content, alias)
}

func indent(s, prefix string) string {
	lines := strings.SplitAfter(s, "\n")
	for i, line := range lines {
		if line != "" {
			lines[i] = prefix + line
		}
	}
	return strings.Join(lines, "")
}

// resolveAlias returns the alias to write and what, if anything, forced it to
// change. An alias already declared is left alone: that is a re-pin, which
// AddPin reports, not a name collision. An explicit alias is never renamed —
// the user chose it, and their steps invoke it by that name.
func resolveAlias(path, alias string, explicit bool) (string, string, error) {
	conflict, err := function.Conflicts(path, alias)
	if err != nil {
		return "", "", err
	}
	if conflict == "" {
		return alias, "", nil
	}
	if explicit {
		return "", "", clierrors.New("function.alias_taken", "Function alias unavailable",
			fmt.Sprintf("Alias %q already names %s %s in this config.", alias, article(conflict), conflict)).
			WithSuggestions("Choose another alias: circleci function add <name> --as <alias>").
			WithExitCode(clierrors.ExitBadArguments)
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
// reference, defaulting to the function's latest release, along with the
// published version it names.
func resolveReference(ctx context.Context, client *apiclient.Client, name, version string, spin bool) (function.Reference, apiclient.FunctionVersion, error) {
	sp := iostream.Spinner(ctx, spin, fmt.Sprintf("Resolving %s", name))
	fn, err := client.GetFunctionByName(ctx, name)
	sp.Stop()
	if err != nil {
		return function.Reference{}, apiclient.FunctionVersion{}, apiErr(err, name)
	}

	if version == "" {
		if fn.LatestVersion == "" {
			return function.Reference{}, apiclient.FunctionVersion{}, noVersionsErr(name)
		}
		latest, _ := fn.Find(fn.LatestVersion)
		return function.Reference{Path: fn.Name, Version: fn.LatestVersion}, latest, nil
	}

	found, ok := fn.Find(version)
	if !ok {
		versions := make([]string, 0, len(fn.Versions))
		for _, v := range fn.Versions {
			versions = append(versions, v.Version)
		}
		return function.Reference{}, apiclient.FunctionVersion{}, versionNotPublishedErr(name, version, versions)
	}
	return function.Reference{Path: fn.Name, Version: version}, found, nil
}

func article(kind string) string {
	if kind == "orb" {
		return "an"
	}
	return "a"
}
