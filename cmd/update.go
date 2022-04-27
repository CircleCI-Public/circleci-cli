package cmd

import (
	"fmt"
	"time"

	"github.com/CircleCI-Public/circleci-cli/settings"
	"github.com/CircleCI-Public/circleci-cli/update"
	"github.com/CircleCI-Public/circleci-cli/version"

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
		Short:  "This command has no effect, and is kept for backwards compatiblity",
		PersistentPreRun: func(_ *cobra.Command, _ []string) {
			opts.cfg.SkipUpdateCheck = true
		},
		PreRun: func(cmd *cobra.Command, args []string) {
			opts.args = args
		},
		RunE: func(_ *cobra.Command, _ []string) error {
			return nil
		},
	})

	update.PersistentFlags().BoolVar(&opts.dryRun, "check", false, "Check if there are any updates available without installing")

	return update
}

func updateCLI(opts updateCommandOptions) error {
	slug := "CircleCI-Public/circleci-cli"

	spr := spinner.New(spinner.CharSets[14], 100*time.Millisecond)
	spr.Suffix = " Checking for updates..."
	spr.Start()

	check, err := update.CheckForUpdates(opts.cfg.GitHubAPI, slug, version.Version, version.PackageManager())
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
