package cmd

import (
	"fmt"
	"net/http"
	"os"
	"runtime"

	"github.com/CircleCI-Public/circleci-cli/api/graphql"
	"github.com/CircleCI-Public/circleci-cli/clitest"
	"github.com/CircleCI-Public/circleci-cli/settings"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

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
				FileUsed: tempSettings.Config.Path,
			},
			noPrompt: false,
			tty: setupTestUI{
				host:            tempSettings.TestServer.URL(),
				token:           token,
				confirmEndpoint: true,
				confirmToken:    true,
			},
		}
		opts.cl = tempSettings.NewFakeClient(opts.cfg.Endpoint, token)
	})

	AfterEach(func() {
		tempSettings.Close()
	})

	Context("with happy diagnostic responses", func() {
		BeforeEach(func() {
			query := `query { me { name } }`
			request := graphql.NewRequest(query)
			request.SetToken(token)
			expected, err := request.Encode()
			Expect(err).ShouldNot(HaveOccurred())

			response := `{ "me": { "name": "zomg" } }`

			tempSettings.AppendPostHandler(token, clitest.MockRequestResponse{
				Status:   http.StatusOK,
				Request:  expected.String(),
				Response: response})
		})

		Describe("new config file", func() {
			It("should set file permissions to 0600", func() {

				err := setup(opts)
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
					err := setup(opts)
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
					err := setup(opts)
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
			// Here we want to actually validate the token in our test too
			query := `query { me { name } }`
			request := graphql.NewRequest(query)
			request.SetToken(token)
			expected, err := request.Encode()
			Expect(err).ShouldNot(HaveOccurred())

			tempSettings.AppendPostHandler(token, clitest.MockRequestResponse{
				Status:   http.StatusUnauthorized,
				Request:  expected.String(),
				Response: `{}`})
		})

		It("should show an error", func() {
			opts.tty = setupTestUI{
				host:            tempSettings.TestServer.URL(),
				token:           token,
				confirmEndpoint: true,
				confirmToken:    true,
			}

			output := clitest.WithCapturedOutput(func() {
				err := setup(opts)
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
