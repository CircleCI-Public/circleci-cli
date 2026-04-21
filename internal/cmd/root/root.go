package root

import (
	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"

	cmdapi "github.com/CircleCI-Public/circleci-cli-v2/internal/cmd/api"
	"github.com/CircleCI-Public/circleci-cli-v2/internal/cmd/artifacts"
	"github.com/CircleCI-Public/circleci-cli-v2/internal/cmd/completion"
	"github.com/CircleCI-Public/circleci-cli-v2/internal/cmd/envvar"
	"github.com/CircleCI-Public/circleci-cli-v2/internal/cmd/job"
	cmdlogs "github.com/CircleCI-Public/circleci-cli-v2/internal/cmd/logs"
	"github.com/CircleCI-Public/circleci-cli-v2/internal/cmd/pipeline"
	"github.com/CircleCI-Public/circleci-cli-v2/internal/cmd/project"
	"github.com/CircleCI-Public/circleci-cli-v2/internal/cmd/runner"
	"github.com/CircleCI-Public/circleci-cli-v2/internal/cmd/settings"
	"github.com/CircleCI-Public/circleci-cli-v2/internal/cmd/workflow"
)

// NewRootCmd builds the root cobra command and wires all subcommands.
func NewRootCmd(version string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "circleci",
		Short: "The CircleCI CLI",
		Long: heredoc.Doc(`
			Work with CircleCI from the command line.

			Run 'circleci help <command>' for usage of a specific command.

			Docs:    https://circleci.com/docs/local-cli/
			Support: https://github.com/CircleCI-Public/circleci-cli/issues
		`),
		SilenceErrors: true, // main.go handles error printing
		SilenceUsage:  true,
	}

	cmd.Version = version
	cmd.SetVersionTemplate("circleci version {{.Version}}\n")

	cmd.PersistentFlags().BoolP("quiet", "q", false, "suppress informational output; data on stdout is unaffected")

	cmd.AddCommand(cmdapi.NewAPICmd())
	cmd.AddCommand(artifacts.NewArtifactsCmd())
	cmd.AddCommand(completion.NewCompletionCmd())
	cmd.AddCommand(envvar.NewEnvVarCmd())
	cmd.AddCommand(job.NewJobCmd())
	cmd.AddCommand(cmdlogs.NewLogsCmd())
	cmd.AddCommand(pipeline.NewPipelineCmd())
	cmd.AddCommand(project.NewProjectCmd())
	cmd.AddCommand(runner.NewRunnerCmd())
	cmd.AddCommand(settings.NewSettingsCmd())
	cmd.AddCommand(workflow.NewWorkflowCmd())

	return cmd
}
