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

package cmdconfig

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"

	clierrors "github.com/CircleCI-Public/circleci-cli/clikit/errors"
	"github.com/CircleCI-Public/circleci-cli/clikit/iostream"
	"github.com/CircleCI-Public/circleci-cli/internal/apiclient"
	"github.com/CircleCI-Public/circleci-cli/internal/cmdutil"
	"github.com/CircleCI-Public/circleci-cli/internal/configoptimize/apicompile"
	"github.com/CircleCI-Public/circleci-cli/internal/configoptimize/engine"
	"github.com/CircleCI-Public/circleci-cli/internal/configoptimize/module/cache"
	"github.com/CircleCI-Public/circleci-cli/internal/configoptimize/pipelineconfig"
	"github.com/CircleCI-Public/circleci-cli/internal/configoptimize/pricing/static"
	"github.com/CircleCI-Public/circleci-cli/internal/configoptimize/publish"
	"github.com/CircleCI-Public/circleci-cli/internal/configoptimize/registry"
	"github.com/CircleCI-Public/circleci-cli/internal/configoptimize/report"
	"github.com/CircleCI-Public/circleci-cli/internal/configoptimize/usage"
)

type optimizeOptions struct {
	output, org, params, rates, usage string
	inPlace, force, jsonOut, verbose  bool
	only                              []string
}

func newOptimizeCmd() *cobra.Command {
	var o optimizeOptions
	cmd := &cobra.Command{
		Use:   "optimize [<path>]",
		Short: "Report changes that lower a config's credit spend",
		Annotations: map[string]string{
			"help:arguments": heredoc.Docf(`
				%[1]s<path>%[1]s is the config, by default %[1]s.circleci/config.yml%[1]s; %[1]s-%[1]s reads stdin.
			`, "`"),
		},
		Long: heredoc.Doc(`
			Find cheaper resource classes, cache keys and DLC. JSON fields: schema_version, command, summary.result, findings[].id
		`),
		Example: heredoc.Doc(`
			# Report what could change
			$ circleci config optimize

			# Write the optimized config, sizing classes from usage
			$ circleci config optimize --usage usage.json -o optimized.yml

			# Only the cache keys, as JSON
			$ circleci config optimize --only cache --json
		`),
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			path := cmdutil.DefaultPipelineConfigPath
			if len(args) == 1 {
				path = args[0]
			}
			return runOptimize(cmd.Context(), path, o)
		},
	}
	f := cmd.Flags()
	f.StringVarP(&o.output, "output", "o", "", "write the optimized config to this file (- for stdout)")
	f.BoolVar(&o.inPlace, "in-place", false, "replace <path>; needs a clean git worktree")
	f.BoolVarP(&o.force, "force", "f", false, "overwrite the -o file, or skip the clean-worktree check")
	f.StringVar(&o.usage, "usage", "", "per-job CPU and memory usage (JSON file), for resource classes")
	f.StringVar(&o.rates, "credit-rates", "", "credits per minute per class (YAML file; default built-in gen1)")
	f.StringSliceVar(&o.only, "only", nil, "checks to run: resource-class, cache, dlc (default all)")
	f.StringVar(&o.params, "pipeline-parameters", "", "pipeline parameters as a YAML map or path to a YAML file")
	f.BoolVarP(&o.verbose, "verbose", "v", false, "also list report-only findings, with evidence")
	cmdutil.AddJSONFlag(cmd, &o.jsonOut)
	cmdutil.AddJQFlag(cmd)
	cmdutil.AddOrgFlag(cmd, &o.org, cmdutil.OrgFlag{Purpose: "for private orb resolution", DefaultsToGitRemote: true})
	cmd.MarkFlagsMutuallyExclusive("output", "in-place")
	cmd.MarkFlagsMutuallyExclusive("verbose", "json")
	return cmd
}

func runOptimize(ctx context.Context, path string, o optimizeOptions) error {
	if err := optimizeFlagsCheck(path, o); err != nil {
		return err
	}
	// file is the config's path on disk, "" when it is read from stdin.
	file := path
	if path == "-" {
		file = ""
	}
	var (
		before publish.Snapshot
		err    error
	)
	switch {
	case o.inPlace:
		before, err = publish.PreflightInPlace(file, o.force)
	case o.output != "" && o.output != "-":
		err = publish.PreflightToFile(o.output, file, o.force)
	}
	if err != nil {
		return optimizeWriteErr(err)
	}
	in, client, err := optimizeInput(ctx, path, file, o)
	if err != nil {
		return err
	}
	if !o.inPlace && o.output == "" {
		doc, err := engine.Analyze(ctx, in)
		if err != nil {
			return optimizeEngineErr(client, err)
		}
		if o.jsonOut {
			return iostream.PrintJSON(ctx, doc)
		}
		return report.WriteText(iostream.Out(ctx), doc, report.TextOptions{Verbose: o.verbose})
	}
	return optimizeWrite(ctx, client, in, file, before, o)
}

// optimizeFlagsCheck refuses flag combinations cobra cannot express.
func optimizeFlagsCheck(path string, o optimizeOptions) error {
	switch {
	case o.output == "-" && o.jsonOut:
		return optimizeArgsErr("args.conflicting_flags", "-o - writes the config to stdout, so --json cannot use it",
			"Write the config to a file with -o FILE, or leave out --json")
	case o.inPlace && path == "-":
		return optimizeArgsErr("args.conflicting_flags", "--in-place needs a config file, not stdin",
			"Pass the config's path, or write the result with -o FILE")
	case o.force && !o.inPlace && (o.output == "" || o.output == "-"):
		return optimizeArgsErr("args.missing_flag", "--force only applies with -o FILE or --in-place",
			"Add -o FILE or --in-place, or leave out --force")
	}
	return nil
}

// optimizeInput loads the inputs and builds the engine's input. path is the
// config as given ("-" for stdin), file its path on disk ("" for stdin).
func optimizeInput(ctx context.Context, path, file string, o optimizeOptions) (engine.AnalyzeInput, *apiclient.Client, error) {
	var none engine.AnalyzeInput
	rates, _, err := static.Load(o.rates)
	if err != nil {
		return none, nil, optimizeArgsErr("args.invalid_value", fmt.Sprintf("--credit-rates %s: %v", o.rates, err),
			"Pass a table with flat_charges, credits_per_minute, ladders and sizes, or leave it out to use the built-in one")
	}
	var usageData *usage.Data
	if o.usage != "" {
		if usageData, err = usage.Load(o.usage); err != nil {
			return none, nil, optimizeArgsErr("args.invalid_value", fmt.Sprintf("--usage %s: %v", o.usage, err),
				"Export usage as JSON with per-job CPU and memory samples")
		}
	}
	modules, err := registry.ParseChecks(o.only)
	if err != nil {
		return none, nil, optimizeArgsErr("args.invalid_value", "--only "+err.Error(),
			"Pass one or more of: "+strings.Join(registry.CheckNames(), ", "))
	}
	var repo string // only the cache check reads the checkout
	if len(modules) == 0 || slices.Contains(modules, cache.Name) {
		repo = engine.RepoRoot(file)
	}
	selected := registry.Select(registry.Catalog(registry.Deps{
		Pricing: rates, Repo: repo, Usage: usageData}), modules)
	params, err := pipelineParamsFlag(o.params)
	if err != nil {
		return none, nil, err
	}
	src, err := readConfigInput(ctx, path)
	if err != nil {
		return none, nil, err
	}
	client := cmdutil.LoadClientOptionalAuth(ctx)
	orgID, err := optionalAuthOrgID(ctx, client, o.org, "circleci config optimize", "Or drop --org to compile against public orbs only")
	if err != nil {
		return none, nil, err
	}
	return engine.AnalyzeInput{
		Path: path, Config: []byte(src), Params: params, Modules: selected,
		Compiler: apicompile.Compiler{Client: client, OrgID: orgID}, Policy: registry.Policy(),
	}, client, nil
}

// optimizeWrite applies the changes and writes the config, then reports what
// was written: JSON on stdout, or the text report on stderr. file is the
// config's path on disk ("" for stdin) and before what PreflightInPlace saw
// of it.
func optimizeWrite(ctx context.Context, client *apiclient.Client, in engine.AnalyzeInput, file string, before publish.Snapshot, o optimizeOptions) error {
	res, err := engine.Apply(ctx, in)
	if err != nil {
		return optimizeEngineErr(client, err)
	}
	switch {
	case o.inPlace && bytes.Equal(res.Output, in.Config):
		err = publish.VerifyUnchanged(file, in.Config)
	case o.inPlace:
		err = publish.InPlace(file, before, in.Config, res.Output, o.force)
	case o.output == "-":
		_, err = iostream.Out(ctx).Write(res.Output)
	default:
		err = publish.ToFile(o.output, res.Output, file, o.force)
	}
	incomplete, partial := errors.AsType[*publish.IncompleteError](err)
	if err != nil && !partial {
		return optimizeWriteErr(err)
	}
	if o.jsonOut {
		if err := iostream.PrintJSON(ctx, res.Document); err != nil {
			return err
		}
	} else {
		// Built first, then printed with ErrPrint, which honors --quiet.
		var text strings.Builder
		if err := report.WriteText(&text, res.Document, report.TextOptions{Verbose: o.verbose}); err != nil {
			return err
		}
		iostream.ErrPrint(ctx, text.String())
	}
	if partial {
		return clierrors.New(incomplete.Code, "Output incomplete", incomplete.Error()).
			WithSuggestions("Check the file, then run the command again if it is not complete").
			WithExitCode(clierrors.ExitGeneralError)
	}
	if res.Rejected > 0 {
		return clierrors.New("config.optimize_rejected", "Changes rejected",
			fmt.Sprintf("%d planned change(s) did not compile as planned and were left out", res.Rejected)).
			WithSuggestions("See the rejected changes and their reasons in the report above").
			WithExitCode(clierrors.ExitValidationFail)
	}
	return nil
}

// optimizeEngineErr maps a failed compile call the way config process does,
// and passes the engine's own CLIErrors through.
func optimizeEngineErr(client *apiclient.Client, err error) error {
	transport, ok := errors.AsType[*pipelineconfig.TransportError](err)
	if !ok {
		if _, isCLI := errors.AsType[*clierrors.CLIError](err); isCLI {
			return err
		}
		return clierrors.New("config.optimize_failed", "Optimize failed", err.Error()).
			WithSuggestions("Run again with --debug for details").
			WithExitCode(clierrors.ExitGeneralError)
	}
	return compileAPIErr(client, transport.Err, "compile")
}

func optimizeArgsErr(code, message, suggestion string) error {
	return clierrors.New(code, "Invalid flags", message).
		WithSuggestions(suggestion).
		WithExitCode(clierrors.ExitBadArguments)
}

func optimizeWriteErr(err error) error {
	if refused, ok := errors.AsType[*publish.RefusedError](err); ok {
		e := clierrors.New(refused.Code, "Output not written", refused.Reason).WithExitCode(clierrors.ExitBadArguments)
		if refused.Suggestion != "" {
			e = e.WithSuggestions(refused.Suggestion)
		}
		return e
	}
	return clierrors.New("output.write_failed", "Could not write output file", err.Error()).
		WithSuggestions("Check that the directory exists and is writable").
		WithExitCode(clierrors.ExitGeneralError)
}
