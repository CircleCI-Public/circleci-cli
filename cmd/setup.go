package cmd

import (
	"fmt"
	"strings"

	"github.com/CircleCI-Public/circleci-cli/settings"
	"github.com/manifoldco/promptui"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

var testing = false

type setupOptions struct {
	*settings.Config
	args []string
}

func newSetupCommand(config *settings.Config) *cobra.Command {
	opts := setupOptions{
		Config: config,
	}

	setupCommand := &cobra.Command{
		Use:   "setup",
		Short: "Setup the CLI with your credentials",
		PreRun: func(cmd *cobra.Command, args []string) {
			opts.args = args

			if err := opts.Setup(); err != nil {
				panic(err)
			}
		},
		RunE: func(_ *cobra.Command, _ []string) error {
			return setup(opts)
		},
	}

	setupCommand.Flags().BoolVar(&testing, "testing", false, "Enable test mode to bypass interactive UI.")
	if err := setupCommand.Flags().MarkHidden("testing"); err != nil {
		panic(err)
	}

	return setupCommand
}

// We can't properly run integration tests on code that calls PromptUI.
// This is because the first call to PromptUI reads from stdin correctly,
// but subsequent calls return EOF.
// The `userInterface` is created here to allow us to pass a mock user
// interface for testing.
type userInterface interface {
	readSecretStringFromUser(opts setupOptions, message string) (string, error)
	readStringFromUser(opts setupOptions, message string, defaultValue string) string
	askUserToConfirm(opts setupOptions, message string) bool
}

type interactiveUI struct {
}

func (interactiveUI) readSecretStringFromUser(_ setupOptions, message string) (string, error) {
	prompt := promptui.Prompt{
		Label: message,
		Mask:  '*',
	}

	token, err := prompt.Run()

	if err != nil {
		return "", err
	}

	return token, nil
}

func (interactiveUI) readStringFromUser(_ setupOptions, message string, defaultValue string) string {
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

func (interactiveUI) askUserToConfirm(_ setupOptions, message string) bool {
	prompt := promptui.Prompt{
		Label:     message,
		IsConfirm: true,
	}

	result, err := prompt.Run()
	return err == nil && strings.ToLower(result) == "y"
}

type testingUI struct {
	input   string
	confirm bool
}

func (ui testingUI) readSecretStringFromUser(opts setupOptions, message string) (string, error) {
	opts.Logger.Info(message)
	return ui.input, nil
}

func (ui testingUI) readStringFromUser(opts setupOptions, message string, defaultValue string) string {
	opts.Logger.Info(message)
	return ui.input
}

func (ui testingUI) askUserToConfirm(opts setupOptions, message string) bool {
	opts.Logger.Info(message)
	return ui.confirm
}

func shouldAskForToken(opts setupOptions, ui userInterface) bool {
	if opts.Token == "" {
		return true
	}

	return ui.askUserToConfirm(opts, "A CircleCI token is already set. Do you want to change it")
}

func shouldAskForEndpoint(opts setupOptions, ui userInterface) bool {
	if opts.Endpoint == defaultEndpoint {
		return true
	}

	return ui.askUserToConfirm(opts, fmt.Sprintf("Do you want to reset the endpoint? (default: %s)", defaultEndpoint))
}

func setup(opts setupOptions) error {
	var ui userInterface = interactiveUI{}

	if testing {
		ui = testingUI{
			confirm: true,
			input:   "boondoggle",
		}
	}

	if shouldAskForToken(opts, ui) {
		token, err := ui.readSecretStringFromUser(opts, "CircleCI API Token")
		if err != nil {
			return errors.Wrap(err, "Error reading token")
		}
		opts.Token = token
		opts.Logger.Info("API token has been set.")
	}
	opts.Host = ui.readStringFromUser(opts, "CircleCI Host", defaultHost)
	opts.Logger.Info("CircleCI host has been set.")

	// Reset endpoint to default when running setup
	// This ensures any accidental changes to this field can be fixed simply by rerunning this command.
	if shouldAskForEndpoint(opts, ui) {
		opts.Endpoint = defaultEndpoint
	}

	if err := opts.WriteToDisk(); err != nil {
		return errors.Wrap(err, "Failed to save config file")
	}

	opts.Logger.Info("Setup complete. Your configuration has been saved.")
	return nil
}
