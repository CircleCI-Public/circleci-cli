package cmd

import (
	"github.com/CircleCI-Public/circleci-cli/proxy"
	"github.com/spf13/cobra"
)

func newTestsCommand() *cobra.Command {
	testsCmd := &cobra.Command{
		Use:                "tests",
		Short:              "Collect and split tests so they may be run in parallel.",
		DisableFlagParsing: true,
		Hidden:             true,
		RunE: func(_ *cobra.Command, args []string) error {
			return proxy.Exec([]string{"tests"}, args)
		},
	}
	return testsCmd
}
