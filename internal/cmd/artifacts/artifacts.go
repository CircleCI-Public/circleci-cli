// Package artifacts implements the "circleci artifacts" command.
package artifacts

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"

	"github.com/CircleCI-Public/circleci-cli-v2/internal/artifacts"
	"github.com/CircleCI-Public/circleci-cli-v2/internal/cmdutil"
	clierrors "github.com/CircleCI-Public/circleci-cli-v2/internal/errors"
	"github.com/CircleCI-Public/circleci-cli-v2/internal/gitremote"
	"github.com/CircleCI-Public/circleci-cli-v2/internal/iostream"
)

// NewArtifactsCmd returns the top-level "circleci artifacts" command.
func NewArtifactsCmd() *cobra.Command {
	var (
		jobNumber   int64
		projectSlug string
		branch      string
		downloadDir string
		jsonOut     bool
	)

	cmd := &cobra.Command{
		Use:   "artifacts [<pipeline-id>]",
		Short: "List or download pipeline artifacts",
		Long: heredoc.Doc(`
			List or download artifacts produced by a CircleCI pipeline or job.

			With no arguments, the pipeline is inferred from the current git
			repository's remote and checked-out branch. Pass a pipeline UUID to
			target a specific pipeline. Use --job to scope to a single job number.

			When listing at pipeline level, each artifact is shown with the job
			name and number it came from.

			JSON fields: job_name, job_number, path, url, node_index
		`),
		Example: heredoc.Doc(`
			# List all artifacts for the latest pipeline on the current branch
			$ circleci artifacts

			# List artifacts for a specific pipeline
			$ circleci artifacts 5034460f-c7c4-4c43-9457-de07e2029e7b

			# List artifacts for a specific job number
			$ circleci artifacts --job 123

			# Download all artifacts from the latest pipeline into ./artifacts
			$ circleci artifacts --download ./artifacts

			# Download artifacts for a specific branch
			$ circleci artifacts --branch main --download ./artifacts

			# Output as JSON for scripting
			$ circleci artifacts --json
		`),
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			streams := iostream.FromCmd(cmd)
			return run(ctx, streams, args, jobNumber, projectSlug, branch, downloadDir, jsonOut)
		},
	}

	cmd.Flags().Int64VarP(&jobNumber, "job", "j", 0, "Scope to a single job number")
	cmd.Flags().StringVar(&projectSlug, "project", "", "Project slug (e.g. gh/org/repo); used with --job, defaults to git remote")
	cmd.Flags().StringVarP(&branch, "branch", "b", "", "Branch for pipeline inference (default: current branch)")
	cmd.Flags().StringVarP(&downloadDir, "download", "d", "", "Download artifacts into this directory")
	cmd.Flags().BoolVar(&jsonOut, "json", false, "Output as JSON")

	return cmd
}

func run(ctx context.Context, streams iostream.Streams, args []string, jobNumber int64, projectSlug, branch, downloadDir string, jsonOut bool) error {
	client, cliErr := cmdutil.LoadClient()
	if cliErr != nil {
		return cliErr
	}

	var (
		err     error
		entries []artifacts.Entry
	)

	switch {
	case jobNumber != 0:
		// --job flag: fetch artifacts for a single job
		if projectSlug == "" {
			info, err := gitremote.Detect()
			if err != nil {
				return gitDetectErr(err)
			}
			projectSlug = info.Slug
		}
		entries, err = artifacts.ForJob(ctx, client, projectSlug, jobNumber)
		if err != nil {
			return apiErr(err, fmt.Sprintf("job #%d", jobNumber))
		}

	case len(args) == 1:
		// Explicit pipeline UUID
		pipelineID := args[0]
		sp := streams.Spinner(!jsonOut, fmt.Sprintf("Fetching artifacts for pipeline %s", pipelineID))
		entries, err = artifacts.ForPipeline(ctx, client, pipelineID)
		sp.Stop()
		if err != nil {
			return apiErr(err, pipelineID)
		}

	default:
		// Infer pipeline from git context
		info, err := gitremote.Detect()
		if err != nil {
			return gitDetectErr(err)
		}
		effectiveBranch := branch
		if effectiveBranch == "" {
			effectiveBranch = info.Branch
		}
		sp := streams.Spinner(!jsonOut, fmt.Sprintf("Fetching latest pipeline for %s on branch %s", info.Slug, effectiveBranch))
		pipeline, err := client.GetLatestPipeline(ctx, info.Slug, effectiveBranch)
		sp.Stop()
		if err != nil {
			return apiErr(err, fmt.Sprintf("%s@%s", info.Slug, effectiveBranch))
		}
		entries, err = artifacts.ForPipeline(ctx, client, pipeline.ID)
		if err != nil {
			return apiErr(err, pipeline.ID)
		}
	}

	if len(entries) == 0 {
		if !jsonOut {
			streams.ErrPrintln("No artifacts found.")
			return nil
		}
		entries = []artifacts.Entry{}
	}

	if downloadDir != "" {
		sp := streams.Spinner(!jsonOut, fmt.Sprintf("Downloading %d artifact(s) to %s", len(entries), downloadDir))
		dlErr := artifacts.Download(ctx, client, entries, downloadDir)
		sp.Stop()
		if dlErr != nil {
			return clierrors.New("artifacts.download_failed", "Download failed", dlErr.Error()).
				WithExitCode(clierrors.ExitGeneralError)
		}
		streams.ErrPrintf("%s Downloaded %d artifact(s)\n", streams.Symbol("✓", "OK:"), len(entries))
	}

	if jsonOut {
		enc := json.NewEncoder(streams.Out)
		enc.SetIndent("", "  ")
		return enc.Encode(entries)
	}

	printArtifacts(streams, entries)
	return nil
}

func printArtifacts(streams iostream.Streams, entries []artifacts.Entry) {
	// Print a flat table. Show job columns only when there's more than one job.
	multiJob := false
	first := entries[0].JobNumber
	for _, e := range entries[1:] {
		if e.JobNumber != first {
			multiJob = true
			break
		}
	}

	if multiJob {
		streams.Printf("%-30s  %-6s  %s\n", "JOB", "JOB #", "PATH")
		streams.Printf("%-30s  %-6s  %s\n", "---", "-----", "----")
		for _, e := range entries {
			streams.Printf("%-30s  %-6d  %s\n", e.JobName, e.JobNumber, e.Path)
		}
	} else {
		for _, e := range entries {
			streams.Println(e.Path)
		}
	}
}

func gitDetectErr(err error) *clierrors.CLIError {
	return clierrors.New("git.detect_failed", "Could not detect project from git", err.Error()).
		WithSuggestions(
			"Run from inside a git repository with a GitHub, Bitbucket, or GitLab remote",
			"Or provide a pipeline UUID: circleci artifacts <id>",
		).
		WithExitCode(clierrors.ExitBadArguments)
}

func apiErr(err error, subject string) *clierrors.CLIError {
	return cmdutil.APIErr(err, subject, "artifacts.not_found", "No resource found for %q.")
}
