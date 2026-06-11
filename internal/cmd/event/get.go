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

package event

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"

	"github.com/CircleCI-Public/circleci-cli/internal/apiclient"
	"github.com/CircleCI-Public/circleci-cli/internal/cmdutil"
	"github.com/CircleCI-Public/circleci-cli/internal/gitremote"
	"github.com/CircleCI-Public/circleci-cli/internal/httpcl"
	"github.com/CircleCI-Public/circleci-cli/internal/iostream"
	"github.com/CircleCI-Public/circleci-cli/internal/mdtable"
)

const (
	statusCanceled = "canceled"
)

func newGetCmd() *cobra.Command {
	var (
		projectSlug string
		branch      string
		jsonOut     bool
	)

	cmd := &cobra.Command{
		Use:   "get [<event-id>]",
		Short: "Get an event's status",
		Long: heredoc.Doc(`
			Display the status of a CircleCI event and its workflows.

			When called without arguments, the project and branch are inferred from
			the current git repository's remote and checked-out branch.

			Pass an event UUID to look up a specific event.

			JSON fields: id, phase, outcome, current_outcome, branch, revision,
			             created_at, errors[].type/message,
			             workflows[].id/name/phase/outcome/current_outcome/duration/
			             jobs[].id/name/phase/outcome/current_outcome/type
		`),
		Example: heredoc.Doc(`
			# Get the latest event for the current branch
			$ circleci event get

			# Get an event by UUID
			$ circleci event get 5034460f-c7c4-4c43-9457-de07e2029e7b

			# Output as JSON for scripting
			$ circleci event get --json
		`),
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			client, err := cmdutil.LoadClient(ctx)
			if err != nil {
				return err
			}
			return eventGet(ctx, client, args, projectSlug, branch, jsonOut)
		},
	}

	cmd.Flags().StringVar(&projectSlug, "project", "", "Project slug (e.g. gh/org/repo); used for latest-event lookup")
	cmd.Flags().StringVarP(&branch, "branch", "b", "", "Branch name (defaults to current branch)")
	cmdutil.AddJSONFlag(cmd, &jsonOut)
	cmdutil.AddJQFlag(cmd)

	return cmd
}

type eventGetOutput struct {
	ID             string           `json:"id"`
	Phase          string           `json:"phase"`
	Outcome        string           `json:"outcome,omitempty"`
	CurrentOutcome string           `json:"current_outcome,omitempty"`
	Branch         string           `json:"branch,omitempty"`
	Revision       string           `json:"revision,omitempty"`
	CreatedAt      string           `json:"created_at"`
	Errors         []errorOutput    `json:"errors,omitempty"`
	Workflows      []workflowOutput `json:"workflows"`
}

type errorOutput struct {
	Type    string `json:"type"`
	Message string `json:"message"`
}

type workflowOutput struct {
	ID             string      `json:"id"`
	Name           string      `json:"name"`
	Phase          string      `json:"phase"`
	Outcome        string      `json:"outcome,omitempty"`
	CurrentOutcome string      `json:"current_outcome,omitempty"`
	Duration       string      `json:"duration,omitempty"`
	Jobs           []jobOutput `json:"jobs"`
}

type jobOutput struct {
	ID             string `json:"id"`
	Name           string `json:"name"`
	Phase          string `json:"phase"`
	Outcome        string `json:"outcome,omitempty"`
	CurrentOutcome string `json:"current_outcome,omitempty"`
	Type           string `json:"type,omitempty"`
}

func eventGet(ctx context.Context, client *apiclient.Client, args []string, projectSlug, branch string, jsonOut bool) error {
	var (
		r   *apiclient.Event
		err error
	)

	if len(args) == 1 {
		r, err = client.GetEvent(ctx, args[0])
		if err != nil {
			return apiErr(err, args[0])
		}
	} else {
		info, err := gitremote.Detect()
		if err != nil {
			return cmdutil.GitDetectErr(err, "Or provide an event UUID: circleci event get <uuid>")
		}
		if projectSlug == "" {
			projectSlug = info.Slug
		}

		effectiveBranch := branch
		if effectiveBranch == "" {
			effectiveBranch = info.Branch
		}

		proj, err := client.GetProjectInfo(ctx, projectSlug)
		if err != nil {
			return apiErr(err, projectSlug)
		}

		sp := iostream.Spinner(ctx, !jsonOut, fmt.Sprintf("Fetching latest event for %s on branch %s", projectSlug, effectiveBranch))
		now := time.Now().UTC()
		events, searchErr := client.SearchEvents(ctx, apiclient.EventSearchParams{
			ProjectIDs: []string{proj.ID},
			From:       now.AddDate(0, 0, -90),
			To:         now,
			Filter:     apiclient.BuildEventFilter(effectiveBranch, ""),
			Limit:      1,
		})
		sp.Stop()
		if searchErr != nil {
			return apiErr(searchErr, fmt.Sprintf("%s@%s", projectSlug, effectiveBranch))
		}
		if len(events) == 0 {
			return apiErr(fmt.Errorf("no events found"), fmt.Sprintf("%s@%s", projectSlug, effectiveBranch))
		}
		r = &events[0]
	}

	workflows, err := client.GetEventWorkflows(ctx, r.ID)
	if err != nil {
		// The workflows API can 404 for an event that exists (e.g. workflows
		// not yet materialised) — still show the event, with no workflows.
		if !httpcl.HasStatusCode(err, http.StatusNotFound) {
			return apiErr(err, r.ID)
		}
		workflows = nil
	}

	wfJobs := make([][]apiclient.WorkflowJobV3, len(workflows))
	for i, wf := range workflows {
		jobs, err := client.GetWorkflowJobsV3(ctx, wf.ID)
		if err != nil {
			return apiErr(err, wf.ID)
		}
		wfJobs[i] = jobs
	}

	out := buildOutput(r, workflows, wfJobs)

	if jsonOut {
		return iostream.PrintJSON(ctx, out)
	}

	printRun(ctx, out)
	return nil
}

func buildOutput(r *apiclient.Event, workflows []apiclient.WorkflowV3, wfJobs [][]apiclient.WorkflowJobV3) eventGetOutput {
	wflows := make([]workflowOutput, len(workflows))
	for i, w := range workflows {
		jobs := make([]jobOutput, 0, len(wfJobs[i]))
		for _, j := range wfJobs[i] {
			jobs = append(jobs, jobOutput{
				ID:             j.ID,
				Name:           j.Name,
				Phase:          j.Phase,
				Outcome:        j.Outcome,
				CurrentOutcome: j.CurrentOutcome,
				Type:           j.Type,
			})
		}
		var dur string
		if w.EndedAt != nil {
			dur = formatElapsed(w.EndedAt.Sub(w.CreatedAt))
		}
		wflows[i] = workflowOutput{
			ID:             w.ID,
			Name:           w.Name,
			Phase:          w.Phase,
			Outcome:        w.Outcome,
			CurrentOutcome: w.CurrentOutcome,
			Duration:       dur,
			Jobs:           jobs,
		}
	}

	revision := r.Revision
	if len(revision) > 7 {
		revision = revision[:7]
	}

	errs := make([]errorOutput, len(r.Errors))
	for i, e := range r.Errors {
		errs[i] = errorOutput{Type: e.Type, Message: e.Message}
	}

	return eventGetOutput{
		ID:             r.ID,
		Phase:          r.Phase,
		Outcome:        r.Outcome,
		CurrentOutcome: r.CurrentOutcome,
		Branch:         r.Branch,
		Revision:       revision,
		CreatedAt:      r.CreatedAt.Format("2006-01-02 15:04:05 UTC"),
		Errors:         errs,
		Workflows:      wflows,
	}
}

// deriveDisplayStatus computes a meaningful overall display status from
// workflow phases and outcomes.
func deriveDisplayStatus(r eventGetOutput) string {
	if len(r.Workflows) == 0 {
		return apiclient.PhaseOutcomeStatus(r.Phase, r.Outcome, r.CurrentOutcome)
	}
	for _, wf := range r.Workflows {
		if wf.Outcome == "failed" || wf.Outcome == "errored" || wf.CurrentOutcome == "failed" {
			return "failed"
		}
	}
	for _, wf := range r.Workflows {
		if wf.Phase != "ended" {
			return "running"
		}
	}
	for _, wf := range r.Workflows {
		if wf.Outcome == statusCanceled {
			return statusCanceled
		}
	}
	for _, wf := range r.Workflows {
		if wf.Outcome == "succeeded" {
			return "succeeded"
		}
	}
	return apiclient.PhaseOutcomeStatus(r.Phase, r.Outcome, r.CurrentOutcome)
}

func printRun(ctx context.Context, r eventGetOutput) {
	var md strings.Builder
	md.WriteString("# Event\n")

	_, _ = fmt.Fprintf(&md, "- ID: `%s`\n", r.ID)
	if r.Branch != "" {
		_, _ = fmt.Fprintf(&md, "- Branch: %s\n", r.Branch)
	}
	if r.Revision != "" {
		_, _ = fmt.Fprintf(&md, "- Commit: %s\n", r.Revision)
	}
	_, _ = fmt.Fprintf(&md, "- Status: %s\n", deriveDisplayStatus(r))
	_, _ = fmt.Fprintf(&md, "- Created: %s\n", r.CreatedAt)

	if len(r.Errors) > 0 {
		md.WriteString("\n## Errors\n")
		for _, e := range r.Errors {
			_, _ = fmt.Fprintf(&md, "- **%s**: %s\n", e.Type, e.Message)
		}
	}
	md.WriteString("\n")

	if len(r.Workflows) == 0 {
		md.WriteString("No workflows found for this event.\n")
	} else {
		md.WriteString("## Workflows\n")
		for _, w := range r.Workflows {
			_, _ = fmt.Fprintf(&md, "### %s\n", w.Name)
			_, _ = fmt.Fprintf(&md, "- ID: `%s`\n", w.ID)
			_, _ = fmt.Fprintf(&md, "- Status: %s\n", apiclient.PhaseOutcomeStatus(w.Phase, w.Outcome, w.CurrentOutcome))
			if w.Duration != "" {
				_, _ = fmt.Fprintf(&md, "- Duration: %s\n", w.Duration)
			}
			md.WriteString("#### Jobs\n")
			mdTable := mdtable.New("Name", "Status", "Type", "ID")
			for _, j := range w.Jobs {
				mdTable.Row(j.Name, apiclient.PhaseOutcomeStatus(j.Phase, j.Outcome, j.CurrentOutcome), j.Type, "`"+j.ID+"`")
			}
			md.WriteString(mdTable.Render())
		}
		md.WriteString("\n")
	}

	iostream.PrintMarkdown(ctx, md.String())
}
