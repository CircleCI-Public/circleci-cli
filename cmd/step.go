package cmd

import (
	"github.com/CircleCI-Public/circleci-cli/proxy"
	"github.com/CircleCI-Public/circleci-cli/settings"
	"github.com/spf13/cobra"
)

type stepOptions struct {
	*settings.Config
	args []string
}

func newStepCommand(config *settings.Config) *cobra.Command {
	opts := stepOptions{
		Config: config,
	}

	stepCmd := &cobra.Command{
		Use:                "step",
		Short:              "Execute steps",
		Hidden:             true,
		DisableFlagParsing: true,
	}

	haltCmd := &cobra.Command{
		Use:   "halt",
		Short: "Halt the current job and treat it as successful",
		PreRun: func(cmd *cobra.Command, args []string) {
			opts.args = args

			if err := opts.Setup(); err != nil {
				panic(err)
			}
		},
		RunE: func(_ *cobra.Command, _ []string) error {
			return haltRunE(opts)
		},
		Hidden:             true,
		DisableFlagParsing: true,
	}

	stepCmd.AddCommand(haltCmd)

	return stepCmd
}

func haltRunE(opts stepOptions) error {
	return proxy.Exec([]string{"step", "halt"}, opts.args)
}
