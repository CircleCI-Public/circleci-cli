package local

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"regexp"
	"syscall"

	"github.com/CircleCI-Public/circleci-cli/settings"
	"github.com/mitchellh/mapstructure"
	"github.com/pkg/errors"
	"github.com/spf13/pflag"
	"gopkg.in/yaml.v2"
)

var picardRepo = "circleci/picard"

const DefaultConfigPath = ".circleci/config.yml"

type BuildOptions struct {
	Cfg  *settings.Config
	Args []string
	Help func() error
}

type buildAgentSettings struct {
	LatestSha256 string
}

func UpdateBuildAgent() error {
	latestSha256, err := findLatestPicardSha()

	if err != nil {
		return err
	}

	fmt.Printf("Latest build agent is version %s\n", latestSha256)

	return nil
}

// nolint: gocyclo
// TODO(zzak): This function is fairly complex, we should refactor it
func Execute(opts BuildOptions) error {
	for _, f := range opts.Args {
		if f == "--help" || f == "-h" {
			return opts.Help()
		}
	}

	if err := validateConfigVersion(opts.Args); err != nil {
		return err
	}

	pwd, err := os.Getwd()

	if err != nil {
		return errors.Wrap(err, "Could not find pwd")
	}

	if err = ensureDockerIsAvailable(); err != nil {
		return err
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
		"--volume", fmt.Sprintf("%s:/root/.circleci", settings.SettingsPath()),
		"--workdir", pwd,
		image, "circleci", "build"}

	arguments = append(arguments, opts.Args...)

	if opts.Cfg.Debug {
		_, err = fmt.Fprintf(os.Stderr, "Starting docker with args: %s", arguments)
		if err != nil {
			return err
		}
	}

	dockerPath, err := exec.LookPath("docker")

	if err != nil {
		return errors.Wrap(err, "Could not find a `docker` executable on $PATH; please ensure that docker installed")
	}

	err = syscall.Exec(dockerPath, arguments, os.Environ()) // #nosec
	return errors.Wrap(err, "failed to execute docker")
}

func validateConfigVersion(args []string) error {
	invalidConfigError := `
You attempted to run a local build with version '%s' of configuration.
Local builds do not support that version at this time.
You can use 'circleci config process' to pre-process your config into a version that local builds can run (see 'circleci help config process' for more information)`
	configFlags := pflag.NewFlagSet("configFlags", pflag.ContinueOnError)
	configFlags.ParseErrorsWhitelist.UnknownFlags = true
	var filename string

	configFlags.StringVarP(&filename, "config", "c", DefaultConfigPath, "config file")

	err := configFlags.Parse(args)
	if err != nil {
		return errors.New("Unable to parse flags")
	}

	// #nosec
	configBytes, err := ioutil.ReadFile(filename)
	if err != nil {
		return errors.Wrapf(err, "Unable to read config file")
	}

	version, isVersion := configVersion(configBytes)
	if !isVersion || version == "" {
		return errors.New("Unable to identify config version")
	}

	if version != "2.0" && version != "2" {
		return fmt.Errorf(invalidConfigError, version)
	}

	return nil
}

func configVersion(configBytes []byte) (string, bool) {
	var raw map[string]interface{}
	if err := yaml.Unmarshal(configBytes, &raw); err != nil {
		return "", false
	}

	var configWithVersion struct {
		Version string `yaml:"version"`
	}
	if err := mapstructure.WeakDecode(raw, &configWithVersion); err != nil {
		return "", false
	}
	return configWithVersion.Version, true
}

func picardImage() (string, error) {

	sha := loadCurrentBuildAgentSha()

	if sha == "" {

		fmt.Println("Downloading latest CircleCI build agent...")

		var err error

		sha, err = findLatestPicardSha()

		if err != nil {
			return "", err
		}

	}
	fmt.Printf("Docker image digest: %s\n", sha)
	return fmt.Sprintf("%s@%s", picardRepo, sha), nil
}

func ensureDockerIsAvailable() error {
	if _, err := exec.LookPath("docker"); err != nil {
		return errors.New("could not find `docker` on the PATH; please ensure that docker is installed")
	}

	dockerRunning := exec.Command("docker", "version").Run() == nil // #nosec

	if !dockerRunning {
		return errors.New("failed to connect to docker; please ensure that docker is running, and that `docker version` succeeds")
	}

	return nil
}

// Still depends on a function in cmd/build.go
func findLatestPicardSha() (string, error) {

	if err := ensureDockerIsAvailable(); err != nil {
		return "", err
	}

	outputBytes, err := exec.Command("docker", "pull", picardRepo).CombinedOutput() // #nosec

	if err != nil {
		return "", errors.Wrap(err, "failed to pull latest docker image")
	}

	output := string(outputBytes)
	sha256 := regexp.MustCompile("(?m)sha256:[0-9a-f]+")
	latest := sha256.FindString(output)

	if latest == "" {
		return "", fmt.Errorf("failed to parse sha256 from docker pull output")
	}

	// This function still lives in cmd/build.go
	err = storeBuildAgentSha(latest)

	if err != nil {
		return "", err
	}

	return latest, nil
}

func buildAgentSettingsPath() string {
	return path.Join(settings.SettingsPath(), "build_agent_settings.json")
}

func storeBuildAgentSha(sha256 string) error {
	agentSettings := buildAgentSettings{
		LatestSha256: sha256,
	}

	settingsJSON, err := json.Marshal(agentSettings)

	if err != nil {
		return errors.Wrap(err, "Failed to serialize build agent settings")
	}

	if err = os.MkdirAll(settings.SettingsPath(), 0700); err != nil {
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
		_, er := fmt.Fprint(os.Stderr, "Failed to load build agent settings JSON", err.Error())
		if er != nil {
			panic(er)
		}

		return ""
	}

	var settings buildAgentSettings

	err = json.Unmarshal(settingsJSON, &settings)

	if err != nil {
		_, er := fmt.Fprint(os.Stderr, "Failed to parse build agent settings JSON", err.Error())
		if er != nil {
			panic(er)
		}

		return ""
	}

	return settings.LatestSha256
}
