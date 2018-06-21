package cmd_test

import (
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
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

	Describe("with two YAML files within separate directory structures", func() {
		BeforeEach(func() {
			for _, dirName := range []string{"one", "two"} {
				path := filepath.Join(tempRoot, "orbs", dirName, "commands")
				Expect(os.MkdirAll(path, 0700)).To(Succeed())
				Expect(ioutil.WriteFile(
					filepath.Join(path, "file.yml"),
					[]byte("contents_one: 1\ncontents_two: 2\n"),
					0600),
				).To(Succeed())
			}
		})

		It("collapse all YAML contents using directory structure as keys", func() {
			session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)
			session.Wait()
			Expect(err).ShouldNot(HaveOccurred())
			Eventually(session.Err.Contents()).Should(BeEmpty())

			Eventually(session.Out.Contents()).Should(MatchYAML(`
orbs:
  one:
    commands:
      file:
        contents_one: 1
        contents_two: 2
  two:
    commands:
      file:
        contents_one: 1
        contents_two: 2
`))
			Eventually(session).Should(gexec.Exit(0))
		})
	})
})
