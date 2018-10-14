package cmd

import (
	"io/ioutil"
	"os"

	"github.com/CircleCI-Public/circleci-cli/settings"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("build", func() {

	Describe("loading settings", func() {

		var (
			tempHome string
			opts     buildOptions
		)

		BeforeEach(func() {
			var err error
			opts = buildOptions{
				Config: &settings.Config{
					Debug:    false,
					Token:    "",
					Host:     defaultHost,
					Endpoint: defaultEndpoint,
				},
			}

			err = opts.Setup()
			Expect(err).ToNot(HaveOccurred())

			tempHome, err = ioutil.TempDir("", "circleci-cli-test-")

			Expect(err).ToNot(HaveOccurred())
			Expect(os.Setenv("HOME", tempHome)).To(Succeed())

		})

		AfterEach(func() {
			Expect(os.RemoveAll(tempHome)).To(Succeed())
		})

		It("can load settings", func() {
			Expect(storeBuildAgentSha("deipnosophist")).To(Succeed())
			Expect(loadCurrentBuildAgentSha(opts)).To(Equal("deipnosophist"))
			image, err := picardImage(opts)
			Expect(err).ToNot(HaveOccurred())
			Expect(image).To(Equal("circleci/picard@deipnosophist"))
		})

	})

	Describe("config version is tested", func() {
		It("passes for valid versions", func() {
			err := validateConfigVersion([]string{"--config", "testdata/config-versions/version-2.yml"})
			Expect(err).ToNot(HaveOccurred())

			err = validateConfigVersion([]string{"--config", "testdata/config-versions/version-2-0.yml"})
			Expect(err).ToNot(HaveOccurred())
		})

		It("passes when other flags are used, too", func() {
			err := validateConfigVersion([]string{"--config", "testdata/config-versions/version-2.yml", "--job", "foobar"})
			Expect(err).ToNot(HaveOccurred())
		})

		It("fails when version number is not '2' or '2.0'", func() {
			err := validateConfigVersion([]string{"--config", "testdata/config-versions/version-2-1.yml"})
			Expect(err).To(HaveOccurred())
		})

		It("fails when version number is not specified", func() {
			err := validateConfigVersion([]string{"--config", "testdata/config-versions/version-empty.yml"})
			Expect(err).To(HaveOccurred())
		})

		It("fails when version is not defined", func() {
			err := validateConfigVersion([]string{"--config", "testdata/config-versions/version-none.yml"})
			Expect(err).To(HaveOccurred())
		})
	})
})
