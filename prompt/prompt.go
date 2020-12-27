package prompt

import "github.com/AlecAivazis/survey/v2"

// ReadSecretStringFromUser can be used to read a value from the user by masking their input.
// It's useful for token input in our case.
func ReadSecretStringFromUser(message string) (string, error) {
	secret := ""
	prompt := &survey.Password{
		Message: message,
	}
	err := survey.AskOne(prompt, &secret)
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
	result := true
	prompt := &survey.Confirm{
		Message: message,
	}

	err := survey.AskOne(prompt, &result)
	return err == nil && result
}
