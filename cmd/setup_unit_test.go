package cmd

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"runtime"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/ghttp"
	"github.com/spf13/cobra"

	"github.com/CircleCI-Public/circleci-cli/clitest"
	"github.com/CircleCI-Public/circleci-cli/settings"
)

func dummyCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "test"}
	cmd.SetContext(context.Background())
	return cmd
}

var _ = Describe("Setup with prompts", func() {
	var (
		opts         setupOptions
		tempSettings *clitest.TempSettings
		token        = "boondoggle"
	)

	BeforeEach(func() {
		tempSettings = clitest.WithTempSettings()
		opts = setupOptions{
			cfg: &settings.Config{
				FileUsed:   tempSettings.Config.Path,
				Host:       tempSettings.TestServer.URL(),
				HTTPClient: http.DefaultClient,
			},
			noPrompt: false,
			tty: setupTestUI{
				host:            tempSettings.TestServer.URL(),
				token:           token,
				confirmEndpoint: true,
				confirmToken:    true,
			},
			token: token,
		}
		opts.cl = tempSettings.NewFakeClient(opts.cfg.Endpoint, token)
	})

	AfterEach(func() {
		tempSettings.Close()
	})

	Context("with happy diagnostic responses", func() {
		BeforeEach(func() {
			tempSettings.TestServer.AppendHandlers(CombineHandlers(
				VerifyRequest("GET", "/api/v2/me"),
				RespondWithJSONEncoded(http.StatusOK, map[string]any{
					"name":       "zomg",
					"login":      "zomg",
					"id":         "97491110-fea3-49b1-83da-ffd38ac8840c",
					"avatar_url": "https://avatars.githubusercontent.com/u/980172390812730912?v=4",
				}),
			))
		})

		Describe("new config file", func() {
			It("should set file permissions to 0600", func() {

				err := setup(dummyCmd(), opts)
				Expect(err).ShouldNot(HaveOccurred())

				fileInfo, err := os.Stat(tempSettings.Config.Path)
				Expect(err).ToNot(HaveOccurred())
				if runtime.GOOS != "windows" {
					Expect(fileInfo.Mode().Perm().String()).To(Equal("-rw-------"))
				}
			})
		})

		Describe("existing config file", func() {
			BeforeEach(func() {
				opts.cfg.Host = "https://example.com/graphql"
				opts.cfg.Token = token
			})

			It("should print setup complete", func() {
				opts.tty = setupTestUI{
					host:            tempSettings.TestServer.URL(),
					token:           token,
					confirmEndpoint: true,
					confirmToken:    true,
				}

				output := clitest.WithCapturedOutput(func() {
					err := setup(dummyCmd(), opts)
					Expect(err).ShouldNot(HaveOccurred())
				})

				Expect(output).To(ContainSubstring(fmt.Sprintf(`A CircleCI token is already set. Do you want to change it
CircleCI API Token
API token has been set.
CircleCI Host
CircleCI host has been set.
Do you want to reset the endpoint? (default: graphql-unstable)
Setup complete.
Your configuration has been saved to %s.

Trying to query our API for your profile name... Hello, %s.
`, tempSettings.Config.Path, `zomg`)))

				tempSettings.AssertConfigRereadMatches(fmt.Sprintf(`host: %s
endpoint: graphql-unstable
token: %s
`, tempSettings.TestServer.URL(), token))
			})

			It("should not ask to set token if prompt for existing token is cancelled", func() {
				opts.tty = setupTestUI{
					host:            tempSettings.TestServer.URL(),
					token:           token,
					confirmEndpoint: true,
					confirmToken:    false,
				}

				output := clitest.WithCapturedOutput(func() {
					err := setup(dummyCmd(), opts)
					Expect(err).ShouldNot(HaveOccurred())
				})

				Expect(output).To(ContainSubstring(fmt.Sprintf(`A CircleCI token is already set. Do you want to change it
CircleCI Host
CircleCI host has been set.
Do you want to reset the endpoint? (default: graphql-unstable)
Setup complete.
Your configuration has been saved to %s.

Trying to query our API for your profile name... Hello, %s.
`, tempSettings.Config.Path, `zomg`)))

				tempSettings.AssertConfigRereadMatches(fmt.Sprintf(`host: %s
endpoint: graphql-unstable
token: %s
`, tempSettings.TestServer.URL(), token))
			})
		})
	})

	Context("when whoami query returns an auth error", func() {
		BeforeEach(func() {
			tempSettings.TestServer.AppendHandlers(CombineHandlers(
				VerifyRequest("GET", "/api/v2/me"),
				RespondWithJSONEncoded(http.StatusUnauthorized, map[string]any{
					"message": "You must log in first",
				}),
			))
		})

		It("should show an error", func() {
			opts.tty = setupTestUI{
				host:            tempSettings.TestServer.URL(),
				token:           token,
				confirmEndpoint: true,
				confirmToken:    true,
			}

			output := clitest.WithCapturedOutput(func() {
				err := setup(dummyCmd(), opts)
				Expect(err).ShouldNot(HaveOccurred())
			})

			Expect(output).To(ContainSubstring(fmt.Sprintf(`CircleCI API Token
API token has been set.
CircleCI Host
CircleCI host has been set.
Do you want to reset the endpoint? (default: graphql-unstable)
Setup complete.
Your configuration has been saved to %s.

Trying to query our API for your profile name... 
Unable to query our API for your profile name, please check your settings.
`, tempSettings.Config.Path)))
		})
	})
})
