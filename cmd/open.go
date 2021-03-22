package cmd

import (
	"github.com/CircleCI-Public/circleci-cli/git"
	"github.com/CircleCI-Public/circleci-cli/paths"
	"github.com/pkg/browser"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

var errorMessage = `
Unable detect which URL should be opened. This command is intended to be run from
a git repository with a remote named 'origin' that is hosted on Github or Bitbucket
Error`

func openProjectInBrowser() error {

	remote, err := git.InferProjectFromGitRemotes()

	if err != nil {
		return errors.Wrap(err, errorMessage)
	}

	return browser.OpenURL(paths.ProjectUrl(remote))
}

func newOpenCommand() *cobra.Command {

	openCommand := &cobra.Command{
		Use:   "open",
		Short: "Open the current project in the browser.",
		RunE: func(_ *cobra.Command, _ []string) error {
			return openProjectInBrowser()
		},
	}
	return openCommand
}
