package cmd

import (
	"fmt"

	"github.com/CircleCI-Public/circleci-cli/api"
	"github.com/CircleCI-Public/circleci-cli/git"
	"github.com/CircleCI-Public/circleci-cli/settings"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

type options struct {
	cfg *settings.Config
}

func followProject(opts options) error {

	remote, err := git.InferProjectFromGitRemotes()

	if err != nil {
		return errors.Wrap(err, errorMessage)
	}

	vcsShort := "gh"
	if remote.VcsType == "BITBUCKET" {
		vcsShort = "bb"
	}
	res, err := api.FollowProject(opts.cfg.Host, vcsShort, remote.Organization, remote.Project, opts.cfg.Token)
	if err != nil {
		return err
	}
	if res.Followed {
		fmt.Println("Project successfully followed!")
	} else if res.Message == "Project not found" {
		fmt.Println("Unable to determine project slug for CircleCI (slug is case sensitive).")
	}

	return nil
}

func followProjectCommand(config *settings.Config) *cobra.Command {
	opts := options{
		cfg: config,
	}
	followCommand := &cobra.Command{
		Use:   "follow",
		Short: "Attempt to follow the project for the current git repository.",
		RunE: func(_ *cobra.Command, _ []string) error {
			return followProject(opts)
		},
	}
	return followCommand
}
