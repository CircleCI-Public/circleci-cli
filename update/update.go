package update

import (
	"net/http"

	"github.com/blang/semver"
	"github.com/google/go-github/github"
	"github.com/pkg/errors"
	"github.com/rhysd/go-github-selfupdate/selfupdate"
)

// Options contains everything we need to check for or perform updates of the CLI.
type Options struct {
	Current        semver.Version
	Latest         *selfupdate.Release
	PackageManager string

	updater   *selfupdate.Updater
	githubAPI string
	slug      string
}

// NewOptions returns a new instance of Options container after setting up several members.
func NewOptions(githubAPI, slug, current, packageManager string) (*Options, error) {
	updater, err := selfupdate.NewUpdater(selfupdate.Config{
		EnterpriseBaseURL: githubAPI,
	})
	if err != nil {
		return nil, err
	}

	return &Options{
		Current:        semver.MustParse(current),
		PackageManager: packageManager,

		githubAPI: githubAPI,
		slug:      slug,
		updater:   updater,
	}, nil
}

// LatestRelease will set the last known release as a member on the Options instance.
// We'll also return true or false if any release was found.
func (options *Options) LatestRelease() (bool, error) {
	latest, found, err := options.updater.DetectLatest(options.slug)
	options.Latest = latest

	if err != nil {
		if errResponse, ok := err.(*github.ErrorResponse); ok && errResponse.Response.StatusCode == http.StatusUnauthorized {
			return false, errors.Wrap(err, "Your Github token is invalid. Check the [github] section in ~/.gitconfig\n")
		}

		return false, errors.Wrap(err, "error finding latest release")
	}

	return found, nil
}

// NewerVersionAvailable will tell us if the current version is the same as the latest version found from the GitHub releases API.
func (options *Options) NewerVersionAvailable() bool {
	return options.Latest.Version.Equals(options.Current)
}

// UpdateToLatest will execute the updater and replace the current CLI with the latest version available.
func (options *Options) UpdateToLatest() (*selfupdate.Release, error) {
	return options.updater.UpdateSelf(options.Current, options.slug)
}
