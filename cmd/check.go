package cmd

import (
	"log"
	"os"
	"time"

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
		log := log.New(os.Stderr, "", 0)
		slug := "CircleCI-Public/circleci-cli"

		spr := spinner.New(spinner.CharSets[14], 100*time.Millisecond)
		spr.Writer = os.Stderr
		spr.Suffix = " Checking for updates..."
		spr.Start()

		check, err := update.CheckForUpdates(opts.GitHubAPI, slug, version.Version, version.PackageManager())

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
			return err
		}

		if update.IsLatestVersion(check) {
			spr.Suffix = "Already up-to-date."
			time.Sleep(300 * time.Millisecond)
			spr.Stop()

			updateCheck.LastUpdateCheck = time.Now()
			err = updateCheck.WriteToDisk()
			return err
		}
		spr.Stop()

		if opts.Debug {
			log.Println(update.DebugVersion(check))
			log.Println("")
		}

		log.Println(update.ReportVersion(check))
		log.Println(update.HowToUpdate(check))

		log.Println("") // Print a new-line after all of that

		updateCheck.LastUpdateCheck = time.Now()
		err = updateCheck.WriteToDisk()
		if err != nil {
			return err
		}
	}

	return nil
}
