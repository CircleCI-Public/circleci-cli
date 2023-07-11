package cmd

import (
	"os"

	"github.com/CircleCI-Public/circleci-cli/cmd/create_telemetry"
	"github.com/CircleCI-Public/circleci-cli/settings"
	"github.com/CircleCI-Public/circleci-cli/telemetry"
	"github.com/spf13/cobra"
)

func newCompletionCommand(config *settings.Config) *cobra.Command {
	completionCmd := &cobra.Command{
		Use:   "completion",
		Short: "Generate shell completion scripts",
		PersistentPreRun: func(cmd *cobra.Command, _ []string) {
			telemetryClient := create_telemetry.CreateTelemetry(config)
			defer telemetryClient.Close()
			telemetryClient.Track(telemetry.CreateCompletionCommand(create_telemetry.GetCommandInformation(cmd, false)))
		},
		Run: func(cmd *cobra.Command, _ []string) {
			err := cmd.Help()
			if err != nil {
				panic(err)
			}
		},
	}

	bashCommand := &cobra.Command{
		Use:   "bash",
		Short: "Generate bash completion scripts",
		Run: func(cmd *cobra.Command, _ []string) {
			err := cmd.Root().GenBashCompletion(os.Stdout)
			if err != nil {
				panic(err)
			}
		},
	}

	zshCommand := &cobra.Command{
		Use:   "zsh",
		Short: "Generate zsh completion scripts",
		Run: func(cmd *cobra.Command, _ []string) {
			err := cmd.Root().GenZshCompletion(os.Stdout)
			if err != nil {
				panic(err)
			}
		},
	}

	completionCmd.AddCommand(bashCommand)
	completionCmd.AddCommand(zshCommand)

	return completionCmd
}
