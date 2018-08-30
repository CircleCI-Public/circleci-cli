package cmd

import (
	"fmt"
	"os/exec"
	"regexp"

	"github.com/CircleCI-Public/circleci-cli/version"
	"github.com/pkg/errors"
	"github.com/rhysd/go-github-selfupdate/selfupdate"

	"github.com/blang/semver"
	"github.com/spf13/cobra"
)

func newUpdateCommand() *cobra.Command {
	update := &cobra.Command{
		Use:   "update",
		Short: "Update the tool",
	}

	update.AddCommand(&cobra.Command{
		Use:   "check",
		Short: "Check if there are any updates available",
		RunE:  checkForUpdates,
	})

	update.AddCommand(&cobra.Command{
		Use:   "install",
		Short: "Update the tool to the latest version",
		RunE:  installUpdate,
	})

	update.AddCommand(&cobra.Command{
		Use:   "build-agent",
		Short: "Update the build agent to the latest version",
		RunE:  updateBuildAgent,
	})

	return update
}

var picardRepo = "circleci/picard"

func updateBuildAgent(cmd *cobra.Command, args []string) error {
	latestSha256, err := findLatestPicardSha()

	if err != nil {
		return err
	}

	Logger.Infof("Latest build agent is version %s", latestSha256)

	return nil
}

// Still depends on a function in cmd/build.go
func findLatestPicardSha() (string, error) {
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

func checkForUpdates(cmd *cobra.Command, args []string) error {
	return update(true)

}

func installUpdate(cmd *cobra.Command, args []string) error {
	return update(false)

}

func update(dryRun bool) error {
	updater := selfupdate.DefaultUpdater()

	slug := "CircleCI-Public/circleci-cli"

	latest, found, err := updater.DetectLatest(slug)

	if err != nil {
		return errors.Wrap(err, "error finding latest release")
	}

	if !found {
		return errors.New("no updates were found")
	}

	current := semver.MustParse(version.Version)

	Logger.Debug("Latest version: %s", latest.Version)
	Logger.Debug("Published: %s", latest.PublishedAt)
	Logger.Debug("Current Version: %s", current)

	if latest.Version.Equals(current) {
		Logger.Info("Already up-to-date.")
		return nil
	}

	if dryRun {
		Logger.Infof("A new release is available (%s)", latest.Version)
		Logger.Infof("You are running %s", current)
		Logger.Infof("You can update with `circleci update install`")
		return nil
	}

	release, err := updater.UpdateSelf(current, slug)

	if err != nil {
		return errors.Wrap(err, "failed to install update")
	}

	Logger.Infof("Updated to %s", release.Version)
	return nil
}
