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
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/MakeNowJust/heredoc"
	"github.com/google/uuid"
	"github.com/spf13/cobra"

	clierrors "github.com/CircleCI-Public/circleci-cli/clikit/errors"
	"github.com/CircleCI-Public/circleci-cli/clikit/iostream"
	"github.com/CircleCI-Public/circleci-cli/clikit/mdtable"
	"github.com/CircleCI-Public/circleci-cli/internal/apiclient"
	"github.com/CircleCI-Public/circleci-cli/internal/cmdutil"
	"github.com/CircleCI-Public/circleci-cli/internal/runcompare"
	"github.com/CircleCI-Public/circleci-cli/internal/ui"
)

func newCompareCmd() *cobra.Command {
	var jsonOut bool

	cmd := &cobra.Command{
		Use:   "compare <old-run-id> <new-run-id>",
		Short: "Compare two runs' running time and credits",
		Annotations: map[string]string{
			"help:arguments": heredoc.Docf(`
				%[1]s<old-run-id>%[1]s and %[1]s<new-run-id>%[1]s are run UUIDs, as shown in
				%[1]scircleci run list --json%[1]s. Both runs must have ended.
			`, "`"),
		},
		Long: heredoc.Doc(`
			Compare a run of the old config with a run of the new one by total running
			time (first workflow created to last ended) and total credits. Credits
			arrive shortly after each job, so a just-ended run may read 0 for a while.

			JSON fields: old_run.id/project_id/running_seconds/credits, new_run.*, delta.running_seconds/running_pct/credits/credits_pct
		`),
		Example: heredoc.Doc(`
			# Compare the run before a config change with the run after it
			$ circleci run compare 5034460f-c7c4-4c43-9457-de07e2029e7b 9a1d2c3e-4f5a-4b6c-8d7e-0f1a2b3c4d5e

			# Just the credit difference
			$ circleci run compare <old-run-id> <new-run-id> --json --jq '.delta.credits'

			# Compare main's latest run with your branch's latest run
			$ circleci run compare \
			    "$(circleci run list --branch main --json --jq '.[0].id')" \
			    "$(circleci run list --branch my-change --json --jq '.[0].id')"
		`),
		Args: cobra.MaximumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			if cliErr := cmdutil.RequireArgs(args, "old-run-id", "new-run-id"); cliErr != nil {
				return cliErr
			}
			oldID, err := parseRunID(args[0])
			if err != nil {
				return err
			}
			newID, err := parseRunID(args[1])
			if err != nil {
				return err
			}
			if err := runcompare.Validate(oldID, newID); err != nil {
				return err
			}
			ctx := cmd.Context()
			client, err := cmdutil.LoadClient(ctx)
			if err != nil {
				return err
			}
			return runCompare(ctx, client, oldID, newID, jsonOut)
		},
	}

	cmdutil.AddJSONFlag(cmd, &jsonOut)
	cmdutil.AddJQFlag(cmd)

	return cmd
}

func parseRunID(arg string) (uuid.UUID, error) {
	id, err := uuid.Parse(arg)
	if err != nil {
		return uuid.UUID{}, clierrors.New("args.invalid_run_id", "Invalid run ID",
			fmt.Sprintf("%q is not a valid run UUID.", arg)).
			WithSuggestions("Find run UUIDs with: circleci run list --json").
			WithExitCode(clierrors.ExitBadArguments)
	}
	return id, nil
}

func runCompare(ctx context.Context, client *apiclient.Client, oldID, newID uuid.UUID, jsonOut bool) error {
	sp := iostream.Spinner(ctx, !jsonOut, "Fetching running time and credits for both runs")
	report, err := runcompare.Compare(ctx, client, oldID, newID)
	sp.Stop()
	if err != nil {
		if fe, ok := errors.AsType[*runcompare.FetchError](err); ok {
			return apiErr(fe.Err, fe.RunID.String())
		}
		return err
	}

	for _, r := range []runcompare.RunSummary{report.OldRun, report.NewRun} {
		if r.Credits == 0 {
			iostream.ErrPrintf(ctx, "%s No credits recorded yet for run %s. Charges arrive shortly after each job; try again in a few minutes.\n",
				iostream.SymbolWarn(ctx), r.ID)
		}
	}

	if jsonOut {
		return iostream.PrintJSON(ctx, report)
	}
	iostream.PrintMarkdown(ctx, compareMarkdown(report))
	return nil
}

func compareMarkdown(r *runcompare.Report) string {
	wall := mdtable.New("", "Old", "New", "Δ")
	wall.Row("Total running time",
		ui.FormatElapsed(seconds(r.OldRun.RunningSeconds)),
		ui.FormatElapsed(seconds(r.NewRun.RunningSeconds)),
		withPercent(signedElapsed(r.Delta.RunningSeconds), r.Delta.RunningPct))

	credits := mdtable.New("", "Old", "New", "Δ")
	credits.Row("Total credits",
		fmt.Sprint(r.OldRun.Credits),
		fmt.Sprint(r.NewRun.Credits),
		withPercent(fmt.Sprintf("%+d", r.Delta.Credits), r.Delta.CreditsPct))

	return "# Wall clock\n" + wall.Render() + "\n# Credits\n" + credits.Render()
}

func seconds(s float64) time.Duration {
	return time.Duration(s * float64(time.Second))
}

// signedElapsed renders a running-time difference with an explicit sign, so a
// faster run reads "-2m22s" and a slower one "+2m22s".
func signedElapsed(s float64) string {
	d := seconds(math.Abs(s)).Round(time.Second)
	switch {
	case d == 0:
		return "0s"
	case s < 0:
		return "-" + ui.FormatElapsed(d)
	default:
		return "+" + ui.FormatElapsed(d)
	}
}

func withPercent(delta string, pct *float64) string {
	if pct == nil {
		return delta + " (n/a)"
	}
	return fmt.Sprintf("%s (%+.1f%%)", delta, *pct)
}
