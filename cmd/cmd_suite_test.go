package cmd_test

import (
	"fmt"
	"os"
	"os/exec"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
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

func commandWithHome(bin, home string, args ...string) *exec.Cmd {
	command := exec.Command(bin, args...)

	command.Env = append(os.Environ(),
		fmt.Sprintf("HOME=%s", home),
		fmt.Sprintf("USERPROFILE=%s", home), // windows
	)

	return command
}
