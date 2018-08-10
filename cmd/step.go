package cmd

import (
	"github.com/CircleCI-Public/circleci-cli/proxy"
	"github.com/spf13/cobra"
)

func newStepCommand() *cobra.Command {
	stepCmd := &cobra.Command{
		Use:    "step",
		Short:  "Execute steps",
		Hidden: true,
	}

	haltCmd := &cobra.Command{
		Use:    "halt",
		Short:  "halt current task and treat as a successful",
		RunE:   haltRunE,
		Hidden: true,
	}

	stepCmd.AddCommand(haltCmd)

	return stepCmd
}

func haltRunE(cmd *cobra.Command, args []string) error {
	return proxy.Exec("step halt", args)
}
