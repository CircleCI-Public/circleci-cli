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
	"bufio"
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/MakeNowJust/heredoc"
	"github.com/charmbracelet/x/vt"
	"github.com/google/uuid"
	"github.com/spf13/cobra"
	"golang.org/x/sync/errgroup"

	"github.com/CircleCI-Public/circleci-cli/internal/apiclient"
	"github.com/CircleCI-Public/circleci-cli/internal/cmdutil"
	clierrors "github.com/CircleCI-Public/circleci-cli/internal/errors"
	"github.com/CircleCI-Public/circleci-cli/internal/iostream"
)

func newOutputGetCmd() *cobra.Command {
	var (
		execution int
		stepNum   int
		stripANSI bool
	)

	cmd := &cobra.Command{
		Use:   "get <job-id>",
		Short: "Get the output of a job step",
		Long: heredoc.Doc(`
			Fetch the raw stdout and stderr of a single step within a job and
			print it to the terminal.

			The job is identified by its UUID, shown in the output of
			'circleci workflow get' and 'circleci job get'. The step number
			(--step-num) selects which step's output to fetch.

			For parallel jobs, use --execution to choose which executor's output
			to read; it defaults to the first execution (index 0).

			Stdout and stderr are fetched in parallel and printed together,
			stdout first.

			When writing to a terminal the raw output is passed through unchanged.
			When redirected to a file or pipe it is rendered down to plain text:
			ANSI escapes are removed and progress redraws (carriage returns and
			cursor movement, e.g. Docker pulls) are collapsed to their final
			state. Use --strip-ansi to force this rendering even on a terminal.
		`),
		Example: heredoc.Doc(`
			# Get the output of step 3 in a job
			$ circleci job output get 8e50c384-0083-43d0-bc8f-93f0db589d6b --step-num 3

			# Get the output of step 3 from the second parallel execution
			$ circleci job output get 8e50c384-0083-43d0-bc8f-93f0db589d6b --step-num 3 --execution 1

			# Pipe the output to a file
			$ circleci job output get 8e50c384-0083-43d0-bc8f-93f0db589d6b --step-num 3 > step.log
		`),
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if cliErr := cmdutil.RequireArgs(args, "job-id"); cliErr != nil {
				return cliErr
			}
			if !cmd.Flags().Changed("step-num") {
				return cmdutil.RequireFlag("step-num")
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
			return runOutputGet(ctx, client, jobID, execution, stepNum, stripANSI)
		},
	}

	cmd.Flags().IntVar(&execution, "execution", 0, "Parallel execution index to read output from")
	cmd.Flags().IntVar(&stepNum, "step-num", 0, "Step number whose output to fetch (required)")
	cmd.Flags().BoolVar(&stripANSI, "strip-ansi", false, "Strip ANSI escape codes even when writing to a terminal")

	return cmd
}

func runOutputGet(ctx context.Context, client *apiclient.Client, jobID uuid.UUID, execution, stepNum int, stripANSI bool) error {
	var stdout, stderr []byte

	g, gctx := errgroup.WithContext(ctx)
	g.Go(func() (err error) {
		stdout, err = client.GetJobStdout(gctx, jobID, execution, stepNum)
		return err
	})
	g.Go(func() (err error) {
		stderr, err = client.GetJobStderr(gctx, jobID, execution, stepNum)
		return err
	})
	if err := g.Wait(); err != nil {
		subject := fmt.Sprintf("step %d of job %s", stepNum, jobID)
		return cmdutil.APIErr(err, subject, "job.output_not_found",
			"No output found for %s.")
	}

	// Render ANSI/control sequences down to plain text when the output is not
	// going to a terminal (e.g. piped to a file), or when the user explicitly
	// asked for it. On a real terminal we write the raw bytes and let the user's
	// terminal interpret them.
	strip := stripANSI || !iostream.IsTerminal(ctx)

	out := iostream.Out(ctx)
	writeOutput(out, stdout, strip)
	writeOutput(out, stderr, strip)
	return nil
}

func writeOutput(out io.Writer, b []byte, strip bool) {
	if !strip {
		// Write the raw bytes directly to avoid a string copy.
		_, _ = out.Write(b)
		return
	}
	renderTerminal(out, b)
}

// Dimensions of the virtual terminal used to render captured output. The width
// is wide enough to avoid wrapping typical log and progress-bar lines; the
// scrollback is large enough to retain the full output of a single step.
const (
	renderWidth      = 200
	renderHeight     = 100
	renderScrollback = 100_000
)

// renderTerminal replays captured step output through a virtual terminal and
// writes the resulting plain text to dst. A naive ANSI strip is not enough:
// tools like Docker redraw progress with carriage returns and cursor movement,
// so stripping the escapes alone leaves a pile of stale, half-drawn lines.
// Emulating a terminal collapses those redraws to the final state a human would
// have seen.
func renderTerminal(dst io.Writer, b []byte) {
	if len(b) == 0 {
		return
	}

	e := vt.NewEmulator(renderWidth, renderHeight)
	e.SetScrollbackSize(renderScrollback)
	// Enable line-feed/new-line mode so a bare "\n" also returns to column 0,
	// matching the cooked-mode terminal these tools assume they're writing to.
	_, _ = e.WriteString("\x1b[20h")
	_, _ = e.Write(b)

	// Write straight through a small buffer rather than materialising the whole
	// rendered output as one string; the screen can be tens of thousands of
	// lines once scrollback is included.
	w := bufio.NewWriter(dst)

	// Buffer blank lines and only emit them once real content follows. This
	// drops the trailing blank rows of the fixed-height screen (and any trailing
	// blank scrollback lines) while preserving blank lines between content.
	pendingBlanks := 0
	emit := func(line string) {
		if line == "" {
			pendingBlanks++
			return
		}
		for ; pendingBlanks > 0; pendingBlanks-- {
			_ = w.WriteByte('\n')
		}
		_, _ = w.WriteString(line)
		_ = w.WriteByte('\n')
	}

	sb := e.Scrollback()
	for i := 0; i < sb.Len(); i++ {
		emit(strings.TrimRight(sb.Line(i).String(), " "))
	}
	// strings.Lines is an allocation-free iterator (each line is a reslice of the
	// screen string); strings.Split would allocate a slice plus a header per line.
	for line := range strings.Lines(e.String()) {
		emit(strings.TrimRight(line, " \n"))
	}

	_ = w.Flush()
}
