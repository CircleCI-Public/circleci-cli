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

package pipeline

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"

	"github.com/CircleCI-Public/circleci-cli-v2/internal/apiclient"
	"github.com/CircleCI-Public/circleci-cli-v2/internal/cmdutil"
	clierrors "github.com/CircleCI-Public/circleci-cli-v2/internal/errors"
	"github.com/CircleCI-Public/circleci-cli-v2/internal/gitremote"
	"github.com/CircleCI-Public/circleci-cli-v2/internal/iostream"
)

func newTriggerCmd() *cobra.Command {
	var (
		projectSlug string
		branch      string
		params      []string
		jsonOut     bool
	)

	cmd := &cobra.Command{
		Use:   "trigger",
		Short: "Trigger a new pipeline",
		Long: heredoc.Doc(`
			Trigger a new pipeline for a CircleCI project.

			The project and branch are inferred from the current git repository
			unless overridden with --project or --branch.

			Pass pipeline parameters with --parameter. Values are parsed as
			booleans (true/false), integers, or strings.

			JSON fields: id, number, state, created_at
		`),
		Example: heredoc.Doc(`
			# Trigger a pipeline on the current branch
			$ circleci pipeline trigger

			# Trigger on a specific branch
			$ circleci pipeline trigger --branch main

			# Trigger with pipeline parameters
			$ circleci pipeline trigger --parameter deploy_env=staging --parameter run_e2e=true

			# Output the triggered pipeline as JSON
			$ circleci pipeline trigger --json
		`),
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			streams := iostream.FromCmd(cmd)
			client, err := cmdutil.LoadClient(ctx, cmd)
			if err != nil {
				return err
			}
			return runTrigger(ctx, client, streams, projectSlug, branch, params, jsonOut)
		},
	}

	cmd.Flags().StringVar(&projectSlug, "project", "", "Project slug (e.g. gh/org/repo); defaults to git remote")
	cmd.Flags().StringVarP(&branch, "branch", "b", "", "Branch to trigger (defaults to current branch)")
	cmd.Flags().StringArrayVar(&params, "parameter", nil, "Pipeline parameter as key=value (repeatable)")
	cmd.Flags().BoolVar(&jsonOut, "json", false, "Output the triggered pipeline as JSON")

	return cmd
}

type triggerJSONOutput struct {
	ID        string `json:"id"`
	Number    int64  `json:"number"`
	State     string `json:"state"`
	CreatedAt string `json:"created_at"`
}

func runTrigger(ctx context.Context, client *apiclient.Client, streams iostream.Streams, projectSlug, branch string, params []string, jsonOut bool) error {
	effectiveBranch := branch
	if projectSlug == "" || effectiveBranch == "" {
		info, err := gitremote.Detect()
		if err != nil {
			return clierrors.New("git.detect_failed", "Could not detect project from git",
				err.Error()).
				WithSuggestions(
					"Run from inside a git repository with a GitHub, Bitbucket, or GitLab remote",
					"Or specify the project and branch explicitly",
				).
				WithExitCode(clierrors.ExitBadArguments)
		}
		if projectSlug == "" {
			projectSlug = info.Slug
		}
		if effectiveBranch == "" {
			effectiveBranch = info.Branch
		}
	}

	parsedParams, err := parseParams(params)
	if err != nil {
		return clierrors.New("args.invalid_parameter", "Invalid pipeline parameter",
			err.Error()).
			WithSuggestions("Parameters must be in key=value form, e.g. --parameter deploy_env=staging").
			WithExitCode(clierrors.ExitBadArguments)
	}

	resp, err := client.TriggerPipeline(ctx, projectSlug, effectiveBranch, parsedParams)
	if err != nil {
		return apiErr(err, projectSlug)
	}

	if jsonOut {
		out := triggerJSONOutput{
			ID:        resp.ID,
			Number:    resp.Number,
			State:     resp.State,
			CreatedAt: resp.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		}
		enc := json.NewEncoder(streams.Out)
		enc.SetIndent("", "  ")
		return enc.Encode(out)
	}

	streams.Printf("Triggered pipeline #%d (%s) on %s\n", resp.Number, resp.ID, effectiveBranch)
	return nil
}

// parseParams converts ["key=value", ...] into a map, coercing values to bool
// or int where unambiguous.
func parseParams(params []string) (map[string]any, error) {
	if len(params) == 0 {
		return nil, nil
	}
	result := make(map[string]any, len(params))
	for _, p := range params {
		k, v, found := strings.Cut(p, "=")
		if !found || k == "" {
			return nil, fmt.Errorf("%q is not valid: expected key=value", p)
		}
		switch v {
		case "true":
			result[k] = true
		case "false":
			result[k] = false
		default:
			if n, err := strconv.ParseInt(v, 10, 64); err == nil {
				result[k] = n
			} else {
				result[k] = v
			}
		}
	}
	return result, nil
}
