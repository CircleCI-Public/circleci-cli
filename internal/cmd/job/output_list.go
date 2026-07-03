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
	"bytes"
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
	"github.com/CircleCI-Public/circleci-cli/internal/termrender"
)

// maxStepOutputFetches bounds how many step outputs are fetched concurrently so
// a job with many steps doesn't open an unbounded number of connections.
const maxStepOutputFetches = 8

// DefaultStepOutputTail is the default for --tail: how many lines of each
// step's output are embedded in the human-readable markdown. The markdown is
// rendered through glamour, whose cost (syntax tokenizing + reflow) is
// superlinear and grinds to a halt on a step with hundreds of thousands of
// lines. We keep the tail — the end of a log is where failures and final state
// are — and point at "job output get" for the rest. The --json output is never
// truncated; it always carries full output.
const DefaultStepOutputTail = 200

// jobOutputList is the typed output of "circleci job output list".
type jobOutputList struct {
	ID        uuid.UUID        `json:"id"`
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
		tail      int
	)

	cmd := &cobra.Command{
		Use:   "list <job-id>",
		Short: "List a job's steps with their output",
		Annotations: map[string]string{
			"help:arguments": heredoc.Docf(`
				%[1]s<job-id>%[1]s is the UUID of the job whose steps and output to list. Job
				UUIDs are shown in the output of "circleci workflow get" and
				"circleci job get".
			`, "`"),
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

			Only the last --tail lines of each step are shown (the end of a log
			is where failures and final state are); pass --tail 0 to show every
			line. This limit applies to the rendered view only — --json always
			carries each step's full output.

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

			# Show every line of every step in the rendered view
			$ circleci job output list 8e50c384-0083-43d0-bc8f-93f0db589d6b --tail 0
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
			return runOutputList(ctx, client, jobID, execution, tail, jsonOut)
		},
	}

	cmd.Flags().IntVar(&execution, "execution", 0, "Parallel execution index to list output from")
	cmd.Flags().IntVar(&tail, "tail", DefaultStepOutputTail, "Show only the last N lines of each step's output in the rendered view (0 for all)")
	cmdutil.AddJSONFlag(cmd, &jsonOut)
	cmdutil.AddJQFlag(cmd)

	return cmd
}

// OutputList renders a job's steps with their output exactly as "circleci job
// output list" does. It is exported so interactive callers (e.g. "circleci run
// get") can reuse the same full report without duplicating the fetch/render
// logic. tail bounds how many lines of each step are shown in the rendered view
// (0 for all); DefaultStepOutputTail is the command's default.
func OutputList(ctx context.Context, client *apiclient.Client, jobID uuid.UUID, execution, tail int, jsonOut bool) error {
	return runOutputList(ctx, client, jobID, execution, tail, jsonOut)
}

func runOutputList(ctx context.Context, client *apiclient.Client, jobID uuid.UUID, execution, tail int, jsonOut bool) error {
	job, err := client.GetJobV3(ctx, jobID)
	if err != nil {
		return cmdutil.APIErr(err, jobID.String(), "job.not_found", "No job found for %q.")
	}

	steps, err := executionSteps(job, execution)
	if err != nil {
		return err
	}

	items := make([]stepOutputItem, len(steps))
	g, ctx := errgroup.WithContext(ctx)
	g.SetLimit(maxStepOutputFetches)
	for i, s := range steps {
		g.Go(func() (err error) {
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

	printOutputList(ctx, data, tail)
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
		WithSuggestions("This job ran with " + executionCount(job) + "; check the index with: circleci job get " + job.ID.String()).
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
	_ = termrender.Render(&sb, bytes.NewReader(stdout))
	_ = termrender.Render(&sb, bytes.NewReader(stderr))
	return sb.String(), nil
}

func printOutputList(ctx context.Context, data *jobOutputList, tail int) {
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
		// Embed only the tail in the rendered view; glamour cannot handle a huge
		// code block. The full output is always available via --json or the
		// per-step "job output get" command named in the notice.
		output, omitted := tailLines(s.Output, tail)
		if omitted > 0 {
			_, _ = fmt.Fprintf(&md, "\n_… %d earlier lines hidden; run `circleci job output get %s --step-num %d` for the full output._\n",
				omitted, data.ID, s.Num)
		}
		fence := cmdutil.CodeFence(output)
		// output already ends in a newline, so the closing fence lands on its
		// own line.
		_, _ = fmt.Fprintf(&md, "\n%stext\n", fence)
		md.WriteString(output)
		_, _ = fmt.Fprintf(&md, "%s\n", fence)
	}

	iostream.PrintMarkdown(ctx, md.String())
}

// tailLines returns the last limit lines of s along with the number of lines
// hidden from the front. s is expected to be newline-terminated; if it has
// limit or fewer lines it is returned unchanged with omitted == 0. A limit of
// zero or less means no limit.
func tailLines(s string, limit int) (tail string, omitted int) {
	total := strings.Count(s, "\n")
	if limit <= 0 || total <= limit {
		return s, 0
	}
	omitted = total - limit
	cut := 0
	for n := 0; n < omitted; n++ {
		cut += strings.IndexByte(s[cut:], '\n') + 1
	}
	return s[cut:], omitted
}
