package cmd

import (
	"github.com/circleci/circleci-cli/filetree"
	"github.com/spf13/cobra"
)

var collapseCommand = &cobra.Command{
	Use:   "collapse",
	Short: "Collapse your CircleCI configuration to a single file",
	Run:   collapse,
}

var root string

func init() {
	collapseCommand.Flags().StringVarP(&root, "root", "r", ".circleci", "path to your configuration (default is .circleci)")
	// TODO: Add flag for excluding paths
}

func collapse(cmd *cobra.Command, args []string) {
	tree, err := filetree.NewTree(root)
	if err != nil {
		Logger.FatalOnError("An error occurred", err)
	}

	Logger.Prettyify(tree)
}
