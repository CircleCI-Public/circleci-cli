package cmd_test

import (
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
)

var _ = Describe("collapse", func() {
	var (
		tempRoot string
		command  *exec.Cmd
	)

	BeforeEach(func() {
		var err error
		tempRoot, err = ioutil.TempDir("", "circleci-cli-test-")
		Expect(err).ToNot(HaveOccurred())

		command = exec.Command(pathCLI, "collapse", "-r", tempRoot)
	})

	AfterEach(func() {
		Expect(os.RemoveAll(tempRoot)).To(Succeed())
	})

	Describe("with a single file under root", func() {
		BeforeEach(func() {
			var err error
			_, err = os.OpenFile(
				filepath.Join(tempRoot, "foo"),
				os.O_RDWR|os.O_CREATE,
				0600,
			)
			Expect(err).ToNot(HaveOccurred())
		})

		It("Prints a JSON tree of the nested file-structure", func() {
			session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)
			Expect(err).ShouldNot(HaveOccurred())
			Eventually(session.Err.Contents()).Should(BeEmpty())

			Eventually(session.Out).Should(gbytes.Say("{}"))
			Eventually(session).Should(gexec.Exit(0))
		})
	})
})
