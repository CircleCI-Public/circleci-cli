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

func newGetCmd() *cobra.Command {
	var (
		projectSlug string
		branch      string
		jsonOut     bool
	)

	cmd := &cobra.Command{
		Use:   "get [<pipeline-id-or-number>]",
		Short: "Get a pipeline's status",
		Long: heredoc.Doc(`
			Display the status of a CircleCI pipeline and its workflows.

			When called without arguments, the project and branch are inferred from
			the current git repository's remote and checked-out branch.

			Pass a pipeline number (e.g. 75) or UUID to look up a specific pipeline.
			When using a number, the project is inferred from the git remote unless
			overridden with --project.

			JSON fields: id, number, status, project_slug, branch, revision,
			             created_at, updated_at, trigger,
			             workflows[].id/name/status/jobs[].number/name/status/type
		`),
		Example: heredoc.Doc(`
			# Get the latest pipeline for the current branch
			$ circleci pipeline get

			# Get a pipeline by number
			$ circleci pipeline get 75

			# Get a pipeline by UUID
			$ circleci pipeline get 5034460f-c7c4-4c43-9457-de07e2029e7b

			# Output as JSON for scripting
			$ circleci pipeline get --json
		`),
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			streams := iostream.FromCmd(cmd)
			client, err := cmdutil.LoadClient(ctx, cmd)
			if err != nil {
				return err
			}
			return runGet(ctx, client, streams, args, projectSlug, branch, jsonOut)
		},
	}

	cmd.Flags().StringVar(&projectSlug, "project", "", "Project slug (e.g. gh/org/repo); used when looking up by number")
	cmd.Flags().StringVarP(&branch, "branch", "b", "", "Branch name (defaults to current branch)")
	cmd.Flags().BoolVar(&jsonOut, "json", false, "Output as JSON")

	return cmd
}

// pipelineGetOutput is the JSON shape returned by this command.
type pipelineGetOutput struct {
	ID          string           `json:"id"`
	Number      int64            `json:"number"`
	Status      string           `json:"status"`
	ProjectSlug string           `json:"project_slug"`
	Branch      string           `json:"branch,omitempty"`
	Revision    string           `json:"revision,omitempty"`
	CreatedAt   string           `json:"created_at"`
	UpdatedAt   string           `json:"updated_at"`
	Trigger     triggerOutput    `json:"trigger"`
	Errors      []errorOutput    `json:"errors,omitempty"`
	Workflows   []workflowOutput `json:"workflows"`
}

type errorOutput struct {
	Type    string `json:"type"`
	Message string `json:"message"`
}

type triggerOutput struct {
	Type  string `json:"type"`
	Actor string `json:"actor"`
}

type workflowOutput struct {
	ID     string      `json:"id"`
	Name   string      `json:"name"`
	Status string      `json:"status"`
	Jobs   []jobOutput `json:"jobs"`
}

type jobOutput struct {
	Number int64  `json:"number,omitempty"`
	Name   string `json:"name"`
	Status string `json:"status"`
	Type   string `json:"type"`
}

func runGet(ctx context.Context, client *apiclient.Client, streams iostream.Streams, args []string, projectSlug, branch string, jsonOut bool) error {
	var (
		err      error
		pipeline *apiclient.Pipeline
	)

	if len(args) == 1 {
		arg := args[0]
		if looksLikeNumber(arg) {
			// Pipeline number: need a project slug to resolve it.
			number, _ := strconv.ParseInt(arg, 10, 64)
			if projectSlug == "" {
				info, err := gitremote.Detect()
				if err != nil {
					return clierrors.New("git.detect_failed", "Could not detect project from git",
						err.Error()).
						WithSuggestions(
							"Run from inside a git repository with a GitHub, Bitbucket, or GitLab remote",
							"Or specify the project: circleci pipeline get "+arg+" --project gh/org/repo",
						).
						WithExitCode(clierrors.ExitBadArguments)
				}
				projectSlug = info.Slug
			}
			pipeline, err = client.GetPipelineByNumber(ctx, projectSlug, number)
			if err != nil {
				return apiErr(err, fmt.Sprintf("%s #%s", projectSlug, arg))
			}
		} else {
			// UUID
			pipeline, err = client.GetPipeline(ctx, arg)
			if err != nil {
				return apiErr(err, arg)
			}
		}
	} else {
		// No arg: infer from git context.
		info, err := gitremote.Detect()
		if err != nil {
			return clierrors.New("git.detect_failed", "Could not detect project from git",
				err.Error()).
				WithSuggestions(
					"Run from inside a git repository with a GitHub remote",
					"Or provide a pipeline number or UUID: circleci pipeline get <number>",
				).
				WithExitCode(clierrors.ExitBadArguments)
		}

		effectiveBranch := branch
		if effectiveBranch == "" {
			effectiveBranch = info.Branch
		}

		sp := streams.Spinner(!jsonOut, fmt.Sprintf("Fetching latest pipeline for %s on branch %s", info.Slug, effectiveBranch))
		pipeline, err = client.GetLatestPipeline(ctx, info.Slug, effectiveBranch)
		sp.Stop()
		if err != nil {
			return apiErr(err, fmt.Sprintf("%s@%s", info.Slug, effectiveBranch))
		}
	}

	workflows, err := client.GetPipelineWorkflows(ctx, pipeline.ID)
	if err != nil {
		return apiErr(err, pipeline.ID)
	}

	wfJobs := make([][]apiclient.WorkflowJob, len(workflows))
	for i, wf := range workflows {
		jobs, err := client.GetWorkflowJobs(ctx, wf.ID)
		if err != nil {
			return apiErr(err, wf.ID)
		}
		wfJobs[i] = jobs
	}

	out := buildOutput(pipeline, workflows, wfJobs)

	if jsonOut {
		enc := json.NewEncoder(streams.Out)
		enc.SetIndent("", "  ")
		return enc.Encode(out)
	}

	printPipeline(streams, out)
	return nil
}

func buildOutput(p *apiclient.Pipeline, workflows []apiclient.PipelineWorkflowSummary, wfJobs [][]apiclient.WorkflowJob) pipelineGetOutput {
	wflows := make([]workflowOutput, len(workflows))
	for i, w := range workflows {
		jobs := make([]jobOutput, 0, len(wfJobs[i]))
		for _, j := range wfJobs[i] {
			jobs = append(jobs, jobOutput{
				Number: j.JobNumber,
				Name:   j.Name,
				Status: j.Status,
				Type:   j.Type,
			})
		}
		wflows[i] = workflowOutput{ID: w.ID, Name: w.Name, Status: w.Status, Jobs: jobs}
	}

	var branch, revision string
	if p.VCS != nil {
		branch = p.VCS.Branch
		revision = p.VCS.Revision
		if len(revision) > 7 {
			revision = revision[:7]
		}
	}

	errs := make([]errorOutput, len(p.Errors))
	for i, e := range p.Errors {
		errs[i] = errorOutput{Type: e.Type, Message: e.Message}
	}

	return pipelineGetOutput{
		ID:          p.ID,
		Number:      p.Number,
		Status:      deriveStatus(p.State, wflows),
		ProjectSlug: p.ProjectSlug,
		Branch:      branch,
		Revision:    revision,
		CreatedAt:   p.CreatedAt.Format("2006-01-02 15:04:05 UTC"),
		UpdatedAt:   p.UpdatedAt.Format("2006-01-02 15:04:05 UTC"),
		Trigger:     triggerOutput{Type: p.Trigger.Type, Actor: p.Trigger.Actor.Login},
		Errors:      errs,
		Workflows:   wflows,
	}
}

// deriveStatus computes a meaningful overall status from workflow statuses.
// The pipeline-level state from the API reflects creation/setup lifecycle
// (almost always "created") and is not useful as an execution status.
func deriveStatus(pipelineState string, workflows []workflowOutput) string {
	// A pipeline-level error (e.g. config error) takes priority.
	if pipelineState == "errored" {
		return "errored"
	}
	if len(workflows) == 0 {
		return pipelineState
	}
	// Walk workflows in priority order: failure > running > on_hold > canceled > success.
	for _, wf := range workflows {
		switch wf.Status {
		case "failed", "error", "failing":
			return "failed"
		}
	}
	for _, wf := range workflows {
		if wf.Status == "running" {
			return "running"
		}
	}
	for _, wf := range workflows {
		if wf.Status == "on_hold" {
			return "on_hold"
		}
	}
	for _, wf := range workflows {
		if wf.Status == "canceled" {
			return "canceled"
		}
	}
	for _, wf := range workflows {
		if wf.Status == "success" {
			return "success"
		}
	}
	return pipelineState
}

// looksLikeNumber returns true if s is a plain positive integer (pipeline number),
// as opposed to a UUID (which contains hyphens).
func looksLikeNumber(s string) bool {
	return !strings.Contains(s, "-") && len(s) > 0
}

func printPipeline(streams iostream.Streams, p pipelineGetOutput) {
	streams.Printf("Pipeline #%d  %s\n", p.Number, p.ID)
	streams.Printf("Project:   %s\n", p.ProjectSlug)
	if p.Branch != "" {
		streams.Printf("Branch:    %s\n", p.Branch)
	}
	if p.Revision != "" {
		streams.Printf("Commit:    %s\n", p.Revision)
	}
	streams.Printf("Status:    %s\n", p.Status)
	streams.Printf("Triggered: %s by %s (%s)\n", p.CreatedAt, p.Trigger.Actor, p.Trigger.Type)

	if len(p.Errors) > 0 {
		streams.Printf("\nErrors:\n")
		for _, e := range p.Errors {
			streams.Printf("  [%s] %s\n", e.Type, e.Message)
		}
	}

	if len(p.Workflows) > 0 {
		streams.Printf("\nWorkflows:\n")
		for _, w := range p.Workflows {
			streams.Printf("  %-30s  %s\n", w.Name, w.Status)
			for _, j := range w.Jobs {
				if j.Type == "approval" {
					streams.Printf("    %-36s  %s\n", j.Name, j.Status)
				} else {
					streams.Printf("    %-36s  %s  #%d\n", j.Name, j.Status, j.Number)
				}
			}
		}
	}
}
