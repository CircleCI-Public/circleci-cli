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

package job

import (
	"context"
	"fmt"
	"strconv"
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
		Use:   "get <job-id>",
		Short: "Get job details",
		Annotations: map[string]string{
			"help:arguments": heredoc.Docf(`
				%[1]s<job-id>%[1]s is the UUID of the job to look up. Job UUIDs are shown in
				the output of %[1]scircleci workflow get%[1]s and %[1]scircleci run get --json%[1]s.
			`, "`"),
		},
		Long: heredoc.Doc(`
			Display the status and steps of a CircleCI job.

			Job IDs are shown in the output of 'circleci workflow get' and
			'circleci run get --json'.

			JSON fields: id, name, type, status, started_at, stopped_at,
			             project_id, pipeline_id, workflow_id,
			             executions[].index/steps[].name/type/status/duration/exit_code
		`),
		Example: heredoc.Doc(`
			# Get job details by UUID
			$ circleci job get 8e50c384-0083-43d0-bc8f-93f0db589d6b

			# Output as JSON
			$ circleci job get 8e50c384-0083-43d0-bc8f-93f0db589d6b --json

			# Get a specific field with jq
			$ circleci job get 8e50c384-0083-43d0-bc8f-93f0db589d6b --json | jq '.status'
		`),
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if cliErr := cmdutil.RequireArgs(args, "job-id"); cliErr != nil {
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

// Get renders a job's details exactly as "circleci job get" does. It is
// exported so interactive callers (e.g. "circleci run get") can reuse the same
// output without duplicating the formatting code.
func Get(ctx context.Context, client *apiclient.Client, idStr string, jsonOut bool) error {
	return runGet(ctx, client, idStr, jsonOut)
}

func runGet(ctx context.Context, client *apiclient.Client, idStr string, jsonOut bool) error {
	id, err := uuid.Parse(idStr)
	if err != nil {
		return err
	}

	job, err := client.GetJobV3(ctx, id)
	if err != nil {
		return cmdutil.APIErr(err, id.String(), "job.not_found", "No job found for %q.")
	}

	if jsonOut {
		return iostream.PrintJSON(ctx, job)
	}

	appURL, err := cmdutil.AppURL(ctx)
	if err != nil {
		return err
	}

	u := cmdutil.JobURL(appURL, job.WorkflowID, id)

	printGet(ctx, job, u)
	return nil
}

func printGet(ctx context.Context, j *apiclient.JobV3, u string) {
	iostream.PrintMarkdown(ctx, jobMarkdown(j, u))
}

// jobMarkdown builds the job report (the short per-job summary) as markdown.
// printGet pages it via iostream; the interactive run-get flow renders it into
// its own in-flow pager (see SummaryMarkdown) so esc returns to the picker
// instead of quitting.
func jobMarkdown(j *apiclient.JobV3, u string) string {
	var md strings.Builder
	md.WriteString("# Job\n")

	_, _ = fmt.Fprintf(&md, "- ID: `%s`\n", j.ID)
	_, _ = fmt.Fprintf(&md, "- Name: %s\n", j.Name)
	_, _ = fmt.Fprintf(&md, "- Type: %s\n", j.Type)
	_, _ = fmt.Fprintf(&md, "- URL: %s\n", u)
	_, _ = fmt.Fprintf(&md, "- Status: %s\n", j.Status())
	_, _ = fmt.Fprintf(&md, "- Started: %s\n", j.StartedAt.Format("2006-01-02 15:04:05 UTC"))
	if j.StoppedAt != nil {
		_, _ = fmt.Fprintf(&md, "- Stopped: %s\n", j.StoppedAt.Format("2006-01-02 15:04:05 UTC"))
		_, _ = fmt.Fprintf(&md, "- Duration: %s\n", j.StoppedAt.Sub(j.StartedAt).Round(100*1e6))
	}
	_, _ = fmt.Fprintf(&md, "- Workflow: `%s`\n", j.WorkflowID)
	_, _ = fmt.Fprintf(&md, "- Run: `%s`\n", j.PipelineID)

	for _, exec := range j.Executions {
		if len(j.Executions) == 1 {
			_, _ = fmt.Fprintf(&md, "\n## Steps\n")
		} else {
			_, _ = fmt.Fprintf(&md, "\n## Execution %d\n", exec.Index)
		}
		table := mdtable.New("#", "Name", "Status", "Duration", "Exit Code", "Command")
		for _, s := range exec.Steps {
			exitCode := "-"
			if s.ExitCode != nil {
				exitCode = fmt.Sprintf("%d", *s.ExitCode)
			}
			command := formatCommand(s.Command)
			duration := "-"
			if s.StoppedAt != nil {
				duration = formatDuration(s.StoppedAt.Sub(s.StartedAt).Seconds())
			}
			table.Row(strconv.Itoa(s.Num), s.Name, s.Status(), duration, exitCode, command)
		}
		md.WriteString(table.Render())
	}

	return md.String()
}

// SummaryMarkdown assembles the job report as markdown, for the interactive
// run-get flow's in-flow "job report" pager (the RenderJobSummary callback). It
// mirrors runGet's non-JSON path but returns the markdown rather than paging it.
func SummaryMarkdown(ctx context.Context, client *apiclient.Client, jobID uuid.UUID) (string, error) {
	j, err := client.GetJobV3(ctx, jobID)
	if err != nil {
		return "", cmdutil.APIErr(err, jobID.String(), "job.not_found", "No job found for %q.")
	}
	appURL, err := cmdutil.AppURL(ctx)
	if err != nil {
		return "", err
	}
	return jobMarkdown(j, cmdutil.JobURL(appURL, j.WorkflowID, jobID)), nil
}

func formatCommand(s string) string {
	if s == "" {
		return "-"
	}
	// Commands can be multi-line; keep only the first two lines, wrap each in
	// backticks for a fixed-width font, join them with <br>, and escape pipes
	// so they don't break the markdown table.
	lines := strings.SplitN(s, "\n", 3)
	truncated := len(lines) > 2
	if truncated {
		lines = lines[:2]
	}
	for i, line := range lines {
		lines[i] = "`" + strings.ReplaceAll(line, "|", "\\|") + "`"
	}
	command := strings.Join(lines, " … ")
	if truncated {
		command += " …"
	}
	return command
}

func formatDuration(seconds float64) string {
	if seconds <= 0 {
		return "-"
	}
	ms := int(seconds * 1000)
	if ms < 1000 {
		return fmt.Sprintf("%dms", ms)
	}
	s := int(seconds)
	if s < 60 {
		frac := ms % 1000 / 100
		if frac > 0 {
			return fmt.Sprintf("%d.%ds", s, frac)
		}
		return fmt.Sprintf("%ds", s)
	}
	return fmt.Sprintf("%dm%ds", s/60, s%60)
}
