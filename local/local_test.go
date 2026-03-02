package local

import (
	"io"
	"os"
	"sort"
	"testing"

	"github.com/spf13/pflag"
	"gotest.tools/v3/assert"
)

func TestGenerateDockerCommand(t *testing.T) {
	home, err := os.UserHomeDir()
	assert.NilError(t, err)

	got := generateDockerCommand("/tempdir", "/config/path", "docker-image-name", "/current/directory", "build", "/var/run/docker.sock", "extra-1", "extra-2")
	want := []string{
		"docker",
		"run",
		"--rm",
		"--mount", "type=bind,src=/var/run/docker.sock,dst=/var/run/docker.sock",
		"--mount", "type=bind,src=/config/path,dst=/tempdir/local_build_config.yml",
		"--mount", "type=bind,src=/current/directory,dst=/current/directory",
		"--mount", "type=bind,src=" + home + "/.circleci,dst=/root/.circleci",
		"--workdir", "/current/directory",
		"docker-image-name", "circleci", "build",
		"--config", "/tempdir/local_build_config.yml",
		"--job", "build",
		"extra-1", "extra-2",
	}

	// ConsistOf equivalent: compare sorted copies
	sortedGot := make([]string, len(got))
	copy(sortedGot, got)
	sort.Strings(sortedGot)

	sortedWant := make([]string, len(want))
	copy(sortedWant, want)
	sort.Strings(sortedWant)

	assert.DeepEqual(t, sortedGot, sortedWant)
}

func TestWriteStringToTempFile(t *testing.T) {
	path, err := writeStringToTempFile("/tmp", "cynosure")
	assert.NilError(t, err)
	defer func() { _ = os.Remove(path) }()

	data, err := os.ReadFile(path)
	assert.NilError(t, err)
	assert.Equal(t, string(data), "cynosure")
}

func makeFlags(t *testing.T, args []string) (*pflag.FlagSet, error) {
	t.Helper()
	flags := pflag.NewFlagSet("test", pflag.ContinueOnError)
	AddFlagsForDocumentation(flags)
	flags.Bool("debug", false, "Enable debug logging.")
	flags.SetOutput(io.Discard)
	err := flags.Parse(args)
	return flags, err
}

func TestBuildAgentArguments(t *testing.T) {
	tests := []struct {
		name               string
		input              []string
		expectedArgs       []string
		expectedConfigPath string
		expectedError      string
	}{
		{
			name:               "no args",
			input:              []string{},
			expectedConfigPath: ".circleci/config.yml",
			expectedArgs:       []string{},
		},
		{
			name:               "single letter",
			input:              []string{"-c", "b"},
			expectedConfigPath: "b",
			expectedArgs:       []string{},
		},
		{
			name:               "asking for help",
			input:              []string{"-h", "b"},
			expectedConfigPath: ".circleci/config.yml",
			expectedArgs:       []string{},
			expectedError:      "pflag: help requested",
		},
		{
			name:               "many args",
			input:              []string{"--config", "foo", "--index", "9", "d"},
			expectedConfigPath: "foo",
			expectedArgs:       []string{"--index", "9", "d"},
		},
		{
			name:               "many args, multiple envs",
			input:              []string{"--env", "foo", "--env", "bar", "--env", "baz"},
			expectedConfigPath: ".circleci/config.yml",
			expectedArgs:       []string{"--env", "foo", "--env", "bar", "--env", "baz"},
		},
		{
			name:               "many args, multiple volumes (issue #469)",
			input:              []string{"-v", "/foo:/bar", "--volume", "/bin:/baz", "--volume", "/boo:/bop"},
			expectedConfigPath: ".circleci/config.yml",
			expectedArgs:       []string{"--volume", "/foo:/bar", "--volume", "/bin:/baz", "--volume", "/boo:/bop"},
		},
		{
			name:               "comma in env value (issue #440)",
			input:              []string{"--env", "{\"json\":[\"like\",\"value\"]}"},
			expectedConfigPath: ".circleci/config.yml",
			expectedArgs:       []string{"--env", "{\"json\":[\"like\",\"value\"]}"},
		},
		{
			name:               "args that are not flags",
			input:              []string{"a", "--debug", "b", "--config", "foo", "d"},
			expectedConfigPath: "foo",
			expectedArgs:       []string{"a", "b", "d"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			flags, err := makeFlags(t, tc.input)
			if tc.expectedError != "" {
				assert.Error(t, err, tc.expectedError)
			}
			args, configPath := buildAgentArguments(flags)
			assert.DeepEqual(t, args, tc.expectedArgs)
			assert.Equal(t, configPath, tc.expectedConfigPath)
		})
	}
}
