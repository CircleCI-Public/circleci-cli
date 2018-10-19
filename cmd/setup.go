package cmd

import (
	"fmt"
	"strings"

	"github.com/CircleCI-Public/circleci-cli/client"
	"github.com/CircleCI-Public/circleci-cli/logger"
	"github.com/CircleCI-Public/circleci-cli/settings"
	"github.com/manifoldco/promptui"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

var testing = false

type setupOptions struct {
	cfg  *settings.Config
	cl   *client.Client
	log  *logger.Logger
	args []string
}

func newSetupCommand(config *settings.Config) *cobra.Command {
	opts := setupOptions{
		cfg: config,
	}

	setupCommand := &cobra.Command{
		Use:   "setup",
		Short: "Setup the CLI with your credentials",
		PreRun: func(cmd *cobra.Command, args []string) {
			opts.args = args
			opts.log = logger.NewLogger(config.Debug)
			opts.cl = client.NewClient(config.Host, config.Endpoint, config.Token)
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
	readSecretStringFromUser(log *logger.Logger, message string) (string, error)
	readStringFromUser(log *logger.Logger, message string, defaultValue string) string
	askUserToConfirm(log *logger.Logger, message string) bool
}

type interactiveUI struct {
}

func (interactiveUI) readSecretStringFromUser(_ *logger.Logger, message string) (string, error) {
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

func (interactiveUI) readStringFromUser(_ *logger.Logger, message string, defaultValue string) string {
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

func (interactiveUI) askUserToConfirm(_ *logger.Logger, message string) bool {
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

func (ui testingUI) readSecretStringFromUser(log *logger.Logger, message string) (string, error) {
	log.Info(message)
	return ui.input, nil
}

func (ui testingUI) readStringFromUser(log *logger.Logger, message string, defaultValue string) string {
	log.Info(message)
	return ui.input
}

func (ui testingUI) askUserToConfirm(log *logger.Logger, message string) bool {
	log.Info(message)
	return ui.confirm
}

func shouldAskForToken(token string, log *logger.Logger, ui userInterface) bool {
	if token == "" {
		return true
	}

	return ui.askUserToConfirm(log, "A CircleCI token is already set. Do you want to change it")
}

func shouldAskForEndpoint(endpoint string, log *logger.Logger, ui userInterface) bool {
	if endpoint == defaultEndpoint {
		return true
	}

	return ui.askUserToConfirm(log, fmt.Sprintf("Do you want to reset the endpoint? (default: %s)", defaultEndpoint))
}

func setup(opts setupOptions) error {
	var ui userInterface = interactiveUI{}

	if testing {
		ui = testingUI{
			confirm: true,
			input:   "boondoggle",
		}
	}

	if shouldAskForToken(opts.cfg.Token, opts.log, ui) {
		token, err := ui.readSecretStringFromUser(opts.log, "CircleCI API Token")
		if err != nil {
			return errors.Wrap(err, "Error reading token")
		}
		opts.cfg.Token = token
		opts.log.Info("API token has been set.")
	}
	opts.cfg.Host = ui.readStringFromUser(opts.log, "CircleCI Host", defaultHost)
	opts.log.Info("CircleCI host has been set.")

	// Reset endpoint to default when running setup
	// This ensures any accidental changes to this field can be fixed simply by rerunning this command.
	if shouldAskForEndpoint(opts.cfg.Endpoint, opts.log, ui) {
		opts.cfg.Endpoint = defaultEndpoint
	}

	if err := opts.cfg.WriteToDisk(); err != nil {
		return errors.Wrap(err, "Failed to save config file")
	}

	opts.log.Info("Setup complete. Your configuration has been saved.")
	return nil
}
