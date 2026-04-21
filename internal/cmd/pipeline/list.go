package pipeline

import (
	"context"
	"encoding/json"

	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"

	"github.com/CircleCI-Public/circleci-cli-v2/internal/apiclient"
	"github.com/CircleCI-Public/circleci-cli-v2/internal/cmdutil"
	clierrors "github.com/CircleCI-Public/circleci-cli-v2/internal/errors"
	"github.com/CircleCI-Public/circleci-cli-v2/internal/gitremote"
	"github.com/CircleCI-Public/circleci-cli-v2/internal/iostream"
)

func newListCmd() *cobra.Command {
	var (
		projectSlug string
		branch      string
		limit       int
		jsonOut     bool
	)

	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "List recent pipelines for a project",
		Long: heredoc.Doc(`
			List recent pipelines for a CircleCI project.

			The project is inferred from the current git repository's remote
			unless overridden with --project. Use --branch to filter results
			to a single branch.

			JSON fields: id, number, state, project_slug, branch, revision,
			             created_at, trigger
		`),
		Example: heredoc.Doc(`
			# List recent pipelines for the current project
			$ circleci pipeline list

			# Filter to a specific branch
			$ circleci pipeline list --branch main

			# List pipelines for an explicit project
			$ circleci pipeline list --project gh/org/repo

			# Show more results
			$ circleci pipeline list --limit 25

			# Output as JSON for scripting
			$ circleci pipeline list --json
		`),
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			streams := iostream.FromCmd(cmd)
			return runList(ctx, streams, projectSlug, branch, limit, jsonOut)
		},
	}

	cmd.Flags().StringVar(&projectSlug, "project", "", "Project slug (e.g. gh/org/repo); defaults to git remote")
	cmd.Flags().StringVarP(&branch, "branch", "b", "", "Filter by branch")
	cmd.Flags().IntVar(&limit, "limit", 10, "Maximum number of pipelines to show [default: 10]")
	cmd.Flags().BoolVar(&jsonOut, "json", false, "Output as JSON")

	return cmd
}

type pipelineListEntry struct {
	ID          string `json:"id"`
	Number      int64  `json:"number"`
	State       string `json:"state"`
	ProjectSlug string `json:"project_slug"`
	Branch      string `json:"branch,omitempty"`
	Revision    string `json:"revision,omitempty"`
	CreatedAt   string `json:"created_at"`
	Trigger     struct {
		Type  string `json:"type"`
		Actor string `json:"actor"`
	} `json:"trigger"`
}

func runList(ctx context.Context, streams iostream.Streams, projectSlug, branch string, limit int, jsonOut bool) error {
	client, cliErr := cmdutil.LoadClient()
	if cliErr != nil {
		return cliErr
	}

	if projectSlug == "" {
		info, err := gitremote.Detect()
		if err != nil {
			return clierrors.New("git.detect_failed", "Could not detect project from git",
				err.Error()).
				WithSuggestions(
					"Run from inside a git repository with a GitHub, Bitbucket, or GitLab remote",
					"Or specify the project: circleci pipeline list --project gh/org/repo",
				).
				WithExitCode(clierrors.ExitBadArguments)
		}
		projectSlug = info.Slug
	}

	pipelines, err := client.ListPipelines(ctx, projectSlug, branch, limit)
	if err != nil {
		return apiErr(err, projectSlug)
	}

	entries := make([]pipelineListEntry, len(pipelines))
	for i, p := range pipelines {
		entries[i] = pipelineToListEntry(&p)
	}

	if jsonOut {
		enc := json.NewEncoder(streams.Out)
		enc.SetIndent("", "  ")
		return enc.Encode(entries)
	}

	if len(pipelines) == 0 {
		streams.ErrPrintln("No pipelines found.")
		return nil
	}

	printList(streams, entries)
	return nil
}

func pipelineToListEntry(p *apiclient.Pipeline) pipelineListEntry {
	e := pipelineListEntry{
		ID:          p.ID,
		Number:      p.Number,
		State:       p.State,
		ProjectSlug: p.ProjectSlug,
		CreatedAt:   p.CreatedAt.Format("2006-01-02 15:04 UTC"),
	}
	e.Trigger.Type = p.Trigger.Type
	e.Trigger.Actor = p.Trigger.Actor.Login
	if p.VCS != nil {
		e.Branch = p.VCS.Branch
		e.Revision = p.VCS.Revision
		if len(e.Revision) > 7 {
			e.Revision = e.Revision[:7]
		}
	}
	return e
}

func printList(streams iostream.Streams, entries []pipelineListEntry) {
	for _, e := range entries {
		state := ""
		if e.State == "errored" {
			state = "  [errored]"
		}
		streams.Printf("#%-4d  %-20s  %s  %s  %s%s\n",
			e.Number, e.Branch, e.Revision, e.ID, e.CreatedAt, state)
	}
}
