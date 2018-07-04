package cmd

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"regexp"
	"strings"
	"syscall"

	"github.com/CircleCI-Public/circleci-cli/settings"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

func newBuildCommand() *cobra.Command {

	return &cobra.Command{
		Use:                "build",
		Short:              "Run a build",
		RunE:               runBuild,
		DisableFlagParsing: true,
	}
}

var picardRepo = "circleci/picard"
var circleCiDir = path.Join(settings.UserHomeDir(), ".circleci")
var currentDigestFile = path.Join(circleCiDir, "current_picard_digest")
var latestDigestFile = path.Join(circleCiDir, "latest_picard_digest")

func getDigest(file string) string {

	if _, err := os.Stat(file); !os.IsNotExist(err) {
		digest, err := ioutil.ReadFile(file)

		if err != nil {
			Logger.Error("Could not load digest file", err)
		} else {
			return strings.TrimSpace(string(digest))
		}
	}

	return "" // Unknown digest
}

func newPullLatestImageCommand() *exec.Cmd {
	return exec.Command("docker", "pull", picardRepo)
}

func getLatestImageSha() (string, error) {

	cmd := newPullLatestImageCommand()
	Logger.Info("Pulling latest build image")
	bytes, err := cmd.CombinedOutput()

	if err != nil {
		return "", errors.Wrap(err, "failed to pull latest image")
	}

	output := string(bytes)

	sha256 := regexp.MustCompile("(?m)sha256.*$")

	//latest_image=$(docker pull picardRepo | grep -o "sha256.*$")
	//echo $latest_image > $LATEST_DIGEST_FILE
	latest := sha256.FindString(output)

	if latest == "" {
		return latest, errors.New("Failed to find latest image")
	}

	return latest, nil
}

func picardImageSha1() (string, error) {
	currentDigest := getDigest(currentDigestFile)

	if currentDigest == "" {
		Logger.Debug("no current digest stored - downloading latest image of picard")
		Logger.Info("Downloading latest CircleCI build agent...")

		// TODO - write the digest to a file so that we can
		// use it again.
		return getLatestImageSha()
	}

	// TODO - this should write to a file to record
	// the fact that we have the latest image.
	Logger.Debug("Pulling latest picard image in background")
	// We don't wait for this command, we just `Start` it and run in the background.
	if err := newPullLatestImageCommand().Start(); err != nil {
		return "", errors.Wrap(err, "Fails to start background update of build image")
	}

	return currentDigest, nil

}

func picardImage() (string, error) {
	sha, err := picardImageSha1()

	if err != nil {
		return "", err
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
		"--volume", fmt.Sprintf("%s:/root/.circleci", circleCiDir),
		"--workdir", pwd,
		image, "circleci", "build"}

	arguments = append(arguments, args...)

	Logger.Debug(fmt.Sprintf("Starting docker with args: %s", arguments))

	dockerPath, err := exec.LookPath("docker")

	if err != nil {
		return errors.Wrap(err, "Could not find a `docker` executable on $PATH; please ensure that docker installed")
	}

	if err = syscall.Exec(dockerPath, arguments, os.Environ()); err != nil {
		return errors.Wrap(err, "failed to execute docker")
	}

	return nil
}
