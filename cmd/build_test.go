package cmd

import (
	"io/ioutil"
	"os"

	"github.com/CircleCI-Public/circleci-cli/logger"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("build", func() {

	Describe("loading settings", func() {

		var tempHome string

		BeforeEach(func() {

			Logger = logger.NewLogger(false)
			var err error
			tempHome, err = ioutil.TempDir("", "circleci-cli-test-")

			Expect(err).ToNot(HaveOccurred())
			Expect(os.Setenv("HOME", tempHome)).To(Succeed())

		})

		AfterEach(func() {
			Expect(os.RemoveAll(tempHome)).To(Succeed())
		})

		It("can load settings", func() {

			Expect(storeBuildAgentSha("deipnosophist")).To(Succeed())
			Expect(loadCurrentBuildAgentSha()).To(Equal("deipnosophist"))
			image, err := picardImage()
			Expect(err).ToNot(HaveOccurred())
			Expect(image).To(Equal("circleci/picard@deipnosophist"))
		})

	})
})
