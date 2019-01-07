package cmd

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"

	"github.com/CircleCI-Public/circleci-cli/client"
	"github.com/CircleCI-Public/circleci-cli/settings"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/ghttp"
)

type temporarySettings struct {
	home       string
	testServer *ghttp.Server
	configFile *os.File
	configPath string
	updateFile *os.File
	updatePath string
}

func (tempSettings temporarySettings) assertConfigRereadMatches(contents string) {
	file, err := os.Open(tempSettings.configPath)
	Expect(err).ShouldNot(HaveOccurred())

	reread, err := ioutil.ReadAll(file)
	Expect(err).ShouldNot(HaveOccurred())
	Expect(string(reread)).To(Equal(contents))
}

func withTempSettings() *temporarySettings {
	var err error

	tempSettings := &temporarySettings{}

	tempSettings.home, err = ioutil.TempDir("", "circleci-cli-test-")
	Expect(err).ToNot(HaveOccurred())

	settingsPath := filepath.Join(tempSettings.home, ".circleci")

	Expect(os.Mkdir(settingsPath, 0700)).To(Succeed())

	tempSettings.configPath = filepath.Join(settingsPath, "cli.yml")

	tempSettings.configFile, err = os.OpenFile(tempSettings.configPath,
		os.O_RDWR|os.O_CREATE,
		0600,
	)
	Expect(err).ToNot(HaveOccurred())

	tempSettings.updatePath = filepath.Join(settingsPath, "update_check.yml")
	tempSettings.updateFile, err = os.OpenFile(tempSettings.updatePath,
		os.O_RDWR|os.O_CREATE,
		0600,
	)
	Expect(err).ToNot(HaveOccurred())

	tempSettings.testServer = ghttp.NewServer()

	return tempSettings
}

func withCapturedOutput(f func()) string {
	r, w, err := os.Pipe()
	if err != nil {
		panic(err)
	}

	stdout := os.Stdout
	os.Stdout = w
	defer func() {
		os.Stdout = stdout
	}()

	f()
	w.Close()

	var buf bytes.Buffer
	io.Copy(&buf, r)

	return buf.String()
}

type MockRequestResponse struct {
	Request       string
	Status        int
	Response      string
	ErrorResponse string
}

// Test helpers

func appendPostHandler(server *ghttp.Server, authToken string, combineHandlers ...MockRequestResponse) {
	for _, handler := range combineHandlers {

		responseBody := `{ "data": ` + handler.Response + `}`
		if handler.ErrorResponse != "" {
			responseBody = fmt.Sprintf("{ \"data\": %s, \"errors\": %s}", handler.Response, handler.ErrorResponse)
		}

		if authToken == "" {
			server.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("POST", "/graphql-unstable"),
					ghttp.VerifyContentType("application/json; charset=utf-8"),
					// From Gomegas ghttp.VerifyJson to avoid the
					// VerifyContentType("application/json") check
					// that fails with "application/json; charset=utf-8"
					func(w http.ResponseWriter, req *http.Request) {
						body, err := ioutil.ReadAll(req.Body)
						req.Body.Close()
						Expect(err).ShouldNot(HaveOccurred())
						Expect(body).Should(MatchJSON(handler.Request), "JSON Mismatch")
					},
					ghttp.RespondWith(handler.Status, responseBody),
				),
			)
		} else {
			server.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("POST", "/graphql-unstable"),
					ghttp.VerifyHeader(http.Header{
						"Authorization": []string{authToken},
					}),
					ghttp.VerifyContentType("application/json; charset=utf-8"),
					// From Gomegas ghttp.VerifyJson to avoid the
					// VerifyContentType("application/json") check
					// that fails with "application/json; charset=utf-8"
					func(w http.ResponseWriter, req *http.Request) {
						body, err := ioutil.ReadAll(req.Body)
						req.Body.Close()
						Expect(err).ShouldNot(HaveOccurred())
						Expect(body).Should(MatchJSON(handler.Request), "JSON Mismatch")
					},
					ghttp.RespondWith(handler.Status, responseBody),
				),
			)
		}
	}
}

func (tempSettings *temporarySettings) newFakeClient() *client.Client {
	return client.NewClient(tempSettings.testServer.URL(), "/", "token", false)
}

var _ = Describe("Setup with prompts", func() {
	var (
		opts         setupOptions
		tempSettings *temporarySettings
		token        = "boondoggle"
	)

	BeforeEach(func() {
		tempSettings = withTempSettings()
		opts = setupOptions{
			cfg: &settings.Config{
				FileUsed: tempSettings.configPath,
			},
			noPrompt: false,
			tty: setupTestUI{
				host:            tempSettings.testServer.URL(),
				token:           token,
				confirmEndpoint: true,
				confirmToken:    true,
			},
		}
		opts.cl = tempSettings.newFakeClient()

		query := `query IntrospectionQuery {
		    __schema {
		      queryType { name }
		      mutationType { name }
		      types {
		        ...FullType
		      }
		    }
		  }

		  fragment FullType on __Type {
		    kind
		    name
		    description
		    fields(includeDeprecated: true) {
		      name
		    }
		  }`

		request := client.NewRequest(query)
		expected, err := request.Encode()
		Expect(err).ShouldNot(HaveOccurred())

		tmpBytes, err := ioutil.ReadFile(filepath.Join("testdata/diagnostic", "response.json"))
		Expect(err).ShouldNot(HaveOccurred())
		mockResponse := string(tmpBytes)

		appendPostHandler(tempSettings.testServer, "", MockRequestResponse{
			Status:   http.StatusOK,
			Request:  expected.String(),
			Response: mockResponse})

		// Here we want to actually validate the token in our test too
		query = `query { me { name } }`
		request = client.NewRequest(query)
		request.SetToken(token)
		Expect(err).ShouldNot(HaveOccurred())
		expected, err = request.Encode()
		Expect(err).ShouldNot(HaveOccurred())

		response := `{ "me": { "name": "zomg" } }`

		appendPostHandler(tempSettings.testServer, token, MockRequestResponse{
			Status:   http.StatusOK,
			Request:  expected.String(),
			Response: response})
	})

	AfterEach(func() {
		Expect(os.RemoveAll(tempSettings.home)).To(Succeed())
		tempSettings.testServer.Close()
	})

	Describe("new config file", func() {
		It("should set file permissions to 0600", func() {
			err := setup(opts)
			Expect(err).ShouldNot(HaveOccurred())

			fileInfo, err := os.Stat(tempSettings.configPath)
			Expect(err).ToNot(HaveOccurred())
			Expect(fileInfo.Mode().Perm().String()).To(Equal("-rw-------"))
		})
	})

	Describe("existing config file", func() {
		BeforeEach(func() {
			opts.cfg.Host = "https://example.com/graphql"
			opts.cfg.Token = token

			query := `query IntrospectionQuery {
		    __schema {
		      queryType { name }
		      mutationType { name }
		      types {
		        ...FullType
		      }
		    }
		  }

		  fragment FullType on __Type {
		    kind
		    name
		    description
		    fields(includeDeprecated: true) {
		      name
		    }
		  }`

			request := client.NewRequest(query)
			expected, err := request.Encode()
			Expect(err).ShouldNot(HaveOccurred())

			tmpBytes, err := ioutil.ReadFile(filepath.Join("testdata/diagnostic", "response.json"))
			Expect(err).ShouldNot(HaveOccurred())
			mockResponse := string(tmpBytes)

			appendPostHandler(tempSettings.testServer, "", MockRequestResponse{
				Status:   http.StatusOK,
				Request:  expected.String(),
				Response: mockResponse})

			// Here we want to actually validate the token in our test too
			query = `query { me { name } }`
			request = client.NewRequest(query)
			request.SetToken(token)
			Expect(err).ShouldNot(HaveOccurred())
			expected, err = request.Encode()
			Expect(err).ShouldNot(HaveOccurred())

			response := `{ "me": { "name": "zomg" } }`

			appendPostHandler(tempSettings.testServer, token, MockRequestResponse{
				Status:   http.StatusOK,
				Request:  expected.String(),
				Response: response})
		})

		It("should print setup complete", func() {
			opts.tty = setupTestUI{
				host:            tempSettings.testServer.URL(),
				token:           token,
				confirmEndpoint: true,
				confirmToken:    true,
			}

			output := withCapturedOutput(func() {
				err := setup(opts)
				Expect(err).ShouldNot(HaveOccurred())
			})

			Expect(output).To(Equal(fmt.Sprintf(`A CircleCI token is already set. Do you want to change it
CircleCI API Token
API token has been set.
CircleCI Host
CircleCI host has been set.
Do you want to reset the endpoint? (default: graphql-unstable)
Setup complete.
Your configuration has been saved to %s.

Trying an introspection query on API to verify your setup... Ok.
Trying to query your username given the provided token... Hello, %s.
`, tempSettings.configPath, `zomg`)))

			tempSettings.assertConfigRereadMatches(fmt.Sprintf(`host: %s
endpoint: graphql-unstable
token: %s
`, tempSettings.testServer.URL(), token))
		})
	})
})
