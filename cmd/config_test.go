package cmd_test

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/CircleCI-Public/circleci-cli/api/compile_config"
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
			config_string := "version: 2.1"
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
				_, err = io.WriteString(stdin, config_string)
				Expect(err).ToNot(HaveOccurred())
				stdin.Close()

				reqOptions := &compile_config.Options{PipelineValues: pipeline.LocalPipelineValues()}

				body := &compile_config.CompileConfigRequest{ConfigYml: config_string, Options: *reqOptions}

				Expect(err).ShouldNot(HaveOccurred())
				rawRequest, err := json.Marshal(body)
				Expect(err).ToNot(HaveOccurred())
				expReq = string(rawRequest)
			})

			It("returns an error when validating a config", func() {
				expResp := `{
					"valid": false,
					"errors": [
						{"message": "error1"}
					]
				}`
				fmt.Printf("*****address: %v", tempSettings.TestServer.URL())

				By("setting up a mock server")
				tempSettings.AppendRESTConfigCompileHandler(token, clitest.MockRequestResponse{
					Status:   http.StatusOK,
					Request:  expReq,
					Response: expResp,
				})

				fmt.Printf("request: %s", expReq)

				By("running the command")
				session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)
				fmt.Printf("^^^^^^error: %+v", session.Err)
				Expect(err).ShouldNot(HaveOccurred())
				Eventually(session.Err, time.Second*3).Should(gbytes.Say("message: error1"))
				Eventually(session).Should(clitest.ShouldFail())
			})

			It("returns successfully when validating a config", func() {
				expResp := `{
					"valid":true
				}`

				tempSettings.AppendRESTConfigCompileHandler(token, clitest.MockRequestResponse{
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
			config_string := "version: 2.1"
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
				_, err = io.WriteString(stdin, config_string)
				Expect(err).ToNot(HaveOccurred())
				stdin.Close()

				pipelineParams, err := json.Marshal(pipeline.Parameters{
					"foo": "test",
					"bar": true,
					"baz": 10,
				})

				reqOptions := &compile_config.Options{PipelineValues: pipeline.LocalPipelineValues(), PipelineParameters: string(pipelineParams)}

				body := &compile_config.CompileConfigRequest{ConfigYml: config_string, Options: *reqOptions}

				Expect(err).ToNot(HaveOccurred())

				rawRequest, err := json.Marshal(body)

				Expect(err).ShouldNot(HaveOccurred())

				expReq = string(rawRequest)
			})

			It("returns successfully when validating a config", func() {

				fmt.Printf("*****raw request: %s", expReq)

				expResp := `{
					"valid": true
				}`

				tempSettings.AppendRESTConfigCompileHandler(token, clitest.MockRequestResponse{
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
			config_string := "version: 2.1"
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
				_, err = io.WriteString(stdin, config_string)
				Expect(err).ToNot(HaveOccurred())
				stdin.Close()

				reqOptions := &compile_config.Options{PipelineValues: pipeline.LocalPipelineValues(), OwnerId: orgId}

				body := &compile_config.CompileConfigRequest{ConfigYml: config_string, Options: *reqOptions}

				rawRequest, err := json.Marshal(body)
				Expect(err).ShouldNot(HaveOccurred())

				expReq = string(rawRequest)
			})

			It("returns an error when validating a config with a private orb", func() {
				expResp := `{
					"valid": false,
					"errors": [
									{"message": "permission denied"}
								]
				}`

				tempSettings.AppendRESTConfigCompileHandler(token, clitest.MockRequestResponse{
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
			config_string := "version: 2.1"
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
				_, err = io.WriteString(stdin, config_string)
				Expect(err).ToNot(HaveOccurred())
				stdin.Close()

				reqOptions := &compile_config.Options{PipelineValues: pipeline.LocalPipelineValues()}

				body := &compile_config.CompileConfigRequest{ConfigYml: config_string, Options: *reqOptions}

				Expect(err).ShouldNot(HaveOccurred())
				rawRequest, err := json.Marshal(body)
				Expect(err).ToNot(HaveOccurred())
				expReq = string(rawRequest)
			})

			It("returns an error when validating a config with a private orb", func() {
				expResp := `{
						"errors": [
									{"message": "permission denied"}
						]
					}`

				tempSettings.AppendRESTCollborationsHandler(token, clitest.MockRequestResponse{
					Status:   http.StatusOK,
					Request:  expReq,
					Response: expResp,
				})

				tempSettings.AppendRESTConfigCompileHandler(token, clitest.MockRequestResponse{
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
					"valid": true
				}`

				tempSettings.AppendRESTCollborationsHandler(token, clitest.MockRequestResponse{
					Status:   http.StatusOK,
					Request:  expReq,
					Response: expResp,
				})

				tempSettings.AppendRESTConfigCompileHandler(token, clitest.MockRequestResponse{
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
