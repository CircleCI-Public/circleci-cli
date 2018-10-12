package cmd

import "github.com/spf13/cobra"

func newSwitchCommand() *cobra.Command {
	return &cobra.Command{
		Use:    "switch",
		RunE:   runSwitch,
		Hidden: true,
	}
}

func runSwitch(cmd *cobra.Command, args []string) error {
	Config.Logger.Infoln("You've already updated to the latest CLI. Please see `circleci help` for usage.")
	return nil
}
