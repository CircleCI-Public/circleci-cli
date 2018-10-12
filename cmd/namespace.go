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
		Use:   "create <name> <vcs-type> <org-name>",
		Short: "Create a namespace",
		Long: `Create a namespace.
Please note that at this time all namespaces created in the registry are world-readable.`,
		RunE:        createNamespace,
		Args:        cobra.ExactArgs(3),
		Annotations: make(map[string]string),
	}

	createCmd.Annotations["<name>"] = "The name to give your new namespace"
	createCmd.Annotations["<vcs-type>"] = `Your VCS provider, can be either "github" or "bitbucket"`
	createCmd.Annotations["<org-name>"] = `The name used for your organization`

	namespaceCmd.AddCommand(createCmd)

	return namespaceCmd
}

func createNamespace(cmd *cobra.Command, args []string) error {
	ctx := context.Background()
	namespaceName := args[0]

	_, err := api.CreateNamespace(ctx, Config, namespaceName, args[2], strings.ToUpper(args[1]))

	if err != nil {
		return err
	}

	Config.Logger.Infof("Namespace `%s` created.", namespaceName)
	Config.Logger.Info("Please note that any orbs you publish in this namespace are open orbs and are world-readable.")
	return nil
}
