package cmd

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
	"unicode/utf8"

	"github.com/CircleCI-Public/circleci-cli/version"
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

	return update
}

func trimFirstRune(s string) string {
	_, i := utf8.DecodeRuneInString(s)
	return s[i:]
}

func checkForUpdates(cmd *cobra.Command, args []string) error {

	url := "https://api.github.com/repos/CircleCI-Public/circleci-cli/releases/latest"

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return err
	}

	req.Header.Set("User-Agent", version.UserAgent())

	client := http.Client{}
	res, err := client.Do(req)
	if err != nil {
		return err
	}

	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return err
	}

	var release struct {
		// There are other fields in this response that we could use to download the
		// binaries on behalf of the user.
		// https://developer.github.com/v3/repos/releases/#get-the-latest-release
		HTML string `json:"html_url"`
		Tag  string `json:"tag_name"`
	}

	if err := json.Unmarshal(body, &release); err != nil {
		return err
	}

	latest := trimFirstRune(release.Tag)

	Logger.Debug("Latest version: %s", latest)
	Logger.Debug("Current Version: %s", version.Version)

	if latest == version.Version {
		Logger.Info("Already up-to-date.")
	} else {
		Logger.Infof("A new release is available (%s)", release.Tag)
		Logger.Infof("You are running %s", version.Version)
		Logger.Infof("You can download it from %s", release.HTML)
	}

	return nil
}
