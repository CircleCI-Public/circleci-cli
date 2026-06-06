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
	"fmt"
	"strings"

	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"

	"github.com/CircleCI-Public/circleci-cli/internal/apiclient"
	"github.com/CircleCI-Public/circleci-cli/internal/cmdutil"
	"github.com/CircleCI-Public/circleci-cli/internal/iostream"
	"github.com/CircleCI-Public/circleci-cli/internal/mdtable"
)

func newGetCmd() *cobra.Command {
	var jsonOut bool

	cmd := &cobra.Command{
		Use:   "get <workflow-id>",
		Short: "Get workflow details",
		Long: heredoc.Doc(`
			Display the status and jobs of a CircleCI workflow.

			Workflow IDs are shown in the output of 'circleci run get'.

			JSON fields: id, name, status, run_id, run_number,
			             project_name, created_at, stopped_at,
			             jobs[].id/name/status
		`),
		Example: heredoc.Doc(`
			# Get workflow details
			$ circleci workflow get 5034460f-c7c4-4c43-9457-de07e2029e7b

			# Output as JSON
			$ circleci workflow get 5034460f-c7c4-4c43-9457-de07e2029e7b --json

			# Get workflow ID from a run
			$ circleci run get | grep -A1 "Workflows"
		`),
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if cliErr := cmdutil.RequireArgs(args, "workflow-id"); cliErr != nil {
				return cliErr
			}
			ctx := cmd.Context()
			client, err := cmdutil.LoadClient(ctx)
			if err != nil {
				return err
			}
			return runGet(ctx, client, args[0], jsonOut)
		},
	}

	cmdutil.AddJSONFlag(cmd, &jsonOut)
	cmdutil.AddJQFlag(cmd)
	return cmd
}

type workflowGetOutput struct {
	ID          string      `json:"id"`
	Name        string      `json:"name"`
	Status      string      `json:"status"`
	RunID       string      `json:"run_id"`
	RunNumber   int64       `json:"run_number"`
	ProjectName string      `json:"project_name"`
	CreatedAt   string      `json:"created_at"`
	StoppedAt   string      `json:"stopped_at,omitempty"`
	Jobs        []jobOutput `json:"jobs"`
}

type jobOutput struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Status string `json:"status"`
}

func runGet(ctx context.Context, client *apiclient.Client, id string, jsonOut bool) error {
	wf, err := client.GetWorkflow(ctx, id)
	if err != nil {
		return apiErr(err, id)
	}

	jobs, err := client.GetWorkflowJobsV3(ctx, id)
	if err != nil {
		return apiErr(err, id)
	}

	out := workflowGetOutput{
		ID:          wf.ID,
		Name:        wf.Name,
		Status:      wf.Status,
		RunID:       wf.PipelineID,
		RunNumber:   wf.PipelineNumber,
		ProjectName: projectNameFromSlug(wf.ProjectSlug),
		CreatedAt:   wf.CreatedAt.Format("2006-01-02 15:04:05 UTC"),
	}
	if wf.StoppedAt != nil {
		out.StoppedAt = wf.StoppedAt.Format("2006-01-02 15:04:05 UTC")
	}
	for _, j := range jobs {
		out.Jobs = append(out.Jobs, jobOutput{
			ID:     j.ID,
			Name:   j.Name,
			Status: j.Status,
		})
	}

	if jsonOut {
		return iostream.PrintJSON(ctx, out)
	}

	printGet(ctx, out)
	return nil
}

func printGet(ctx context.Context, w workflowGetOutput) {
	var md strings.Builder
	md.WriteString("# Workflow\n")

	_, _ = fmt.Fprintf(&md, "- ID: `%s`\n", w.ID)
	_, _ = fmt.Fprintf(&md, "- Name: %s\n", w.Name)
	_, _ = fmt.Fprintf(&md, "- Run:\n")
	_, _ = fmt.Fprintf(&md, "  - Number: #%d\n", w.RunNumber)
	_, _ = fmt.Fprintf(&md, "  - ID: `%s`\n", w.RunID)
	_, _ = fmt.Fprintf(&md, "- Project: %s\n", w.ProjectName)
	_, _ = fmt.Fprintf(&md, "- Status: %s\n", w.Status)
	_, _ = fmt.Fprintf(&md, "- Created: %s\n", w.CreatedAt)
	if w.StoppedAt != "" {
		_, _ = fmt.Fprintf(&md, "- Stopped:  %s\n", w.StoppedAt)
	}

	if len(w.Jobs) > 0 {
		_, _ = fmt.Fprintf(&md, "\n## Jobs\n")
		table := mdtable.New("Name", "Status", "ID")
		for _, j := range w.Jobs {
			table.Row(j.Name, j.Status, j.ID)
		}
		md.WriteString(table.Render())
	}
	iostream.PrintMarkdown(ctx, md.String())
}

func projectNameFromSlug(slug string) string {
	parts := strings.Split(slug, "/")
	if len(parts) > 0 {
		return parts[len(parts)-1]
	}
	return slug
}
