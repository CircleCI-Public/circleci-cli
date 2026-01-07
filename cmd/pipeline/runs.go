package pipeline

import (
	"github.com/CircleCI-Public/circleci-cli/cmd/validator"
	"github.com/spf13/cobra"
)

func newRunsCommand(ops *pipelineOpts, preRunE validator.Validator) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "runs",
		Short: "Operate on pipeline runs",
	}

	cmd.AddCommand(newRunsLatestCommand(ops, preRunE))

	return cmd
}
