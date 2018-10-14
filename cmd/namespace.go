package cmd

import (
	"context"
	"strings"

	"github.com/CircleCI-Public/circleci-cli/api"
	"github.com/CircleCI-Public/circleci-cli/settings"
	"github.com/spf13/cobra"
)

type namespaceOptions struct {
	*settings.Config
	args []string
}

func newNamespaceCommand(config *settings.Config) *cobra.Command {
	opts := namespaceOptions{
		Config: config,
	}

	namespaceCmd := &cobra.Command{
		Use:   "namespace",
		Short: "Operate on namespaces",
	}

	createCmd := &cobra.Command{
		Use:   "create <name> <vcs-type> <org-name>",
		Short: "Create a namespace",
		Long: `Create a namespace.
Please note that at this time all namespaces created in the registry are world-readable.`,
		PreRun: func(cmd *cobra.Command, args []string) {
			opts.args = args

			if err := opts.Setup(); err != nil {
				panic(err)
			}
		},
		RunE: func(_ *cobra.Command, _ []string) error {
			return createNamespace(opts)
		},
		Args:        cobra.ExactArgs(3),
		Annotations: make(map[string]string),
	}

	createCmd.Annotations["<name>"] = "The name to give your new namespace"
	createCmd.Annotations["<vcs-type>"] = `Your VCS provider, can be either "github" or "bitbucket"`
	createCmd.Annotations["<org-name>"] = `The name used for your organization`

	namespaceCmd.AddCommand(createCmd)

	return namespaceCmd
}

func createNamespace(opts namespaceOptions) error {
	ctx := context.Background()
	namespaceName := opts.args[0]

	_, err := api.CreateNamespace(ctx, opts.Config, namespaceName, opts.args[2], strings.ToUpper(opts.args[1]))

	if err != nil {
		return err
	}

	opts.Logger.Infof("Namespace `%s` created.", namespaceName)
	opts.Logger.Info("Please note that any orbs you publish in this namespace are open orbs and are world-readable.")
	return nil
}
