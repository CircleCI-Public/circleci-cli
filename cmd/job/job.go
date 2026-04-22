package job

import (
	"github.com/spf13/cobra"

	jobapi "github.com/CircleCI-Public/circleci-cli/api/job"
	"github.com/CircleCI-Public/circleci-cli/cmd/validator"
	"github.com/CircleCI-Public/circleci-cli/settings"
)

type jobOpts struct {
	client jobapi.JobClient
}

// NewJobCommand generates a cobra command for job-level operations.
func NewJobCommand(config *settings.Config, preRunE validator.Validator) *cobra.Command {
	jos := jobOpts{}

	command := &cobra.Command{
		Use:   "job",
		Short: "Operate on jobs",
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			if jos.client != nil {
				return nil
			}
			client, err := jobapi.NewJobRestClient(*config)
			if err != nil {
				return err
			}
			jos.client = client
			return nil
		},
	}

	command.AddCommand(newLogsCommand(&jos, preRunE))
	command.AddCommand(newTestsCommand(&jos, preRunE))

	return command
}
