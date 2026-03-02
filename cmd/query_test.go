package cmd_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/CircleCI-Public/circleci-cli/testhelpers"
	"gotest.tools/v3/assert"
)

func TestQueryFromStdin(t *testing.T) {
	ts := testhelpers.WithTempSettings(t)
	binary := testhelpers.BuildCLI(t)

	token := "mytoken"
	responseData := `{
	"hero": {
		"name": "R2-D2",
		"friends": [
			{
				"name": "Luke Skywalker"
			},
			{
				"name": "Han Solo"
			},
			{
				"name": "Leia Organa"
			}
		]
	}
}`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, r.Method, "POST")
		assert.Equal(t, r.URL.Path, "/graphql-unstable")
		assert.Equal(t, r.Header.Get("Authorization"), token)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"data": ` + responseData + `}`))
	}))
	t.Cleanup(func() { server.Close() })

	query := `query {
	hero {
		name
		friends {
			name
		}
	}
}
`

	cmd := exec.Command(binary,
		"query", "-",
		"--skip-update-check",
		"--token", token,
		"--host", server.URL,
	)
	cmd.Stdin = strings.NewReader(query)
	cmd.Env = append(os.Environ(),
		fmt.Sprintf("HOME=%s", ts.Home),
		fmt.Sprintf("USERPROFILE=%s", ts.Home),
	)

	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	assert.NilError(t, err)
	assert.Equal(t, stderr.String(), "")

	// Compare as JSON to ignore whitespace differences.
	var expected, actual interface{}
	assert.NilError(t, json.Unmarshal([]byte(responseData), &expected))
	assert.NilError(t, json.Unmarshal([]byte(stdout.String()), &actual))
	assert.DeepEqual(t, actual, expected)
}
