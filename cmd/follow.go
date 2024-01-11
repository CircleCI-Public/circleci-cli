package cmd

import (
	"fmt"

	"github.com/CircleCI-Public/circleci-cli/api"
	"github.com/CircleCI-Public/circleci-cli/git"
	"github.com/CircleCI-Public/circleci-cli/settings"
	"github.com/CircleCI-Public/circleci-cli/telemetry"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

type options struct {
	cfg *settings.Config
}

// followProject gets the remote data and attempts to follow its git project
func followProject(opts options) error {

	remote, err := git.InferProjectFromGitRemotes()
	if err != nil {
		return errors.Wrap(err, errorMessage)
	}

	//check that project url contains github or bitbucket; our legacy vcs
	if remote.VcsType == git.GitHub || remote.VcsType == git.Bitbucket {
		vcsShort := "gh"
		if remote.VcsType == git.Bitbucket {
			vcsShort = "bb"
		}
		res, err := api.FollowProject(*opts.cfg, vcsShort, remote.Organization, remote.Project)
		if err != nil {
			return err
		}
		if res.Followed {
			fmt.Println("Project successfully followed!")
		} else if res.Message == "Project not found" {
			fmt.Println("Unable to determine project slug for CircleCI (slug is case sensitive).")
		}

	} else {
		//if not warn user their vcs is not supported
		return errors.New(errorMessage)
	}
	return nil
}

// followProjectCommand follow cobra command creation
func followProjectCommand(config *settings.Config) *cobra.Command {
	opts := options{
		cfg: config,
	}
	followCommand := &cobra.Command{
		Use:   "follow",
		Short: "Attempt to follow the project for the current git repository.\nThis command is intended to be run from a git repository with a remote named 'origin' that is hosted on Github or Bitbucket only. NOTE: this command is deprecated and is not reliable on Github projects created after September 2023",
		RunE: func(cmd *cobra.Command, _ []string) error {
			err := followProject(opts)

			telemetryClient, ok := telemetry.FromContext(cmd.Context())
			if ok {
				_ = telemetryClient.Track(telemetry.CreateFollowEvent(err))
			}

			return err
		},
		Deprecated: "This command is deprecated and is not reliable on Github projects created after September 2023",
	}
	return followCommand
}
