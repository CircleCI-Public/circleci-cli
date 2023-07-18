package local_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"time"

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
	var (
		execSettings *executeSettings
		command      *exec.Cmd
	)

	BeforeEach(func() {
		execSettings = newExecuteSettings()
		_, err := execSettings.config.Write(
			[]byte(`version: 2.1
jobs:
  build:
    docker:
      - image: cimg/base:2023.03
    steps:
      - run: echo "hello world"`,
			),
		)
		Expect(err).ShouldNot(HaveOccurred())

		command = exec.Command(pathCLI, "local", "execute", "build")
		command.Dir = execSettings.projectDir
	})

	AfterEach(func() {
		execSettings.close()
	})

	It("should run a local job", func() {
		session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)
		Expect(err).ShouldNot(HaveOccurred())
		Eventually(session, time.Minute).Should(gexec.Exit(0))
	})
})
