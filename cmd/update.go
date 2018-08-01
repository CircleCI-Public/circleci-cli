package cmd

import (
	"github.com/spf13/cobra"
)

func newUpdateCommand() *cobra.Command {
	update := &cobra.Command{
		Use:    "update",
		Short:  "Update the tool",
		Hidden: true,
		RunE:   update,
	}

	return update
}
