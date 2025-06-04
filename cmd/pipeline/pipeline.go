package pipeline

import (
	"github.com/spf13/cobra"

	pipelineapi "github.com/CircleCI-Public/circleci-cli/api/pipeline"
	"github.com/CircleCI-Public/circleci-cli/cmd/validator"
	"github.com/CircleCI-Public/circleci-cli/prompt"
	"github.com/CircleCI-Public/circleci-cli/settings"
)

// UserInputReader displays a message and reads a user input value
type UserInputReader interface {
	ReadStringFromUser(msg string) string
	AskConfirm(msg string) bool
}

type pipelineOpts struct {
	pipelineClient pipelineapi.PipelineClient
	reader         UserInputReader
}

// PipelineOption configures a command created by NewPipelineCommand
type PipelineOption interface {
	apply(*pipelineOpts)
}

type promptReader struct{}

func (p promptReader) ReadStringFromUser(msg string) string {
	return prompt.ReadStringFromUser(msg, "")
}

func (p promptReader) AskConfirm(msg string) bool {
	return prompt.AskUserToConfirm(msg)
}

// NewPipelineCommand generates a cobra command for managing pipelines
func NewPipelineCommand(config *settings.Config, preRunE validator.Validator, opts ...PipelineOption) *cobra.Command {
	pos := pipelineOpts{
		reader: &promptReader{},
	}
	for _, o := range opts {
		o.apply(&pos)
	}
	command := &cobra.Command{
		Use:   "pipeline",
		Short: "Operate on pipelines",
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			client, err := pipelineapi.NewPipelineRestClient(*config)
			if err != nil {
				return err
			}
			pos.pipelineClient = client
			return nil
		},
	}

	command.AddCommand(newCreateCommand(&pos, preRunE))
	command.AddCommand(newListCommand(&pos, preRunE))
	command.AddCommand(newConfigTestRunCommand(&pos, preRunE))

	return command
}

type customReaderPipelineOption struct {
	r UserInputReader
}

func (c customReaderPipelineOption) apply(opts *pipelineOpts) {
	opts.reader = c.r
}

// CustomReader returns a PipelineOption that sets a given UserInputReader to a pipeline command
func CustomReader(r UserInputReader) PipelineOption {
	return customReaderPipelineOption{r}
}
