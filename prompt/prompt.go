package prompt

import (
	"strings"

	"github.com/manifoldco/promptui"
)

// ReadSecretStringFromUser can be used to read a value from the user by masking their input.
// It's useful for token input in our case.
func ReadSecretStringFromUser(message string) (string, error) {
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

// ReadStringFromUser can be used to read any value from the user or the defaultValue when provided.
func ReadStringFromUser(message string, defaultValue string) string {
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

// AskUserToConfirm will prompt the user to confirm with the provided message.
func AskUserToConfirm(message string) bool {
	prompt := promptui.Prompt{
		Label:     message,
		IsConfirm: true,
	}

	result, err := prompt.Run()
	return err == nil && strings.ToLower(result) == "y"
}
