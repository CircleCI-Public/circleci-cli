package cmd_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/CircleCI-Public/circleci-cli/settings"
	"github.com/CircleCI-Public/circleci-cli/testhelpers"
	"gotest.tools/v3/assert"
)

func TestCheckUpdateAutoCheckWithNewRelease(t *testing.T) {
	ts := testhelpers.WithTempSettings(t)

	updateCheck := &settings.UpdateCheck{
		LastUpdateCheck: time.Time{},
	}
	updateCheck.FileUsed = ts.UpdateFile
	err := updateCheck.WriteToDisk()
	assert.NilError(t, err)

	response := fmt.Sprintf(`
[
  {
    "id": 1,
    "tag_name": "v1.0.0",
    "name": "v1.0.0",
    "published_at": "2013-02-27T19:35:32Z",
    "assets": [
      {
        "id": 1,
        "name": "%s_%s.zip",
        "label": "short description",
        "content_type": "application/zip",
        "size": 1024
      }
    ]
  }
]
`, runtime.GOOS, runtime.GOARCH)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v3/repos/CircleCI-Public/circleci-cli/releases" {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(response))
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(func() { server.Close() })

	checkCLI := buildCLIWithLdflags(t,
		"-X github.com/CircleCI-Public/circleci-cli/cmd.AutoUpdate=false -X github.com/CircleCI-Public/circleci-cli/version.packageManager=release",
	)

	result := testhelpers.RunCLI(t, checkCLI,
		[]string{"help", "--skip-update-check=false", "--github-api", server.URL},
		fmt.Sprintf("HOME=%s", ts.Home),
		fmt.Sprintf("USERPROFILE=%s", ts.Home),
	)

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	assert.Assert(t, strings.Contains(result.Stderr, "You are running 0.0.0-dev"),
		"expected stderr to contain version info, got: %s", result.Stderr)
	assert.Assert(t, strings.Contains(result.Stderr, "A new release is available"),
		"expected stderr to contain new release info, got: %s", result.Stderr)
	assert.Assert(t, strings.Contains(result.Stderr, "You can update with `circleci update install`"),
		"expected stderr to contain update instructions, got: %s", result.Stderr)
}
