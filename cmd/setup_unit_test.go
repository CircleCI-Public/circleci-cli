package cmd

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/CircleCI-Public/circleci-cli/settings"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

type temporarySettings struct {
	home       string
	configFile *os.File
	configPath string
	updateFile *os.File
	updatePath string
}

func (tempSettings temporarySettings) writeToConfigAndClose(contents []byte) {
	_, err := tempSettings.configFile.Write(contents)
	Expect(err).ToNot(HaveOccurred())
	Expect(tempSettings.configFile.Close()).To(Succeed())
}

func (tempSettings temporarySettings) assertConfigRereadMatches(contents string) {
	file, err := os.Open(tempSettings.configPath)
	Expect(err).ShouldNot(HaveOccurred())

	reread, err := ioutil.ReadAll(file)
	Expect(err).ShouldNot(HaveOccurred())
	Expect(string(reread)).To(Equal(contents))
}

func withTempSettings() *temporarySettings {
	var err error

	tempSettings := &temporarySettings{}

	tempSettings.home, err = ioutil.TempDir("", "circleci-cli-test-")
	Expect(err).ToNot(HaveOccurred())

	settingsPath := filepath.Join(tempSettings.home, ".circleci")

	Expect(os.Mkdir(settingsPath, 0700)).To(Succeed())

	tempSettings.configPath = filepath.Join(settingsPath, "cli.yml")

	tempSettings.configFile, err = os.OpenFile(tempSettings.configPath,
		os.O_RDWR|os.O_CREATE,
		0600,
	)
	Expect(err).ToNot(HaveOccurred())

	tempSettings.updatePath = filepath.Join(settingsPath, "update_check.yml")
	tempSettings.updateFile, err = os.OpenFile(tempSettings.updatePath,
		os.O_RDWR|os.O_CREATE,
		0600,
	)
	Expect(err).ToNot(HaveOccurred())

	return tempSettings
}

func withCapturedOutput(f func()) string {
	r, w, err := os.Pipe()
	if err != nil {
		panic(err)
	}

	stdout := os.Stdout
	os.Stdout = w
	defer func() {
		os.Stdout = stdout
	}()

	f()
	w.Close()

	var buf bytes.Buffer
	io.Copy(&buf, r)

	return buf.String()
}

var _ = Describe("Setup with prompts", func() {
	var (
		tempSettings *temporarySettings
		opts         setupOptions
	)

	BeforeEach(func() {
		tempSettings = withTempSettings()
		opts = setupOptions{
			cfg: &settings.Config{
				FileUsed: tempSettings.configPath,
			},
			noPrompt: false,
			tty: setupTestUI{
				host:            "boondoggle",
				token:           "boondoggle",
				confirmEndpoint: true,
				confirmToken:    true,
			},
		}
	})

	AfterEach(func() {
		Expect(os.RemoveAll(tempSettings.home)).To(Succeed())
	})

	Describe("new config file", func() {
		It("should set file permissions to 0600", func() {
			// TODO: remove this flag once we mock the GraphQL client in these tests
			opts.integrationTesting = true

			err := setup(opts)
			Expect(err).ShouldNot(HaveOccurred())

			fileInfo, err := os.Stat(tempSettings.configPath)
			Expect(err).ToNot(HaveOccurred())
			Expect(fileInfo.Mode().Perm().String()).To(Equal("-rw-------"))
		})
	})

	Describe("existing config file", func() {
		BeforeEach(func() {
			// TODO: remove this flag once we mock the GraphQL client in these tests
			opts.integrationTesting = true

			opts.cfg.Host = "https://example.com/graphql"
			opts.cfg.Token = "fooBarBaz"
		})

		It("should print setup complete", func() {
			opts.tty = setupTestUI{
				host:            opts.cfg.Host,
				token:           opts.cfg.Token,
				confirmEndpoint: true,
				confirmToken:    true,
			}

			output := withCapturedOutput(func() {
				err := setup(opts)
				Expect(err).ShouldNot(HaveOccurred())
			})

			Expect(output).To(Equal(fmt.Sprintf(`A CircleCI token is already set. Do you want to change it
CircleCI API Token
API token has been set.
CircleCI Host
CircleCI host has been set.
Do you want to reset the endpoint? (default: graphql-unstable)
Setup complete.
Your configuration has been saved to %s.
`, tempSettings.configPath)))

			tempSettings.assertConfigRereadMatches(`host: https://example.com/graphql
endpoint: graphql-unstable
token: fooBarBaz
`)
		})
	})
})
