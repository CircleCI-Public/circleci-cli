package cmd

import (
	"fmt"
	"strings"

	"github.com/CircleCI-Public/circleci-cli/api"
	"github.com/CircleCI-Public/circleci-cli/api/graphql"
	"github.com/CircleCI-Public/circleci-cli/prompt"
	"github.com/CircleCI-Public/circleci-cli/settings"
	"github.com/spf13/cobra"
)

type namespaceOptions struct {
	cfg  *settings.Config
	cl   *graphql.Client
	args []string

	// Allows user to skip y/n confirm when creating a namespace
	noPrompt bool
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
		Use:   "create <name> <vcs-type> <org-name>",
		Short: "Create a namespace",
		Long: `Create a namespace.
Please note that at this time all namespaces created in the registry are world-readable.`,
		PreRunE: func(_ *cobra.Command, args []string) error {
			opts.args = args
			opts.cl = graphql.NewClient(config.Host, config.Endpoint, config.Token, config.Debug)

			return validateToken(opts.cfg)
		},
		RunE: func(_ *cobra.Command, _ []string) error {
			if opts.integrationTesting {
				opts.tty = createNamespaceTestUI{
					confirm: true,
				}
			}

			return createNamespace(opts)
		},
		Args:        cobra.ExactArgs(3),
		Annotations: make(map[string]string),
	}

	createCmd.Annotations["<name>"] = "The name to give your new namespace"
	createCmd.Annotations["<vcs-type>"] = `Your VCS provider, can be either "github" or "bitbucket"`
	createCmd.Annotations["<org-name>"] = `The name used for your organization`

	createCmd.Flags().BoolVar(&opts.integrationTesting, "integration-testing", false, "Enable test mode to bypass interactive UI.")
	if err := createCmd.Flags().MarkHidden("integration-testing"); err != nil {
		panic(err)
	}
	createCmd.Flags().BoolVar(&opts.noPrompt, "no-prompt", false, "Disable prompt to bypass interactive UI.")

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

func createNamespace(opts namespaceOptions) error {
	namespaceName := opts.args[0]

	if !opts.noPrompt {
		fmt.Printf(`You are creating a namespace called "%s".

This is the only namespace permitted for your %s organization, %s.

To change the namespace, you will have to contact CircleCI customer support.

`, namespaceName, strings.ToLower(opts.args[1]), opts.args[2])
	}

	confirm := fmt.Sprintf("Are you sure you wish to create the namespace: `%s`", namespaceName)
	if opts.noPrompt || opts.tty.askUserToConfirm(confirm) {
		_, err := api.CreateNamespace(opts.cl, namespaceName, opts.args[2], strings.ToUpper(opts.args[1]))

		if err != nil {
			return err
		}

		fmt.Printf("Namespace `%s` created.\n", namespaceName)
		fmt.Println("Please note that any orbs you publish in this namespace are open orbs and are world-readable.")
	}

	return nil
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

		fmt.Printf("Namespace `%s` renamed to `%s`. `%s` is an alias for `%s` so existing usages will continue to work, unless you delete the %s alias with `namespace delete-alias %s`", oldName, newName, oldName, newName, oldName, oldName)
	}
	return nil
}
