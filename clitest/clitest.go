// Package clitest contains common utilities and helpers for testing the CLI
package clitest

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"

	"github.com/CircleCI-Public/circleci-cli/api/graphql"
	"github.com/onsi/gomega/gexec"
	"github.com/onsi/gomega/ghttp"
	"github.com/onsi/gomega/types"

	"github.com/onsi/gomega"
)

// On Unix, we want to assert that processed exited with 255
// On Windows, it should be -1.
func ShouldFail() types.GomegaMatcher {
	failureCode := 255
	if runtime.GOOS == "windows" {
		failureCode = -1
	}
	return gexec.Exit(failureCode)
}

// TempSettings contains useful settings for testing the CLI
type TempSettings struct {
	Home       string
	TestServer *ghttp.Server
	Config     *TmpFile
	Update     *TmpFile
}

// Close should be called in an AfterEach and cleans up the temp directory and server process
func (settings *TempSettings) Close() error {
	settings.TestServer.Close()
	settings.Config.Close()
	settings.Update.Close()
	return os.RemoveAll(settings.Home)
}

// AssertConfigRereadMatches re-opens the config file and checks it's contents against the given string
func (tempSettings TempSettings) AssertConfigRereadMatches(contents string) {
	file, err := os.Open(tempSettings.Config.Path)
	gomega.Expect(err).ShouldNot(gomega.HaveOccurred())

	reread, err := io.ReadAll(file)
	gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
	gomega.Expect(string(reread)).To(gomega.ContainSubstring(contents))
}

// WithTempSettings should be called in a BeforeEach and returns a new TempSettings with everything setup for you
func WithTempSettings() *TempSettings {
	var err error

	tempSettings := &TempSettings{}

	tempSettings.Home, err = os.MkdirTemp("", "circleci-cli-test-")
	gomega.Expect(err).ToNot(gomega.HaveOccurred())

	settingsPath := filepath.Join(tempSettings.Home, ".circleci")

	gomega.Expect(os.Mkdir(settingsPath, 0700)).To(gomega.Succeed())

	tempSettings.Config = OpenTmpFile(settingsPath, "cli.yml")
	tempSettings.Update = OpenTmpFile(settingsPath, "update_check.yml")

	tempSettings.TestServer = ghttp.NewServer()

	return tempSettings
}

// NewFakeClient returns a new *client.Client with the TestServer set and the provided endpoint, token.
func (tempSettings *TempSettings) NewFakeClient(endpoint, token string) *graphql.Client {
	return graphql.NewClient(http.DefaultClient, tempSettings.TestServer.URL(), endpoint, token, false)
}

// MockRequestResponse is a helpful type for mocking HTTP handlers.
type MockRequestResponse struct {
	Request       string
	Status        int
	Response      string
	ErrorResponse string
}

func (tempSettings *TempSettings) AppendRESTPostHandler(combineHandlers ...MockRequestResponse) {
	for _, handler := range combineHandlers {
		responseBody := handler.Response
		if handler.ErrorResponse != "" {
			responseBody = handler.ErrorResponse
		}

		tempSettings.TestServer.AppendHandlers(
			ghttp.CombineHandlers(
				ghttp.VerifyRequest("POST", "/api/v2/context"),
				ghttp.VerifyContentType("application/json"),
				func(w http.ResponseWriter, req *http.Request) {
					body, err := io.ReadAll(req.Body)
					gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
					err = req.Body.Close()
					gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
					gomega.Expect(handler.Request).Should(gomega.MatchJSON(body), "JSON Mismatch")
				},
				ghttp.RespondWith(handler.Status, responseBody),
			),
		)
	}
}

// AppendPostHandler stubs out the provided MockRequestResponse.
// When authToken is an empty string no token validation is performed.
func (tempSettings *TempSettings) AppendPostHandler(authToken string, combineHandlers ...MockRequestResponse) {
	for _, handler := range combineHandlers {
		responseBody := `{ "data": ` + handler.Response + `}`
		if handler.ErrorResponse != "" {
			responseBody = fmt.Sprintf("{ \"data\": %s, \"errors\": %s}", handler.Response, handler.ErrorResponse)
		}

		if authToken == "" {
			tempSettings.TestServer.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("POST", "/graphql-unstable"),
					ghttp.VerifyContentType("application/json; charset=utf-8"),
					// From Gomegas ghttp.VerifyJson to avoid the
					// VerifyContentType("application/json") check
					// that fails with "application/json; charset=utf-8"
					func(w http.ResponseWriter, req *http.Request) {
						body, err := io.ReadAll(req.Body)
						gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
						err = req.Body.Close()
						gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
						gomega.Expect(handler.Request).Should(gomega.MatchJSON(body), "JSON Mismatch")
					},
					ghttp.RespondWith(handler.Status, responseBody),
				),
			)
		} else {
			tempSettings.TestServer.AppendHandlers(
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
						body, err := io.ReadAll(req.Body)
						gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
						err = req.Body.Close()
						gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
						gomega.Expect(body).Should(gomega.MatchJSON(handler.Request), "JSON Mismatch")
					},
					ghttp.RespondWith(handler.Status, responseBody),
				),
			)
		}
	}
}

// TmpFile wraps a temporary file on disk for utility.
type TmpFile struct {
	RootDir string
	Path    string
	File    *os.File
}

func (tempFile *TmpFile) Close() error {
	return tempFile.File.Close()
}

// Write will write the given contents to the file on disk and close it.
func (f TmpFile) Write(contents []byte) {
	_, err := f.File.Write(contents)
	gomega.Expect(err).ToNot(gomega.HaveOccurred())
	gomega.Expect(f.File.Close()).To(gomega.Succeed())
}

// OpenTmpFile will create a new temporary file in the provided directory with a name of the given path.
func OpenTmpFile(directory string, path string) *TmpFile {
	var (
		config = &TmpFile{}
		err    error
	)

	config.RootDir = directory
	config.Path = filepath.Join(directory, path)

	err = os.MkdirAll(filepath.Dir(config.Path), 0700)
	gomega.Expect(err).ToNot(gomega.HaveOccurred())

	var file *os.File
	file, err = os.OpenFile(
		config.Path,
		os.O_RDWR|os.O_CREATE,
		0600,
	)
	gomega.Expect(err).ToNot(gomega.HaveOccurred())

	config.File = file

	return config
}

// WithCapturedOutput will call the provided function and capture any output to stdout which is returned as a string.
func WithCapturedOutput(f func()) string {
	r, w, err := os.Pipe()
	gomega.Expect(err).ToNot(gomega.HaveOccurred())

	stdout := os.Stdout
	os.Stdout = w
	defer func() {
		os.Stdout = stdout
	}()

	f()
	err = w.Close()
	gomega.Expect(err).ToNot(gomega.HaveOccurred())

	var buf bytes.Buffer
	_, err = io.Copy(&buf, r)
	gomega.Expect(err).ToNot(gomega.HaveOccurred())

	return buf.String()
}
