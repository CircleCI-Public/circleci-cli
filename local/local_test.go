package local_test

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/CircleCI-Public/circleci-cli/clitest"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
)

type executeSettings struct {
	projectDir string
	config     *os.File
}

func newExecuteSettings() *executeSettings {
	projectDir, err := os.MkdirTemp("", "circleci-cli-test-project")
	Expect(err).ToNot(HaveOccurred())

	circleCIPath := filepath.Join(projectDir, ".circleci")
	Expect(os.Mkdir(circleCIPath, 0700)).To(Succeed())

	configFilePath := filepath.Join(circleCIPath, "config.yml")
	configFile, err := os.OpenFile(configFilePath, os.O_CREATE|os.O_RDWR, 0600)
	Expect(err).ToNot(HaveOccurred())

	return &executeSettings{
		projectDir: projectDir,
		config:     configFile,
	}
}

func (es *executeSettings) close() {
	es.config.Close()
	os.RemoveAll(es.projectDir)
}

var _ = Describe("Execute integration tests", func() {
	const configData = `version: 2.1
jobs:
  build:
    docker:
      - image: cimg/base:2023.03
    steps:
      - run: echo "hello world"`

	var execSettings *executeSettings

	BeforeEach(func() {
		execSettings = newExecuteSettings()
		_, err := execSettings.config.Write([]byte(configData))
		Expect(err).ShouldNot(HaveOccurred())
	})

	AfterEach(func() {
		execSettings.close()
	})

	It("should run a local job", func() {
		command := exec.Command(pathCLI, "local", "execute", "build")
		command.Dir = execSettings.projectDir

		session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)
		Expect(err).ShouldNot(HaveOccurred())
		Eventually(session, time.Minute).Should(gexec.Exit(0))
	})

	It("should run a local job with a custom temp dir in flags", func() {
		tempDir, err := os.MkdirTemp("", "circleci-cli-test-tmp")
		Expect(err).ShouldNot(HaveOccurred())

		command := exec.Command(pathCLI, "local", "execute", "--temp-dir", tempDir, "build")
		command.Dir = execSettings.projectDir

		session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)
		Expect(err).ShouldNot(HaveOccurred())
		Eventually(session, time.Minute).Should(gexec.Exit(0))

		tempConfigData, err := os.ReadFile(execSettings.config.Name())
		Expect(err).ShouldNot(HaveOccurred())
		Expect(string(tempConfigData)).To(Equal(configData))
	})

	It("should run a local job with a custom temp dir in settings", func() {
		tempDir, err := os.MkdirTemp("", "circleci-cli-test-tmp")
		Expect(err).ShouldNot(HaveOccurred())

		tempSettings := clitest.WithTempSettings()
		defer tempSettings.Close()

		tempSettings.Config.Write([]byte(fmt.Sprintf("temp_dir: '%s'", tempDir)))

		command := exec.Command(pathCLI, "local", "execute", "build")
		command.Dir = execSettings.projectDir
		command.Env = append(
			os.Environ(),
			fmt.Sprintf("HOME=%s", tempSettings.Home),
			fmt.Sprintf("USERPROFILE=%s", tempSettings.Home), // windows
		)

		session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)
		Expect(err).ShouldNot(HaveOccurred())
		Eventually(session, time.Minute).Should(gexec.Exit(0))

		tempConfigData, err := os.ReadFile(execSettings.config.Name())
		Expect(err).ShouldNot(HaveOccurred())
		Expect(string(tempConfigData)).To(Equal(configData))
	})
})
