package workflow

import (
	"github.com/spf13/cobra"

	workflowapi "github.com/CircleCI-Public/circleci-cli/api/workflow"
	"github.com/CircleCI-Public/circleci-cli/cmd/validator"
	"github.com/CircleCI-Public/circleci-cli/settings"
)

type workflowOpts struct {
	workflowClient workflowapi.WorkflowClient
}

func NewWorkflowCommand(config *settings.Config, preRunE validator.Validator) *cobra.Command {
	pos := workflowOpts{}

	command := &cobra.Command{
		Use:   "workflow",
		Short: "Operate on workflows",
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			client, err := workflowapi.NewWorkflowRestClient(*config)
			if err != nil {
				return err
			}
			pos.workflowClient = client
			return nil
		},
	}

	command.AddCommand(newJobsCommand(&pos, preRunE))

	return command
}
