package runner

import (
	"io"

	"github.com/olekukonko/tablewriter"
	"github.com/spf13/cobra"

	"github.com/CircleCI-Public/circleci-cli/api/runner"
	"github.com/CircleCI-Public/circleci-cli/cmd/validator"
)

func newResourceClassCommand(o *runnerOpts, preRunE validator.Validator) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "resource-class",
		Short: "Operate on runner resource-classes",
	}

	genToken := false
	createCmd := &cobra.Command{
		Use:     "create <resource-class> <description>",
		Short:   "Create a resource-class",
		Args:    cobra.ExactArgs(2),
		PreRunE: preRunE,
		RunE: func(_ *cobra.Command, args []string) error {
			cmd.PrintErr(terms)

			rc, err := o.r.CreateResourceClass(args[0], args[1])
			if err != nil {
				return err
			}
			table := newResourceClassTable(cmd.OutOrStdout())
			defer table.Render()
			appendResourceClass(table, *rc)

			if !genToken {
				return nil
			}

			token, err := o.r.CreateToken(args[0], "default")
			if err != nil {
				return err
			}
			return generateConfig(*token, cmd.OutOrStdout())
		},
	}
	createCmd.PersistentFlags().BoolVar(&genToken, "generate-token", false,
		"Generate a default token")
	cmd.AddCommand(createCmd)

	forceDelete := false
	deleteCmd := &cobra.Command{
		Use:     "delete <resource-class>",
		Short:   "Delete a resource-class",
		Aliases: []string{"rm"},
		Args:    cobra.ExactArgs(1),
		PreRunE: preRunE,
		RunE: func(_ *cobra.Command, args []string) error {
			rc, err := o.r.GetResourceClassByName(args[0])
			if err != nil {
				return err
			}
			return o.r.DeleteResourceClass(rc.ID, forceDelete)
		},
	}
	deleteCmd.PersistentFlags().BoolVarP(&forceDelete, "force", "f", false,
		"Delete resource-class and any associated tokens")
	cmd.AddCommand(deleteCmd)

	cmd.AddCommand(&cobra.Command{
		Use:     "list <namespace>",
		Short:   "List resource-classes for a namespace",
		Aliases: []string{"ls"},
		Args:    cobra.ExactArgs(1),
		PreRunE: preRunE,
		RunE: func(_ *cobra.Command, args []string) error {
			rcs, err := o.r.GetResourceClassesByNamespace(args[0])
			if err != nil {
				return err
			}

			table := newResourceClassTable(cmd.OutOrStdout())
			defer table.Render()
			for _, rc := range rcs {
				appendResourceClass(table, rc)
			}

			return nil
		},
	})

	return cmd
}

func newResourceClassTable(writer io.Writer) *tablewriter.Table {
	table := tablewriter.NewWriter(writer)
	table.SetHeader([]string{"Resource Class", "Description"})
	return table
}

func appendResourceClass(table *tablewriter.Table, rc runner.ResourceClass) {
	table.Append([]string{rc.ResourceClass, rc.Description})
}

const terms = "If you have not already agreed to Runner Terms in a signed Order, " +
	"then by continuing to install Runner, " +
	"you are agreeing to CircleCI's Runner Terms which are found at: https://circleci.com/legal/runner-terms/.\n" +
	"If you already agreed to Runner Terms in a signed Order, " +
	"the Runner Terms in the signed Order supersede the Runner Terms in the web address above.\n" +
	"If you did not already agree to Runner Terms through a signed Order and do not agree to the Runner Terms in the web address above, " +
	"please do not install or use Runner.\n\n"
