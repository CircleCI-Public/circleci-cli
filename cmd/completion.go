package cmd

import (
	"os"

	"github.com/spf13/cobra"
)

func newCompletionCommand() *cobra.Command {
	completionCmd := &cobra.Command{
		Use:   "completion",
		Short: "Generate shell completion scripts",
		Run: func(cmd *cobra.Command, _ []string) {
			cmd.Help()
		},
	}

	bashCommand := &cobra.Command{
		Use:   "bash",
		Short: "Generate bash completion scripts",
		Run: func(cmd *cobra.Command, _ []string) {
			cmd.Root().GenBashCompletion(os.Stdout)
		},
	}

	zshCommand := &cobra.Command{
		Use:   "zsh",
		Short: "Generate zsh completion scripts",
		Run: func(cmd *cobra.Command, _ []string) {
			cmd.Root().GenZshCompletion(os.Stdout)
		},
	}

	completionCmd.AddCommand(bashCommand)
	completionCmd.AddCommand(zshCommand)

	return completionCmd
}
