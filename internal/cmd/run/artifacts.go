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
	"path/filepath"
	"strings"

	"github.com/MakeNowJust/heredoc"
	"github.com/google/uuid"
	"github.com/spf13/cobra"
	"golang.org/x/sync/errgroup"

	clierrors "github.com/CircleCI-Public/circleci-cli/clikit/errors"
	"github.com/CircleCI-Public/circleci-cli/clikit/iostream"
	"github.com/CircleCI-Public/circleci-cli/internal/apiclient"
	"github.com/CircleCI-Public/circleci-cli/internal/artifacts"
	"github.com/CircleCI-Public/circleci-cli/internal/cmdutil"
)

// maxArtifactFetches bounds how many jobs' artifact lists are fetched at once.
const maxArtifactFetches = 8

func newArtifactsCmd() *cobra.Command {
	var (
		projectSlug string
		branch      string
		status      string
		outputDir   string
		jsonOut     bool
	)

	cmd := &cobra.Command{
		Use:   "artifacts [<run-id>]",
		Short: "List and download the artifacts of every job in a run",
		Annotations: map[string]string{
			"help:arguments": heredoc.Docf(`
				%[1]s<run-id>%[1]s is optional: a run UUID or number. Omit it to use the latest
				run for the project and branch (see %[1]s--project%[1]s and %[1]s--branch%[1]s).
			`, "`"),
		},
		Long: heredoc.Doc(`
			Lists each job's artifact paths; --json adds URLs; -o saves to <dir>/<workflow>/<job>/.

			JSON fields: id, phase, outcome, current_outcome, branch, tag, revision, repository_url, commit.subject/url/author_name/author_login, created_at, errors[].type/message, warnings[].type/message/description/url, workflows[].id/name/phase/outcome/current_outcome/duration/jobs[].id/name/phase/outcome/current_outcome/type/artifacts[].path/url/execution
		`),
		Example: heredoc.Doc(`
			# List the artifacts of the latest run on the current branch
			$ circleci run artifacts

			# List the artifacts of the last successful run
			$ circleci run artifacts --status success

			# Output a run's artifacts, with their URLs, as JSON
			$ circleci run artifacts 5034460f-c7c4-4c43-9457-de07e2029e7b --json

			# Download the last successful run's artifacts into ./artifacts
			$ circleci run artifacts --status success -o ./artifacts
		`),
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			if len(args) == 1 && status != "" {
				return clierrors.New("args.conflicting_flags", "Conflicting arguments",
					"--status selects the latest run, so it cannot be combined with a run ID.").
					WithSuggestions(
						"List that run's artifacts: circleci run artifacts "+args[0],
						"Or drop the run ID: circleci run artifacts --status "+status,
					).
					WithExitCode(clierrors.ExitBadArguments)
			}
			if err := validateRunStatus(status); err != nil {
				return err
			}
			client, err := cmdutil.LoadClient(ctx)
			if err != nil {
				return err
			}
			return runArtifacts(ctx, client, args, projectSlug, branch, status, outputDir, jsonOut)
		},
	}

	cmd.Flags().StringVar(&projectSlug, "project", "", "project slug (e.g. gh/org/repo); defaults to git remote")
	cmd.Flags().StringVarP(&branch, "branch", "b", "", "branch name (defaults to the current branch, or main when --project is set)")
	cmd.Flags().StringVar(&status, "status", "", statusFlagUsage+"; picks the latest such run")
	cmd.Flags().StringVarP(&outputDir, "output", "o", "", "download artifacts into this directory")
	cmdutil.AddJSONFlag(cmd, &jsonOut)
	cmdutil.AddJQFlag(cmd)

	return cmd
}

// runArtifactsOutput is run get's JSON with each job's artifacts added. Each
// level embeds run get's struct and shadows its child list with one carrying
// artifacts; encoding/json prefers the shallower field, so the JSON has run
// get's fields once, with the artifact-bearing lists in place of the originals.
type runArtifactsOutput struct {
	runGetOutput
	Workflows []workflowArtifactsOutput `json:"workflows"`
}

type workflowArtifactsOutput struct {
	workflowOutput
	Jobs []jobArtifactsOutput `json:"jobs"`
}

type jobArtifactsOutput struct {
	jobOutput
	Artifacts []artifacts.Entry `json:"artifacts"`
}

func runArtifacts(ctx context.Context, client *apiclient.Client, args []string, projectSlug, branch, status, outputDir string, jsonOut bool) error {
	r, err := resolveArtifactsRun(ctx, client, args, projectSlug, branch, status, jsonOut)
	if err != nil {
		return err
	}

	summary, err := fetchRunOutput(ctx, client, r)
	if err != nil {
		return err
	}
	out, err := collectRunArtifacts(ctx, client, summary)
	if err != nil {
		return err
	}

	total := countArtifacts(out)
	if total == 0 && !jsonOut {
		iostream.ErrPrintf(ctx, "No artifacts found in run %s.\n", out.ID)
		return nil
	}

	if outputDir != "" && total > 0 {
		sp := iostream.Spinner(ctx, !jsonOut, fmt.Sprintf("Downloading %d artifact(s) to %q", total, outputDir))
		dlErr := downloadRunArtifacts(ctx, client, out, outputDir)
		sp.Stop()
		if dlErr != nil {
			return clierrors.New("artifacts.download_failed", "Download failed", dlErr.Error()).
				WithSuggestions(fmt.Sprintf("Check that %q is writable and retry", outputDir)).
				WithExitCode(clierrors.ExitGeneralError)
		}
		if !jsonOut {
			iostream.ErrPrintf(ctx, "%s Downloaded %d artifact(s)\n", iostream.SymbolOK(ctx), total)
		}
	}

	if jsonOut {
		return iostream.PrintJSON(ctx, out)
	}
	iostream.PrintMarkdown(ctx, runArtifactsMarkdown(out))
	return nil
}

// resolveArtifactsRun loads the run named by args[0] (a UUID or run number), or
// with no argument the latest run for the project and branch, narrowed to
// status when it is set.
func resolveArtifactsRun(ctx context.Context, client *apiclient.Client, args []string, projectSlug, branch, status string, jsonOut bool) (*apiclient.RunV3, error) {
	if len(args) == 1 {
		id, name, err := resolveRunRef(ctx, client, "artifacts", args[0], projectSlug)
		if err != nil {
			return nil, err
		}
		r, err := client.GetRunV3(ctx, id)
		if err != nil {
			return nil, apiErr(err, name)
		}
		return r, nil
	}

	r, slug, br, err := latestRun(ctx, client, "artifacts", projectSlug, branch, status, !jsonOut)
	if err != nil {
		return nil, err
	}
	if r == nil {
		what := "run"
		if status != "" {
			what = fmt.Sprintf("run with status %q", status)
		}
		return nil, clierrors.New("run.not_found", "Not found",
			fmt.Sprintf("No %s found for %q in the last %d days.", what, slug+"@"+br, latestRunWindowDays)).
			WithSuggestions("List recent runs: circleci run list --project " + slug + " --branch " + br).
			WithExitCode(clierrors.ExitNotFound)
	}
	return r, nil
}

// collectRunArtifacts fetches the artifacts of every job in summary
// concurrently and returns summary with them attached. A job with nothing to
// fetch (see jobMayHaveArtifacts) gets an empty list without an API call; any
// other failure fails the whole listing.
func collectRunArtifacts(ctx context.Context, client *apiclient.Client, summary runGetOutput) (runArtifactsOutput, error) {
	out := runArtifactsOutput{runGetOutput: summary, Workflows: make([]workflowArtifactsOutput, len(summary.Workflows))}
	out.runGetOutput.Workflows = nil

	g, gctx := errgroup.WithContext(ctx)
	g.SetLimit(maxArtifactFetches)
	for i, wf := range summary.Workflows {
		jobs := make([]jobArtifactsOutput, len(wf.Jobs))
		for k, j := range wf.Jobs {
			jobs[k] = jobArtifactsOutput{jobOutput: j, Artifacts: []artifacts.Entry{}}
			if !jobMayHaveArtifacts(j) {
				continue
			}
			g.Go(func() error {
				entries, err := artifacts.ForJob(gctx, client, j.ID.String())
				if err != nil {
					return cmdutil.APIErr(err, j.ID.String(), "artifacts.not_found", "No resource found for %q.")
				}
				jobs[k].Artifacts = entries
				return nil
			})
		}
		wfOut := workflowArtifactsOutput{workflowOutput: wf, Jobs: jobs}
		wfOut.workflowOutput.Jobs = nil
		out.Workflows[i] = wfOut
	}
	if err := g.Wait(); err != nil {
		return runArtifactsOutput{}, err
	}
	return out, nil
}

// jobMayHaveArtifacts reports whether a job is worth asking for artifacts: an
// approval job runs nothing, and a job with no ID or one still created or
// queued has not run, so none of them can have stored any.
func jobMayHaveArtifacts(j jobOutput) bool {
	switch {
	case j.Type == apiclient.JobTypeApproval, j.ID == uuid.Nil:
		return false
	case j.Phase == apiclient.PhaseCreated, j.Phase == apiclient.PhaseQueued:
		return false
	default:
		return true
	}
}

func countArtifacts(out runArtifactsOutput) int {
	n := 0
	for _, wf := range out.Workflows {
		for _, j := range wf.Jobs {
			n += len(j.Artifacts)
		}
	}
	return n
}

// workflowLabels names each workflow for headings and download directories.
// A run can hold several workflows with one name (a rerun adds a new "ci" beside
// the original), so a repeated name carries its workflow's short ID.
func workflowLabels(wfs []workflowArtifactsOutput) []string {
	count := map[string]int{}
	for _, wf := range wfs {
		count[wf.Name]++
	}
	labels := make([]string, len(wfs))
	for i, wf := range wfs {
		labels[i] = wf.Name
		if count[wf.Name] > 1 {
			labels[i] += "-" + wf.ID.String()[:8]
		}
	}
	return labels
}

// downloadRunArtifacts saves each job's artifacts under dir/<workflow>/<job>/,
// with artifacts.Download laying out the files inside each job's directory.
func downloadRunArtifacts(ctx context.Context, client *apiclient.Client, out runArtifactsOutput, dir string) error {
	labels := workflowLabels(out.Workflows)
	for i, wf := range out.Workflows {
		for _, j := range wf.Jobs {
			if len(j.Artifacts) == 0 {
				continue
			}
			jobDir := filepath.Join(dir, artifacts.SafeDirName(labels[i]), artifacts.SafeDirName(j.Name))
			if err := artifacts.Download(ctx, client, j.Artifacts, jobDir); err != nil {
				return err
			}
		}
	}
	return nil
}

// runArtifactsMarkdown renders the run heading followed by a section per job.
func runArtifactsMarkdown(out runArtifactsOutput) string {
	var md strings.Builder
	fmt.Fprintf(&md, "# Artifacts for run %s\n", out.ID)
	fmt.Fprintf(&md, "- Status: %s\n", apiclient.PhaseOutcomeStatus(out.Phase, out.Outcome, out.CurrentOutcome))
	if out.Branch != "" {
		fmt.Fprintf(&md, "- Branch: %s\n", out.Branch)
	}
	if out.Tag != "" {
		fmt.Fprintf(&md, "- Tag: %s\n", out.Tag)
	}
	md.WriteString("\n")

	labels := workflowLabels(out.Workflows)
	var jobs []artifacts.JobArtifacts
	for i, wf := range out.Workflows {
		for _, j := range wf.Jobs {
			jobs = append(jobs, artifacts.JobArtifacts{Workflow: labels[i], Job: j.Name, Entries: j.Artifacts})
		}
	}
	md.WriteString(artifacts.FormatRunMarkdown(jobs))
	return md.String()
}
