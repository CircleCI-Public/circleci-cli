package cmd

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"regexp"
	"strconv"
	"strings"

	"github.com/CircleCI-Public/circleci-cli/settings"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

var dockerAPIVersion = "1.23"

var job = "build"
var nodeTotal = 1
var checkoutKey = "~/.ssh/id_rsa"
var skipCheckout = true

func newBuildCommand() *cobra.Command {

	command := &cobra.Command{
		Use:   "build",
		Short: "Run a build",
		RunE:  runBuild,
	}

	/*
		TODO: Support these flags:
				--branch string         Git branch
			-e, --env -e VAR=VAL        Set environment variables, e.g. -e VAR=VAL
				--index int             node index of parallelism
				--repo-url string       Git Url
				--revision string       Git Revision
			-v, --volume stringSlice    Volume bind-mounting
	*/

	flags := command.Flags()
	flags.StringVarP(&dockerAPIVersion, "docker-api-version", "d", dockerAPIVersion, "The Docker API version to use")
	flags.StringVar(&checkoutKey, "checkout-key", checkoutKey, "Git Checkout key")
	flags.StringVar(&job, "job", job, "job to be executed")
	flags.BoolVar(&skipCheckout, "skip-checkout", skipCheckout, "use local path as-is")
	flags.IntVar(&nodeTotal, "node-total", nodeTotal, "total number of parallel nodes")

	return command
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

func streamOutout(stream io.Reader) {
	scanner := bufio.NewScanner(stream)
	go func() {
		for scanner.Scan() {
			Logger.Info(scanner.Text())
		}
	}()
}

func runBuild(cmd *cobra.Command, args []string) error {

	ctx := context.Background()

	pwd, err := os.Getwd()

	if err != nil {
		return errors.Wrap(err, "Could not find pwd")
	}

	image, err := picardImage()

	if err != nil {
		return errors.Wrap(err, "Could not find picard image")
	}

	// TODO: marc:
	// I can't find a way to pass `-it` and have carriage return
	// characters work correctly.
	arguments := []string{"run", "--rm",
		"-e", fmt.Sprintf("DOCKER_API_VERSION=%s", dockerAPIVersion),
		"-v", "/var/run/docker.sock:/var/run/docker.sock",
		"-v", fmt.Sprintf("%s:%s", pwd, pwd),
		"-v", fmt.Sprintf("%s:/root/.circleci", circleCiDir),
		"--workdir", pwd,
		image, "circleci", "build",

		// Proxied arguments
		"--config", configPath,
		"--skip-checkout", strconv.FormatBool(skipCheckout),
		"--node-total", strconv.Itoa(nodeTotal),
		"--checkout-key", checkoutKey,
		"--job", job}

	arguments = append(arguments, args...)

	Logger.Debug(fmt.Sprintf("Starting docker with args: %s", arguments))

	build := exec.CommandContext(ctx, "docker", arguments...)

	build.Stdin = os.Stdin

	stdout, err := build.StdoutPipe()

	if err != nil {
		return errors.Wrap(err, "Failed to connect to stdout")
	}

	stderr, err := build.StderrPipe()

	if err != nil {
		return errors.Wrap(err, "Failed to connect to stderr")
	}

	streamOutout(stdout)
	streamOutout(stderr)

	return build.Run()
}
