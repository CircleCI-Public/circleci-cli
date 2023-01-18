package local

import (
	"io/ioutil"
	"os"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	"github.com/spf13/pflag"
)

var _ = Describe("build", func() {

	Describe("invoking docker", func() {

		It("can generate a command line", func() {
			home, err := os.UserHomeDir()
			Expect(err).NotTo(HaveOccurred())
			Expect(generateDockerCommand("/config/path", "docker-image-name", "/current/directory", "build", "extra-1", "extra-2")).To(ConsistOf(
				"docker",
				"run",
				"--interactive",
				"--tty",
				"--rm",
				"--volume", "/var/run/docker.sock:/var/run/docker.sock",
				"--volume", "/config/path:/tmp/local_build_config.yml",
				"--volume", "/current/directory:/current/directory",
				"--volume", home+"/.circleci:/root/.circleci",
				"--workdir", "/current/directory",
				"docker-image-name", "circleci", "build",
				"--config", "/tmp/local_build_config.yml",
				"--job", "build",
				"extra-1", "extra-2",
			))
		})

		It("can write temp files", func() {
			path, err := writeStringToTempFile("cynosure")
			Expect(err).NotTo(HaveOccurred())
			defer os.Remove(path)
			Expect(ioutil.ReadFile(path)).To(BeEquivalentTo("cynosure"))
		})
	})

	Describe("argument parsing", func() {

		makeFlags := func(args []string) (*pflag.FlagSet, error) {
			flags := pflag.NewFlagSet("test", pflag.ContinueOnError)
			AddFlagsForDocumentation(flags)
			// add a 'debug' flag - the build command will inherit this from the
			// root command when not testing in isolation.
			flags.Bool("debug", false, "Enable debug logging.")
			flags.SetOutput(ioutil.Discard)
			err := flags.Parse(args)
			return flags, err
		}

		type TestCase struct {
			input              []string
			expectedArgs       []string
			expectedConfigPath string
			expectedError      string
		}

		DescribeTable("extracting config", func(testCase TestCase) {
			flags, err := makeFlags(testCase.input)
			if testCase.expectedError != "" {
				Expect(err).To(MatchError(testCase.expectedError))
			}
			args, configPath := buildAgentArguments(flags)
			Expect(args).To(Equal(testCase.expectedArgs))
			Expect(configPath).To(Equal(testCase.expectedConfigPath))

		},
			Entry("no args", TestCase{
				input:              []string{},
				expectedConfigPath: ".circleci/config.yml",
				expectedArgs:       []string{},
			}),

			Entry("single letter", TestCase{
				input:              []string{"-c", "b"},
				expectedConfigPath: "b",
				expectedArgs:       []string{},
			}),

			Entry("asking for help", TestCase{
				input:              []string{"-h", "b"},
				expectedConfigPath: ".circleci/config.yml",
				expectedArgs:       []string{},
				expectedError:      "pflag: help requested",
			}),

			Entry("many args", TestCase{
				input:              []string{"--config", "foo", "--index", "9", "d"},
				expectedConfigPath: "foo",
				expectedArgs:       []string{"--index", "9", "d"},
			}),

			Entry("many args, multiple envs", TestCase{
				input:              []string{"--env", "foo", "--env", "bar", "--env", "baz"},
				expectedConfigPath: ".circleci/config.yml",
				expectedArgs:       []string{"--env", "foo", "--env", "bar", "--env", "baz"},
			}),

			Entry("many args, multiple volumes (issue #469)", TestCase{
				input:              []string{"-v", "/foo:/bar", "--volume", "/bin:/baz", "--volume", "/boo:/bop"},
				expectedConfigPath: ".circleci/config.yml",
				expectedArgs:       []string{"--volume", "/foo:/bar", "--volume", "/bin:/baz", "--volume", "/boo:/bop"},
			}),

			Entry("comma in env value (issue #440)", TestCase{
				input:              []string{"--env", "{\"json\":[\"like\",\"value\"]}"},
				expectedConfigPath: ".circleci/config.yml",
				expectedArgs:       []string{"--env", "{\"json\":[\"like\",\"value\"]}"},
			}),

			Entry("args that are not flags", TestCase{
				input:              []string{"a", "--debug", "b", "--config", "foo", "d"},
				expectedConfigPath: "foo",
				expectedArgs:       []string{"a", "b", "d"},
			}))

	})
})
