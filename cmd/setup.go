package cmd

import (
	"strings"

	"github.com/manifoldco/promptui"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var testing = false

func newSetupCommand() *cobra.Command {
	setupCommand := &cobra.Command{
		Use:   "setup",
		Short: "Setup the CLI with your credentials",
		RunE:  setup,
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
	readStringFromUser(message string, defaultValue string) string
	askUserToConfirm(message string) bool
}

type interactiveUI struct {
}

func (interactiveUI) readStringFromUser(message string, defaultValue string) string {
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

func (interactiveUI) askUserToConfirm(message string) bool {
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

func (ui testingUI) readStringFromUser(message string, defaultValue string) string {
	Logger.Info(message)
	return ui.input
}

func (ui testingUI) askUserToConfirm(message string) bool {
	Logger.Info(message)
	return ui.confirm
}

func shouldAskForToken(token string, ui userInterface) bool {

	if token == "" {
		return true
	}

	return ui.askUserToConfirm("A CircleCI token is already set. Do you want to change it")
}

func setup(cmd *cobra.Command, args []string) error {
	token := viper.GetString("token")

	var ui userInterface = interactiveUI{}

	if testing {
		ui = testingUI{
			confirm: true,
			input:   "boondoggle",
		}
	}

	if shouldAskForToken(token, ui) {
		viper.Set("token", ui.readStringFromUser("CircleCI API Token", ""))
		Logger.Info("API token has been set.")
	}
	viper.Set("host", ui.readStringFromUser("CircleCI Host", defaultHost))
	Logger.Info("CircleCI host has been set.")

	// Marc: I can't find a way to prevent the verbose flag from
	// being written to the config file, so set it to false in
	// the config file.
	viper.Set("verbose", false)

	if err := viper.WriteConfig(); err != nil {
		return errors.Wrap(err, "Failed to save config file")
	}

	Logger.Info("Setup complete. Your configuration has been saved.")
	return nil
}
