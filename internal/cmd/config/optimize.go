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
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"

	clierrors "github.com/CircleCI-Public/circleci-cli/clikit/errors"
	"github.com/CircleCI-Public/circleci-cli/clikit/iostream"
	"github.com/CircleCI-Public/circleci-cli/internal/apiclient"
	"github.com/CircleCI-Public/circleci-cli/internal/cmdutil"
	"github.com/CircleCI-Public/circleci-cli/internal/configoptimize/apicompile"
	"github.com/CircleCI-Public/circleci-cli/internal/configoptimize/engine"
	"github.com/CircleCI-Public/circleci-cli/internal/configoptimize/pipelineconfig"
	"github.com/CircleCI-Public/circleci-cli/internal/configoptimize/pricing/static"
	"github.com/CircleCI-Public/circleci-cli/internal/configoptimize/registry"
	"github.com/CircleCI-Public/circleci-cli/internal/configoptimize/report"
	"github.com/CircleCI-Public/circleci-cli/internal/configoptimize/usage"
)

type optimizeOptions struct {
	org, params, rates, usage string
	jsonOut, verbose          bool
	only                      []string
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
			Find cheaper resource classes from per-job usage. JSON fields: schema_version, command, summary.result, findings[].id
		`),
		Example: heredoc.Doc(`
			# Report cheaper resource classes, sized from usage
			$ circleci config optimize --usage usage.json

			# Price the classes with your own credit rates
			$ circleci config optimize --usage usage.json --credit-rates rates.yml

			# The report as JSON
			$ circleci config optimize --usage usage.json --json
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
	f.StringVar(&o.usage, "usage", "", "per-job CPU and memory usage (JSON file), for resource classes")
	f.StringVar(&o.rates, "credit-rates", "", "credits per minute per class (YAML file; default built-in gen1)")
	f.StringSliceVar(&o.only, "only", nil, "checks to run: resource-class (default all)")
	f.StringVar(&o.params, "pipeline-parameters", "", "pipeline parameters as a YAML map or path to a YAML file")
	f.BoolVarP(&o.verbose, "verbose", "v", false, "also list report-only findings, with evidence")
	cmdutil.AddJSONFlag(cmd, &o.jsonOut)
	cmdutil.AddJQFlag(cmd)
	cmdutil.AddOrgFlag(cmd, &o.org, cmdutil.OrgFlag{Purpose: "for private orb resolution", DefaultsToGitRemote: true})
	cmd.MarkFlagsMutuallyExclusive("verbose", "json")
	return cmd
}

func runOptimize(ctx context.Context, path string, o optimizeOptions) error {
	in, client, err := optimizeInput(ctx, path, o)
	if err != nil {
		return err
	}
	doc, err := engine.Analyze(ctx, in)
	if err != nil {
		return optimizeEngineErr(client, err)
	}
	if o.jsonOut {
		return iostream.PrintJSON(ctx, doc)
	}
	return report.WriteText(iostream.Out(ctx), doc, report.TextOptions{Verbose: o.verbose})
}

// optimizeInput loads the inputs and builds the engine's input. path is the
// config as given ("-" for stdin).
func optimizeInput(ctx context.Context, path string, o optimizeOptions) (engine.AnalyzeInput, *apiclient.Client, error) {
	var none engine.AnalyzeInput
	rates, _, err := static.Load(o.rates)
	if err != nil {
		return none, nil, optimizeArgsErr(fmt.Sprintf("--credit-rates %s: %v", o.rates, err),
			"Pass a table with flat_charges, credits_per_minute, ladders and sizes, or leave it out to use the built-in one")
	}
	var usageData *usage.Data
	if o.usage != "" {
		if usageData, err = usage.Load(o.usage); err != nil {
			return none, nil, optimizeArgsErr(fmt.Sprintf("--usage %s: %v", o.usage, err),
				"Export usage as JSON with per-job CPU and memory samples")
		}
	}
	modules, err := registry.ParseChecks(o.only)
	if err != nil {
		return none, nil, optimizeArgsErr("--only "+err.Error(),
			"Pass one or more of: "+strings.Join(registry.CheckNames(), ", "))
	}
	selected := registry.Select(registry.Catalog(registry.Deps{
		Pricing: rates, Usage: usageData}), modules)
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

func optimizeArgsErr(message, suggestion string) error {
	return clierrors.New("args.invalid_value", "Invalid flags", message).
		WithSuggestions(suggestion).
		WithExitCode(clierrors.ExitBadArguments)
}
