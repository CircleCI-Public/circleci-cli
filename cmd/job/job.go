package job

import (
	"github.com/spf13/cobra"

	jobapi "github.com/CircleCI-Public/circleci-cli/api/job"
	"github.com/CircleCI-Public/circleci-cli/cmd/validator"
	"github.com/CircleCI-Public/circleci-cli/settings"
)

type jobOpts struct {
	jobClient jobapi.JobClient
}

func NewJobCommand(config *settings.Config, preRunE validator.Validator) *cobra.Command {
	pos := jobOpts{}

	command := &cobra.Command{
		Use:   "job",
		Short: "Operate on jobs",
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			pos.jobClient = jobapi.NewClient(*config)
			return nil
		},
	}

	command.AddCommand(newLogsCommand(&pos, preRunE))
	command.AddCommand(newImageTagCommand(&pos, preRunE))

	return command
}
