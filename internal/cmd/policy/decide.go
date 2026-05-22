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
	"encoding/json"
	"fmt"
	"os"

	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/CircleCI-Public/circleci-cli/internal/apiclient"
	"github.com/CircleCI-Public/circleci-cli/internal/cmdutil"
	clierrors "github.com/CircleCI-Public/circleci-cli/internal/errors"
	"github.com/CircleCI-Public/circleci-cli/internal/iostream"
)

func newDecideCmd() *cobra.Command {
	var (
		ownerID   string
		policyCtx string
		inputFile string
		meta      string
		metaFile  string
		strict    bool
		jsonOut   bool
	)

	cmd := &cobra.Command{
		Use:   "decide",
		Short: "Evaluate a config against remote policies",
		Long: heredoc.Doc(`
			Evaluate a CircleCI pipeline config against the remote policy bundle
			and return a policy decision.

			The decision status will be one of: PASS, SOFT_FAIL, HARD_FAIL, ERROR.
			With --strict, exits non-zero for HARD_FAIL or ERROR decisions.

			JSON fields: status, enabled_rules, hard_failures, soft_failures,
			             violations, metadata
		`),
		Example: heredoc.Doc(`
			# Evaluate a config against remote policies
			$ circleci policy decide --owner-id 462d67f8-b232-4da4-a7de-0c86dd667d3f --input .circleci/config.yml

			# Exit non-zero on hard failures
			$ circleci policy decide --owner-id 462d67f8-b232-4da4-a7de-0c86dd667d3f --input .circleci/config.yml --strict

			# Pass metadata alongside the decision
			$ circleci policy decide --owner-id 462d67f8-b232-4da4-a7de-0c86dd667d3f --input .circleci/config.yml --meta '{"project_id":"abc"}'

			# Output decision as JSON
			$ circleci policy decide --owner-id 462d67f8-b232-4da4-a7de-0c86dd667d3f --input .circleci/config.yml --json
		`),
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			client, err := cmdutil.LoadClient(ctx)
			if err != nil {
				return err
			}
			return runDecide(ctx, client, ownerID, policyCtx, inputFile, meta, metaFile, strict, jsonOut)
		},
	}

	cmd.Flags().StringVar(&ownerID, "owner-id", "", "Organization UUID (required)")
	cmd.Flags().StringVar(&policyCtx, "policy-context", "config", "Policy context")
	cmd.Flags().StringVar(&inputFile, "input", "", "Path to input file (e.g. .circleci/config.yml) (required)")
	cmd.Flags().StringVar(&meta, "meta", "", "Decision metadata as a JSON string")
	cmd.Flags().StringVar(&metaFile, "metafile", "", "Path to decision metadata file (YAML or JSON)")
	cmd.Flags().BoolVar(&strict, "strict", false, "Exit non-zero for HARD_FAIL or ERROR decisions")
	cmdutil.AddJSONFlag(cmd, &jsonOut)
	cmdutil.AddJQFlag(cmd)
	_ = cmd.MarkFlagRequired("owner-id")
	_ = cmd.MarkFlagRequired("input")

	return cmd
}

func runDecide(ctx context.Context, client *apiclient.Client, ownerID, policyCtx, inputFile, meta, metaFile string, strict, jsonOut bool) error {
	inputBytes, err := os.ReadFile(inputFile) //nolint:gosec // inputFile is user-supplied
	if err != nil {
		return clierrors.New("policy.decide_input_read_failed", "Could not read input file", err.Error()).
			WithSuggestions("Check that the file exists and is readable").
			WithExitCode(clierrors.ExitBadArguments)
	}

	metadata, err := readMetadata(meta, metaFile)
	if err != nil {
		return err
	}

	decision, err := client.MakeDecision(ctx, ownerID, policyCtx, string(inputBytes), metadata)
	if err != nil {
		return policyAPIErr(err, ownerID)
	}

	if strict {
		var parsed struct {
			Status string `json:"status"`
		}
		if jerr := json.Unmarshal(decision, &parsed); jerr == nil {
			if parsed.Status == "HARD_FAIL" || parsed.Status == "ERROR" {
				_ = cmdutil.WriteJSON(iostream.Out(ctx), decision)
				return clierrors.New("policy.hard_fail", "Policy decision failed",
					fmt.Sprintf("Policy decision status: %s", parsed.Status)).
					WithExitCode(clierrors.ExitValidationFail)
			}
		}
	}

	if jsonOut {
		return iostream.PrintJSON(ctx, decision)
	}
	return cmdutil.WriteJSON(iostream.Out(ctx), decision)
}

func readMetadata(meta, metaFile string) (map[string]any, error) {
	if meta != "" && metaFile != "" {
		return nil, clierrors.New("policy.meta_conflict", "Conflicting flags",
			"Use either --meta or --metafile, not both.").
			WithExitCode(clierrors.ExitBadArguments)
	}
	var metadata map[string]any
	if meta != "" {
		if err := json.Unmarshal([]byte(meta), &metadata); err != nil {
			return nil, clierrors.New("policy.meta_invalid", "Invalid --meta value", err.Error()).
				WithSuggestions("Pass a valid JSON object, e.g.: --meta '{\"key\":\"value\"}'").
				WithExitCode(clierrors.ExitBadArguments)
		}
	}
	if metaFile != "" {
		raw, err := os.ReadFile(metaFile) //nolint:gosec // metaFile is user-supplied
		if err != nil {
			return nil, clierrors.New("policy.metafile_read_failed", "Could not read metafile", err.Error()).
				WithExitCode(clierrors.ExitBadArguments)
		}
		if err := yaml.Unmarshal(raw, &metadata); err != nil {
			return nil, clierrors.New("policy.metafile_invalid", "Invalid metafile content", err.Error()).
				WithSuggestions("The metafile must be valid YAML or JSON").
				WithExitCode(clierrors.ExitBadArguments)
		}
	}
	return metadata, nil
}
