package acceptance_test

import (
	"fmt"
	"os"
	"testing"

	"github.com/CircleCI-Public/circleci-cli-v2/internal/testing/binary"
)

func TestMain(m *testing.M) {
	path, err := binary.BuildBinary()
	if err != nil {
		fmt.Fprintf(os.Stderr, "skipping acceptance tests: %v\n", err)
		os.Exit(0)
	}
	fmt.Fprintf(os.Stderr, "built circleci binary: %s\n", path)
	os.Exit(m.Run())
}
