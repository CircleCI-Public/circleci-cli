package cmd

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"syscall"

	"github.com/CircleCI-Public/circleci-cli/settings"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

func newBuildCommand() *cobra.Command {
	buildCommand := &cobra.Command{
		Use:                "build",
		Short:              "Run a build",
		RunE:               runBuild,
		DisableFlagParsing: true,
	}

	return buildCommand
}

func circleCiDir() string {
	return path.Join(settings.UserHomeDir(), ".circleci")
}

func buildAgentSettingsPath() string {
	return path.Join(circleCiDir(), "build_agent_settings.json")
}

type buildAgentSettings struct {
	LatestSha256 string
}

func storeBuildAgentSha(sha256 string) error {
	settings := buildAgentSettings{
		LatestSha256: sha256,
	}

	settingsJSON, err := json.Marshal(settings)

	if err != nil {
		return errors.Wrap(err, "Failed to serialize build agent settings")
	}

	if err = os.MkdirAll(circleCiDir(), 0700); err != nil {
		return errors.Wrap(err, "Could not create settings directory")
	}

	err = ioutil.WriteFile(buildAgentSettingsPath(), settingsJSON, 0644)

	return errors.Wrap(err, "Failed to write build agent settings file")
}

func loadCurrentBuildAgentSha() string {
	if _, err := os.Stat(buildAgentSettingsPath()); os.IsNotExist(err) {
		return ""
	}

	settingsJSON, err := ioutil.ReadFile(buildAgentSettingsPath())

	if err != nil {
		Logger.Error("Faild to load build agent settings JSON", err)
		return ""
	}

	var settings buildAgentSettings

	err = json.Unmarshal(settingsJSON, &settings)

	if err != nil {
		Logger.Error("Faild to parse build agent settings JSON", err)
		return ""
	}

	return settings.LatestSha256
}

func picardImage() (string, error) {

	sha := loadCurrentBuildAgentSha()

	if sha == "" {

		Logger.Info("Downloading latest CircleCI build agent...")

		var err error

		sha, err = findLatestPicardSha()

		if err != nil {
			return "", err
		}

	}
	Logger.Infof("Docker image digest: %s", sha)
	return fmt.Sprintf("%s@%s", picardRepo, sha), nil
}

func runBuild(cmd *cobra.Command, args []string) error {

	pwd, err := os.Getwd()

	if err != nil {
		return errors.Wrap(err, "Could not find pwd")
	}

	image, err := picardImage()

	if err != nil {
		return errors.Wrap(err, "Could not find picard image")
	}

	// TODO: marc:
	// We are passing the current environment to picard,
	// so DOCKER_API_VERSION is only passed when it is set
	// explicitly. The old bash script sets this to `1.23`
	// when not explicitly set. Is this OK?
	arguments := []string{"docker", "run", "--interactive", "--tty", "--rm",
		"--volume", "/var/run/docker.sock:/var/run/docker.sock",
		"--volume", fmt.Sprintf("%s:%s", pwd, pwd),
		"--volume", fmt.Sprintf("%s:/root/.circleci", circleCiDir()),
		"--workdir", pwd,
		image, "circleci", "build"}

	arguments = append(arguments, args...)

	Logger.Debug(fmt.Sprintf("Starting docker with args: %s", arguments))

	dockerPath, err := exec.LookPath("docker")

	if err != nil {
		return errors.Wrap(err, "Could not find a `docker` executable on $PATH; please ensure that docker installed")
	}

	err = syscall.Exec(dockerPath, arguments, os.Environ()) // #nosec
	return errors.Wrap(err, "failed to execute docker")
}
