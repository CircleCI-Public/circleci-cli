package cmd

import (
	"fmt"
	"strings"

	"github.com/CircleCI-Public/circleci-cli/api"
	"github.com/CircleCI-Public/circleci-cli/client"
	"github.com/CircleCI-Public/circleci-cli/settings"
	"github.com/manifoldco/promptui"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

type setupOptions struct {
	cfg      *settings.Config
	cl       *client.Client
	noPrompt bool
	// Add host and token for use with --no-prompt
	host  string
	token string
	args  []string
	// This lets us pass in our own interface for testing
	tty setupUserInterface
	// Linked with --integration-testing flag for stubbing UI in gexec tests
	integrationTesting bool
}

// setupUserInterface is created to allow us to pass a mock user interface for testing.
// Since we can't properly run integration tests on code that calls PromptUI.
// This is because the first call to PromptUI reads from stdin correctly,
// but subsequent calls return EOF.
type setupUserInterface interface {
	readSecretStringFromUser(message string) (string, error)
	readTokenFromUser(message string) (string, error)

	readStringFromUser(message string, defaultValue string) string
	readHostFromUser(message string, defaultValue string) string

	askUserToConfirm(message string) bool
	askUserToConfirmEndpoint(message string) bool
	askUserToConfirmToken(message string) bool
}

// setupUI implements the setupUserInterface used by the real program, not in tests.
type setupInteractiveUI struct{}

// readSecretStringFromUser can be used to read a value from the user by masking their input.
// It's useful for token input in our case.
func (setupInteractiveUI) readSecretStringFromUser(message string) (string, error) {
	prompt := promptui.Prompt{
		Label: message,
		Mask:  '*',
	}

	secret, err := prompt.Run()

	if err != nil {
		return "", err
	}

	return secret, nil
}

// readStringFromUser can be used to read any value from the user or the defaultValue when provided.
func (setupInteractiveUI) readStringFromUser(message string, defaultValue string) string {
	prompt := promptui.Prompt{
		Label: message,
	}

	if defaultValue != "" {
		prompt.Default = defaultValue
	}

	token, err := prompt.Run()

	if err != nil {
		panic(err)
	}

	return token
}

// readHostFromUser implements the setupInteractiveUI interface for asking a user's input.
func (ui setupInteractiveUI) readHostFromUser(message string, defaultValue string) string {
	return ui.readStringFromUser(message, defaultValue)
}

// readTokenFromUser implements the setupInteractiveUI interface for asking a user's token.
func (ui setupInteractiveUI) readTokenFromUser(message string) (string, error) {
	return ui.readSecretStringFromUser(message)
}

// askUserToConfirm will prompt the user to confirm with the provided message.
func (setupInteractiveUI) askUserToConfirm(message string) bool {
	prompt := promptui.Prompt{
		Label:     message,
		IsConfirm: true,
	}

	result, err := prompt.Run()
	return err == nil && strings.ToLower(result) == "y"
}

func (ui setupInteractiveUI) askUserToConfirmEndpoint(message string) bool {
	return ui.askUserToConfirm(message)
}

func (ui setupInteractiveUI) askUserToConfirmToken(message string) bool {
	return ui.askUserToConfirm(message)
}

// setupTestUI implements the setupUserInterface for our testing purposes.
type setupTestUI struct {
	host            string
	token           string
	confirmEndpoint bool
	confirmToken    bool
}

func (setupTestUI) readStringFromUser(message string, defaultValue string) string {
	return ""
}

func (setupTestUI) readSecretStringFromUser(message string) (string, error) {
	return "", nil
}

// readHostFromUser implements the setupTestUI interface for asking a user's input.
// It works by simply printing the message to standard output and passing the input through.
func (ui setupTestUI) readHostFromUser(message string, defaultValue string) string {
	fmt.Println(message)
	return ui.host
}

// readTokenFromUser implements the setupTestUI interface for asking a user's token.
// It works by simply printing the message to standard output and passing the token through.
func (ui setupTestUI) readTokenFromUser(message string) (string, error) {
	fmt.Println(message)
	return ui.token, nil
}

// askUserToConfirm implements the setupTestUI interface for returning the confirm prompt.
func (ui setupTestUI) askUserToConfirm(message string) bool {
	return true
}

// askUserToConfirmEndpoint works by printing the provided message to standard out and returning a Confirm dialogue up the chain.
func (ui setupTestUI) askUserToConfirmEndpoint(message string) bool {
	fmt.Println(message)
	return ui.confirmEndpoint
}

// askUserToConfirmToken works by printing the provided message to standard out and returning a Confirm dialogue up the chain.
func (ui setupTestUI) askUserToConfirmToken(message string) bool {
	fmt.Println(message)
	return ui.confirmToken
}

// shouldAskForToken wraps an askUserToConfirm dialogue only if their token is empty or blank.
func shouldAskForToken(token string, ui setupUserInterface) bool {
	if token == "" {
		return true
	}

	return ui.askUserToConfirmToken("A CircleCI token is already set. Do you want to change it")
}

// shouldAskForEndpoint wraps an askUserToConfirm dialogue only if their endpoint has changed from the default value.
func shouldAskForEndpoint(endpoint string, ui setupUserInterface, defaultValue string) bool {
	if endpoint == defaultValue {
		return true
	}

	return ui.askUserToConfirmEndpoint(fmt.Sprintf("Do you want to reset the endpoint? (default: %s)", defaultValue))
}

func newSetupCommand(config *settings.Config) *cobra.Command {
	opts := setupOptions{
		cfg: config,
		tty: setupInteractiveUI{},
	}

	setupCommand := &cobra.Command{
		Use:   "setup",
		Short: "Setup the CLI with your credentials",
		PreRun: func(cmd *cobra.Command, args []string) {
			opts.args = args
		},
		RunE: func(_ *cobra.Command, _ []string) error {
			if opts.integrationTesting {
				opts.tty = setupTestUI{
					host:            "boondoggle",
					token:           "boondoggle",
					confirmEndpoint: true,
					confirmToken:    true,
				}
			}

			return setup(opts)
		},
	}

	setupCommand.Flags().BoolVar(&opts.integrationTesting, "integration-testing", false, "Enable test mode to bypass interactive UI.")
	if err := setupCommand.Flags().MarkHidden("integration-testing"); err != nil {
		panic(err)
	}

	setupCommand.Flags().BoolVar(&opts.noPrompt, "no-prompt", false, "Disable prompt to bypass interactive UI. (MUST supply --host and --token)")

	setupCommand.Flags().StringVar(&opts.host, "host", "", "URL to your CircleCI host")
	if err := setupCommand.Flags().MarkHidden("host"); err != nil {
		panic(err)
	}

	setupCommand.Flags().StringVar(&opts.token, "token", "", "your token for using CircleCI")
	if err := setupCommand.Flags().MarkHidden("token"); err != nil {
		panic(err)
	}

	return setupCommand
}

func setup(opts setupOptions) error {
	if opts.noPrompt {
		return setupNoPrompt(opts)
	}

	if shouldAskForToken(opts.cfg.Token, opts.tty) {
		token, err := opts.tty.readTokenFromUser("CircleCI API Token")
		if err != nil {
			return errors.Wrap(err, "Error reading token")
		}
		opts.cfg.Token = token
		fmt.Println("API token has been set.")
	}
	opts.cfg.Host = opts.tty.readHostFromUser("CircleCI Host", defaultHost)
	fmt.Println("CircleCI host has been set.")

	// Reset endpoint to default when running setup
	// This ensures any accidental changes to this field can be fixed simply by rerunning this command.
	if shouldAskForEndpoint(opts.cfg.Endpoint, opts.tty, defaultEndpoint) {
		opts.cfg.Endpoint = defaultEndpoint
	}

	if err := opts.cfg.WriteToDisk(); err != nil {
		return errors.Wrap(err, "Failed to save config file")
	}

	fmt.Printf("Setup complete.\nYour configuration has been saved to %s.\n", opts.cfg.FileUsed)

	if !opts.integrationTesting {
		// Reset client after setup config
		opts.cl = client.NewClient(opts.cfg.Host, opts.cfg.Endpoint, opts.cfg.Token, opts.cfg.Debug)

		fmt.Printf("\n")
		fmt.Printf("Trying an introspection query on API to verify your setup... ")

		responseIntro, err := api.IntrospectionQuery(opts.cl)
		if responseIntro.Schema.QueryType.Name == "" {
			fmt.Println("\nUnable to make a query against the GraphQL API, please check your settings")
		} else {
			fmt.Println("Ok.")
		}

		fmt.Printf("Trying to query your username given the provided token... ")
		responseWho, err := api.WhoamiQuery(opts.cl)

		if err != nil {
			return err
		}

		if responseWho.Me.Name != "" {
			fmt.Printf("Hello, %s.\n", responseWho.Me.Name)
		}
	}

	return nil
}

func shouldKeepExistingConfig(opts setupOptions) bool {
	// Host will always be set, since it has a default value of circleci.com
	// We assume by an empty token there is no existing config.
	if opts.cfg.Token == "" {
		return false
	}

	// If they pass either host or token with a value this will be false, overwriting their existing config
	return opts.host == "" && opts.token == ""
}

func setupNoPrompt(opts setupOptions) error {
	if shouldKeepExistingConfig(opts) {
		fmt.Printf("Setup has kept your existing configuration at %s.\n", opts.cfg.FileUsed)
		return nil
	}

	// Throw an error if both flags are blank are blank!
	if opts.host == "" && opts.token == "" {
		return errors.New("No existing host or token saved.\nThe proper format is `circleci setup --host HOST --token TOKEN --no-prompt")
	}

	config := settings.Config{}

	// First calling load will ensure the new config can be saved to disk
	if err := config.LoadFromDisk(); err != nil {
		return errors.Wrap(err, "Failed to create config file on disk")
	}

	// Use the default endpoint since we don't expose that to users
	config.Endpoint = defaultEndpoint
	config.Host = opts.host   // Set new host to flag
	config.Token = opts.token // Set new token to flag

	// Reset their host if the flag was blank
	if opts.host == "" {
		fmt.Println("Host unchanged from existing config. Use --host with --no-prompt to overwrite it.")
		config.Host = opts.cfg.Host
	}

	// Reset their token if the flag was blank
	if opts.token == "" {
		fmt.Println("Token unchanged from existing config. Use --token with --no-prompt to overwrite it.")
		config.Token = opts.cfg.Token
	}

	// Then save the new config to disk
	if err := config.WriteToDisk(); err != nil {
		return errors.Wrap(err, "Failed to save config file")
	}

	fmt.Printf("Setup complete.\nYour configuration has been saved to %s.\n", config.FileUsed)
	return nil
}
