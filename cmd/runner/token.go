package runner

import (
	"time"

	"github.com/CircleCI-Public/circleci-cli/cmd/validator"
	"github.com/olekukonko/tablewriter"
	"github.com/spf13/cobra"
)

func newTokenCommand(o *runnerOpts, preRunE validator.Validator) *cobra.Command {
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
			token, err := o.r.CreateToken(args[0], args[1])
			if err != nil {
				return err
			}
			return generateConfig(*token, cmd.OutOrStdout())
		},
	})

	cmd.AddCommand(&cobra.Command{
		Use:     "delete <token-id>",
		Short:   "Delete a token",
		Aliases: []string{"rm"},
		Args:    cobra.ExactArgs(1),
		PreRunE: preRunE,
		RunE: func(_ *cobra.Command, args []string) error {
			return o.r.DeleteToken(args[0])
		},
	})

	cmd.AddCommand(&cobra.Command{
		Use:     "list <resource-class>",
		Aliases: []string{"ls"},
		Short:   "List tokens for a resource-class",
		Args:    cobra.ExactArgs(1),
		PreRunE: preRunE,
		RunE: func(_ *cobra.Command, args []string) error {
			tokens, err := o.r.GetRunnerTokensByResourceClass(args[0])
			if err != nil {
				return err
			}

			table := tablewriter.NewWriter(cmd.OutOrStdout())
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
