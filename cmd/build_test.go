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

		var (
			tempHome string
			log      *logger.Logger
		)

		BeforeEach(func() {
			var err error
			log = logger.NewLogger(false)

			tempHome, err = ioutil.TempDir("", "circleci-cli-test-")

			Expect(err).ToNot(HaveOccurred())
			Expect(os.Setenv("HOME", tempHome)).To(Succeed())

		})

		AfterEach(func() {
			Expect(os.RemoveAll(tempHome)).To(Succeed())
		})

		It("can load settings", func() {
			Expect(storeBuildAgentSha("deipnosophist")).To(Succeed())
			Expect(loadCurrentBuildAgentSha(log)).To(Equal("deipnosophist"))
			image, err := picardImage(log)
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

	Describe("local execute", func() {
		It("provides a help documentation when provided with a --help flag", func() {
			mockOptions := buildOptions{
				args: []string{"--help"},
			}
			called := false
			mockHelp := func() error {
				called = true
				return nil
			}
			runExecute(mockOptions, mockHelp)
			Expect(called).To(BeTrue())
		})

		It("provides a help documentation when provided with a --help flag mixed with other flags", func() {
			mockOptions := buildOptions{
				args: []string{"--skip-checkout", "--help"},
			}
			called := false
			mockHelp := func() error {
				called = true
				return nil
			}
			runExecute(mockOptions, mockHelp)
			Expect(called).To(BeTrue())
		})

		It("provides a help documentation when provided with a -h flag", func() {
			mockOptions := buildOptions{
				args: []string{"-h"},
			}
			called := false
			mockHelp := func() error {
				called = true
				return nil
			}
			runExecute(mockOptions, mockHelp)
			Expect(called).To(BeTrue())
		})

		It("provides a help documentation when provided with a -h flag mixed with other flags", func() {
			mockOptions := buildOptions{
				args: []string{"--skip-checkout", "-h"},
			}
			called := false
			mockHelp := func() error {
				called = true
				return nil
			}
			runExecute(mockOptions, mockHelp)
			Expect(called).To(BeTrue())
		})
	})
})
