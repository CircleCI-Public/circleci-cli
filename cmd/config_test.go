package cmd_test

import (
	"fmt"
	"os/exec"
	"path/filepath"

	"github.com/CircleCI-Public/circleci-cli/clitest"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
	"gotest.tools/v3/golden"
)

var _ = Describe("Config", func() {
	Describe("pack", func() {
		var (
			command      *exec.Cmd
			results      []byte
			tempSettings *clitest.TempSettings
		)

		BeforeEach(func() {
			tempSettings = clitest.WithTempSettings()
		})

		AfterEach(func() {
			tempSettings.Close()
		})

		Describe("a .circleci folder with config.yml and local orbs folder containing the hugo orb", func() {
			BeforeEach(func() {
				command = exec.Command(pathCLI,
					"config", "pack",
					"--skip-update-check",
					"testdata/hugo-pack/.circleci")
				results = golden.Get(GinkgoT(), filepath.FromSlash("hugo-pack/result.yml"))
			})

			It("pack all YAML contents as expected", func() {
				session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)
				session.Wait()
				Expect(err).ShouldNot(HaveOccurred())
				Eventually(session.Err.Contents()).Should(BeEmpty())
				Eventually(session.Out.Contents()).Should(MatchYAML(results))
				Eventually(session).Should(gexec.Exit(0))
			})
		})

		Describe("local orbs folder with mixed inline and local commands, jobs, etc", func() {
			BeforeEach(func() {
				var path string = "nested-orbs-and-local-commands-etc"
				command = exec.Command(pathCLI,
					"config", "pack",
					"--skip-update-check",
					filepath.Join("testdata", path, "test"))
				results = golden.Get(GinkgoT(), filepath.FromSlash(fmt.Sprintf("%s/result.yml", path)))
			})

			It("pack all YAML contents as expected", func() {
				session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)
				session.Wait()
				Expect(err).ShouldNot(HaveOccurred())
				Eventually(session.Err.Contents()).Should(BeEmpty())
				Eventually(session.Out.Contents()).Should(MatchYAML(results))
				Eventually(session).Should(gexec.Exit(0))
			})
		})

		Describe("an orb containing local executors and commands in folder", func() {
			BeforeEach(func() {
				command = exec.Command(pathCLI,
					"config", "pack",
					"--skip-update-check",
					"testdata/myorb/test")

				results = golden.Get(GinkgoT(), filepath.FromSlash("myorb/result.yml"))
			})

			It("pack all YAML contents as expected", func() {
				session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)
				session.Wait()
				Expect(err).ShouldNot(HaveOccurred())
				Eventually(session.Err.Contents()).Should(BeEmpty())
				Eventually(session.Out.Contents()).Should(MatchYAML(results))
				Eventually(session).Should(gexec.Exit(0))
			})
		})

		It("packs all YAML contents as expected", func() {
			command = exec.Command(pathCLI,
				"config", "pack",
				"--skip-update-check",
				"testdata/hugo-pack/.circleci")
			results = golden.Get(GinkgoT(), filepath.FromSlash("hugo-pack/result.yml"))
			session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)
			session.Wait()
			Expect(err).ShouldNot(HaveOccurred())
			Eventually(session.Err.Contents()).Should(BeEmpty())
			Eventually(session.Out.Contents()).Should(MatchYAML(results))
			Eventually(session).Should(gexec.Exit(0))
		})

		It("given a .circleci folder with config.yml and local orb, packs all YAML contents as expected", func() {
			command = exec.Command(pathCLI,
				"config", "pack",
				"--skip-update-check",
				"testdata/hugo-pack/.circleci")
			results = golden.Get(GinkgoT(), filepath.FromSlash("hugo-pack/result.yml"))
			session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)
			session.Wait()
			Expect(err).ShouldNot(HaveOccurred())
			Eventually(session.Err.Contents()).Should(BeEmpty())
			Eventually(session.Out.Contents()).Should(MatchYAML(results))
			Eventually(session).Should(gexec.Exit(0))
		})

		It("given a local orbs folder with mixed inline and local commands, jobs, etc, packs all YAML contents as expected", func() {
			var path string = "nested-orbs-and-local-commands-etc"
			command = exec.Command(pathCLI,
				"config", "pack",
				"--skip-update-check",
				filepath.Join("testdata", path, "test"))
			results = golden.Get(GinkgoT(), filepath.FromSlash(fmt.Sprintf("%s/result.yml", path)))
			session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)
			session.Wait()
			Expect(err).ShouldNot(HaveOccurred())
			Eventually(session.Err.Contents()).Should(BeEmpty())
			Eventually(session.Out.Contents()).Should(MatchYAML(results))
			Eventually(session).Should(gexec.Exit(0))
		})

		It("returns an error when validating a config", func() {
			var path string = "nested-orbs-and-local-commands-etc"
			command = exec.Command(pathCLI,
				"config", "pack",
				"--skip-update-check",
				filepath.Join("testdata", path, "test"))
			results = golden.Get(GinkgoT(), filepath.FromSlash(fmt.Sprintf("%s/result.yml", path)))
			session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)
			session.Wait()
			Expect(err).ShouldNot(HaveOccurred())
			Eventually(session.Err.Contents()).Should(BeEmpty())
			Eventually(session.Out.Contents()).Should(MatchYAML(results))
			Eventually(session).Should(gexec.Exit(0))
		})

		It("packs successfully given an orb containing local executors and commands in folder", func() {
			command = exec.Command(pathCLI,
				"config", "pack",
				"--skip-update-check",
				"testdata/myorb/test")

			results = golden.Get(GinkgoT(), filepath.FromSlash("myorb/result.yml"))
			session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)
			session.Wait()
			Expect(err).ShouldNot(HaveOccurred())
			Eventually(session.Err.Contents()).Should(BeEmpty())
			Eventually(session.Out.Contents()).Should(MatchYAML(results))
			Eventually(session).Should(gexec.Exit(0))
		})

		It("packs as expected given a large nested config including rails orbs", func() {
			var path string = "test-with-large-nested-rails-orb"
			command = exec.Command(pathCLI,
				"config", "pack",
				"--skip-update-check",
				filepath.Join("testdata", path, "test"))
			results = golden.Get(GinkgoT(), filepath.FromSlash(fmt.Sprintf("%s/result.yml", path)))
			session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)
			session.Wait()
			Expect(err).ShouldNot(HaveOccurred())
			Eventually(session.Err.Contents()).Should(BeEmpty())
			Eventually(session.Out.Contents()).Should(MatchYAML(results))
			Eventually(session).Should(gexec.Exit(0))
		})

		It("prints an error given a config which is a list and not a map", func() {
			config := clitest.OpenTmpFile(filepath.Join(tempSettings.Home, "myorb"), "config.yaml")
			command = exec.Command(pathCLI,
				"config", "pack",
				"--skip-update-check",
				config.RootDir,
			)
			config.Write([]byte(`[]`))

			expected := fmt.Sprintf("Error: Failed trying to marshal the tree to YAML : expected a map, got a `[]interface {}` which is not supported at this time for \"%s\"\n", config.Path)
			session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)
			Expect(err).ShouldNot(HaveOccurred())
			stderr := session.Wait().Err.Contents()
			Expect(string(stderr)).To(Equal(expected))
			Eventually(session).Should(clitest.ShouldFail())
			config.Close()
		})

		Describe("with a large nested config including rails orb", func() {
			BeforeEach(func() {
				var path string = "test-with-large-nested-rails-orb"
				command = exec.Command(pathCLI,
					"config", "pack",
					"--skip-update-check",
					filepath.Join("testdata", path, "test"))
				results = golden.Get(GinkgoT(), filepath.FromSlash(fmt.Sprintf("%s/result.yml", path)))
			})

			It("pack all YAML contents as expected", func() {
				session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)
				session.Wait()
				Expect(err).ShouldNot(HaveOccurred())
				Eventually(session.Err.Contents()).Should(BeEmpty())
				Eventually(session.Out.Contents()).Should(MatchYAML(results))
				Eventually(session).Should(gexec.Exit(0))
			})
		})

		Context("config is a list and not a map", func() {
			var config *clitest.TmpFile

			BeforeEach(func() {
				config = clitest.OpenTmpFile(filepath.Join(tempSettings.Home, "myorb"), "config.yaml")

				command = exec.Command(pathCLI,
					"config", "pack",
					"--skip-update-check",
					config.RootDir,
				)
			})

			AfterEach(func() {
				config.Close()
			})

			It("prints an error about invalid YAML", func() {
				config.Write([]byte(`[]`))

				expected := fmt.Sprintf("Error: Failed trying to marshal the tree to YAML : expected a map, got a `[]interface {}` which is not supported at this time for \"%s\"\n", config.Path)

				session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)

				Expect(err).ShouldNot(HaveOccurred())

				stderr := session.Wait().Err.Contents()
				Expect(string(stderr)).To(Equal(expected))
				Eventually(session).Should(clitest.ShouldFail())
			})
		})
	})
})
