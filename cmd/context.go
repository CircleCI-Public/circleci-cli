package cmd

import (
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/CircleCI-Public/circleci-cli/api"
	"github.com/CircleCI-Public/circleci-cli/api/context"
	"github.com/CircleCI-Public/circleci-cli/api/graphql"
	"github.com/CircleCI-Public/circleci-cli/prompt"
	"github.com/olekukonko/tablewriter"
	"github.com/pkg/errors"

	"github.com/CircleCI-Public/circleci-cli/settings"
	"github.com/spf13/cobra"
)

var (
	orgIDUsage = "Your organization id. You can obtain it on the \"Overview\" section of your organization settings page."

	orgID              string
	vcsType            string
	orgName            string
	initiatedArgs      []string
	integrationTesting bool
)

func MultiExactArgs(numbers ...int) cobra.PositionalArgs {
	numList := make([]string, len(numbers))
	for i, n := range numbers {
		numList[i] = strconv.Itoa(n)
	}

	return func(cmd *cobra.Command, args []string) error {
		for _, n := range numbers {
			if len(args) == n {
				return nil
			}
		}
		return fmt.Errorf("accepts %s arg(s), received %d", strings.Join(numList, ", "), len(args))
	}
}

func newContextCommand(config *settings.Config) *cobra.Command {
	var contextClient context.ContextInterface

	initClient := func(cmd *cobra.Command, args []string) (e error) {
		initiatedArgs = args
		if orgID == "" && len(args) < 2 {
			_ = cmd.Usage()
			return errors.New("need to define either --org-id or <vcsType> and <orgName> arguments")
		}
		if orgID != "" {
			contextClient = context.NewContextClient(config, orgID, "", "")
		}
		if orgID == "" && len(args) >= 2 {
			vcsType = args[0]
			orgName = args[1]
			initiatedArgs = args[2:]
			contextClient = context.NewContextClient(config, "", vcsType, orgName)
		}

		return validateToken(config)
	}

	command := &cobra.Command{
		Use: "context",
		Long: `Contexts provide a mechanism for securing and sharing environment variables across
projects. The environment variables are defined as name/value pairs and
are injected at runtime.`,
		Short: "For securing and sharing environment variables across projects",
	}

	listCommand := &cobra.Command{
		Short:   "List all contexts",
		Use:     "list --org-id <org-id>",
		PreRunE: initClient,
		RunE: func(cmd *cobra.Command, args []string) error {
			gqlClient := graphql.NewClient(config.HTTPClient, config.Host, config.Endpoint, config.Token, false)
			params := api.GetOrganizationParams{
				OrgID:   orgID,
				VCSType: vcsType,
				OrgName: orgName,
			}
			params.OrgID = orgID
			params.VCSType = vcsType
			params.OrgName = orgName
			org, err := api.GetOrganization(gqlClient, params)
			if err != nil {
				return err
			}

			return listContexts(contextClient, org.Organization.Name, org.Organization.ID)
		},
		Args: MultiExactArgs(0, 2),
		Example: `circleci context list --org-id 00000000-0000-0000-0000-000000000000
(deprecated usage) circleci context list <vcs-type> <org-name>`,
	}
	listCommand.Flags().StringVar(&orgID, "org-id", "", orgIDUsage)

	showContextCommand := &cobra.Command{
		Short:   "Show a list of all environment variables stored in a context.",
		Use:     "show --org-id <org-id> <context-name>",
		PreRunE: initClient,
		RunE: func(cmd *cobra.Command, args []string) error {
			return showContext(contextClient, initiatedArgs[0])
		},
		Args: MultiExactArgs(1, 3),
		Example: `circleci context show --org-id --org-id 00000000-0000-0000-0000-000000000000 contextName
(deprecated usage) circleci context show github orgName contextName`,
	}
	showContextCommand.Flags().StringVar(&orgID, "org-id", "", orgIDUsage)

	storeCommand := &cobra.Command{
		Short:   "Store a new environment variable in the named context. The value is read from stdin.",
		Use:     "store-secret --org-id <org-id> <context-name> <secret-name>",
		PreRunE: initClient,
		RunE: func(cmd *cobra.Command, args []string) error {
			var prompt storeEnvVarPrompt
			if integrationTesting {
				prompt = testPrompt{"value"}
			} else {
				prompt = secretPrompt{}
			}
			return storeEnvVar(contextClient, prompt, initiatedArgs[0], initiatedArgs[1])
		},
		Args: MultiExactArgs(2, 4),
		Example: `circleci context store-secret --org-id 00000000-0000-0000-0000-000000000000 contextName secretName
(deprecated usage) circleci context store-secret github orgName contextName secretName`,
	}
	storeCommand.Flags().StringVar(&orgID, "org-id", "", orgIDUsage)
	storeCommand.Flags().BoolVar(&integrationTesting, "integration-testing", false, "Enable test mode to setup rest API")
	if err := storeCommand.Flags().MarkHidden("integration-testing"); err != nil {
		panic(err)
	}

	removeCommand := &cobra.Command{
		Short:   "Remove an environment variable from the named context",
		Use:     "remove-secret --org-id <org-id> <context-name> <secret name>",
		PreRunE: initClient,
		RunE: func(cmd *cobra.Command, args []string) error {
			return removeEnvVar(contextClient, initiatedArgs[0], initiatedArgs[1])
		},
		Args: MultiExactArgs(2, 4),
		Example: `circleci context remove-secret --org-id 00000000-0000-0000-0000-000000000000 contextName secretName
(deprecated usage) circleci context remove-secret github orgName contextName secretName`,
	}
	removeCommand.Flags().StringVar(&orgID, "org-id", "", orgIDUsage)

	createContextCommand := &cobra.Command{
		Short:   "Create a new context",
		Use:     "create --org-id <org-id> <context-name>",
		PreRunE: initClient,
		RunE: func(cmd *cobra.Command, args []string) error {
			return createContext(contextClient, initiatedArgs[0])
		},
		Args: MultiExactArgs(1, 3),
		Example: `circleci context create --org-id 00000000-0000-0000-0000-000000000000 contextName
(deprecated usage) circleci context create github OrgName contextName`,
	}
	createContextCommand.Flags().StringVar(&orgID, "org-id", "", orgIDUsage)
	createContextCommand.Flags().BoolVar(&integrationTesting, "integration-testing", false, "Enable test mode to setup rest API")
	if err := createContextCommand.Flags().MarkHidden("integration-testing"); err != nil {
		panic(err)
	}

	force := false
	deleteContextCommand := &cobra.Command{
		Short:   "Delete the named context",
		Use:     "delete --org-id <org-id> <context-name>",
		PreRunE: initClient,
		RunE: func(cmd *cobra.Command, args []string) error {
			gqlClient := graphql.NewClient(config.HTTPClient, config.Host, config.Endpoint, config.Token, false)
			params := api.GetOrganizationParams{
				OrgID:   orgID,
				VCSType: vcsType,
				OrgName: orgName,
			}
			params.OrgID = orgID
			params.VCSType = vcsType
			params.OrgName = orgName
			org, err := api.GetOrganization(gqlClient, params)
			if err != nil {
				return err
			}
			return deleteContext(contextClient, org.Organization.Name, force, initiatedArgs[0])
		},
		Args: MultiExactArgs(1, 3),
		Example: `circleci context delete --org-id 00000000-0000-0000-0000-000000000000 contextName
(deprecated usage) circleci context create github OrgName contextName`,
	}
	deleteContextCommand.Flags().BoolVarP(&force, "force", "f", false, "Delete the context without asking for confirmation.")
	deleteContextCommand.Flags().StringVar(&orgID, "org-id", "", orgIDUsage)

	command.AddCommand(listCommand)
	command.AddCommand(showContextCommand)
	command.AddCommand(storeCommand)
	command.AddCommand(removeCommand)
	command.AddCommand(createContextCommand)
	command.AddCommand(deleteContextCommand)

	return command
}

func listContexts(contextClient context.ContextInterface, orgName string, orgId string) error {
	contexts, err := contextClient.Contexts()
	if err != nil {
		return err
	}

	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeader([]string{"Organization", "Org ID", "Name", "Created At"})
	for _, context := range contexts {
		table.Append([]string{
			orgName,
			orgId,
			context.Name,
			context.CreatedAt.Format(time.RFC3339),
		})
	}
	table.Render()
	return nil
}

func showContext(client context.ContextInterface, contextName string) error {
	context, err := client.ContextByName(contextName)
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

	for _, envVar := range envVars {
		table.Append([]string{envVar.Variable, "••••"})
	}
	table.Render()

	return nil
}

// createContext determines if the context is being created via orgid or vcs and org name
// and navigates to corresponding function accordingly
func createContext(client context.ContextInterface, name string) error {
	err := client.CreateContext(name)
	if err == nil {
		fmt.Printf("Created context %s.\n", name)
	}
	return err
}

func removeEnvVar(client context.ContextInterface, contextName, varName string) error {
	context, err := client.ContextByName(contextName)
	if err != nil {
		return err
	}
	err = client.DeleteEnvironmentVariable(context.ID, varName)
	if err == nil {
		fmt.Printf("Removed secret %s from context %s.\n", varName, contextName)
	}
	return err
}

type storeEnvVarPrompt interface {
	askForValue() (string, error)
}

type secretPrompt struct{}

func (secretPrompt) askForValue() (string, error) {
	stat, _ := os.Stdin.Stat()
	if (stat.Mode() & os.ModeCharDevice) == 0 {
		bytes, err := io.ReadAll(os.Stdin)
		return string(bytes), err
	} else {
		return prompt.ReadSecretStringFromUser("Enter secret value")
	}
}

type testPrompt struct{ value string }

func (me testPrompt) askForValue() (string, error) {
	return me.value, nil
}

func storeEnvVar(client context.ContextInterface, prompt storeEnvVarPrompt, contextName, varName string) error {
	context, err := client.ContextByName(contextName)
	if err != nil {
		return err
	}

	secretValue, err := prompt.askForValue()
	if err != nil {
		return errors.Wrap(err, "Failed to read secret value from stdin")
	}

	err = client.CreateEnvironmentVariable(context.ID, varName, secretValue)
	if err != nil {
		fmt.Printf("Saved environment variable %s in context %s.\n", varName, contextName)
	}
	return err
}

func deleteContext(client context.ContextInterface, orgName string, force bool, contextName string) error {
	context, err := client.ContextByName(contextName)
	if err != nil {
		return err
	}

	shouldDelete := force || prompt.AskUserToConfirm(fmt.Sprintf("Are you sure that you want to delete this context: %s %s (y/n)?", orgName, context.Name))
	if !shouldDelete {
		fmt.Printf("Cancelling context deletion")
		return nil
	}

	err = client.DeleteContext(context.ID)
	if err == nil {
		fmt.Printf("Deleted context %s.\n", contextName)
	}
	return err
}
