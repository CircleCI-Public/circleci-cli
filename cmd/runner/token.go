package runner

import (
	"os"
	"time"

	"github.com/olekukonko/tablewriter"
	"github.com/spf13/cobra"

	"github.com/CircleCI-Public/circleci-cli/api/runner"
)

func newTokenCommand(r *runner.Runner, preRunE validator) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "token",
		Short: "Operate on runner tokens",
	}

	cmd.AddCommand(&cobra.Command{
		Use:     "create <resource-class> <nickname>",
		Short:   "Create a token for a resource-class",
		Args:    cobra.ExactArgs(2),
		PreRunE: preRunE,
		RunE: func(_ *cobra.Command, args []string) error {
			token, err := r.CreateToken(args[0], args[1])
			if err != nil {
				return err
			}
			return generateConfig(*token, os.Stdout)
		},
	})

	cmd.AddCommand(&cobra.Command{
		Use:     "delete <token-id>",
		Short:   "Delete a token",
		Aliases: []string{"rm"},
		Args:    cobra.ExactArgs(1),
		PreRunE: preRunE,
		RunE: func(_ *cobra.Command, args []string) error {
			return r.DeleteToken(args[0])
		},
	})

	cmd.AddCommand(&cobra.Command{
		Use:     "list <resource-class>",
		Aliases: []string{"ls"},
		Short:   "List tokens for a resource-class",
		Args:    cobra.ExactArgs(1),
		PreRunE: preRunE,
		RunE: func(_ *cobra.Command, args []string) error {
			tokens, err := r.GetRunnerTokensByResourceClass(args[0])
			if err != nil {
				return err
			}

			table := tablewriter.NewWriter(os.Stdout)
			defer table.Render()
			table.SetHeader([]string{"ID", "Nickname", "Created At"})
			for _, token := range tokens {
				table.Append([]string{token.ID, token.Nickname, token.CreatedAt.Format(time.RFC3339)})
			}
			return nil
		},
	})

	return cmd
}
