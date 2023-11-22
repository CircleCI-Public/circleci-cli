package pipeline

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/spf13/cobra"
	"gotest.tools/v3/golden"

	"github.com/CircleCI-Public/circleci-cli/settings"
)

func TestUsage(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer ts.Close()

	preRunE := func(cmd *cobra.Command, args []string) error { return nil }
	cmd := NewCommand(&settings.Config{HTTPClient: ts.Client()}, preRunE)
	testSubCommandUsage(t, cmd.Name(), cmd)
}

func testSubCommandUsage(t *testing.T, prefix string, parent *cobra.Command) {
	t.Helper()
	t.Run(parent.Name(), func(t *testing.T) {
		golden.Assert(t, parent.UsageString(), fmt.Sprintf("%s-expected-usage.txt", prefix))
		for _, cmd := range parent.Commands() {
			testSubCommandUsage(t, fmt.Sprintf("%s/%s", prefix, cmd.Name()), cmd)
		}
	})
}
