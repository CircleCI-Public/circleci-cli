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
	"github.com/mitchellh/mapstructure"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"gopkg.in/yaml.v2"
)

// These options are purely here to retain a mock of the structure of the flags used by `build`.
// They don't reflect the entire structure or available flags, only those which are public in the original command.
type proxyBuildArgs struct {
	configFilename string
	taskInfo       struct {
		NodeTotal     int
		NodeIndex     int
		Job           string
		SkipCheckout  bool
		Volumes       []string
		Revision      string
		Branch        string
		RepositoryURI string
	}
	checkoutKey string
	envVarArgs  []string
}

func addConfigFlag(filename *string, flagset *pflag.FlagSet) {
	flagset.StringVarP(filename, "config", "c", defaultConfigPath, "config file")
}

func newLocalExecuteCommand() *cobra.Command {
	buildCommand := &cobra.Command{
		Use:                "execute",
		Short:              "Run a job in a container on the local machine",
		RunE:               runExecute,
		DisableFlagParsing: true,
	}

	// Used as a convenience work-around when DisableFlagParsing is enabled
	// Allows help command to access the combined rollup of flags
	opts := proxyBuildArgs{}
	flags := buildCommand.Flags()

	addConfigFlag(&opts.configFilename, flags)
	flags.StringVar(&opts.taskInfo.Job, "job", "build", "job to be executed")
	flags.IntVar(&opts.taskInfo.NodeTotal, "node-total", 1, "total number of parallel nodes")
	flags.IntVar(&opts.taskInfo.NodeIndex, "index", 0, "node index of parallelism")

	flags.BoolVar(&opts.taskInfo.SkipCheckout, "skip-checkout", true, "use local path as-is")
	flags.StringSliceVarP(&opts.taskInfo.Volumes, "volume", "v", nil, "Volume bind-mounting")

	flags.StringVar(&opts.checkoutKey, "checkout-key", "~/.ssh/id_rsa", "Git Checkout key")
	flags.StringVar(&opts.taskInfo.Revision, "revision", "", "Git Revision")
	flags.StringVar(&opts.taskInfo.Branch, "branch", "", "Git branch")
	flags.StringVar(&opts.taskInfo.RepositoryURI, "repo-url", "", "Git Url")

	flags.StringArrayVarP(&opts.envVarArgs, "env", "e", nil, "Set environment variables, e.g. `-e VAR=VAL`")

	return buildCommand
}

func newBuildCommand() *cobra.Command {
	cmd := newLocalExecuteCommand()
	cmd.Hidden = true
	cmd.Use = "build"
	return cmd
}

func newLocalCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "local",
		Short: "Debug jobs on the local machine",
	}
	cmd.AddCommand(newLocalExecuteCommand())
	return cmd
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
		Config.Logger.Error("Failed to load build agent settings JSON", err)
		return ""
	}

	var settings buildAgentSettings

	err = json.Unmarshal(settingsJSON, &settings)

	if err != nil {
		Config.Logger.Error("Failed to parse build agent settings JSON", err)
		return ""
	}

	return settings.LatestSha256
}

func picardImage() (string, error) {

	sha := loadCurrentBuildAgentSha()

	if sha == "" {

		Config.Logger.Info("Downloading latest CircleCI build agent...")

		var err error

		sha, err = findLatestPicardSha()

		if err != nil {
			return "", err
		}

	}
	Config.Logger.Infof("Docker image digest: %s", sha)
	return fmt.Sprintf("%s@%s", picardRepo, sha), nil
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

func validateConfigVersion(args []string) error {
	invalidConfigError := `
You attempted to run a local build with version '%s' of configuration.
Local builds do not support that version at this time.
You can use 'circleci config process' to pre-process your config into a version that local builds can run (see 'circleci help config process' for more information)`
	configFlags := pflag.NewFlagSet("configFlags", pflag.ContinueOnError)
	configFlags.ParseErrorsWhitelist.UnknownFlags = true
	var filename string
	addConfigFlag(&filename, configFlags)

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

func ensureDockerIsAvailable() error {
	if _, err := exec.LookPath("docker"); err != nil {
		return errors.New("could not find `docker` on the PATH; please ensure than docker is installed")
	}

	dockerRunning := exec.Command("docker", "version").Run() == nil // #nosec

	if !dockerRunning {
		return errors.New("failed to connect to docker; please ensure that docker is running, and that `docker version` succeeds")
	}

	return nil
}

func runExecute(cmd *cobra.Command, args []string) error {
	if err := validateConfigVersion(args); err != nil {
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
		"--volume", fmt.Sprintf("%s:/root/.circleci", circleCiDir()),
		"--workdir", pwd,
		image, "circleci", "build"}

	arguments = append(arguments, args...)

	Config.Logger.Debug(fmt.Sprintf("Starting docker with args: %s", arguments))

	dockerPath, err := exec.LookPath("docker")

	if err != nil {
		return errors.Wrap(err, "Could not find a `docker` executable on $PATH; please ensure that docker installed")
	}

	err = syscall.Exec(dockerPath, arguments, os.Environ()) // #nosec
	return errors.Wrap(err, "failed to execute docker")
}
