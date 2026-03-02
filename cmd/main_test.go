package cmd_test

import (
	"os"
	"testing"

	"github.com/CircleCI-Public/circleci-cli/testhelpers"
)

func TestMain(m *testing.M) {
	code := m.Run()
	testhelpers.CleanupBuildArtifacts()
	os.Exit(code)
}
