package ui

import (
	"fmt"
	"strings"

	"github.com/CircleCI-Public/circleci-cli/logger"
	"github.com/manifoldco/promptui"
)

// We can't properly run integration tests on code that calls PromptUI.
// This is because the first call to PromptUI reads from stdin correctly,
// but subsequent calls return EOF.
// The `UserInterface` is created here to allow us to pass a mock user
// interface for testing.
type UserInterface interface {
	ReadSecretStringFromUser(log *logger.Logger, message string) (string, error)
	ReadStringFromUser(log *logger.Logger, message string, defaultValue string) string
	AskUserToConfirm(log *logger.Logger, message string) bool
}

type InteractiveUI struct {
}

func (InteractiveUI) ReadSecretStringFromUser(_ *logger.Logger, message string) (string, error) {
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

func (InteractiveUI) ReadStringFromUser(_ *logger.Logger, message string, defaultValue string) string {
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

func (InteractiveUI) AskUserToConfirm(_ *logger.Logger, message string) bool {
	prompt := promptui.Prompt{
		Label:     message,
		IsConfirm: true,
	}

	result, err := prompt.Run()
	return err == nil && strings.ToLower(result) == "y"
}

type TestingUI struct {
	Input   string
	Confirm bool
}

func (ui TestingUI) ReadSecretStringFromUser(log *logger.Logger, message string) (string, error) {
	log.Info(message)
	return ui.Input, nil
}

func (ui TestingUI) ReadStringFromUser(log *logger.Logger, message string, defaultValue string) string {
	log.Info(message)
	return ui.Input
}

func (ui TestingUI) AskUserToConfirm(log *logger.Logger, message string) bool {
	log.Info(message)
	return ui.Confirm
}

func ShouldAskForToken(token string, log *logger.Logger, ui UserInterface) bool {
	if token == "" {
		return true
	}

	return ui.AskUserToConfirm(log, "A CircleCI token is already set. Do you want to change it")
}

func ShouldAskForEndpoint(endpoint string, log *logger.Logger, ui UserInterface, defaultValue string) bool {
	if endpoint == defaultValue {
		return true
	}

	return ui.AskUserToConfirm(log, fmt.Sprintf("Do you want to reset the endpoint? (default: %s)", defaultValue))
}
