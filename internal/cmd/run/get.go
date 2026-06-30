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

package run

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/MakeNowJust/heredoc"
	"github.com/google/uuid"
	"github.com/spf13/cobra"
	"golang.org/x/sync/errgroup"

	"github.com/CircleCI-Public/circleci-cli/internal/apiclient"
	"github.com/CircleCI-Public/circleci-cli/internal/cmd/job"
	"github.com/CircleCI-Public/circleci-cli/internal/cmd/workflow"
	"github.com/CircleCI-Public/circleci-cli/internal/cmdutil"
	"github.com/CircleCI-Public/circleci-cli/internal/gitremote"
	"github.com/CircleCI-Public/circleci-cli/internal/httpcl"
	"github.com/CircleCI-Public/circleci-cli/internal/iostream"
	"github.com/CircleCI-Public/circleci-cli/internal/mdtable"
	"github.com/CircleCI-Public/circleci-cli/internal/ui"
)

const (
	statusCanceled     = "canceled"
	maxWorkflowFetches = 8
)

func newGetCmd() *cobra.Command {
	var (
		projectSlug string
		branch      string
		jsonOut     bool
	)

	cmd := &cobra.Command{
		Use:   "get [<run-id>]",
		Short: "Get a run's status",
		Annotations: map[string]string{
			"help:arguments": heredoc.Doc(`
				<run-id> is optional and is the UUID of the run to look up. When
				omitted, the latest run is resolved from the project and branch
				inferred from the current git repository's remote and checked-out
				branch (override with --project and --branch).
			`),
		},
		Long: heredoc.Doc(`
			Display the status of a CircleCI run and its workflows.

			When called without arguments, the project and branch are inferred from
			the current git repository's remote and checked-out branch.

			Pass a run UUID to look up a specific run.

			JSON fields: id, phase, outcome, current_outcome, branch, tag, revision,
			             created_at, errors[].type/message,
			             workflows[].id/name/phase/outcome/current_outcome/duration/
			             jobs[].id/name/phase/outcome/current_outcome/type
		`),
		Example: heredoc.Doc(`
			# Get the latest run for the current branch
			$ circleci run get

			# Get a run by UUID
			$ circleci run get 5034460f-c7c4-4c43-9457-de07e2029e7b

			# Output as JSON for scripting
			$ circleci run get --json
		`),
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			client, err := cmdutil.LoadClient(ctx)
			if err != nil {
				return err
			}
			return runGet(ctx, client, args, projectSlug, branch, jsonOut)
		},
	}

	cmd.Flags().StringVar(&projectSlug, "project", "", "Project slug (e.g. gh/org/repo); used for latest-run lookup")
	cmd.Flags().StringVarP(&branch, "branch", "b", "", "Branch name (defaults to current branch)")
	cmdutil.AddJSONFlag(cmd, &jsonOut)
	cmdutil.AddJQFlag(cmd)

	return cmd
}

type runGetOutput struct {
	ID             uuid.UUID        `json:"id"`
	Phase          string           `json:"phase"`
	Outcome        string           `json:"outcome,omitempty"`
	CurrentOutcome string           `json:"current_outcome,omitempty"`
	Branch         string           `json:"branch,omitempty"`
	Tag            string           `json:"tag,omitempty"`
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
	ID             uuid.UUID   `json:"id"`
	Name           string      `json:"name"`
	Phase          string      `json:"phase"`
	Outcome        string      `json:"outcome,omitempty"`
	CurrentOutcome string      `json:"current_outcome,omitempty"`
	Duration       string      `json:"duration,omitempty"`
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

func runGet(ctx context.Context, client *apiclient.Client, args []string, projectSlug, branch string, jsonOut bool) error {
	// With no run ID and an interactive terminal, walk the user through a series
	// of pickers (run → workflow → job) instead of silently resolving the latest
	// run. JSON output stays non-interactive so scripting is unaffected.
	if len(args) == 0 && !jsonOut && iostream.IsInteractive(ctx) {
		return runGetInteractive(ctx, client, projectSlug, branch)
	}

	var r *apiclient.RunV3

	if len(args) == 1 {
		id, err := uuid.Parse(args[0])
		if err != nil {
			return err
		}

		r, err = client.GetRunV3(ctx, id)
		if err != nil {
			return apiErr(err, id.String())
		}
	} else {
		info, err := gitremote.Detect()
		if err != nil {
			return cmdutil.GitDetectErr(err, "Or provide a run UUID: circleci run get <uuid>")
		}
		if projectSlug == "" {
			projectSlug = info.Slug
		}

		effectiveBranch := branch
		if effectiveBranch == "" {
			effectiveBranch = info.Branch
		}

		proj, err := client.GetProjectBySlug(ctx, projectSlug)
		if err != nil {
			return apiErr(err, projectSlug)
		}

		sp := iostream.Spinner(ctx, !jsonOut, fmt.Sprintf("Fetching latest run for %s on branch %s", projectSlug, effectiveBranch))
		now := time.Now().UTC()
		runs, searchErr := client.SearchRunsV3(ctx, apiclient.RunSearchParams{
			ProjectIDs: []string{proj.ID.String()},
			From:       now.AddDate(0, 0, -90),
			To:         now,
			Filter:     apiclient.BuildRunFilter(effectiveBranch, ""),
			Limit:      1,
		})
		sp.Stop()
		if searchErr != nil {
			return apiErr(searchErr, fmt.Sprintf("%s@%s", projectSlug, effectiveBranch))
		}
		if len(runs) == 0 {
			return apiErr(fmt.Errorf("no runs found"), fmt.Sprintf("%s@%s", projectSlug, effectiveBranch))
		}
		r = &runs[0]
	}

	return displayRun(ctx, client, r, jsonOut)
}

// displayRun fetches a run's workflows and their jobs, then renders the run
// summary as JSON or markdown. This is the shared output path for both the
// direct "circleci run get" invocation and the interactive picker's
// "see all workflows" choice, so both produce identical output.
func displayRun(ctx context.Context, client *apiclient.Client, r *apiclient.RunV3, jsonOut bool) error {
	workflows, err := client.GetRunWorkflowsV3(ctx, r.ID)
	if err != nil {
		// The workflows API can 404 for a run that exists (e.g. workflows
		// not yet materialised) — still show the run, with no workflows.
		if !httpcl.HasStatusCode(err, http.StatusNotFound) {
			return apiErr(err, r.ID.String())
		}
		workflows = nil
	}

	wfJobs := make([][]apiclient.WorkflowJobV3, len(workflows))
	for i, wf := range workflows {
		g, ctx := errgroup.WithContext(ctx)
		g.SetLimit(maxWorkflowFetches)
		g.Go(func() error {
			jobs, err := client.GetWorkflowJobsV3(ctx, wf.ID)
			if err != nil {
				return apiErr(err, wf.ID.String())
			}
			wfJobs[i] = jobs
			return nil
		})
		err = g.Wait()
		if err != nil {
			return err
		}
	}

	out := buildOutput(r, workflows, wfJobs)

	if jsonOut {
		return iostream.PrintJSON(ctx, out)
	}

	appURL, err := cmdutil.AppURL(ctx)
	if err != nil {
		return err
	}

	u := cmdutil.RunURL(appURL, r.ID)

	printRun(ctx, out, u)
	return nil
}

// runGetInteractive drives the interactive picker flow used when "circleci run
// get" is run in a terminal with no run ID:
//
//  1. Pick from the 10 most recent runs on the current branch.
//  2. Pick a workflow, or "see all workflows" to print the run summary
//     (identical to "circleci run get <id>").
//  3. For a chosen workflow, pick a job, or "all jobs in workflow" to print the
//     workflow summary (identical to "circleci workflow get <id>"). Picking a
//     job prints the job summary (identical to "circleci job get <id>").
//
// Each terminal choice reuses the existing command's output code so the
// rendered markdown matches its non-interactive counterpart exactly. The
// pickers are cancellable with esc/ctrl+c, which returns nil (no output).
func runGetInteractive(ctx context.Context, client *apiclient.Client, projectSlug, branch string) error {
	effectiveBranch := branch
	// Only consult the git remote for whatever the flags did not supply, so
	// --project and --branch together work outside a repository.
	if projectSlug == "" || effectiveBranch == "" {
		info, err := gitremote.Detect()
		if err != nil {
			return cmdutil.GitDetectErr(err, "Or provide a run UUID: circleci run get <uuid>")
		}
		if projectSlug == "" {
			projectSlug = info.Slug
		}
		if effectiveBranch == "" {
			effectiveBranch = info.Branch
		}
	}

	proj, err := client.GetProjectBySlug(ctx, projectSlug)
	if err != nil {
		return apiErr(err, projectSlug)
	}

	sp := iostream.Spinner(ctx, true, fmt.Sprintf("Fetching recent runs for %s on branch %s", projectSlug, effectiveBranch))
	now := time.Now().UTC()
	runs, err := client.SearchRunsV3(ctx, apiclient.RunSearchParams{
		ProjectIDs: []string{proj.ID.String()},
		From:       now.AddDate(0, 0, -90),
		To:         now,
		Filter:     apiclient.BuildRunFilter(effectiveBranch, ""),
		Limit:      10,
	})
	sp.Stop()
	if err != nil {
		return apiErr(err, fmt.Sprintf("%s@%s", projectSlug, effectiveBranch))
	}
	if len(runs) == 0 {
		return apiErr(fmt.Errorf("no runs found"), fmt.Sprintf("%s@%s", projectSlug, effectiveBranch))
	}

	runItems := make([]ui.RunGetItem, len(runs))
	for i := range runs {
		e := toListEntry(&runs[i])
		ref := e.Branch
		if ref == "" {
			ref = e.Tag
		}
		// The status symbol is supplied as an icon (the picker colors it and
		// renders raw text, so it cannot use the emoji from PhaseOutcomeStatus);
		// the label is e.g. "03d8295 [main] - 20 seconds ago".
		runItems[i] = ui.RunGetItem{
			ID:    runs[i].ID,
			Icon:  apiclient.PhaseOutcomeSymbol(runs[i].Phase, runs[i].Outcome, runs[i].CurrentOutcome),
			Label: fmt.Sprintf("%s [%s] - %s", e.Revision, ref, relativeTime(runs[i].CreatedAt)),
		}
	}

	model := ui.NewRunGetFlow(ctx, ui.RunGetFlowOptions{
		Runs:           runItems,
		Color:          iostream.ColorEnabled(ctx),
		FetchWorkflows: workflowItems(client),
		FetchJobs:      jobItems(client),
	})

	final, err := tea.NewProgram(model,
		tea.WithContext(ctx),
		tea.WithInput(iostream.In(ctx)),
		tea.WithOutput(iostream.Err(ctx)),
	).Run()
	if err != nil {
		return err
	}

	res := final.(ui.RunGetFlowModel).Result()
	if res.Err != nil {
		return res.Err
	}

	// The picker only collected the choice; the matching summary is printed now,
	// after the program has exited, reusing each command's existing output code.
	switch res.Action {
	case ui.RunGetActionShowRun:
		for i := range runs {
			if runs[i].ID == res.RunID {
				return displayRun(ctx, client, &runs[i], false)
			}
		}
		return nil
	case ui.RunGetActionShowWorkflow:
		return workflow.Get(ctx, client, res.WorkflowID.String(), false)
	case ui.RunGetActionShowJob:
		return job.Get(ctx, client, res.JobID.String(), false)
	case ui.RunGetActionCancel:
		return nil
	default:
		return nil
	}
}

// workflowItems returns a fetch closure for the run-get picker: it lists a
// run's workflows as selectable items labelled with a status symbol and name. A
// 404 means the run exists but has no workflows yet, returned as an empty list
// so the picker still offers the run summary.
func workflowItems(client *apiclient.Client) func(context.Context, uuid.UUID) ([]ui.RunGetItem, error) {
	return func(ctx context.Context, runID uuid.UUID) ([]ui.RunGetItem, error) {
		wfs, err := client.GetRunWorkflowsV3(ctx, runID)
		if err != nil {
			if httpcl.HasStatusCode(err, http.StatusNotFound) {
				return nil, nil
			}
			return nil, apiErr(err, runID.String())
		}
		items := make([]ui.RunGetItem, len(wfs))
		for i, w := range wfs {
			items[i] = ui.RunGetItem{
				ID:    w.ID,
				Icon:  apiclient.PhaseOutcomeSymbol(w.Phase, w.Outcome, w.CurrentOutcome),
				Label: w.Name,
			}
		}
		return items, nil
	}
}

// jobItems returns a fetch closure for the run-get picker: it lists a
// workflow's jobs as selectable items labelled with a status symbol and name.
func jobItems(client *apiclient.Client) func(context.Context, uuid.UUID) ([]ui.RunGetItem, error) {
	return func(ctx context.Context, workflowID uuid.UUID) ([]ui.RunGetItem, error) {
		jobs, err := client.GetWorkflowJobsV3(ctx, workflowID)
		if err != nil {
			return nil, apiErr(err, workflowID.String())
		}
		items := make([]ui.RunGetItem, len(jobs))
		for i, j := range jobs {
			items[i] = ui.RunGetItem{
				ID:    j.ID,
				Icon:  apiclient.PhaseOutcomeSymbol(j.Phase, j.Outcome, j.CurrentOutcome),
				Label: j.Name,
			}
		}
		return items, nil
	}
}

func buildOutput(r *apiclient.RunV3, workflows []apiclient.WorkflowV3, wfJobs [][]apiclient.WorkflowJobV3) runGetOutput {
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

	return runGetOutput{
		ID:             r.ID,
		Phase:          r.Phase,
		Outcome:        r.Outcome,
		CurrentOutcome: r.CurrentOutcome,
		Branch:         r.Branch,
		Tag:            r.Tag,
		Revision:       revision,
		CreatedAt:      r.CreatedAt.Format("2006-01-02 15:04:05 UTC"),
		Errors:         errs,
		Workflows:      wflows,
	}
}

// deriveDisplayStatus computes a meaningful overall display status from
// workflow phases and outcomes.
func deriveDisplayStatus(r runGetOutput) string {
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

func printRun(ctx context.Context, r runGetOutput, u string) {
	var md strings.Builder
	md.WriteString("# Run\n")

	_, _ = fmt.Fprintf(&md, "- ID: `%s`\n", r.ID)
	if r.Branch != "" {
		_, _ = fmt.Fprintf(&md, "- Branch: %s\n", r.Branch)
	}
	if r.Tag != "" {
		_, _ = fmt.Fprintf(&md, "- Tag: %s\n", r.Tag)
	}
	if r.Revision != "" {
		_, _ = fmt.Fprintf(&md, "- Commit: %s\n", r.Revision)
	}
	_, _ = fmt.Fprintf(&md, "- URL: <%s>\n", u)
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
		md.WriteString("No workflows found for this run.\n")
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
				mdTable.Row(j.Name, apiclient.PhaseOutcomeStatus(j.Phase, j.Outcome, j.CurrentOutcome), j.Type, "`"+j.ID.String()+"`")
			}
			md.WriteString(mdTable.Render())
		}
		md.WriteString("\n")
	}

	iostream.PrintMarkdown(ctx, md.String())
}

// relativeTime renders how long ago t was in coarse, human-friendly units
// (e.g. "20 seconds ago", "3 minutes ago"). Future times clamp to "0 seconds
// ago" so clock skew never produces a negative count.
func relativeTime(t time.Time) string {
	d := time.Since(t)
	if d < 0 {
		d = 0
	}
	switch {
	case d < time.Minute:
		return pluralAgo(int(d.Seconds()), "second")
	case d < time.Hour:
		return pluralAgo(int(d.Minutes()), "minute")
	case d < 24*time.Hour:
		return pluralAgo(int(d.Hours()), "hour")
	default:
		return pluralAgo(int(d.Hours()/24), "day")
	}
}

func pluralAgo(n int, unit string) string {
	if n == 1 {
		return fmt.Sprintf("1 %s ago", unit)
	}
	return fmt.Sprintf("%d %ss ago", n, unit)
}
