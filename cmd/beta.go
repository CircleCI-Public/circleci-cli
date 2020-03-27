package cmd

import (
	"context"
	"fmt"

	"github.com/CircleCI-Public/circleci-cli/flaky_tests"
	"github.com/CircleCI-Public/circleci-cli/settings"
	"github.com/spf13/cobra"
)

func newBetaCommand(config *settings.Config) *cobra.Command {

	command := &cobra.Command{
		Use:   "beta",
		Short: "These commands are not yet stable.",
	}

	flakyCommand := &cobra.Command{
		Short: "Find the flaky tests in this project",
		Use:   "flaky-tests",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			result, err := flaky_tests.DoIt(
				ctx,
				config.Token,
				"gh/circleci/circle",
				"master")
			if err != nil {
				return err
			}
			fmt.Println(result)
			return nil
		},
	}

	command.AddCommand(flakyCommand)
	return command
}
