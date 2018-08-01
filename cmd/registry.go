package cmd

import (
	"context"
	"strings"

	"github.com/CircleCI-Public/circleci-cli/api"
	"github.com/spf13/cobra"
)

func newRegistryCommand() *cobra.Command {
	registryCmd := &cobra.Command{
		Use:   "registry",
		Short: "Operate on the registry",
	}

	namespaceCommand := &cobra.Command{
		Use:   "namespace",
		Short: "Operate on orb namespaces (create, etc.)",
	}

	createNamespaceCommand := &cobra.Command{
		Use:   "create [name] [vcs] [org-name]",
		Short: "create an namespace",
		RunE:  createNamespace,
		Args:  cobra.ExactArgs(3),
	}

	// "org-name", "", "organization name (required)"
	// "vcs", "github", "organization vcs, e.g. 'github', 'bitbucket'"

	namespaceCommand.AddCommand(createNamespaceCommand)

	registryCmd.AddCommand(namespaceCommand)

	return registryCmd
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
