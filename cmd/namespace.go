package cmd

import (
	"fmt"

	"github.com/CircleCI-Public/circleci-cli/api"
	"github.com/CircleCI-Public/circleci-cli/api/graphql"
	"github.com/CircleCI-Public/circleci-cli/prompt"
	"github.com/CircleCI-Public/circleci-cli/settings"
	"github.com/CircleCI-Public/circleci-cli/telemetry"
	"github.com/google/uuid"
	"github.com/spf13/cobra"
)

type namespaceOptions struct {
	cfg  *settings.Config
	cl   *graphql.Client
	args []string

	// Allows user to skip y/n confirm when creating a namespace
	noPrompt bool
	orgID    *string
	// This lets us pass in our own interface for testing
	tty createNamespaceUserInterface
	// Linked with --integration-testing flag for stubbing UI in gexec tests
	integrationTesting bool
}

type createNamespaceUserInterface interface {
	askUserToConfirm(message string) bool
}

type createNamespaceInteractiveUI struct{}

func (createNamespaceInteractiveUI) askUserToConfirm(message string) bool {
	return prompt.AskUserToConfirm(message)
}

type createNamespaceTestUI struct {
	confirm bool
}

func (ui createNamespaceTestUI) askUserToConfirm(message string) bool {
	fmt.Println(message)
	return ui.confirm
}

func newNamespaceCommand(config *settings.Config) *cobra.Command {
	opts := namespaceOptions{
		cfg: config,
		tty: createNamespaceInteractiveUI{},
	}

	namespaceCmd := &cobra.Command{
		Use:   "namespace",
		Short: "Operate on namespaces",
	}

	createCmd := &cobra.Command{
		Use:   "create <name> --org-id <your-organization-id>",
		Short: "Create a namespace",
		Long: `Create a namespace.
Please note that at this time all namespaces created in the registry are world-readable.`,
		PreRunE: func(_ *cobra.Command, args []string) error {
			opts.args = args
			opts.cl = graphql.NewClient(config.HTTPClient, config.Host, config.Endpoint, config.Token, config.Debug)

			return validateToken(opts.cfg)
		},
		RunE: func(cmd *cobra.Command, _ []string) error {
			if opts.integrationTesting {
				opts.tty = createNamespaceTestUI{
					confirm: true,
				}
			}

			err := createNamespace(cmd, opts)

			telemetryClient, ok := telemetry.FromContext(cmd.Context())
			if ok {
				_ = telemetryClient.Track(telemetry.CreateNamespaceEvent(telemetry.GetCommandInformation(cmd, true)))
			}

			return err
		},
		Args:        cobra.RangeArgs(1, 1),
		Annotations: make(map[string]string),
		Example:     `  circleci namespace create NamespaceName --org-id 00000000-0000-0000-0000-000000000000`,
	}

	createCmd.Annotations["<name>"] = "The name to give your new namespace"

	createCmd.Flags().BoolVar(&opts.integrationTesting, "integration-testing", false, "Enable test mode to bypass interactive UI.")
	if err := createCmd.Flags().MarkHidden("integration-testing"); err != nil {
		panic(err)
	}
	createCmd.Flags().BoolVar(&opts.noPrompt, "no-prompt", false, "Disable prompt to bypass interactive UI.")
	opts.orgID = createCmd.Flags().String("org-id", "", "The id of your organization.")

	namespaceCmd.AddCommand(createCmd)

	return namespaceCmd
}

func deleteNamespaceAlias(opts namespaceOptions) error {
	aliasName := opts.args[0]
	confirm := fmt.Sprintf("Are you sure you wish to delete the namespace alias %s? You should make sure that all configs and orbs that refer to it this way are updated to the new name first.", aliasName)
	if opts.noPrompt || opts.tty.askUserToConfirm(confirm) {
		err := api.DeleteNamespaceAlias(opts.cl, aliasName)
		return err
	}
	return nil
}

func createNamespaceWithOrgId(opts namespaceOptions, namespaceName, orgId string) error {
	if !opts.noPrompt {
		fmt.Printf(`You are creating a namespace called "%s".

This is the only namespace permitted for your organization with id %s.

To change the namespace, you will have to contact CircleCI customer support.

`, namespaceName, orgId)
	}

	confirm := fmt.Sprintf("Are you sure you wish to create the namespace: `%s`", namespaceName)
	if opts.noPrompt || opts.tty.askUserToConfirm(confirm) {
		_, err := api.CreateNamespaceWithOwnerID(opts.cl, namespaceName, orgId)

		if err != nil {
			return err
		}

		fmt.Printf("Namespace `%s` created.\n", namespaceName)
		fmt.Println("Please note that any orbs you publish in this namespace are open orbs and are world-readable.")
	}
	return nil
}

func createNamespace(cmd *cobra.Command, opts namespaceOptions) error {
	namespaceName := opts.args[0]
	_, err := uuid.Parse(*opts.orgID)
	if err == nil {
		return createNamespaceWithOrgId(opts, namespaceName, *opts.orgID)
	}
	return cmd.Help()
}

func renameNamespace(opts namespaceOptions) error {
	oldName := opts.args[0]
	newName := opts.args[1]

	confirm := fmt.Sprintf("Are you sure you wish to rename the namespace `%s` to `%s`?", oldName, newName)
	if opts.noPrompt || opts.tty.askUserToConfirm(confirm) {
		_, err := api.RenameNamespace(opts.cl, oldName, newName)

		if err != nil {
			return err
		}

		fmt.Printf("Namespace `%s` renamed to `%s`. `%s` is an alias for `%s` so existing usages will continue to work, unless you delete the `%s` alias with `delete-namespace-alias %s`", oldName, newName, oldName, newName, oldName, oldName)
	}
	return nil
}
