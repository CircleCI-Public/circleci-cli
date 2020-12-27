package runner

import (
	"os"

	"github.com/olekukonko/tablewriter"
	"github.com/spf13/cobra"

	"github.com/CircleCI-Public/circleci-cli/api/runner"
)

func newResourceClassCommand(r *runner.Runner, preRunE validator) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "resource-class",
		Short: "Operate on runner resource-classes",
	}

	cmd.AddCommand(&cobra.Command{
		Use:     "create <resource-class> <description>",
		Short:   "Create a resource-class",
		Args:    cobra.ExactArgs(2),
		PreRunE: preRunE,
		RunE: func(_ *cobra.Command, args []string) error {
			rc, err := r.CreateResourceClass(args[0], args[1])
			if err != nil {
				return err
			}
			table := newResourceClassTable()
			defer table.Render()
			appendResourceClass(table, *rc)
			return nil
		},
	})

	cmd.AddCommand(&cobra.Command{
		Use:     "delete <resource-class>",
		Short:   "Delete a resource-class",
		Aliases: []string{"rm"},
		Args:    cobra.ExactArgs(1),
		PreRunE: preRunE,
		RunE: func(_ *cobra.Command, args []string) error {
			rc, err := r.GetResourceClassByName(args[0])
			if err != nil {
				return err
			}
			return r.DeleteResourceClass(rc.ID)
		},
	})

	cmd.AddCommand(&cobra.Command{
		Use:     "list <namespace>",
		Short:   "List resource-classes for a namespace",
		Aliases: []string{"ls"},
		Args:    cobra.ExactArgs(1),
		PreRunE: preRunE,
		RunE: func(_ *cobra.Command, args []string) error {
			rcs, err := r.GetResourceClassesByNamespace(args[0])
			if err != nil {
				return err
			}

			table := newResourceClassTable()
			defer table.Render()
			for _, rc := range rcs {
				appendResourceClass(table, rc)
			}

			return nil
		},
	})

	return cmd
}

func newResourceClassTable() *tablewriter.Table {
	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeader([]string{"Resource Class", "Description"})
	return table
}

func appendResourceClass(table *tablewriter.Table, rc runner.ResourceClass) {
	table.Append([]string{rc.ResourceClass, rc.Description})
}
