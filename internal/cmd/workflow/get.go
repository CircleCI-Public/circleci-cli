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
	"github.com/google/uuid"
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
		Annotations: map[string]string{
			"help:arguments": heredoc.Docf(`
				%[1]s<workflow-id>%[1]s is the UUID of the workflow to look up. Workflow IDs are
				shown in the output of %[1]scircleci run get%[1]s.
			`, "`"),
		},
		Long: heredoc.Doc(`
			Display the status and jobs of a CircleCI workflow.

			Workflow IDs are shown in the output of 'circleci run get'.

			JSON fields: id, name, phase, outcome, current_outcome, run_id,
			             created_at, ended_at,
			             jobs[].id/name/phase/outcome/current_outcome/type
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
	ID             uuid.UUID   `json:"id"`
	Name           string      `json:"name"`
	Phase          string      `json:"phase"`
	Outcome        string      `json:"outcome,omitempty"`
	CurrentOutcome string      `json:"current_outcome,omitempty"`
	RunID          uuid.UUID   `json:"run_id"`
	CreatedAt      string      `json:"created_at"`
	EndedAt        string      `json:"ended_at,omitempty"`
	Jobs           []jobOutput `json:"jobs"`
}

type jobOutput struct {
	ID             uuid.UUID `json:"id"`
	Name           string    `json:"name"`
	Phase          string    `json:"phase"`
	Outcome        string    `json:"outcome,omitempty"`
	CurrentOutcome string    `json:"current_outcome,omitempty"`
	Type           string    `json:"type,omitempty"`
}

// Get renders a workflow's details exactly as "circleci workflow get" does.
// It is exported so interactive callers (e.g. "circleci run get") can reuse
// the same output without duplicating the formatting code.
func Get(ctx context.Context, client *apiclient.Client, idStr string, jsonOut bool) error {
	return runGet(ctx, client, idStr, jsonOut)
}

func runGet(ctx context.Context, client *apiclient.Client, idStr string, jsonOut bool) error {
	id, err := uuid.Parse(idStr)
	if err != nil {
		return err
	}

	wf, err := client.GetWorkflowV3(ctx, id)
	if err != nil {
		return apiErr(err, id.String())
	}

	jobs, err := client.GetWorkflowJobsV3(ctx, id)
	if err != nil {
		return apiErr(err, id.String())
	}

	out := workflowGetOutput{
		ID:             wf.ID,
		Name:           wf.Name,
		Phase:          wf.Phase,
		Outcome:        wf.Outcome,
		CurrentOutcome: wf.CurrentOutcome,
		RunID:          wf.RunID,
		CreatedAt:      wf.CreatedAt.Format("2006-01-02 15:04:05 UTC"),
	}
	if wf.EndedAt != nil {
		out.EndedAt = wf.EndedAt.Format("2006-01-02 15:04:05 UTC")
	}
	for _, j := range jobs {
		out.Jobs = append(out.Jobs, jobOutput{
			ID:             j.ID,
			Name:           j.Name,
			Phase:          j.Phase,
			Outcome:        j.Outcome,
			CurrentOutcome: j.CurrentOutcome,
			Type:           j.Type,
		})
	}

	if jsonOut {
		return iostream.PrintJSON(ctx, out)
	}

	appURL, err := cmdutil.AppURL(ctx)
	if err != nil {
		return err
	}

	u := cmdutil.WorkflowURL(appURL, id)

	printGet(ctx, out, u)
	return nil
}

func printGet(ctx context.Context, w workflowGetOutput, u string) {
	var md strings.Builder
	md.WriteString("# Workflow\n")

	_, _ = fmt.Fprintf(&md, "- ID: `%s`\n", w.ID)
	_, _ = fmt.Fprintf(&md, "- Name: %s\n", w.Name)
	_, _ = fmt.Fprintf(&md, "- Run: `%s`\n", w.RunID)
	_, _ = fmt.Fprintf(&md, "- URL: %s\n", u)
	_, _ = fmt.Fprintf(&md, "- Status: %s\n", apiclient.PhaseOutcomeStatus(w.Phase, w.Outcome, w.CurrentOutcome))
	_, _ = fmt.Fprintf(&md, "- Created: %s\n", w.CreatedAt)
	if w.EndedAt != "" {
		_, _ = fmt.Fprintf(&md, "- Ended: %s\n", w.EndedAt)
	}

	if len(w.Jobs) > 0 {
		_, _ = fmt.Fprintf(&md, "\n## Jobs\n")
		table := mdtable.New("Name", "Status", "Type", "ID")
		for _, j := range w.Jobs {
			table.Row(j.Name, apiclient.PhaseOutcomeStatus(j.Phase, j.Outcome, j.CurrentOutcome), j.Type, "`"+j.ID.String()+"`")
		}
		md.WriteString(table.Render())
	}
	iostream.PrintMarkdown(ctx, md.String())
}
