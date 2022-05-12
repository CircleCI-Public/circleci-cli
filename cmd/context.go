package cmd

import (
	"bufio"
	"fmt"
	"io/ioutil"
	"os"
	"strings"
	"time"

	"github.com/CircleCI-Public/circleci-cli/api"
	"github.com/olekukonko/tablewriter"
	"github.com/pkg/errors"

	"github.com/CircleCI-Public/circleci-cli/settings"
	"github.com/spf13/cobra"
)

func newContextCommand(config *settings.Config) *cobra.Command {
	var contextClient api.ContextInterface

	initClient := func(cmd *cobra.Command, args []string) (e error) {
		contextClient, e = api.NewContextRestClient(*config)
		if e != nil {
			return e
		}

		// If we're on cloud, we're good.
		if config.Host == defaultHost || contextClient.(*api.ContextRestClient).EnsureExists() == nil {
			return validateToken(config)
		}

		contextClient = api.NewContextGraphqlClient(config.HTTPClient, config.Host, config.Endpoint, config.Token, config.Debug)

		return validateToken(config)
	}

	command := &cobra.Command{
		Use: "context ",
		Long: `Contexts provide a mechanism for securing and sharing environment variables across 
		projects. The environment variables are defined as name/value pairs and 
		are injected at runtime.`,
		Short: "For securing and sharing environment variables across projects",
	}

	listCommand := &cobra.Command{
		Use:     "list <vcs-type> <org-name>",
		Short:   "List all contexts",
		Example: "circleci context list github OrgName",
		PreRunE: initClient,
		RunE: func(cmd *cobra.Command, args []string) error {
			return listContexts(contextClient, args[0], args[1])
		},
		Args: cobra.ExactArgs(2),
	}

	showContextCommand := &cobra.Command{
		Use:     "show <vcs-type> <org-name> <context-name>",
		Short:   "Show a context",
		Example: "circleci context show github OrgName contextName",
		PreRunE: initClient,
		RunE: func(cmd *cobra.Command, args []string) error {
			return showContext(contextClient, args[0], args[1], args[2])
		},
		Args: cobra.ExactArgs(3),
	}

	storeCommand := &cobra.Command{
		Use:     "store-secret <vcs-type> <org-name> <context-name> <secret name>",
		Short:   `Store a new environment variable in this context. The value is read from stdin.`,
		Example: "circleci context store-secret github OrgName contextName secretName",
		PreRunE: initClient,
		RunE: func(cmd *cobra.Command, args []string) error {
			return storeEnvVar(contextClient, args[0], args[1], args[2], args[3])
		},
		Args: cobra.ExactArgs(4),
	}

	removeCommand := &cobra.Command{
		Use:     "remove-secret <vcs-type> <org-name> <context-name> <secret name>",
		Short:   "Remove an environment variable from the named context",
		Example: "circleci context remove-secret github OrgName contextName secretName",
		PreRunE: initClient,
		RunE: func(cmd *cobra.Command, args []string) error {
			return removeEnvVar(contextClient, args[0], args[1], args[2], args[3])
		},
		Args: cobra.ExactArgs(4),
	}

	createContextCommand := &cobra.Command{
		Use:     "create <vcs-type> <org-name> <context-name>",
		Short:   "Create a new context",
		Example: "circleci context create github OrgName contextName",
		PreRunE: initClient,
		RunE: func(cmd *cobra.Command, args []string) error {
			return createContext(contextClient, args[0], args[1], args[2])
		},
		Args: cobra.ExactArgs(3),
	}

	force := false
	deleteContextCommand := &cobra.Command{
		Use:     "delete <vcs-type> <org-name> <context-name>",
		Short:   "Delete the named context",
		Example: "circleci context delete github OrgName contextName",
		PreRunE: initClient,
		RunE: func(cmd *cobra.Command, args []string) error {
			return deleteContext(contextClient, force, args[0], args[1], args[2])
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

func listContexts(contextClient api.ContextInterface, vcs, org string) error {
	contexts, err := contextClient.Contexts(vcs, org)

	if err != nil {
		return err
	}

	table := tablewriter.NewWriter(os.Stdout)

	table.SetHeader([]string{"Provider", "Organization", "Name", "Created At"})

	for _, context := range *contexts {
		table.Append([]string{
			vcs,
			org,
			context.Name,
			context.CreatedAt.Format(time.RFC3339),
		})
	}
	table.Render()

	return nil
}

func showContext(client api.ContextInterface, vcsType, orgName, contextName string) error {
	context, err := client.ContextByName(vcsType, orgName, contextName)
	if err != nil {
		return err
	}
	envVars, err := client.EnvironmentVariables(context.ID)
	if err != nil {
		return err
	}

	fmt.Printf("Context: %s\n", context.Name)

	table := tablewriter.NewWriter(os.Stdout)

	table.SetHeader([]string{"Environment Variable", "Value"})

	for _, envVar := range *envVars {
		table.Append([]string{envVar.Variable, "••••"})
	}
	table.Render()

	return nil
}

func readSecretValue() (string, error) {
	stat, _ := os.Stdin.Stat()
	if (stat.Mode() & os.ModeCharDevice) == 0 {
		bytes, err := ioutil.ReadAll(os.Stdin)
		return string(bytes), err
	} else {
		fmt.Print("Enter secret value and press enter: ")
		reader := bufio.NewReader(os.Stdin)
		str, err := reader.ReadString('\n')
		return strings.TrimRight(str, "\n"), err
	}
}

func createContext(client api.ContextInterface, vcsType, orgName, contextName string) error {
	err := client.CreateContext(vcsType, orgName, contextName)
	return err
}

func removeEnvVar(client api.ContextInterface, vcsType, orgName, contextName, varName string) error {
	context, err := client.ContextByName(vcsType, orgName, contextName)
	if err != nil {
		return err
	}
	return client.DeleteEnvironmentVariable(context.ID, varName)
}

func storeEnvVar(client api.ContextInterface, vcsType, orgName, contextName, varName string) error {

	context, err := client.ContextByName(vcsType, orgName, contextName)

	if err != nil {
		return err
	}
	secretValue, err := readSecretValue()

	if err != nil {
		return errors.Wrap(err, "Failed to read secret value from stdin")
	}

	err = client.CreateEnvironmentVariable(context.ID, varName, secretValue)
	return err
}

func askForConfirmation(message string) bool {
	fmt.Println(message)
	var response string
	if _, err := fmt.Scanln(&response); err != nil {
		return false
	}
	return strings.HasPrefix(strings.ToLower(response), "y")
}

func deleteContext(client api.ContextInterface, force bool, vcsType, orgName, contextName string) error {

	context, err := client.ContextByName(vcsType, orgName, contextName)

	if err != nil {
		return err
	}

	message := fmt.Sprintf("Are you sure that you want to delete this context: %s/%s %s (y/n)?",
		vcsType, orgName, context.Name)

	shouldDelete := force || askForConfirmation(message)

	if !shouldDelete {
		return errors.New("OK, cancelling")
	}

	return client.DeleteContext(context.ID)
}
