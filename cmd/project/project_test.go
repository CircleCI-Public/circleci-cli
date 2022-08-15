package project_test

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/CircleCI-Public/circleci-cli/cmd/project"
	"github.com/CircleCI-Public/circleci-cli/cmd/validator"
	"github.com/CircleCI-Public/circleci-cli/settings"
	"github.com/olekukonko/tablewriter"
	"github.com/spf13/cobra"
	"gotest.tools/v3/assert"
)

const (
	vcsType     = "github"
	orgName     = "test-org"
	projectName = "test-project"
)

func tableString(header []string, rows [][]string) string {
	res := &strings.Builder{}
	table := tablewriter.NewWriter(res)
	table.SetHeader(header)
	for _, r := range rows {
		table.Append(r)
	}
	table.Render()
	return res.String()
}

func getListProjectsArg() []string {
	return []string{
		"secret",
		"list",
		vcsType,
		orgName,
		projectName,
	}
}

func TestListSecrets(t *testing.T) {
	var handler http.HandlerFunc = func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, r.Method, "GET")
		assert.Equal(t, r.URL.String(), fmt.Sprintf("/project/%s/%s/%s/envvar", vcsType, orgName, projectName))
		response := `{
			"items": [{
				"name": "foo",
				"value": "xxxx1234"
			}],
			"next_page_token": ""
		}`
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, err := w.Write([]byte(response))
		assert.NilError(t, err)
	}
	server := httptest.NewServer(handler)
	defer server.Close()

	cmd, stdout, _ := scaffoldCMD(
		server.URL,
		func(cmd *cobra.Command, args []string) error {
			return nil
		},
	)
	cmd.SetArgs(getListProjectsArg())
	err := cmd.Execute()
	assert.NilError(t, err)

	expect := tableString(
		[]string{"Environment Variable", "Value"},
		[][]string{{"foo", "xxxx1234"}},
	)
	res := stdout.String()
	assert.Equal(t, res, expect)
}

func TestListSecretsErrorWithValidator(t *testing.T) {
	const errorMsg = "validator error"
	var handler http.HandlerFunc = func(_ http.ResponseWriter, _ *http.Request) {}
	server := httptest.NewServer(handler)
	defer server.Close()

	cmd, _, _ := scaffoldCMD(
		server.URL,
		func(_ *cobra.Command, _ []string) error {
			return fmt.Errorf(errorMsg)
		},
	)
	cmd.SetArgs(getListProjectsArg())
	err := cmd.Execute()
	assert.Error(t, err, errorMsg)
}

func TestListSecretsErrorWithAPIResponse(t *testing.T) {
	const errorMsg = "api error"
	var handler http.HandlerFunc = func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, err := w.Write([]byte(fmt.Sprintf(`{"message": "%s"}`, errorMsg)))
		assert.NilError(t, err)
	}
	server := httptest.NewServer(handler)
	defer server.Close()

	cmd, _, _ := scaffoldCMD(
		server.URL,
		func(cmd *cobra.Command, args []string) error {
			return nil
		},
	)
	cmd.SetArgs(getListProjectsArg())
	err := cmd.Execute()
	assert.Error(t, err, errorMsg)
}

func scaffoldCMD(
	baseURL string,
	validator validator.Validator,
) (*cobra.Command, *bytes.Buffer, *bytes.Buffer) {
	config := &settings.Config{
		Token:      "testtoken",
		HTTPClient: http.DefaultClient,
		Host:       baseURL,
	}
	cmd := project.NewProjectCommand(config, validator)

	stdout := new(bytes.Buffer)
	stderr := new(bytes.Buffer)
	cmd.SetOut(stdout)
	cmd.SetErr(stderr)

	return cmd, stdout, stderr
}
