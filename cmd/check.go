package cmd

import (
	"time"

	"github.com/CircleCI-Public/circleci-cli/logger"
	"github.com/CircleCI-Public/circleci-cli/settings"
	"github.com/CircleCI-Public/circleci-cli/update"
	"github.com/CircleCI-Public/circleci-cli/version"
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

		check, err := update.CheckForUpdates(opts.GitHubAPI, slug, version.Version, PackageManager)
		if err != nil {
			return err
		}

		if update.IsLatestVersion(check) {
			log.Info("Already up-to-date.")
			return nil
		}

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
