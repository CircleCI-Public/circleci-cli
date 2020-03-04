package cmd

import (
	"github.com/CircleCI-Public/circleci-cli/local"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("build", func() {

	Describe("local execute", func() {
		It("provides a help documentation when provided with a --help flag", func() {
			called := false
			mockHelp := func() error {
				called = true
				return nil
			}
			mockOptions := local.BuildOptions{
				Args: []string{"--help"},
				Help: mockHelp,
			}
			err := local.Execute(mockOptions)
			Expect(err).ToNot(HaveOccurred())
			Expect(called).To(BeTrue())
		})

		It("provides a help documentation when provided with a --help flag mixed with other flags", func() {
			called := false
			mockHelp := func() error {
				called = true
				return nil
			}
			mockOptions := local.BuildOptions{
				Args: []string{"--skip-checkout", "--help"},
				Help: mockHelp,
			}
			err := local.Execute(mockOptions)
			Expect(err).ToNot(HaveOccurred())
			Expect(called).To(BeTrue())
		})

		It("provides a help documentation when provided with a -h flag", func() {
			called := false
			mockHelp := func() error {
				called = true
				return nil
			}
			mockOptions := local.BuildOptions{
				Args: []string{"-h"},
				Help: mockHelp,
			}
			err := local.Execute(mockOptions)
			Expect(err).ToNot(HaveOccurred())
			Expect(called).To(BeTrue())
		})

		It("provides a help documentation when provided with a -h flag mixed with other flags", func() {
			called := false
			mockHelp := func() error {
				called = true
				return nil
			}
			mockOptions := local.BuildOptions{
				Args: []string{"--skip-checkout", "-h"},
				Help: mockHelp,
			}
			err := local.Execute(mockOptions)
			Expect(err).ToNot(HaveOccurred())
			Expect(called).To(BeTrue())
		})
	})
})
