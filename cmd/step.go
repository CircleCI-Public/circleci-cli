package cmd

import (
	"github.com/CircleCI-Public/circleci-cli/proxy"
	"github.com/spf13/cobra"
)

func newStepCommand() *cobra.Command {
	stepCmd := &cobra.Command{
		Use:                "step",
		Short:              "Execute steps",
		Hidden:             true,
		DisableFlagParsing: true,
	}

	haltCmd := &cobra.Command{
		Use:                "halt",
		Short:              "Halt the current job and treat it as successful",
		RunE:               haltRunE,
		Hidden:             true,
		DisableFlagParsing: true,
	}

	stepCmd.AddCommand(haltCmd)

	return stepCmd
}

func haltRunE(cmd *cobra.Command, args []string) error {
	return proxy.Exec([]string{"step", "halt"}, args)
}
