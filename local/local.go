package local

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path"
	"strings"
	"syscall"

	"github.com/pkg/errors"
	"github.com/spf13/pflag"

	"github.com/CircleCI-Public/circleci-cli/config"
	"github.com/CircleCI-Public/circleci-cli/settings"
)

var picardRepo = "circleci/picard"

const DefaultConfigPath = ".circleci/config.yml"
const DefaultDockerSocketPath = "/var/run/docker.sock"

func Execute(flags *pflag.FlagSet, cfg *settings.Config, args []string) error {
	var err error
	var configResponse *config.ConfigResponse

	// Get temp dir from flags
	tempDir, _ := flags.GetString("temp-dir")
	if tempDir == "" {
		// If not specified, get from config
		tempDir = cfg.TempDir
		if tempDir == "" {
			// If not specified, use system default
			tempDir = os.TempDir()
		}
	}

	processedArgs, configPath := buildAgentArguments(flags)

	compiler, err := config.NewWithConfig(cfg)
	if err != nil {
		return err
	}

	//if no orgId provided use org slug
	orgID, _ := flags.GetString("org-id")
	if strings.TrimSpace(orgID) != "" {
		configResponse, err = compiler.ConfigQuery(configPath, orgID, nil, config.LocalPipelineValues(nil))
		if err != nil {
			return err
		}
	} else {
		orgSlug, _ := flags.GetString("org-slug")
		configResponse, err = compiler.ConfigQuery(configPath, orgSlug, nil, config.LocalPipelineValues(nil))
		if err != nil {
			return err
		}
	}

	if !configResponse.Valid {
		return fmt.Errorf("config errors %v", configResponse.Errors)
	}

	processedConfigPath, err := writeStringToTempFile(tempDir, configResponse.OutputYaml)

	// The file at processedConfigPath must be left in place until after the call
	// to `docker run` has completed. Typically, we would `defer` a call to remove
	// the file. In this case, we execute `docker` using `syscall.Exec`, which
	// replaces the current process, and no more go code will execute at that
	// point, so we cannot delete the file easily. We choose to leave the file
	// in-place in the temporary directory.

	if err != nil {
		return err
	}

	pwd, err := os.Getwd()

	if err != nil {
		return err
	}

	dockerPath, err := ensureDockerIsAvailable()

	if err != nil {
		return err
	}

	picardVersion, _ := flags.GetString("build-agent-version")
	image, err := picardImage(os.Stdout, picardVersion)
	if err != nil {
		return errors.Wrap(err, "Could not find picard image")
	}

	job := args[0]
	dockerSocketPath, _ := flags.GetString("docker-socket-path")
	arguments := generateDockerCommand(tempDir, processedConfigPath, image, pwd, job, dockerSocketPath, processedArgs...)

	if cfg.Debug {
		_, err = fmt.Fprintf(os.Stderr, "Starting docker with args: %s", arguments)
		if err != nil {
			return err
		}
	}

	if err != nil {
		return errors.Wrap(err, "Could not find a `docker` executable on $PATH; please ensure that docker installed")
	}

	err = syscall.Exec(dockerPath, arguments, os.Environ()) // #nosec
	return errors.Wrap(err, "failed to execute docker")
}

// The `local execute` command proxies execution to the picard docker container,
// and ultimately to `build-agent`. We want to pass most arguments passed to the
// `local execute` command on to build-agent
// These options are here to retain a mock of the flags used by `build-agent`.
// They don't reflect the entire structure or available flags, only those which
// are public in the original command.
func AddFlagsForDocumentation(flags *pflag.FlagSet) {
	flags.StringP("config", "c", DefaultConfigPath, "config file")
	flags.Int("node-total", 1, "total number of parallel nodes")
	flags.Int("index", 0, "node index of parallelism")
	flags.Bool("skip-checkout", true, "use local path as-is")
	flags.StringArrayP("volume", "v", nil, "volume bind-mounting")
	flags.String("checkout-key", "~/.ssh/id_rsa", "git checkout key")
	flags.String("revision", "", "git revision")
	flags.String("branch", "", "git branch")
	flags.String("repo-url", "", "git URL")
	flags.StringArrayP("env", "e", nil, "set environment variables, e.g. `-e VAR=VAL`")
	flags.String("docker-socket-path", DefaultDockerSocketPath, "path to the host's docker socket")
}

// Given the full set of flags that were passed to this command, return the path
// to the config file, and the list of supplied args _except_ for the `--config`
// or `-c` argument, and except for --debug and --org-slug which are consumed by
// this program.
// The `build-agent` can only deal with config version 2.0. In order to feed
// version 2.0 config to it, we need to process the supplied config file using the
// GraphQL API, and feed the result of that into `build-agent`. The first step of
// that process is to find the local path to the config file. This is supplied with
// the `config` flag.
func buildAgentArguments(flags *pflag.FlagSet) ([]string, string) {

	var result []string = []string{}

	// build a list of all supplied flags, that we will pass on to build-agent
	flags.Visit(func(flag *pflag.Flag) {
		if flag.Name != "build-agent-version" &&
			flag.Name != "org-slug" &&
			flag.Name != "config" &&
			flag.Name != "temp-dir" &&
			flag.Name != "debug" &&
			flag.Name != "org-id" &&
			flag.Name != "docker-socket-path" {
			result = append(result, unparseFlag(flags, flag)...)
		}
	})
	result = append(result, flags.Args()...)

	configPath, _ := flags.GetString("config")

	return result, configPath
}

func picardImage(output io.Writer, picardVersion string) (string, error) {
	fmt.Fprintf(output, "Fetching latest build environment...\n")

	sha, err := getPicardSha(output, picardVersion)
	if err != nil {
		return "", err
	}

	_, _ = fmt.Fprintf(output, "Docker image digest: %s\n", sha)
	return fmt.Sprintf("%s@%s", picardRepo, sha), nil
}

func getPicardSha(output io.Writer, picardVersion string) (string, error) {
	// If the version was passed as argument, we take it
	if picardVersion != "" {
		return picardVersion, nil
	}

	var sha string
	var err error

	sha, err = loadBuildAgentShaFromConfig()
	if sha != "" && err == nil {
		return sha, nil
	}
	if err != nil && !os.IsNotExist(err) {
		fmt.Fprintf(output, "Unable to parse JSON file %s because: %s\n", buildAgentSettingsPath(), err)
		fmt.Fprintf(output, "Falling back to latest build-agent version\n")
	}

	// We are freezing build-agent cli as we would like to deprecate this path
	fixedSha := "sha256:008ba7f4223f1e26c11df9575283491b620074fa96da6961e0dcde47fb757014"
	return fixedSha, nil
}

func ensureDockerIsAvailable() (string, error) {

	dockerPath, err := exec.LookPath("docker")

	if err != nil {
		return "", errors.New("could not find `docker` on the PATH; please ensure that docker is installed")
	}

	dockerRunning := exec.Command(dockerPath, "version").Run() == nil // #nosec

	if !dockerRunning {
		return "", errors.New("failed to connect to docker; please ensure that docker is running, and that `docker version` succeeds")
	}

	return dockerPath, nil
}

// Write data to a temp file, and return the path to that file.
func writeStringToTempFile(tempDir, data string) (string, error) {
	if tempDir == "" {
		tempDir = os.TempDir()
	}
	f, err := os.CreateTemp(tempDir, "*_circleci_config.yml")

	if err != nil {
		return "", errors.Wrap(err, "Error creating temporary config file")
	}

	if _, err = f.WriteString(data); err != nil {
		return "", errors.Wrap(err, "Error writing processed config to temporary file")
	}

	return f.Name(), nil
}

func generateDockerCommand(tempDir, configPath, image, pwd string, job string, dockerSocketPath string, arguments ...string) []string {
	configPathInsideContainer := fmt.Sprintf("%s/local_build_config.yml", tempDir)
	core := []string{"docker", "run", "--rm",
		"--mount", fmt.Sprintf("type=bind,src=%s,dst=/var/run/docker.sock", dockerSocketPath),
		"--mount", fmt.Sprintf("type=bind,src=%s,dst=%s", configPath, configPathInsideContainer),
		"--mount", fmt.Sprintf("type=bind,src=%s,dst=%s", pwd, pwd),
		"--mount", fmt.Sprintf("type=bind,src=%s,dst=/root/.circleci", settings.SettingsPath()),
		"--workdir", pwd,
		image, "circleci", "build", "--config", configPathInsideContainer, "--job", job}
	return append(core, arguments...)
}

// Convert the given flag back into a list of strings suitable to be passed on
// the command line to run docker.
// https://github.com/CircleCI-Public/circleci-cli/issues/391
func unparseFlag(flags *pflag.FlagSet, flag *pflag.Flag) []string {
	flagName := "--" + flag.Name
	result := []string{}
	switch flag.Value.Type() {
	// A stringArray type argument is collapsed into a single flag:
	// `--foo 1 --foo 2` will result in a single `foo` flag with an array of values.
	case "stringArray":
		for _, value := range flag.Value.(pflag.SliceValue).GetSlice() {
			result = append(result, flagName, value)
		}
	default:
		result = append(result, flagName, flag.Value.String())
	}
	return result
}

type buildAgentSettings struct {
	LatestSha256 string
}

func loadBuildAgentShaFromConfig() (string, error) {
	if _, err := os.Stat(buildAgentSettingsPath()); os.IsNotExist(err) {
		// Settings file does not exist.
		return "", nil
	}

	file, err := os.Open(buildAgentSettingsPath())
	if err != nil {
		return "", errors.Wrap(err, "Could not open build settings config")
	}
	defer file.Close()

	var settings buildAgentSettings

	buf, err := io.ReadAll(file)
	if err != nil {
		return "", errors.Wrap(err, "Couldn't read from build settings file")
	}

	if err = json.Unmarshal(buf, &settings); err != nil {
		return "", errors.Wrap(err, "Could not parse build settings config")
	}

	return settings.LatestSha256, nil
}

func buildAgentSettingsPath() string {
	return path.Join(settings.SettingsPath(), "build_agent_settings.json")
}
