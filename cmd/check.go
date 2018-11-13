package cmd

import (
	"time"

	"github.com/CircleCI-Public/circleci-cli/logger"
	"github.com/CircleCI-Public/circleci-cli/settings"
	"github.com/CircleCI-Public/circleci-cli/update"
	"github.com/CircleCI-Public/circleci-cli/version"
	"github.com/briandowns/spinner"
)

func checkForUpdates(opts *settings.Config) error {
	if opts.SkipUpdateCheck {
		return nil
	}

	updateCheck := &settings.UpdateCheck{
		LastUpdateCheck: time.Time{},
	}

	err := updateCheck.Load()
	if err != nil {
		return err
	}

	if update.ShouldCheckForUpdates(updateCheck) {
		log := logger.NewLogger(opts.Debug)
		slug := "CircleCI-Public/circleci-cli"

		spr := spinner.New(spinner.CharSets[14], 100*time.Millisecond)
		spr.Suffix = " Checking for updates..."
		spr.Start()

		check, err := update.CheckForUpdates(opts.GitHubAPI, slug, version.Version, PackageManager)

		if err != nil {
			spr.Stop()
			return err
		}

		if !check.Found {
			spr.Suffix = "No updates found."
			time.Sleep(300 * time.Millisecond)
			spr.Stop()

			updateCheck.LastUpdateCheck = time.Now()
			err = updateCheck.WriteToDisk()
			if err != nil {
				return err
			}

			return nil
		}

		if update.IsLatestVersion(check) {
			spr.Suffix = "Already up-to-date."
			time.Sleep(300 * time.Millisecond)
			spr.Stop()

			updateCheck.LastUpdateCheck = time.Now()
			err = updateCheck.WriteToDisk()
			if err != nil {
				return err
			}

			return nil
		}
		spr.Stop()

		log.Debug(update.DebugVersion(check))
		log.Info(update.ReportVersion(check))
		log.Info(update.HowToUpdate(check))

		log.Info("\n") // Print a new-line after all of that

		updateCheck.LastUpdateCheck = time.Now()
		err = updateCheck.WriteToDisk()
		if err != nil {
			return err
		}
	}

	return nil
}
