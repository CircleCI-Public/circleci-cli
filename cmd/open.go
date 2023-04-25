package cmd

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/CircleCI-Public/circleci-cli/git"
	"github.com/pkg/browser"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

// errorMessage string containing the error message displayed in both the open command and the follow command
var errorMessage = `
This command is intended to be run from a git repository with a remote named 'origin' that is hosted on Github or Bitbucket only. 
We are not currently supporting any other hosts.`

// projectUrl uses the provided values to create the url to open
func projectUrl(remote *git.Remote) string {
	return fmt.Sprintf("https://app.circleci.com/pipelines/%s/%s/%s",
		url.PathEscape(strings.ToLower(string(remote.VcsType))),
		url.PathEscape(remote.Organization),
		url.PathEscape(remote.Project))
}

// openProjectInBrowser takes the created url and opens a browser to it
func openProjectInBrowser() error {
	remote, err := git.InferProjectFromGitRemotes()
	if err != nil {
		return errors.Wrap(err, errorMessage)
	}
	//check that project url contains github or bitbucket; our legacy vcs
	if remote.VcsType == git.GitHub || remote.VcsType == git.Bitbucket {
		return browser.OpenURL(projectUrl(remote))
	}
	//if not warn user their vcs is not supported
	return errors.New(errorMessage)
}

// newOpenCommand creates the cli command open
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
