package cmd

import (
	"context"
	"strings"

	"github.com/CircleCI-Public/circleci-cli/api"
	"github.com/spf13/cobra"
)

func newNamespaceCommand() *cobra.Command {
	namespaceCmd := &cobra.Command{
		Use:   "namespace",
		Short: "Operate on namespaces",
	}

	createCmd := &cobra.Command{
		Use:   "create NAME VCS-TYPE ORG-NAME",
		Short: "create a namespace",
		RunE:  createNamespace,
		Args:  cobra.ExactArgs(3),
	}

	// "org-name", "", "organization name (required)"
	// "vcs", "github", "organization vcs, e.g. 'github', 'bitbucket'"

	namespaceCmd.AddCommand(createCmd)

	return namespaceCmd
}

func createNamespace(cmd *cobra.Command, args []string) error {
	var err error
	ctx := context.Background()

	response, err := api.CreateNamespace(ctx, Logger, args[0], args[2], strings.ToUpper(args[1]))

	if err != nil {
		return err
	}

	if len(response.Errors) > 0 {
		return response.ToError()
	}

	Logger.Info("Namespace created")
	return nil
}
