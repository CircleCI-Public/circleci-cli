package cmd

import (
	"fmt"
	"net/http"
	"os/exec"
	"regexp"

	"github.com/CircleCI-Public/circleci-cli/logger"
	"github.com/CircleCI-Public/circleci-cli/settings"
	"github.com/CircleCI-Public/circleci-cli/ui"
	"github.com/CircleCI-Public/circleci-cli/version"
	"github.com/google/go-github/github"
	"github.com/pkg/errors"
	"github.com/rhysd/go-github-selfupdate/selfupdate"

	"github.com/blang/semver"
	"github.com/spf13/cobra"
)

type updateOptions struct {
	cfg    *settings.Config
	log    *logger.Logger
	dryRun bool
	noTTY  bool
	args   []string
}

func newUpdateCommand(config *settings.Config) *cobra.Command {
	opts := updateOptions{
		cfg:    config,
		dryRun: false,
	}

	update := &cobra.Command{
		Use:   "update",
		Short: "Update the tool to the latest version",
		PreRun: func(cmd *cobra.Command, args []string) {
			opts.args = args
			opts.log = logger.NewLogger(config.Debug)
		},
		RunE: func(_ *cobra.Command, _ []string) error {
			return updateCLI(opts)
		},
	}

	update.AddCommand(&cobra.Command{
		Use:    "check",
		Hidden: true,
		Short:  "Check if there are any updates available",
		PreRun: func(cmd *cobra.Command, args []string) {
			opts.args = args
			opts.dryRun = true
			opts.log = logger.NewLogger(config.Debug)
		},
		RunE: func(_ *cobra.Command, _ []string) error {
			opts.noTTY = true
			return updateCLI(opts)
		},
	})

	update.AddCommand(&cobra.Command{
		Use:    "install",
		Hidden: true,
		Short:  "Update the tool to the latest version",
		PreRun: func(cmd *cobra.Command, args []string) {
			opts.args = args
			opts.log = logger.NewLogger(config.Debug)
		},
		RunE: func(_ *cobra.Command, _ []string) error {
			opts.noTTY = true
			return updateCLI(opts)
		},
	})

	update.AddCommand(&cobra.Command{
		Use:    "build-agent",
		Hidden: true,
		Short:  "Update the build agent to the latest version",
		PreRun: func(cmd *cobra.Command, args []string) {
			opts.args = args
			opts.log = logger.NewLogger(config.Debug)
		},
		RunE: func(_ *cobra.Command, _ []string) error {
			return updateBuildAgent(opts)
		},
	})

	update.PersistentFlags().BoolVar(&opts.noTTY, "no-tty", false, "Enable non-interactive mode for clients without a terminal")

	update.Flags().BoolVar(&testing, "testing", false, "Enable test mode to bypass interactive UI.")
	if err := update.Flags().MarkHidden("testing"); err != nil {
		panic(err)
	}

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

func updateCLI(opts updateOptions) error {
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

	if opts.dryRun {
		reportVersion(opts.log, current, latest.Version)
		opts.log.Infof("You can update with `circleci update install`")
		return nil
	}

	if !opts.noTTY {
		reportVersion(opts.log, current, latest.Version)
		// prompt for user to update
		var tty ui.UserInterface = ui.InteractiveUI{}

		if testing {
			tty = ui.TestingUI{
				Confirm: true,
			}
		}

		if tty.AskUserToConfirm(opts.log, "Would you like to update") {
			release, err := updater.UpdateSelf(current, slug)

			if err != nil {
				return errors.Wrap(err, "failed to install update")
			}

			opts.log.Infof("Updated to %s", release.Version)
		} else {
			opts.log.Infof("You can update with `circleci update install`")
		}

		return nil
	}

	release, err := updater.UpdateSelf(current, slug)

	if err != nil {
		return errors.Wrap(err, "failed to install update")
	}

	opts.log.Infof("Updated to %s", release.Version)
	return nil
}

func reportVersion(log *logger.Logger, current, latest semver.Version) {
	log.Infof("A new release is available (%s)", latest)
	log.Infof("You are running %s", current)
}
