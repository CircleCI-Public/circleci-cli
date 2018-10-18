package cmd

import (
	"fmt"
	"net/http"
	"os/exec"
	"regexp"

	"github.com/CircleCI-Public/circleci-cli/logger"
	"github.com/CircleCI-Public/circleci-cli/settings"
	"github.com/CircleCI-Public/circleci-cli/version"
	"github.com/google/go-github/github"
	"github.com/pkg/errors"
	"github.com/rhysd/go-github-selfupdate/selfupdate"

	"github.com/blang/semver"
	"github.com/spf13/cobra"
)

type updateOptions struct {
	cfg  *settings.Config
	log  *logger.Logger
	args []string
}

func newUpdateCommand(config *settings.Config) *cobra.Command {
	opts := updateOptions{
		cfg: config,
	}

	update := &cobra.Command{
		Use:   "update",
		Short: "Update the tool",
	}

	update.AddCommand(&cobra.Command{
		Use:   "check",
		Short: "Check if there are any updates available",
		PreRun: func(cmd *cobra.Command, args []string) {
			opts.args = args
			opts.log = logger.NewLogger(config.Debug)
		},
		RunE: func(_ *cobra.Command, _ []string) error {
			return checkForUpdates(opts)
		},
	})

	update.AddCommand(&cobra.Command{
		Use:   "install",
		Short: "Update the tool to the latest version",
		PreRun: func(cmd *cobra.Command, args []string) {
			opts.args = args
			opts.log = logger.NewLogger(config.Debug)
		},
		RunE: func(_ *cobra.Command, _ []string) error {
			return installUpdate(opts)
		},
	})

	update.AddCommand(&cobra.Command{
		Use:   "build-agent",
		Short: "Update the build agent to the latest version",
		PreRun: func(cmd *cobra.Command, args []string) {
			opts.args = args
			opts.log = logger.NewLogger(config.Debug)
		},
		RunE: func(_ *cobra.Command, _ []string) error {
			return updateBuildAgent(opts)
		},
	})

	return update
}

var picardRepo = "circleci/picard"

func updateBuildAgent(opts updateOptions) error {
	latestSha256, err := findLatestPicardSha()

	if err != nil {
		return err
	}

	opts.log.Infof("Latest build agent is version %s", latestSha256)

	return nil
}

// Still depends on a function in cmd/build.go
func findLatestPicardSha() (string, error) {

	if err := ensureDockerIsAvailable(); err != nil {
		return "", err
	}

	outputBytes, err := exec.Command("docker", "pull", picardRepo).CombinedOutput() // #nosec

	if err != nil {
		return "", errors.Wrap(err, "failed to pull latest docker image")
	}

	output := string(outputBytes)
	sha256 := regexp.MustCompile("(?m)sha256.*$")
	latest := sha256.FindString(output)

	if latest == "" {
		return "", fmt.Errorf("failed to parse sha256 from docker pull output")
	}

	// This function still lives in cmd/build.go
	err = storeBuildAgentSha(latest)

	if err != nil {
		return "", err
	}

	return latest, nil
}

func checkForUpdates(opts updateOptions) error {
	return update(opts, true)

}

func installUpdate(opts updateOptions) error {
	return update(opts, false)

}

func update(opts updateOptions, dryRun bool) error {
	updater := selfupdate.DefaultUpdater()

	slug := "CircleCI-Public/circleci-cli"

	latest, found, err := updater.DetectLatest(slug)

	if err != nil {
		if errResponse, ok := err.(*github.ErrorResponse); ok && errResponse.Response.StatusCode == http.StatusUnauthorized {
			return errors.Wrap(err, "Your Github token is invalid. Check the [github] section in ~/.gitconfig\n")
		}

		return errors.Wrap(err, "error finding latest release")
	}

	if !found {
		return errors.New("no updates were found")
	}

	current := semver.MustParse(version.Version)

	opts.log.Debug("Latest version: %s", latest.Version)
	opts.log.Debug("Published: %s", latest.PublishedAt)
	opts.log.Debug("Current Version: %s", current)

	if latest.Version.Equals(current) {
		opts.log.Info("Already up-to-date.")
		return nil
	}

	if dryRun {
		opts.log.Infof("A new release is available (%s)", latest.Version)
		opts.log.Infof("You are running %s", current)
		opts.log.Infof("You can update with `circleci update install`")
		return nil
	}

	release, err := updater.UpdateSelf(current, slug)

	if err != nil {
		return errors.Wrap(err, "failed to install update")
	}

	opts.log.Infof("Updated to %s", release.Version)
	return nil
}
