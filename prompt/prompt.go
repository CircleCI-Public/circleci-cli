package prompt

import (
	"github.com/AlecAivazis/survey/v2"
	"github.com/erikgeiser/promptkit/confirmation"
	"github.com/erikgeiser/promptkit/textinput"
)

// ReadSecretStringFromUser can be used to read a value from the user by masking their input.
// It's useful for token input in our case.
func ReadSecretStringFromUser(message string) (string, error) {
	secret := ""
	input := textinput.New(message)
	input.Hidden = true
	secret, err := input.RunPrompt()
	if err != nil {
		return "", err
	}
	return secret, nil
}

// ReadStringFromUser can be used to read any value from the user or the defaultValue when provided.
func ReadStringFromUser(message string, defaultValue string) string {
	token := ""
	prompt := &survey.Input{
		Message: message,
	}

	if defaultValue != "" {
		prompt.Default = defaultValue
	}

	err := survey.AskOne(prompt, &token)
	if err != nil {
		panic(err)
	}

	return token
}

// AskUserToConfirm will prompt the user to confirm with the provided message.
func AskUserToConfirm(message string) bool {
	input := confirmation.New(message, confirmation.Yes)
	result, err := input.RunPrompt()
	return err == nil && result
}
