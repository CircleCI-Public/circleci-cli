package runner

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/CircleCI-Public/circleci-cli-v2/internal/cmdutil"
	clierrors "github.com/CircleCI-Public/circleci-cli-v2/internal/errors"
	"github.com/CircleCI-Public/circleci-cli-v2/internal/httpcl"
	"github.com/CircleCI-Public/circleci-cli-v2/internal/iostream"
	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"
)

func newTasksCmd() *cobra.Command {
	var resourceClass string
	var jsonOut bool

	cmd := &cobra.Command{
		Use:   "task",
		Short: "Show task counts for a runner resource class",
		Long: heredoc.Doc(`
			Show the number of unclaimed and running tasks for a CircleCI runner
			resource class.

			Unclaimed tasks are queued and waiting for a runner to pick them up.
			Running tasks are actively executing on a runner instance.

			JSON fields: resource_class, unclaimed, running
		`),
		Example: heredoc.Doc(`
			# Show task counts for a resource class
			$ circleci runner task --resource-class my-org/my-runner

			# Output as JSON
			$ circleci runner task --resource-class my-org/my-runner --json

			# Check for a backlog across multiple classes
			$ for rc in my-org/build my-org/deploy; do
			    circleci runner task --resource-class $rc
			  done
		`),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			streams := iostream.FromCmd(cmd)
			return runTasks(ctx, streams, resourceClass, jsonOut)
		},
	}

	cmd.Flags().StringVar(&resourceClass, "resource-class", "", "Resource class to query (namespace/name)")
	cmd.Flags().BoolVar(&jsonOut, "json", false, "Output as JSON")
	_ = cmd.MarkFlagRequired("resource-class")

	return cmd
}

type tasksOutput struct {
	ResourceClass string `json:"resource_class"`
	Unclaimed     int    `json:"unclaimed"`
	Running       int    `json:"running"`
}

func runTasks(ctx context.Context, streams iostream.Streams, resourceClass string, jsonOut bool) error {
	client, cliErr := cmdutil.LoadClient()
	if cliErr != nil {
		return cliErr
	}

	counts, err := client.GetRunnerTaskCounts(ctx, resourceClass)
	if err != nil {
		if httpcl.HasStatusCode(err, http.StatusNotFound) {
			return clierrors.New("runner.not_found", "Resource class not found",
				"No runner resource class found for "+resourceClass+".").
				WithSuggestions("List available resource classes with: circleci runner resource-class list").
				WithExitCode(clierrors.ExitNotFound)
		}
		return apiErr(err, resourceClass)
	}

	out := tasksOutput{
		ResourceClass: resourceClass,
		Unclaimed:     counts.Unclaimed,
		Running:       counts.Running,
	}

	if jsonOut {
		enc := json.NewEncoder(streams.Out)
		enc.SetIndent("", "  ")
		return enc.Encode(out)
	}

	streams.Printf("Resource class: %s\n", out.ResourceClass)
	streams.Printf("  Unclaimed:    %d\n", out.Unclaimed)
	streams.Printf("  Running:      %d\n", out.Running)
	return nil
}
