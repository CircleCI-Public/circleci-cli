package cmd

import (
	"fmt"
	"io"

	"github.com/a8m/envsubst"
	"github.com/spf13/cobra"
)

func newEnvCmd() *cobra.Command {
	var envCmd = &cobra.Command{
		Use:   "env",
		Short: "Manage environment variables",
	}
	var substCmd = &cobra.Command{
		Use:   "subst",
		Short: "Substitute environment variables in a string",
		RunE:  substRunE,
	}
	envCmd.AddCommand(substCmd)
	return envCmd
}

// Accepts a string as an argument, or reads from stdin if no argument is provided.
func substRunE(cmd *cobra.Command, args []string) error {
	var input string
	if len(args) > 0 {
		input = args[0]
	} else {
		// Read from stdin
		b, err := io.ReadAll(cmd.InOrStdin())
		if err != nil {
			return err
		}
		input = string(b)
	}
	if input == "" {
		return nil
	}
	output, err := envsubst.String(input)
	if err != nil {
		return err
	}
	_, err = fmt.Fprint(cmd.OutOrStdout(), output)
	return err
}
