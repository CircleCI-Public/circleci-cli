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
	"net/http"
	"strings"
	"time"

	"github.com/MakeNowJust/heredoc"
	"github.com/google/uuid"
	"github.com/spf13/cobra"
	"golang.org/x/sync/errgroup"

	"github.com/CircleCI-Public/circleci-cli/internal/apiclient"
	"github.com/CircleCI-Public/circleci-cli/internal/cmdutil"
	clierrors "github.com/CircleCI-Public/circleci-cli/internal/errors"
	"github.com/CircleCI-Public/circleci-cli/internal/httpcl"
	"github.com/CircleCI-Public/circleci-cli/internal/iostream"
)

// maxStepOutputFetches bounds how many step outputs are fetched concurrently so
// a job with many steps doesn't open an unbounded number of connections.
const maxStepOutputFetches = 8

// jobOutputList is the typed output of "circleci job output list".
type jobOutputList struct {
	ID        string           `json:"id"`
	Name      string           `json:"name"`
	Execution int              `json:"execution"`
	Steps     []stepOutputItem `json:"steps"`
}

// stepOutputItem is a single step plus its terminal-processed output.
type stepOutputItem struct {
	Num       int        `json:"num"`
	Name      string     `json:"name"`
	Type      string     `json:"type"`
	Phase     string     `json:"phase"`
	Outcome   string     `json:"outcome,omitempty"`
	StartedAt time.Time  `json:"started_at"`
	StoppedAt *time.Time `json:"stopped_at,omitempty"`
	ExitCode  *int       `json:"exit_code,omitempty"`
	Command   string     `json:"command,omitempty"`
	Output    string     `json:"output"`
}

func newOutputListCmd() *cobra.Command {
	var (
		execution int
		jsonOut   bool
	)

	cmd := &cobra.Command{
		Use:   "list <job-id>",
		Short: "List a job's steps with their output",
		Annotations: map[string]string{
			"help:arguments": heredoc.Doc(`
				<job-id> is the UUID of the job whose steps and output to list. Job
				UUIDs are shown in the output of "circleci workflow get" and
				"circleci job get".
			`),
		},
		Long: heredoc.Doc(`
			List every step in a job alongside its terminal-processed output.

			The job is identified by its UUID, shown in the output of
			'circleci workflow get' and 'circleci job get'. For parallel jobs,
			use --execution to choose which executor's steps to list; it defaults
			to the first execution (index 0).

			Each step's stdout and stderr are fetched and replayed through a
			virtual terminal so progress redraws (carriage returns, cursor
			movement) collapse to the final state a human would have seen.

			JSON fields: id, name, execution,
			             steps[].num/name/type/phase/outcome/started_at/stopped_at/
			             exit_code/command/output
		`),
		Example: heredoc.Doc(`
			# List the steps and output of a job
			$ circleci job output list 8e50c384-0083-43d0-bc8f-93f0db589d6b

			# List output for the second parallel execution
			$ circleci job output list 8e50c384-0083-43d0-bc8f-93f0db589d6b --execution 1

			# As JSON, with the output of the failing step
			$ circleci job output list 8e50c384-0083-43d0-bc8f-93f0db589d6b --json \
			    | jq '.steps[] | select(.exit_code != 0) | .output'
		`),
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if cliErr := cmdutil.RequireArgs(args, "job-id"); cliErr != nil {
				return cliErr
			}
			jobID, err := uuid.Parse(args[0])
			if err != nil {
				return clierrors.New("args.invalid_job_id", "Invalid job ID",
					fmt.Sprintf("%q is not a valid job UUID.", args[0])).
					WithExitCode(clierrors.ExitBadArguments)
			}
			ctx := cmd.Context()
			client, err := cmdutil.LoadClient(ctx)
			if err != nil {
				return err
			}
			return runOutputList(ctx, client, jobID, execution, jsonOut)
		},
	}

	cmd.Flags().IntVar(&execution, "execution", 0, "Parallel execution index to list output from")
	cmdutil.AddJSONFlag(cmd, &jsonOut)
	cmdutil.AddJQFlag(cmd)

	return cmd
}

func runOutputList(ctx context.Context, client *apiclient.Client, jobID uuid.UUID, execution int, jsonOut bool) error {
	job, err := client.GetJobV3(ctx, jobID.String())
	if err != nil {
		return cmdutil.APIErr(err, jobID.String(), "job.not_found", "No job found for %q.")
	}

	steps, err := executionSteps(job, execution)
	if err != nil {
		return err
	}

	items := make([]stepOutputItem, len(steps))
	g := new(errgroup.Group)
	g.SetLimit(maxStepOutputFetches)
	for i, s := range steps {
		g.Go(func() error {
			out, err := renderStepOutput(ctx, client, jobID, execution, s.Num)
			if err != nil {
				return err
			}
			items[i] = stepOutputItem{
				Num:       s.Num,
				Name:      s.Name,
				Type:      s.Type,
				Phase:     s.Phase,
				Outcome:   s.Outcome,
				StartedAt: s.StartedAt,
				StoppedAt: s.StoppedAt,
				ExitCode:  s.ExitCode,
				Command:   s.Command,
				Output:    out,
			}
			return nil
		})
	}
	if err := g.Wait(); err != nil {
		return cmdutil.APIErr(err, jobID.String(), "job.output_error",
			"Failed to fetch step output for job %q.")
	}

	data := &jobOutputList{ID: job.ID, Name: job.Name, Execution: execution, Steps: items}

	if jsonOut {
		return iostream.PrintJSON(ctx, data)
	}

	printOutputList(ctx, data)
	return nil
}

// executionSteps returns the steps for the requested execution index.
func executionSteps(job *apiclient.JobV3, execution int) ([]apiclient.JobV3Step, error) {
	for _, exec := range job.Executions {
		if exec.Index == execution {
			return exec.Steps, nil
		}
	}
	return nil, clierrors.New("job.execution_not_found", "Execution not found",
		fmt.Sprintf("Job %q has no execution with index %d.", job.ID, execution)).
		WithSuggestions("This job ran with " + executionCount(job) + "; check the index with: circleci job get " + job.ID).
		WithExitCode(clierrors.ExitNotFound)
}

func executionCount(job *apiclient.JobV3) string {
	n := len(job.Executions)
	if n == 1 {
		return "1 execution (index 0)"
	}
	return fmt.Sprintf("%d executions (indexes 0-%d)", n, n-1)
}

// renderStepOutput fetches a step's stdout and stderr and replays them through a
// virtual terminal. Steps with no output (404) render as empty rather than
// failing the whole listing.
func renderStepOutput(ctx context.Context, client *apiclient.Client, jobID uuid.UUID, execution, stepNum int) (string, error) {
	var stdout, stderr []byte

	// Independent (not WithContext) so a missing stderr doesn't cancel a present
	// stdout fetch, and vice versa.
	g := new(errgroup.Group)
	g.Go(func() error {
		b, err := client.GetJobStdout(ctx, jobID, execution, stepNum)
		if err != nil && !httpcl.HasStatusCode(err, http.StatusNotFound) {
			return err
		}
		stdout = b
		return nil
	})
	g.Go(func() error {
		b, err := client.GetJobStderr(ctx, jobID, execution, stepNum)
		if err != nil && !httpcl.HasStatusCode(err, http.StatusNotFound) {
			return err
		}
		stderr = b
		return nil
	})
	if err := g.Wait(); err != nil {
		return "", err
	}

	var sb strings.Builder
	renderTerminal(&sb, stdout)
	renderTerminal(&sb, stderr)
	return sb.String(), nil
}

func printOutputList(ctx context.Context, data *jobOutputList) {
	var md strings.Builder
	md.WriteString("# Job Output\n")
	_, _ = fmt.Fprintf(&md, "- ID: `%s`\n", data.ID)
	_, _ = fmt.Fprintf(&md, "- Name: %s\n", data.Name)
	_, _ = fmt.Fprintf(&md, "- Execution: %d\n", data.Execution)

	for _, s := range data.Steps {
		_, _ = fmt.Fprintf(&md, "\n## %d: %s\n", s.Num, s.Name)
		_, _ = fmt.Fprintf(&md, "- Status: %s\n", apiclient.PhaseOutcomeStatus(s.Phase, s.Outcome, ""))

		duration := "-"
		if s.StoppedAt != nil {
			duration = formatDuration(s.StoppedAt.Sub(s.StartedAt).Seconds())
		}
		_, _ = fmt.Fprintf(&md, "- Duration: %s\n", duration)
		if s.ExitCode != nil {
			_, _ = fmt.Fprintf(&md, "- Exit Code: %d\n", *s.ExitCode)
		}
		if s.Command != "" {
			_, _ = fmt.Fprintf(&md, "- Command: %s\n", formatCommand(s.Command))
		}
		md.WriteString("### Output\n")

		if strings.TrimSpace(s.Output) == "" {
			md.WriteString("\n_(no output)_\n")
			continue
		}
		fence := codeFence(s.Output)
		// s.Output already ends in a newline, so the closing fence lands on its
		// own line.
		_, _ = fmt.Fprintf(&md, "\n%stext\n", fence)
		md.WriteString(s.Output)
		_, _ = fmt.Fprintf(&md, "%s\n", fence)
	}

	iostream.PrintMarkdown(ctx, md.String())
}

// codeFence returns a backtick fence long enough to wrap s without a run of
// backticks inside s prematurely closing the block (CommonMark fencing rule).
func codeFence(s string) string {
	longest, run := 0, 0
	for _, r := range s {
		if r == '`' {
			run++
			if run > longest {
				longest = run
			}
			continue
		}
		run = 0
	}
	n := 3
	if longest+1 > n {
		n = longest + 1
	}
	return strings.Repeat("`", n)
}
