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

package workflow

import (
	"context"
	"encoding/json"
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

func newListCmd() *cobra.Command {
	var (
		projectSlug string
		branch      string
		limit       int
		jsonOut     bool
	)

	cmd := &cobra.Command{
		Use:     "list [<pipeline-id>]",
		Aliases: []string{"ls"},
		Short:   "List workflows for a pipeline or recent pipelines",
		Long: heredoc.Doc(`
			List workflows for a CircleCI pipeline.

			With no argument, lists workflows for recent pipelines in the current
			project, grouped by pipeline. Use --branch to filter to a specific branch
			and --limit to control how many pipelines are shown.

			Pass a pipeline UUID or pipeline number to list workflows for a single
			pipeline. Pipeline numbers are shown in 'circleci pipeline list'; UUIDs
			are shown in 'circleci pipeline list --json'.

			When passing a pipeline number, the project is inferred from the
			current git repository unless overridden with --project.

			JSON fields (single pipeline):  id, name, status
			JSON fields (recent pipelines): pipeline_id, pipeline_number, id, name, status
		`),
		Example: heredoc.Doc(`
			# List workflows for recent pipelines in the current project
			$ circleci workflow list

			# Filter to a specific branch
			$ circleci workflow list --branch main

			# List workflows by pipeline UUID
			$ circleci workflow list 9e0c9d52-3b7e-4cd6-b5f7-bfc5e4a07e81

			# List workflows by pipeline number
			$ circleci workflow list 75

			# List workflows for a pipeline in a specific project
			$ circleci workflow list 75 --project gh/myorg/myrepo

			# Output as JSON
			$ circleci workflow list --json
		`),
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			streams := iostream.FromCmd(cmd)
			client, err := cmdutil.LoadClient(ctx, cmd)
			if err != nil {
				return err
			}
			if len(args) == 0 {
				return runListRecent(ctx, client, streams, projectSlug, branch, limit, jsonOut)
			}
			return runList(ctx, client, streams, args[0], projectSlug, jsonOut)
		},
	}

	cmd.Flags().StringVar(&projectSlug, "project", "", "Project slug (e.g. gh/org/repo); defaults to git remote")
	cmd.Flags().StringVarP(&branch, "branch", "b", "", "Filter by branch (recent-pipelines mode)")
	cmd.Flags().IntVar(&limit, "limit", 10, "Number of recent pipelines to show (recent-pipelines mode)")
	cmd.Flags().BoolVar(&jsonOut, "json", false, "Output as JSON")
	return cmd
}

type workflowListOutput struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Status string `json:"status"`
}

type workflowRecentOutput struct {
	PipelineID     string `json:"pipeline_id"`
	PipelineNumber int64  `json:"pipeline_number"`
	ID             string `json:"id"`
	Name           string `json:"name"`
	Status         string `json:"status"`
}

func runList(ctx context.Context, client *apiclient.Client, streams iostream.Streams, arg, projectSlug string, jsonOut bool) error {
	pipelineID, err := resolvePipelineID(ctx, client, arg, projectSlug)
	if err != nil {
		return err
	}

	workflows, err := client.GetPipelineWorkflows(ctx, pipelineID)
	if err != nil {
		return apiErr(err, pipelineID)
	}

	var out []workflowListOutput
	for _, wf := range workflows {
		out = append(out, workflowListOutput{
			ID:     wf.ID,
			Name:   wf.Name,
			Status: wf.Status,
		})
	}

	if jsonOut {
		if out == nil {
			out = []workflowListOutput{}
		}
		enc := json.NewEncoder(streams.Out)
		enc.SetIndent("", "  ")
		return enc.Encode(out)
	}

	if len(out) == 0 {
		streams.Printf("No workflows found for pipeline %s.\n", arg)
		return nil
	}
	for _, wf := range out {
		streams.Printf("%-36s  %-28s  %s\n", wf.ID, wf.Name, wf.Status)
	}
	return nil
}

func runListRecent(ctx context.Context, client *apiclient.Client, streams iostream.Streams, projectSlug, branch string, limit int, jsonOut bool) error {
	if projectSlug == "" {
		info, gitErr := gitremote.Detect()
		if gitErr != nil {
			return clierrors.New("git.detect_failed", "Could not detect project from git", gitErr.Error()).
				WithSuggestions(
					"Run from inside a git repository with a GitHub, Bitbucket, or GitLab remote",
					"Or specify --project explicitly",
				).
				WithExitCode(clierrors.ExitBadArguments)
		}
		projectSlug = info.Slug
	}

	pipelines, err := client.ListPipelines(ctx, projectSlug, branch, limit)
	if err != nil {
		return apiErr(err, projectSlug)
	}

	if jsonOut {
		var out []workflowRecentOutput
		for _, p := range pipelines {
			workflows, wErr := client.GetPipelineWorkflows(ctx, p.ID)
			if wErr != nil {
				return apiErr(wErr, p.ID)
			}
			for _, wf := range workflows {
				out = append(out, workflowRecentOutput{
					PipelineID:     p.ID,
					PipelineNumber: p.Number,
					ID:             wf.ID,
					Name:           wf.Name,
					Status:         wf.Status,
				})
			}
		}
		if out == nil {
			out = []workflowRecentOutput{}
		}
		enc := json.NewEncoder(streams.Out)
		enc.SetIndent("", "  ")
		return enc.Encode(out)
	}

	if len(pipelines) == 0 {
		streams.Printf("No pipelines found for project %s.\n", projectSlug)
		return nil
	}

	for i, p := range pipelines {
		if i > 0 {
			streams.Printf("\n")
		}
		branchName := ""
		revision := ""
		if p.VCS != nil {
			branchName = p.VCS.Branch
			revision = p.VCS.Revision
			if len(revision) > 7 {
				revision = revision[:7]
			}
		}
		streams.Printf("Pipeline #%d  %s  %s\n", p.Number, branchName, revision)

		workflows, wErr := client.GetPipelineWorkflows(ctx, p.ID)
		if wErr != nil {
			return apiErr(wErr, p.ID)
		}
		if len(workflows) == 0 {
			streams.Printf("  (no workflows)\n")
			continue
		}
		for _, wf := range workflows {
			streams.Printf("  %-36s  %-28s  %s\n", wf.ID, wf.Name, wf.Status)
		}
	}
	return nil
}

// resolvePipelineID returns a pipeline UUID from either a UUID string or a
// pipeline number (requires project slug resolution from git if not provided).
func resolvePipelineID(ctx context.Context, client *apiclient.Client, arg, projectSlug string) (string, error) {
	if strings.Contains(arg, "-") {
		return arg, nil
	}

	number, err := strconv.ParseInt(arg, 10, 64)
	if err != nil {
		return "", clierrors.New("args.invalid_pipeline_id", "Invalid pipeline ID",
			"Expected a pipeline UUID or pipeline number, got: "+arg).
			WithSuggestions("Use 'circleci pipeline list' to find pipeline IDs and numbers").
			WithExitCode(clierrors.ExitBadArguments)
	}

	if projectSlug == "" {
		info, gitErr := gitremote.Detect()
		if gitErr != nil {
			return "", clierrors.New("git.detect_failed", "Could not detect project from git", gitErr.Error()).
				WithSuggestions(
					"Run from inside a git repository with a GitHub, Bitbucket, or GitLab remote",
					"Or specify --project explicitly",
				).
				WithExitCode(clierrors.ExitBadArguments)
		}
		projectSlug = info.Slug
	}

	pipeline, pipelineErr := client.GetPipelineByNumber(ctx, projectSlug, number)
	if pipelineErr != nil {
		return "", apiErr(pipelineErr, arg)
	}
	return pipeline.ID, nil
}
