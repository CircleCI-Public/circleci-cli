package cmd

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/CircleCI-Public/circleci-cli/api"
	"github.com/CircleCI-Public/circleci-cli/client"
	"github.com/olekukonko/tablewriter"
	"github.com/pkg/errors"

	"github.com/CircleCI-Public/circleci-cli/settings"
	"github.com/spf13/cobra"
)

func newContextCommand(config *settings.Config) *cobra.Command {

	var cl *client.Client

	initClient := func(cmd *cobra.Command, args []string) error {
		cl = client.NewClient(config.Host, config.Endpoint, config.Token, config.Debug)
		return validateToken(config)
	}

	command := &cobra.Command{
		Use:   "context",
		Short: "Contexts provide a mechanism for securing and sharing environment variables across projects. The environment variables are defined as name/value pairs and are injected at runtime.",
	}

	listCommand := &cobra.Command{
		Short:   "List all contexts",
		Use:     "list <vcs-type> <org-name>",
		PreRunE: initClient,
		RunE: func(cmd *cobra.Command, args []string) error {
			return listContexts(cl, args[0], args[1])
		},
		Args: cobra.ExactArgs(2),
	}

	showContextCommand := &cobra.Command{
		Short:   "Show a context",
		Use:     "show <vcs-type> <org-name> <context-name>",
		PreRunE: initClient,
		RunE: func(cmd *cobra.Command, args []string) error {
			return showContext(cl, args[0], args[1], args[2])
		},
		Args: cobra.ExactArgs(3),
	}

	storeCommand := &cobra.Command{
		Short:   "Store a new environment variable in the named context. The value is read from stdin.",
		Use:     "store-secret <vcs-type> <org-name> <context-name> <secret name>",
		PreRunE: initClient,
		RunE: func(cmd *cobra.Command, args []string) error {
			return storeEnvVar(cl, args[0], args[1], args[2], args[3])
		},
		Args: cobra.ExactArgs(4),
	}

	removeCommand := &cobra.Command{
		Short:   "Remove an environment variable from the named context",
		Use:     "remove-secret <vcs-type> <org-name> <context-name> <secret name>",
		PreRunE: initClient,
		RunE: func(cmd *cobra.Command, args []string) error {
			return removeEnvVar(cl, args[0], args[1], args[2], args[3])
		},
		Args: cobra.ExactArgs(4),
	}

	createContextCommand := &cobra.Command{
		Short:   "Create a new context",
		Use:     "create <vcs-type> <org-name> <context-name>",
		PreRunE: initClient,
		RunE: func(cmd *cobra.Command, args []string) error {
			return createContext(cl, args[0], args[1], args[2])
		},
		Args: cobra.ExactArgs(3),
	}

	force := false
	deleteContextCommand := &cobra.Command{
		Short:   "Delete the named context",
		Use:     "delete <vcs-type> <org-name> <context-name>",
		PreRunE: initClient,
		RunE: func(cmd *cobra.Command, args []string) error {
			return deleteContext(cl, force, args[0], args[1], args[2])
		},
		Args: cobra.ExactArgs(3),
	}

	deleteContextCommand.Flags().BoolVarP(&force, "force", "f", false, "Delete the context without asking for confirmation.")

	command.AddCommand(listCommand)
	command.AddCommand(showContextCommand)
	command.AddCommand(storeCommand)
	command.AddCommand(removeCommand)
	command.AddCommand(createContextCommand)
	command.AddCommand(deleteContextCommand)

	return command
}

func listContexts(client *client.Client, vcs, org string) error {

	contexts, err := api.ListContexts(client, org, vcs)

	if err != nil {
		return err

	}

	table := tablewriter.NewWriter(os.Stdout)

	table.SetHeader([]string{"Provider", "Organization", "Name", "Created At"})

	for _, context := range contexts.Organization.Contexts.Edges {

		table.Append([]string{
			vcs,
			org,
			context.Node.Name,
			context.Node.CreatedAt,
		})
	}
	table.Render()

	return nil
}

func contextByName(client *client.Client, vcsType, orgName, contextName string) (*api.CircleCIContext, error) {

	contexts, err := api.ListContexts(client, orgName, vcsType)

	if err != nil {
		return nil, err
	}

	for _, c := range contexts.Organization.Contexts.Edges {
		if c.Node.Name == contextName {
			return &c.Node, nil
		}
	}

	return nil, fmt.Errorf("Could not find a context named '%s' in the '%s' organization.", contextName, orgName)
}

func showContext(client *client.Client, vcsType, orgName, contextName string) error {

	context, err := contextByName(client, vcsType, orgName, contextName)

	if err != nil {
		return err
	}

	fmt.Printf("Context: %s\n", context.Name)

	table := tablewriter.NewWriter(os.Stdout)

	table.SetHeader([]string{"Environment Variable", "Value"})

	for _, envVar := range context.Resources {
		table.Append([]string{envVar.Variable, "••••" + envVar.TruncatedValue})
	}
	table.Render()

	return nil
}

// ReadSecretValue reads a secret from a buffer
func ReadSecretValue() (string, error) {
	stat, _ := os.Stdin.Stat()

	buffSize, err := os.Stdin.Stat()
	if err != nil {
		return "", err
	}

	reader := bufio.NewReaderSize(os.Stdin, int(buffSize.Size()))

	if (stat.Mode() & os.ModeCharDevice) == 0 {
		bytes := make([]byte, buffSize.Size())
		_, err := io.ReadFull(reader, bytes)
		return string(bytes), err
	}

	fmt.Print("Enter secret value and press enter: ")

	str, err := reader.ReadString('\n')
	return strings.TrimRight(str, "\n"), err
}

func createContext(client *client.Client, vcsType, orgName, contextName string) error {
	return api.CreateContext(client, vcsType, orgName, contextName)
}

func removeEnvVar(client *client.Client, vcsType, orgName, contextName, varName string) error {
	context, err := contextByName(client, vcsType, orgName, contextName)
	if err != nil {
		return err
	}
	return api.DeleteEnvironmentVariable(client, context.ID, varName)
}

func storeEnvVar(client *client.Client, vcsType, orgName, contextName, varName string) error {

	context, err := contextByName(client, vcsType, orgName, contextName)

	if err != nil {
		return err
	}
	secretValue, err := ReadSecretValue()

	if err != nil {
		return errors.Wrap(err, "Failed to read secret value from stdin")
	}

	return api.StoreEnvironmentVariable(client, context.ID, varName, secretValue)
}

func askForConfirmation(message string) bool {
	fmt.Println(message)
	var response string
	if _, err := fmt.Scanln(&response); err != nil {
		return false
	}
	return strings.HasPrefix(strings.ToLower(response), "y")
}

func deleteContext(client *client.Client, force bool, vcsType, orgName, contextName string) error {

	context, err := contextByName(client, vcsType, orgName, contextName)

	if err != nil {
		return err
	}

	message := fmt.Sprintf("Are you sure that you want to delete this context: %s/%s %s (y/n)?",
		vcsType, orgName, context.Name)

	shouldDelete := force || askForConfirmation(message)

	if !shouldDelete {
		return errors.New("OK, cancelling")
	}

	return api.DeleteContext(client, context.ID)
}
