package trigger

import (
	"github.com/spf13/cobra"

	"github.com/CircleCI-Public/circleci-cli/api/pipeline"
	triggerapi "github.com/CircleCI-Public/circleci-cli/api/trigger"
	"github.com/CircleCI-Public/circleci-cli/cmd/validator"
	"github.com/CircleCI-Public/circleci-cli/prompt"
	"github.com/CircleCI-Public/circleci-cli/settings"
)

// UserInputReader displays a message and reads a user input value
type UserInputReader interface {
	ReadStringFromUser(msg string) string
	AskConfirm(msg string) bool
}

type triggerOpts struct {
	triggerClient  triggerapi.TriggerClient
	pipelineClient pipeline.PipelineClient
	reader         UserInputReader
}

// TriggerOption configures a command created by NewTriggerCommand
type TriggerOption interface {
	apply(*triggerOpts)
}

type promptReader struct{}

func (p promptReader) ReadStringFromUser(msg string) string {
	return prompt.ReadStringFromUser(msg, "")
}

func (p promptReader) AskConfirm(msg string) bool {
	return prompt.AskUserToConfirm(msg)
}

// NewTriggerCommand generates a cobra command for managing triggers
func NewTriggerCommand(config *settings.Config, preRunE validator.Validator, opts ...TriggerOption) *cobra.Command {
	pos := triggerOpts{
		reader: &promptReader{},
	}
	for _, o := range opts {
		o.apply(&pos)
	}
	command := &cobra.Command{
		Use:   "trigger",
		Short: "Operate on triggers",
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			triggerClient, err := triggerapi.NewTriggerRestClient(*config)
			if err != nil {
				return err
			}
			pos.triggerClient = triggerClient

			pipelineClient, err := pipeline.NewPipelineRestClient(*config)
			if err != nil {
				return err
			}
			pos.pipelineClient = pipelineClient

			return nil
		},
	}

	command.AddCommand(newCreateCommand(&pos, preRunE))

	return command
}

type customReaderTriggerOption struct {
	r UserInputReader
}

func (c customReaderTriggerOption) apply(opts *triggerOpts) {
	opts.reader = c.r
}

// CustomReader returns a TriggerOption that sets a given UserInputReader to a trigger command
func CustomReader(r UserInputReader) TriggerOption {
	return customReaderTriggerOption{r}
}
