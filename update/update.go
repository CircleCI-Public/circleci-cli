package update

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os/exec"
	"strings"

	"github.com/blang/semver"
	"github.com/google/go-github/github"
	"github.com/pkg/errors"
	"github.com/rhysd/go-github-selfupdate/selfupdate"
)

// CheckForUpdates will check for updates given the proper package manager
func CheckForUpdates(githubAPI, slug, current, packageManager string) (*Options, error) {
	check := &Options{
		Current:        semver.MustParse(current),
		PackageManager: packageManager,

		githubAPI: githubAPI,
		slug:      slug,
	}

	switch check.PackageManager {
	case "source":
		err := checkFromSource(check)
		if err != nil {
			return nil, err
		}
	case "homebrew":
		err := checkFromHomebrew(check)
		if err != nil {
			return nil, err
		}
	}

	return check, nil
}

func checkFromSource(check *Options) error {
	updater, err := selfupdate.NewUpdater(selfupdate.Config{
		EnterpriseBaseURL: check.githubAPI,
	})
	if err != nil {
		return err
	}

	check.updater = updater

	found, err := latestRelease(check)
	if err != nil {
		return err
	}

	if !found {
		return errors.New("no updates were found")
	}

	return nil
}

/*

$ brew outdated
circleci (0.1.1248) < 0.1.3923
docker (18.06.0) < 18.06.1
goreleaser (0.83.0) < 0.92.1

*/
func checkFromHomebrew(check *Options) error {
	brew, err := exec.LookPath("brew")
	if err != nil {
		return errors.Wrap(err, "Expected to find `brew` in your $PATH but wasn't able to find it")
	}

	command := exec.Command(brew, "outdated", "--json=v1") // #nosec
	out, err := command.Output()
	if err != nil {
		return errors.Wrap(err, "failed to check for updates via `brew`")
	}

	var outdated HomebrewOutdated

	err = json.Unmarshal(out, &outdated)
	if err != nil {
		return errors.Wrap(err, "failed to parse output of `brew outdated --json=v1`")
	}

	for _, o := range outdated {
		if o.Name == "circleci" {
			if len(o.InstalledVersions) > 0 {
				check.Current = semver.MustParse(o.InstalledVersions[0])
			}

			check.Latest = &selfupdate.Release{
				Version: semver.MustParse(o.CurrentVersion),
			}
		}
	}

	return nil
}

// HomebrewOutdated wraps the JSON output from running `brew outdated --json=v1`
/*

For example:

[
  {
    "name": "circleci",
    "installed_versions": [
      "0.1.1248"
    ],
    "current_version": "0.1.3923",
    "pinned": false,
    "pinned_version": null
  },
]
*/
type HomebrewOutdated []struct {
	Name              string   `json:"name"`
	InstalledVersions []string `json:"installed_versions"`
	CurrentVersion    string   `json:"current_version"`
	Pinned            bool     `json:"pinned"`
	PinnedVersion     string   `json:"pinned_version"`
}

// Options contains everything we need to check for or perform updates of the CLI.
type Options struct {
	Current        semver.Version
	Latest         *selfupdate.Release
	PackageManager string

	updater   *selfupdate.Updater
	githubAPI string
	slug      string
}

// latestRelease will set the last known release as a member on the Options instance.
// We'll return false if no releases were found, are you sure you have the right project?
func latestRelease(opts *Options) (bool, error) {
	latest, found, err := opts.updater.DetectLatest(opts.slug)
	opts.Latest = latest

	if err != nil {
		if errResponse, ok := err.(*github.ErrorResponse); ok && errResponse.Response.StatusCode == http.StatusUnauthorized {
			return false, errors.Wrap(err, "Your Github token is invalid. Check the [github] section in ~/.gitconfig\n")
		}

		return false, errors.Wrap(err, "error finding latest release")
	}

	return found, nil
}

// IsLatestVersion will tell us if the current version is the same as the latest version found from the GitHub releases API.
func IsLatestVersion(opts *Options) bool {
	return opts.Latest.Version.Equals(opts.Current)
}

// InstallLatest will execute the updater and replace the current CLI with the latest version available.
func InstallLatest(opts *Options) (string, error) {
	release, err := opts.updater.UpdateSelf(opts.Current, opts.slug)
	if err != nil {
		return "", errors.Wrap(err, "failed to install update")
	}

	return fmt.Sprintf("Updated to %s", release.Version), nil
}

// DebugVersion returns a nicely formatted string representing the state of the current version.
// Intended to be printed to standard error for developers.
func DebugVersion(opts *Options) string {
	return strings.Join([]string{
		fmt.Sprintf("Latest version: %s", opts.Latest.Version),
		fmt.Sprintf("Published: %s", opts.Latest.PublishedAt),
		fmt.Sprintf("Current Version: %s", opts.Current),
	}, "\n")
}

// ReportVersion returns a nicely formatted string representing the state of the current version.
// Intended to be printed to the user.
func ReportVersion(opts *Options) string {
	return strings.Join([]string{
		fmt.Sprintf("You are running %s", opts.Current),
		fmt.Sprintf("A new release is available (%s)", opts.Latest.Version),
		"\n",
	}, "\n")
}

// HowToUpdate returns a message teaching the user how to update to the latest version.
func HowToUpdate(opts *Options) string {
	switch opts.PackageManager {
	case "homebrew":
		return "You can update with `brew upgrade circleci`"
	case "source":
		return strings.Join([]string{
			"You can visit the Github releases page for the CLI to manually download and install:",
			"https://github.com/CircleCI-Public/circleci-cli/releases",
		}, "\n")
	}

	// Do nothing if we don't expect one of the supported package managers above
	return ""
}
