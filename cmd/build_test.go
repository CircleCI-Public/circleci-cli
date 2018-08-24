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

	Describe("config version is tested", func() {
		It("passes for valid versions", func() {
			var err error
			validateConfigVersion([]string{"--config", "testdata/config-versions/version-2.yml"})
			Expect(err).ToNot(HaveOccurred())
			validateConfigVersion([]string{"--config", "testdata/config-versions/version-2-0.yml"})
			Expect(err).ToNot(HaveOccurred())
		})

		It("fails when version number is not '2' or '2.0'", func() {
			var err error
			validateConfigVersion([]string{"--config", "testdata/config-versions/version-2-1.yml"})
			Expect(err).To(HaveOccurred())
		})

		It("fails when version number is not specified", func() {
			var err error
			validateConfigVersion([]string{"--config", "testdata/config-versions/version-empty.yml"})
			Expect(err).To(HaveOccurred())
		})

		It("fails when version is not defined", func() {
			var err error
			validateConfigVersion([]string{"--config", "testdata/config-versions/version-none.yml"})
			Expect(err).To(HaveOccurred())
		})
	})
})
