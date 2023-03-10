package project

import (
	projectapi "github.com/CircleCI-Public/circleci-cli/api/project"
	"github.com/CircleCI-Public/circleci-cli/cmd/validator"
	"github.com/CircleCI-Public/circleci-cli/prompt"

	"github.com/CircleCI-Public/circleci-cli/settings"
	"github.com/spf13/cobra"
)

// UserInputReader displays a message and reads a user input value
type UserInputReader interface {
	ReadSecretString(msg string) (string, error)
	AskConfirm(msg string) bool
}

type projectOpts struct {
	client projectapi.ProjectClient
	reader UserInputReader
}

// ProjectOption configures a command created by NewProjectCommand
type ProjectOption interface {
	apply(*projectOpts)
}

type promptReader struct{}

func (p promptReader) ReadSecretString(msg string) (string, error) {
	return prompt.ReadSecretStringFromUser(msg)
}

func (p promptReader) AskConfirm(msg string) bool {
	return prompt.AskUserToConfirm(msg)
}

// NewProjectCommand generates a cobra command for managing projects
func NewProjectCommand(config *settings.Config, preRunE validator.Validator, opts ...ProjectOption) *cobra.Command {
	pos := projectOpts{
		reader: &promptReader{},
	}
	for _, o := range opts {
		o.apply(&pos)
	}
	command := &cobra.Command{
		Use:   "project",
		Short: "Operate on projects",
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			client, err := projectapi.NewProjectRestClient(*config)
			if err != nil {
				return err
			}
			pos.client = client
			return nil
		},
	}

	command.AddCommand(newProjectEnvironmentVariableCommand(&pos, preRunE))

	return command
}

type customReaderProjectOption struct {
	r UserInputReader
}

func (c customReaderProjectOption) apply(opts *projectOpts) {
	opts.reader = c.r
}

// CustomReader returns a ProjectOption that sets a given UserInputReader to a project command
func CustomReader(r UserInputReader) ProjectOption {
	return customReaderProjectOption{r}
}
