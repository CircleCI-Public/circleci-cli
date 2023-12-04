package local_test

import (
	"runtime"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
)

var pathCLI string

var _ = BeforeSuite(func() {
	var err error
	pathCLI, err = gexec.Build("github.com/CircleCI-Public/circleci-cli")
	Expect(err).ShouldNot(HaveOccurred())
})

var _ = AfterSuite(func() {
	gexec.CleanupBuildArtifacts()
})

func TestLocal(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Skipping test on non-linux OS")
	}
	RegisterFailHandler(Fail)
	RunSpecs(t, "Local Suite")
}
