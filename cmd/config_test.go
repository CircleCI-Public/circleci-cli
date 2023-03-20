package cmd_test

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/CircleCI-Public/circleci-cli/api/graphql"
	"github.com/CircleCI-Public/circleci-cli/clitest"
	"github.com/CircleCI-Public/circleci-cli/pipeline"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
	"gotest.tools/v3/golden"
)

var _ = Describe("Config", func() {
	Describe("pack", func() {
		var (
			command      *exec.Cmd
			results      []byte
			tempSettings *clitest.TempSettings
			token        string = "testtoken"
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

		Describe("validating configs", func() {
			config := "version: 2.1"
			var expReq string

			BeforeEach(func() {
				command = exec.Command(pathCLI,
					"config", "validate",
					"--skip-update-check",
					"--token", token,
					"--host", tempSettings.TestServer.URL(),
					"-",
				)

				stdin, err := command.StdinPipe()
				Expect(err).ToNot(HaveOccurred())
				_, err = io.WriteString(stdin, config)
				Expect(err).ToNot(HaveOccurred())
				stdin.Close()

				query := `query ValidateConfig ($config: String!, $pipelineParametersJson: String, $pipelineValues: [StringKeyVal!], $orgSlug: String) {
			buildConfig(configYaml: $config, pipelineValues: $pipelineValues) {
				valid,
				errors { message },
				sourceYaml,
				outputYaml
			}
		}`

				r := graphql.NewRequest(query)
				r.SetToken(token)
				r.Variables["config"] = config
				r.Variables["pipelineValues"] = pipeline.PrepareForGraphQL(pipeline.LocalPipelineValues())

				req, err := r.Encode()
				Expect(err).ShouldNot(HaveOccurred())
				expReq = req.String()
			})

			It("returns an error when validating a config", func() {
				expResp := `{
					"buildConfig": {
								"errors": [
									{"message": "error1"}
								]
					}
				}`

				tempSettings.AppendPostHandler(token, clitest.MockRequestResponse{
					Status:   http.StatusOK,
					Request:  expReq,
					Response: expResp,
				})

				session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)
				Expect(err).ShouldNot(HaveOccurred())
				Eventually(session.Err, time.Second*3).Should(gbytes.Say("Error: error1"))
				Eventually(session).Should(clitest.ShouldFail())
			})

			It("returns successfully when validating a config", func() {
				expResp := `{
					"buildConfig": {}
				}`

				tempSettings.AppendPostHandler(token, clitest.MockRequestResponse{
					Status:   http.StatusOK,
					Request:  expReq,
					Response: expResp,
				})

				session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)
				Expect(err).ShouldNot(HaveOccurred())
				Eventually(session.Out, time.Second*3).Should(gbytes.Say("Config input is valid."))
				Eventually(session).Should(gexec.Exit(0))
			})
		})

		Describe("validating configs with pipeline parameters", func() {
			config := "version: 2.1"
			var expReq string

			BeforeEach(func() {
				command = exec.Command(pathCLI,
					"config", "process",
					"--skip-update-check",
					"--token", token,
					"--host", tempSettings.TestServer.URL(),
					"--pipeline-parameters", `{"foo": "test", "bar": true, "baz": 10}`,
					"-",
				)

				stdin, err := command.StdinPipe()
				Expect(err).ToNot(HaveOccurred())
				_, err = io.WriteString(stdin, config)
				Expect(err).ToNot(HaveOccurred())
				stdin.Close()

				query := `query ValidateConfig ($config: String!, $pipelineParametersJson: String, $pipelineValues: [StringKeyVal!], $orgSlug: String) {
			buildConfig(configYaml: $config, pipelineValues: $pipelineValues, pipelineParametersJson: $pipelineParametersJson) {
				valid,
				errors { message },
				sourceYaml,
				outputYaml
			}
		}`

				r := graphql.NewRequest(query)
				r.SetToken(token)
				r.Variables["config"] = config
				r.Variables["pipelineValues"] = pipeline.PrepareForGraphQL(pipeline.LocalPipelineValues())

				pipelineParams, err := json.Marshal(pipeline.Parameters{
					"foo": "test",
					"bar": true,
					"baz": 10,
				})
				Expect(err).ToNot(HaveOccurred())
				r.Variables["pipelineParametersJson"] = string(pipelineParams)

				req, err := r.Encode()
				Expect(err).ShouldNot(HaveOccurred())
				expReq = req.String()
			})

			It("returns successfully when validating a config", func() {
				expResp := `{
					"buildConfig": {}
				}`

				tempSettings.AppendPostHandler(token, clitest.MockRequestResponse{
					Status:   http.StatusOK,
					Request:  expReq,
					Response: expResp,
				})

				session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)
				Expect(err).ShouldNot(HaveOccurred())
				Eventually(session).Should(gexec.Exit(0))
			})
		})

		Describe("validating configs with private orbs", func() {
			config := "version: 2.1"
			orgId := "bb604b45-b6b0-4b81-ad80-796f15eddf87"
			var expReq string

			BeforeEach(func() {
				command = exec.Command(pathCLI,
					"config", "validate",
					"--skip-update-check",
					"--token", token,
					"--host", tempSettings.TestServer.URL(),
					"--org-id", orgId,
					"-",
				)

				stdin, err := command.StdinPipe()
				Expect(err).ToNot(HaveOccurred())
				_, err = io.WriteString(stdin, config)
				Expect(err).ToNot(HaveOccurred())
				stdin.Close()

				query := `query ValidateConfig ($config: String!, $pipelineParametersJson: String, $pipelineValues: [StringKeyVal!], $orgId: UUID!) {
			buildConfig(configYaml: $config, pipelineValues: $pipelineValues, orgId: $orgId) {
				valid,
				errors { message },
				sourceYaml,
				outputYaml
			}
		}`

				r := graphql.NewRequest(query)
				r.SetToken(token)
				r.Variables["config"] = config
				r.Variables["orgId"] = orgId
				r.Variables["pipelineValues"] = pipeline.PrepareForGraphQL(pipeline.LocalPipelineValues())

				req, err := r.Encode()
				Expect(err).ShouldNot(HaveOccurred())
				expReq = req.String()
			})

			It("returns an error when validating a config with a private orb", func() {
				expResp := `{
					"buildConfig": {
								"errors": [
									{"message": "permission denied"}
								]
					}
				}`

				tempSettings.AppendPostHandler(token, clitest.MockRequestResponse{
					Status:   http.StatusOK,
					Request:  expReq,
					Response: expResp,
				})

				session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)
				Expect(err).ShouldNot(HaveOccurred())
				Eventually(session.Err, time.Second*3).Should(gbytes.Say("Error: permission denied"))
				Eventually(session).Should(clitest.ShouldFail())
			})
		})

		Describe("validating configs with private orbs Legacy", func() {
			config := "version: 2.1"
			orgSlug := "circleci"
			var expReq string

			BeforeEach(func() {
				command = exec.Command(pathCLI,
					"config", "validate",
					"--skip-update-check",
					"--token", token,
					"--host", tempSettings.TestServer.URL(),
					"--org-slug", orgSlug,
					"-",
				)

				stdin, err := command.StdinPipe()
				Expect(err).ToNot(HaveOccurred())
				_, err = io.WriteString(stdin, config)
				Expect(err).ToNot(HaveOccurred())
				stdin.Close()

				query := `query ValidateConfig ($config: String!, $pipelineParametersJson: String, $pipelineValues: [StringKeyVal!], $orgSlug: String) {
			buildConfig(configYaml: $config, pipelineValues: $pipelineValues, orgSlug: $orgSlug) {
				valid,
				errors { message },
				sourceYaml,
				outputYaml
			}
		}`

				r := graphql.NewRequest(query)
				r.SetToken(token)
				r.Variables["config"] = config
				r.Variables["orgSlug"] = orgSlug
				r.Variables["pipelineValues"] = pipeline.PrepareForGraphQL(pipeline.LocalPipelineValues())

				req, err := r.Encode()
				Expect(err).ShouldNot(HaveOccurred())
				expReq = req.String()
			})

			It("returns an error when validating a config with a private orb", func() {
				expResp := `{
					"buildConfig": {
								"errors": [
									{"message": "permission denied"}
								]
					}
				}`

				tempSettings.AppendPostHandler(token, clitest.MockRequestResponse{
					Status:   http.StatusOK,
					Request:  expReq,
					Response: expResp,
				})

				session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)
				Expect(err).ShouldNot(HaveOccurred())
				Eventually(session.Err, time.Second*3).Should(gbytes.Say("Error: permission denied"))
				Eventually(session).Should(clitest.ShouldFail())
			})

			It("returns successfully when validating a config with private orbs", func() {
				expResp := `{
					"buildConfig": {}
				}`

				tempSettings.AppendPostHandler(token, clitest.MockRequestResponse{
					Status:   http.StatusOK,
					Request:  expReq,
					Response: expResp,
				})

				session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)
				Expect(err).ShouldNot(HaveOccurred())
				Eventually(session.Out, time.Second*3).Should(gbytes.Say("Config input is valid."))
				Eventually(session).Should(gexec.Exit(0))
			})
		})
	})
})
