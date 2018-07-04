package cmd

import (
	"github.com/CircleCI-Public/circleci-cli/filetree"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	yaml "gopkg.in/yaml.v2"
)

var root string

func newCollapseCommand() *cobra.Command {

	collapseCommand := &cobra.Command{
		Use:   "collapse",
		Short: "Collapse your CircleCI configuration to a single file",
		RunE:  collapse,
	}
	collapseCommand.Flags().StringVarP(&root, "root", "r", ".circleci", "path to your configuration (default is .circleci)")

	return collapseCommand
}

func collapse(cmd *cobra.Command, args []string) error {
	tree, err := filetree.NewTree(root)
	if err != nil {
		return errors.Wrap(err, "An error occurred trying to build the tree")
	}

	y, err := yaml.Marshal(&tree)
	if err != nil {
		return errors.Wrap(err, "Failed trying to marshal the tree to YAML ")
	}
	Logger.Infof("%s\n", string(y))
	return nil
}
