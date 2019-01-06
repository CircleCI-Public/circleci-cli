package cmd

import (
	"fmt"
	"os/exec"
	"regexp"
	"time"

	"github.com/CircleCI-Public/circleci-cli/settings"
	"github.com/CircleCI-Public/circleci-cli/update"
	"github.com/CircleCI-Public/circleci-cli/version"
	"github.com/pkg/errors"

	"github.com/spf13/cobra"

	"github.com/briandowns/spinner"
)

type updateCommandOptions struct {
	cfg    *settings.Config
	dryRun bool
	args   []string
}

func newUpdateCommand(config *settings.Config) *cobra.Command {
	opts := updateCommandOptions{
		cfg:    config,
		dryRun: false,
	}

	update := &cobra.Command{
		Use:   "update",
		Short: "Update the tool to the latest version",
		PersistentPreRun: func(_ *cobra.Command, _ []string) {
			opts.cfg.SkipUpdateCheck = true
		},
		PreRun: func(cmd *cobra.Command, args []string) {
			opts.args = args
		},
		RunE: func(_ *cobra.Command, _ []string) error {
			return updateCLI(opts)
		},
	}

	update.AddCommand(&cobra.Command{
		Use:    "check",
		Hidden: true,
		Short:  "Check if there are any updates available",
		PersistentPreRun: func(_ *cobra.Command, _ []string) {
			opts.cfg.SkipUpdateCheck = true
		},
		PreRun: func(cmd *cobra.Command, args []string) {
			opts.args = args
			opts.dryRun = true
		},
		RunE: func(_ *cobra.Command, _ []string) error {
			return updateCLI(opts)
		},
	})

	update.AddCommand(&cobra.Command{
		Use:    "install",
		Hidden: true,
		Short:  "Update the tool to the latest version",
		PersistentPreRun: func(_ *cobra.Command, _ []string) {
			opts.cfg.SkipUpdateCheck = true
		},
		PreRun: func(cmd *cobra.Command, args []string) {
			opts.args = args
		},
		RunE: func(_ *cobra.Command, _ []string) error {
			return updateCLI(opts)
		},
	})

	update.AddCommand(&cobra.Command{
		Use:    "build-agent",
		Hidden: true,
		Short:  "Update the build agent to the latest version",
		PersistentPreRun: func(_ *cobra.Command, _ []string) {
			opts.cfg.SkipUpdateCheck = true
		},
		PreRun: func(cmd *cobra.Command, args []string) {
			opts.args = args
		},
		RunE: func(_ *cobra.Command, _ []string) error {
			return updateBuildAgent()
		},
	})

	update.PersistentFlags().BoolVar(&opts.dryRun, "check", false, "Check if there are any updates available without installing")

	return update
}

var picardRepo = "circleci/picard"

func updateBuildAgent() error {
	latestSha256, err := findLatestPicardSha()

	if err != nil {
		return err
	}

	fmt.Printf("Latest build agent is version %s\n", latestSha256)

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

	spr := spinner.New(spinner.CharSets[14], 100*time.Millisecond)
	spr.Suffix = " Checking for updates..."
	spr.Start()

	check, err := update.CheckForUpdates(opts.cfg.GitHubAPI, slug, version.Version, PackageManager)
	spr.Stop()

	if err != nil {
		return err
	}

	if !check.Found {
		fmt.Println("No updates found.")
		return nil
	}

	if update.IsLatestVersion(check) {
		fmt.Println("Already up-to-date.")
		return nil
	}

	if opts.cfg.Debug {
		fmt.Println(update.DebugVersion(check))
	}
	fmt.Println(update.ReportVersion(check))

	if opts.dryRun {
		fmt.Println(update.HowToUpdate(check))
		return nil
	}

	spr.Suffix = " Installing update..."
	spr.Restart()
	message, err := update.InstallLatest(check)
	spr.Stop()
	if err != nil {
		return err
	}

	fmt.Println(message)

	return nil
}
