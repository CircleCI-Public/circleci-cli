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

	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"

	"github.com/CircleCI-Public/circleci-cli-v2/internal/cmdutil"
	"github.com/CircleCI-Public/circleci-cli-v2/internal/iostream"
)

func newGetCmd() *cobra.Command {
	var jsonOut bool

	cmd := &cobra.Command{
		Use:   "get <workflow-id>",
		Short: "Get workflow details",
		Long: heredoc.Doc(`
			Display the status and jobs of a CircleCI workflow.

			Workflow IDs are shown in the output of 'circleci pipeline get'.

			JSON fields: id, name, status, pipeline_id, pipeline_number,
			             project_slug, created_at, stopped_at, jobs
		`),
		Example: heredoc.Doc(`
			# Get workflow details
			$ circleci workflow get 5034460f-c7c4-4c43-9457-de07e2029e7b

			# Output as JSON
			$ circleci workflow get 5034460f-c7c4-4c43-9457-de07e2029e7b --json

			# Get workflow ID from a pipeline
			$ circleci pipeline get | grep -A1 "Workflows"
		`),
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if cliErr := cmdutil.RequireArgs(args, "workflow-id"); cliErr != nil {
				return cliErr
			}
			ctx := cmd.Context()
			streams := iostream.FromCmd(cmd)
			return runGet(ctx, streams, args[0], jsonOut)
		},
	}

	cmd.Flags().BoolVar(&jsonOut, "json", false, "Output as JSON")
	return cmd
}

type workflowGetOutput struct {
	ID             string      `json:"id"`
	Name           string      `json:"name"`
	Status         string      `json:"status"`
	PipelineID     string      `json:"pipeline_id"`
	PipelineNumber int64       `json:"pipeline_number"`
	ProjectSlug    string      `json:"project_slug"`
	CreatedAt      string      `json:"created_at"`
	StoppedAt      string      `json:"stopped_at,omitempty"`
	Jobs           []jobOutput `json:"jobs"`
}

type jobOutput struct {
	Number int64  `json:"number,omitempty"`
	Name   string `json:"name"`
	Status string `json:"status"`
	Type   string `json:"type"`
}

func runGet(ctx context.Context, streams iostream.Streams, id string, jsonOut bool) error {
	client, cliErr := cmdutil.LoadClient()
	if cliErr != nil {
		return cliErr
	}

	wf, err := client.GetWorkflow(ctx, id)
	if err != nil {
		return apiErr(err, id)
	}

	jobs, err := client.GetWorkflowJobs(ctx, id)
	if err != nil {
		return apiErr(err, id)
	}

	out := workflowGetOutput{
		ID:             wf.ID,
		Name:           wf.Name,
		Status:         wf.Status,
		PipelineID:     wf.PipelineID,
		PipelineNumber: wf.PipelineNumber,
		ProjectSlug:    wf.ProjectSlug,
		CreatedAt:      wf.CreatedAt.Format("2006-01-02 15:04:05 UTC"),
	}
	if wf.StoppedAt != nil {
		out.StoppedAt = wf.StoppedAt.Format("2006-01-02 15:04:05 UTC")
	}
	for _, j := range jobs {
		out.Jobs = append(out.Jobs, jobOutput{
			Number: j.JobNumber,
			Name:   j.Name,
			Status: j.Status,
			Type:   j.Type,
		})
	}

	if jsonOut {
		enc := json.NewEncoder(streams.Out)
		enc.SetIndent("", "  ")
		return enc.Encode(out)
	}

	printGet(streams, out)
	return nil
}

func printGet(streams iostream.Streams, w workflowGetOutput) {
	streams.Printf("Workflow  %s\n", w.ID)
	streams.Printf("Name:     %s\n", w.Name)
	streams.Printf("Pipeline: #%d (%s)\n", w.PipelineNumber, w.PipelineID)
	streams.Printf("Project:  %s\n", w.ProjectSlug)
	streams.Printf("Status:   %s\n", w.Status)
	streams.Printf("Created:  %s\n", w.CreatedAt)
	if w.StoppedAt != "" {
		streams.Printf("Stopped:  %s\n", w.StoppedAt)
	}

	if len(w.Jobs) > 0 {
		streams.Printf("\nJobs:\n")
		for _, j := range w.Jobs {
			if j.Type == "approval" {
				streams.Printf("  %-36s  %s\n", j.Name, j.Status)
			} else {
				streams.Printf("  %-36s  %s  #%d\n", j.Name, j.Status, j.Number)
			}
		}
	}
}
