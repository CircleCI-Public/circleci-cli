// Package deploy implements the `circleci deploy` command group,
// including the `init` subcommand that wires deploy markers into
// an existing .circleci/config.yml with zero manual editing.
package deploy

import (
	"github.com/spf13/cobra"

	"github.com/CircleCI-Public/circleci-cli/prompt"
	"github.com/CircleCI-Public/circleci-cli/settings"
)

// UserInputReader displays a message and reads a user input value.
// It mirrors the pattern used by other command groups so tests can
// supply canned answers without a real TTY.
type UserInputReader interface {
	ReadStringFromUser(msg string, defaultValue string) string
}

type deployOpts struct {
	reader UserInputReader
}

// Option configures a command created by NewDeployCommand.
type Option interface {
	apply(*deployOpts)
}

type promptReader struct{}

func (p promptReader) ReadStringFromUser(msg string, defaultValue string) string {
	return prompt.ReadStringFromUser(msg, defaultValue)
}

// NewDeployCommand returns the top-level `circleci deploy` command group.
func NewDeployCommand(config *settings.Config, opts ...Option) *cobra.Command {
	dopts := deployOpts{
		reader: &promptReader{},
	}
	for _, o := range opts {
		o.apply(&dopts)
	}

	command := &cobra.Command{
		Use:   "deploy",
		Short: "Set up and manage CircleCI deploy markers",
	}

	command.AddCommand(newInitCommand(config, &dopts))

	return command
}

type customReaderOption struct {
	r UserInputReader
}

func (c customReaderOption) apply(opts *deployOpts) {
	opts.reader = c.r
}

// CustomReader returns an Option that replaces the default interactive
// prompt reader with the supplied implementation. Useful for tests.
func CustomReader(r UserInputReader) Option {
	return customReaderOption{r}
}
