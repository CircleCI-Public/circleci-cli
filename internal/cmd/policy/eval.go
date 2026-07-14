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

package policy

import (
	"context"
	"errors"
	"os"

	"github.com/CircleCI-Public/circle-policy-agent/cpa"
	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/CircleCI-Public/circleci-cli/internal/cmdutil"
	"github.com/CircleCI-Public/circleci-cli/internal/configcmd"
	clierrors "github.com/CircleCI-Public/circleci-cli/internal/errors"
	"github.com/CircleCI-Public/circleci-cli/internal/iostream"
)

func newEvalCmd() *cobra.Command {
	var (
		org            string
		policyCtx      string
		inputFile      string
		meta           string
		metaFile       string
		query          string
		noCompile      bool
		pipelineParams string
		jsonOut        bool
	)

	cmd := &cobra.Command{
		Use:   "eval <policy-path>",
		Short: "Evaluate a raw OPA query against policies locally",
		Annotations: map[string]string{
			"help:arguments": heredoc.Docf(`
				%[1]s<policy-path>%[1]s is the path to a .rego policy file or a directory
				of policy files to evaluate.
			`, "`"),
		},
		Long: heredoc.Doc(`
			Run a raw OPA query against a policy bundle entirely locally and print
			the result as JSON.

			Unlike 'policy decide', which returns a CircleCI policy decision from the
			remote service, 'eval' runs the OPA query directly on your machine and
			prints whatever the query evaluates to. It is the escape hatch for
			inspecting arbitrary Rego values while authoring policies.

			By default the --input config is compiled (orbs inlined, parameters
			resolved) before evaluation, and the source config is made available to
			policies with its compiled form nested under a "_compiled_" key. Pass
			--no-compile to evaluate the raw config instead.

			The output is the raw value produced by the OPA --query; it has no fixed
			schema. The default query "data" returns the entire document tree.
		`),
		Example: heredoc.Doc(`
			# Evaluate a policy directory against a compiled config
			$ circleci policy eval ./policies --input .circleci/config.yml

			# Evaluate a specific query
			$ circleci policy eval ./policies --input .circleci/config.yml --query 'data.org.enable_rule'

			# Evaluate the raw (uncompiled) config
			$ circleci policy eval ./policies --input .circleci/config.yml --no-compile

			# Pass decision metadata
			$ circleci policy eval ./policies --input .circleci/config.yml --meta '{"project_id":"abc"}'
		`),
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()

			metadata, err := readMetadata(meta, metaFile)
			if err != nil {
				return err
			}

			input, err := os.ReadFile(inputFile) //nolint:gosec // inputFile is user-supplied
			if err != nil {
				return clierrors.New("policy.eval_input_read_failed", "Could not read input file", err.Error()).
					WithSuggestions("Check that the file exists and is readable").
					WithExitCode(clierrors.ExitBadArguments)
			}

			if !noCompile && policyCtx == "config" {
				params, perr := configcmd.ParsePipelineParams(pipelineParams)
				if perr != nil {
					return clierrors.New("policy.invalid_params", "Invalid pipeline parameters", perr.Error()).
						WithSuggestions("Pass parameters as a YAML map: --pipeline-parameters 'key: value'").
						WithExitCode(clierrors.ExitBadArguments)
				}
				client, cerr := cmdutil.LoadClient(ctx)
				if cerr != nil {
					return cerr
				}
				ownerID, oerr := resolveOwnerID(ctx, client, org, "circleci policy eval")
				if oerr != nil {
					return oerr
				}
				input, err = compileConfig(ctx, client, input, ownerID, params)
				if err != nil {
					return err
				}
			}

			decision, err := getPolicyEvaluationLocally(ctx, args[0], input, metadata, query)
			if err != nil {
				return err
			}

			if jsonOut {
				return iostream.PrintJSON(ctx, decision)
			}
			return cmdutil.WriteJSON(iostream.Out(ctx), decision)
		},
	}

	cmdutil.AddOrgFlag(cmd, &org, cmdutil.OrgFlag{Purpose: "for private orb resolution", DefaultsToGitRemote: true})
	cmd.Flags().StringVar(&policyCtx, "context", "config", "Policy context (config compilation only runs when this is \"config\")")
	cmd.Flags().StringVar(&inputFile, "input", "", "Path to input file (e.g. .circleci/config.yml) (required)")
	cmd.Flags().StringVar(&meta, "meta", "", "Decision metadata as a JSON string")
	cmd.Flags().StringVar(&metaFile, "metafile", "", "Path to decision metadata file (YAML or JSON)")
	cmd.Flags().StringVar(&query, "query", "data", "The OPA query to evaluate")
	cmd.Flags().BoolVar(&noCompile, "no-compile", false, "Evaluate the raw config without compiling it first")
	cmd.Flags().StringVar(&pipelineParams, "pipeline-parameters", "", "Pipeline parameters as a YAML map or path to a YAML file")
	cmdutil.AddJSONFlag(cmd, &jsonOut)
	cmdutil.AddJQFlag(cmd)
	_ = cmd.MarkFlagRequired("input")

	return cmd
}

// getPolicyEvaluationLocally loads the policy bundle at policyPath and evaluates
// the OPA query against the (YAML) input document, seeding data.meta with meta.
// It returns the raw OPA expression value(s) produced by the query.
func getPolicyEvaluationLocally(ctx context.Context, policyPath string, rawInput []byte, meta map[string]any, query string) (any, error) {
	var input any
	if err := yaml.Unmarshal(rawInput, &input); err != nil {
		return nil, clierrors.New("policy.input_invalid", "Could not parse input document", err.Error()).
			WithSuggestions("The input must be valid YAML or JSON").
			WithExitCode(clierrors.ExitBadArguments)
	}

	p, err := cpa.LoadPolicyFromFS(policyPath)
	if err != nil {
		if errors.Is(err, cpa.ErrNoPolicies) {
			return nil, clierrors.New("policy.no_policies", "No policies found", err.Error()).
				WithSuggestions("Pass a path to a .rego file or a directory containing .rego files").
				WithExitCode(clierrors.ExitValidationFail)
		}
		return nil, clierrors.New("policy.load_failed", "Could not load policies", err.Error()).
			WithSuggestions("Check that the policy files are valid Rego").
			WithExitCode(clierrors.ExitValidationFail)
	}

	decision, err := p.Eval(ctx, query, input, cpa.Meta(meta))
	if err != nil {
		return nil, clierrors.New("policy.eval_failed", "Policy evaluation failed", err.Error()).
			WithSuggestions("Check the --query expression").
			WithExitCode(clierrors.ExitValidationFail)
	}
	return decision, nil
}
