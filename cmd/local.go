package cmd

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
	"strings"
	"unicode/utf8"

	"github.com/CircleCI-Public/circleci-cli/version"
	"github.com/manifoldco/promptui"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var testing = false

func newLocalCommand() *cobra.Command {
	localCommand := &cobra.Command{
		Use:   "local",
		Short: "Operate on your local CircleCI CLI",
	}

	setupCommand := &cobra.Command{
		Use:   "setup",
		Short: "Setup the CLI with your credentials",
		RunE:  setup,
	}

	setupCommand.Flags().BoolVar(&testing, "testing", false, "Enable test mode to bypass interactive UI.")
	if err := setupCommand.Flags().MarkHidden("testing"); err != nil {
		panic(err)
	}

	updateCommand := &cobra.Command{
		Use:   "update",
		Short: "Update the tool",
		RunE:  update,
	}

	checkCommand := &cobra.Command{
		Use:   "check",
		Short: "Check the status of your CircleCI CLI.",
		RunE:  check,
	}

	localCommand.AddCommand(setupCommand)
	localCommand.AddCommand(updateCommand)
	localCommand.AddCommand(checkCommand)

	return localCommand
}

func check(cmd *cobra.Command, args []string) error {
	endpoint := viper.GetString("endpoint")
	token := viper.GetString("token")

	Logger.Infoln("\n---\nCircleCI CLI Diagnostics\n---\n")
	Logger.Infof("Config found: %v\n", viper.ConfigFileUsed())

	Logger.Infof("GraphQL API endpoint: %s\n", endpoint)

	if token == "token" || token == "" {
		return errors.New("please set a token")
	}
	Logger.Infoln("OK, got a token.")
	Logger.Infof("Verbose mode: %v\n", viper.GetBool("verbose"))

	return nil
}

func trimFirstRune(s string) string {
	_, i := utf8.DecodeRuneInString(s)
	return s[i:]
}

func update(cmd *cobra.Command, args []string) error {

	url := "https://api.github.com/repos/CircleCI-Public/circleci-cli/releases/latest"

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return err
	}

	req.Header.Set("User-Agent", version.UserAgent())

	client := http.Client{}
	res, err := client.Do(req)
	if err != nil {
		return err
	}

	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return err
	}

	var release struct {
		// There are other fields in this response that we could use to download the
		// binaries on behalf of the user.
		// https://developer.github.com/v3/repos/releases/#get-the-latest-release
		HTML      string `json:"html_url"`
		Tag       string `json:"tag_name"`
		Published string `json:"published_at"`
	}

	if err := json.Unmarshal(body, &release); err != nil {
		return err
	}

	latest := trimFirstRune(release.Tag)

	Logger.Debug("Latest version: %s", latest)
	Logger.Debug("Published: %s", release.Published)
	Logger.Debug("Current Version: %s", version.Version)

	if latest == version.Version {
		Logger.Info("Already up-to-date.")
	} else {
		Logger.Infof("A new release is available (%s)", release.Tag)
		Logger.Infof("You are running %s", version.Version)
		Logger.Infof("You can download it from %s", release.HTML)
	}

	return nil
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
	viper.Set("endpoint", ui.readStringFromUser("CircleCI API End Point", viper.GetString("endpoint")))
	Logger.Info("API endpoint has been set.")

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
