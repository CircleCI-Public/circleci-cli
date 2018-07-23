package cmd_test

import (
	"io"
	"io/ioutil"
	"net/http"
	"os"
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

var _ = AfterSuite(func() {
	gexec.CleanupBuildArtifacts()
})

func TestCmd(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Cmd Suite")
}

// Test helpers

func appendPostHandler(server *ghttp.Server, authToken string, statusCode int, expectedRequestJson string, responseBody string) {
	server.AppendHandlers(
		ghttp.CombineHandlers(
			ghttp.VerifyRequest("POST", "/"),
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
				Expect(body).Should(MatchJSON(expectedRequestJson), "JSON Mismatch")
			},
			ghttp.RespondWith(statusCode, `{ "data": `+responseBody+`}`),
		),
	)
}

type tmpFile struct {
	RootDir string
	Path    string
	File    *os.File
}

func (f tmpFile) close() {
	f.File.Close()
	os.RemoveAll(f.RootDir)
}

func (f tmpFile) write(fileContent string) error {
	_, err := f.File.Write([]byte(fileContent))

	return err
}

func openTmpFile(path string) (tmpFile, error) {
	var (
		config tmpFile = tmpFile{}
		err    error
	)

	tmpDir, err := ioutil.TempDir("", "circleci-cli-test-")
	if err != nil {
		return config, err
	}
	defer os.RemoveAll(tmpDir)

	config.RootDir = tmpDir
	config.Path = filepath.Join(tmpDir, path)

	err = os.MkdirAll(filepath.Dir(config.Path), 0700)
	if err != nil {
		return config, err
	}

	var file *os.File
	file, err = os.OpenFile(
		config.Path,
		os.O_RDWR|os.O_CREATE,
		0600,
	)
	if err != nil {
		return config, err
	}
	defer deferClose(file)

	config.File = file

	return config, nil
}

func deferClose(closer io.Closer) {
	err := closer.Close()
	Expect(err).NotTo(HaveOccurred())
}

func copyTo(source tmpFile, destination string) error {
	in, err := os.Open(source.Path)
	Expect(err).NotTo(HaveOccurred())
	defer deferClose(in)

	dst := filepath.Join(destination, filepath.Base(source.File.Name()))
	out, err := os.OpenFile(dst, os.O_RDWR|os.O_CREATE, 0600)
	Expect(err).NotTo(HaveOccurred())
	defer deferClose(out)

	_, err = io.Copy(out, in)
	Expect(err).NotTo(HaveOccurred())
	return out.Close()
}
