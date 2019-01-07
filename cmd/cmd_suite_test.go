package cmd_test

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
	"github.com/onsi/gomega/ghttp"
)

var pathCLI string

var _ = BeforeSuite(func() {
	var err error
	pathCLI, err = gexec.Build("github.com/CircleCI-Public/circleci-cli")
	Ω(err).ShouldNot(HaveOccurred())
})

type temporarySettings struct {
	home       string
	testServer *ghttp.Server
	configFile *os.File
	configPath string
	updateFile *os.File
	updatePath string
}

func (tempSettings temporarySettings) writeToConfigAndClose(contents []byte) {
	_, err := tempSettings.configFile.Write(contents)
	Expect(err).ToNot(HaveOccurred())
	Expect(tempSettings.configFile.Close()).To(Succeed())
}

func (tempSettings temporarySettings) assertConfigRereadMatches(contents string) {
	file, err := os.Open(tempSettings.configPath)
	Expect(err).ShouldNot(HaveOccurred())

	reread, err := ioutil.ReadAll(file)
	Expect(err).ShouldNot(HaveOccurred())
	Expect(string(reread)).To(Equal(contents))
}

func commandWithHome(bin, home string, args ...string) *exec.Cmd {
	command := exec.Command(bin, args...)

	command.Env = append(os.Environ(),
		fmt.Sprintf("HOME=%s", home),
		fmt.Sprintf("USERPROFILE=%s", home), // windows
	)

	return command
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

var _ = AfterSuite(func() {
	gexec.CleanupBuildArtifacts()
})

func TestCmd(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Cmd Suite")
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

type tmpFile struct {
	rootDir string
	path    string
	file    *os.File
}

func (f tmpFile) write(contents []byte) {
	_, err := f.file.Write(contents)
	Expect(err).ToNot(HaveOccurred())
	Expect(f.file.Close()).To(Succeed())
}

func openTmpFile(directory string, path string) tmpFile {
	var (
		config tmpFile = tmpFile{}
		err    error
	)

	config.rootDir = directory
	config.path = filepath.Join(directory, path)

	err = os.MkdirAll(filepath.Dir(config.path), 0700)
	Expect(err).ToNot(HaveOccurred())

	var file *os.File
	file, err = os.OpenFile(
		config.path,
		os.O_RDWR|os.O_CREATE,
		0600,
	)
	Expect(err).ToNot(HaveOccurred())

	config.file = file

	return config
}
