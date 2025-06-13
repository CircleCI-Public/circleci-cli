package prompt

import (
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
// An optional validator function can be provided to validate the input.
func ReadStringFromUser(message string, defaultValue string, validator ...func(string) error) string {
	input := textinput.New(message)
	input.Placeholder = defaultValue
	input.InitialValue = defaultValue
	input.Validate = func(s string) error { return nil }

	// If a validator is provided, set it on the input
	if len(validator) > 0 && validator[0] != nil {
		input.Validate = validator[0]
	}

	result, err := input.RunPrompt()
	if err != nil {
		panic(err)
	}
	return result
}

// AskUserToConfirm will prompt the user to confirm with the provided message.
func AskUserToConfirm(message string) bool {
	input := confirmation.New(message, confirmation.No)
	result, err := input.RunPrompt()
	return err == nil && result
}

func AskUserToConfirmWithDefault(message string, defaultValue bool) bool {
	def := confirmation.No
	if defaultValue {
		def = confirmation.Yes
	}

	input := confirmation.New(message, def)
	result, err := input.RunPrompt()
	return err == nil && result
}
