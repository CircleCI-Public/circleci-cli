// Package runner implements the "circleci runner" command group.
package runner

import (
	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"

	"github.com/CircleCI-Public/circleci-cli-v2/internal/cmdutil"
	clierrors "github.com/CircleCI-Public/circleci-cli-v2/internal/errors"
)

// NewRunnerCmd returns the "circleci runner" command group.
func NewRunnerCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "runner <command>",
		Short: "Manage self-hosted runners",
		Long: heredoc.Doc(`
			Manage CircleCI self-hosted runner resources.

			Self-hosted runners let you run CircleCI jobs on your own infrastructure.
			Use these commands to manage resource classes, authentication tokens, and
			view connected runner instances.

			Resource class names use the format: namespace/name
			(e.g. my-org/my-runner)
		`),
	}

	cmd.AddCommand(newResourceClassCmd())
	cmd.AddCommand(newTokenCmd())
	cmd.AddCommand(newInstanceCmd())
	cmd.AddCommand(newTasksCmd())

	return cmd
}

func apiErr(err error, subject string) *clierrors.CLIError {
	return cmdutil.APIErr(err, subject,
		"runner.not_found", "No runner resource found for %q.",
		"List available resource classes with: circleci runner resource-class list")
}

func runnerNotEnabledErr() *clierrors.CLIError {
	return clierrors.New("runner.not_enabled", "Runner not available",
		"Self-hosted runners are not available for this token or account. The API returned 404.").
		WithSuggestions(
			"Confirm your token has runner permissions",
			"Check that your plan includes self-hosted runners: https://app.circleci.com/settings/plan",
		).
		WithRef("https://circleci.com/docs/runner-overview/").
		WithExitCode(clierrors.ExitAPIError)
}
