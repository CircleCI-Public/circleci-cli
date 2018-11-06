package cmd

import (
	"fmt"
	"os/exec"
	"regexp"

	"github.com/CircleCI-Public/circleci-cli/logger"
	"github.com/CircleCI-Public/circleci-cli/settings"
	"github.com/CircleCI-Public/circleci-cli/update"
	"github.com/CircleCI-Public/circleci-cli/version"
	"github.com/pkg/errors"

	"github.com/spf13/cobra"
)

type updateCommandOptions struct {
	cfg       *settings.Config
	log       *logger.Logger
	dryRun    bool
	githubAPI string
	args      []string
}

func newUpdateCommand(config *settings.Config) *cobra.Command {
	opts := updateCommandOptions{
		cfg:       config,
		dryRun:    false,
		githubAPI: "https://api.github.com/",
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

	update.PersistentFlags().BoolVar(&opts.dryRun, "check", false, "Check if there are any updates available without installing")

	update.PersistentFlags().StringVar(&opts.githubAPI, "github-api", "https://api.github.com/", "Change the default endpoint to  GitHub API for retreiving updates")
	if err := update.PersistentFlags().MarkHidden("github-api"); err != nil {
		panic(err)
	}

	update.Flags().BoolVar(&testing, "testing", false, "Enable test mode to bypass interactive UI.")
	if err := update.Flags().MarkHidden("testing"); err != nil {
		panic(err)
	}

	return update
}

var picardRepo = "circleci/picard"

func updateBuildAgent(opts updateCommandOptions) error {
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
	sha256 := regexp.MustCompile("(?m)sha256:[0-9a-f]+")
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

func updateCLI(opts updateCommandOptions) error {
	slug := "CircleCI-Public/circleci-cli"

	updaterOptions, err := update.NewOptions(opts.githubAPI, slug, version.Version, PackageManager)
	if err != nil {
		return err
	}

	found, err := updaterOptions.LatestRelease()
	if err != nil {
		return err
	}

	if !found {
		return errors.New("no updates were found")
	}

	debugVersion(opts.log, updaterOptions)

	if updaterOptions.NewerVersionAvailable() {
		opts.log.Info("Already up-to-date.")
		return nil
	}

	reportVersion(opts.log, updaterOptions)

	if opts.dryRun {
		opts.log.Infof("You can update with `circleci update install`")
		return nil
	}

	release, err := updaterOptions.UpdateToLatest()
	if err != nil {
		return errors.Wrap(err, "failed to install update")
	}

	opts.log.Infof("Updated to %s", release.Version)

	return nil
}

func debugVersion(log *logger.Logger, opts *update.Options) {
	log.Debug("Latest version: %s", opts.Latest.Version)
	log.Debug("Published: %s", opts.Latest.PublishedAt)
	log.Debug("Current Version: %s", opts.Current)
}

func reportVersion(log *logger.Logger, opts *update.Options) {
	log.Infof("You are running %s", opts.Current)
	log.Infof("A new release is available (%s)", opts.Latest.Version)
}
