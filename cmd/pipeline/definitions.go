package pipeline

import (
	"github.com/CircleCI-Public/circleci-cli/cmd/validator"
	"github.com/spf13/cobra"
)

func newDefinitionsCommand(ops *pipelineOpts, preRunE validator.Validator) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "definitions",
		Short: "Operate on pipeline definitions",
	}

	cmd.AddCommand(newDefinitionsListCommand(ops, preRunE))

	return cmd
}
