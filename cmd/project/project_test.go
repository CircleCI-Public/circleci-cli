package project_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/olekukonko/tablewriter"
	"github.com/spf13/cobra"
	"gotest.tools/v3/assert"

	"github.com/CircleCI-Public/circleci-cli/cmd/project"
	"github.com/CircleCI-Public/circleci-cli/cmd/validator"
	"github.com/CircleCI-Public/circleci-cli/settings"
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

func equalJSON(j1, j2 string) (bool, error) {
	var j1i, j2i interface{}
	if err := json.Unmarshal([]byte(j1), &j1i); err != nil {
		return false, fmt.Errorf("failed to convert in equalJSON from '%s': %w", j1, err)
	}
	if err := json.Unmarshal([]byte(j2), &j2i); err != nil {
		return false, fmt.Errorf("failed to convert in equalJSON from '%s': %w", j2, err)
	}
	return reflect.DeepEqual(j1i, j2i), nil
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

type testCreateSecretArgs struct {
	variableVal    string // ignored if --env-value flag is contained
	statusCodeGet  int
	statusCodePost int  // ignored if overwriting is canceled
	isOverwrite    bool // ignored if statusCodeGet is http.StatusNotFound
	extraArgs      []string
}

func TestCreateSecret(t *testing.T) {
	const (
		variableVal = "testvar1234"
		variableKey = "foo"
	)
	tests := []struct {
		name    string
		args    testCreateSecretArgs
		want    string
		wantErr bool
	}{
		{
			name: "Create successfully without an existing key",
			args: testCreateSecretArgs{
				variableVal:    variableVal,
				statusCodeGet:  http.StatusNotFound,
				statusCodePost: http.StatusOK,
				extraArgs:      []string{variableKey},
			},
			want: tableString(
				[]string{"Environment Variable", "Value"},
				[][]string{{"foo", "xxxx1234"}},
			),
		},
		{
			name: "Overwrite successfully with an existing key",
			args: testCreateSecretArgs{
				variableVal:    variableVal,
				statusCodeGet:  http.StatusOK,
				statusCodePost: http.StatusOK,
				isOverwrite:    true,
				extraArgs:      []string{variableKey},
			},
			want: tableString(
				[]string{"Environment Variable", "Value"},
				[][]string{{"foo", "xxxx1234"}},
			),
		},
		{
			name: "Cancel overwriting an existing key",
			args: testCreateSecretArgs{
				variableVal:   variableVal,
				statusCodeGet: http.StatusOK,
				isOverwrite:   false,
				extraArgs:     []string{variableKey},
			},
			want: fmt.Sprintln("Canceled"),
		},
		{
			name: "Pass a variable through a commandline argument",
			args: testCreateSecretArgs{
				statusCodeGet:  http.StatusNotFound,
				statusCodePost: http.StatusOK,
				extraArgs:      []string{variableKey, "--env-value", variableVal},
			},
			want: tableString(
				[]string{"Environment Variable", "Value"},
				[][]string{{"foo", "xxxx1234"}},
			),
		},
		{
			name: "Handle an error request from GetEnvironmentVariable",
			args: testCreateSecretArgs{
				variableVal:    variableVal,
				statusCodeGet:  http.StatusInternalServerError,
				statusCodePost: http.StatusOK,
				extraArgs:      []string{variableKey},
			},
			wantErr: true,
		},
		{
			name: "Handle an error request from CreateEnvironmentVariable",
			args: testCreateSecretArgs{
				variableVal:    variableVal,
				statusCodeGet:  http.StatusNotFound,
				statusCodePost: http.StatusInternalServerError,
				extraArgs:      []string{variableKey},
			},
			wantErr: true,
		},
		{
			name: "The process should be rejected if the passed value is empty",
			args: testCreateSecretArgs{
				variableVal:    "",
				statusCodeGet:  http.StatusNotFound,
				statusCodePost: http.StatusOK,
				extraArgs:      []string{variableKey},
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := testCreateSecret(t, &tt.args)
			if (err != nil) != tt.wantErr {
				t.Errorf("Create secret command: error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Create secret command: got = %v, want %v", got, tt.want)
			}
		})
	}
}

type testInputReader struct {
	secret string
	yesNo  bool
}

func (s testInputReader) ReadSecretString(msg string) (string, error) {
	return s.secret, nil
}

func (s testInputReader) AskConfirm(msg string) bool {
	return s.yesNo
}

func testCreateSecret(t *testing.T, args *testCreateSecretArgs) (string, error) {
	const apiResponseBody = `{
		"name": "foo",
		"value": "xxxx1234"
	}`
	var handler http.HandlerFunc = func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case "GET":
			assert.Equal(t, r.URL.String(), fmt.Sprintf("/project/%s/%s/%s/envvar/foo", vcsType, orgName, projectName))
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(args.statusCodeGet)
			if args.statusCodeGet == http.StatusOK {
				_, err := w.Write([]byte(apiResponseBody))
				assert.NilError(t, err)
			}
		case "POST":
			expect := `{
				"name": "foo",
				"value": "testvar1234"
			}`
			assert.Equal(t, r.URL.String(), fmt.Sprintf("/project/%s/%s/%s/envvar", vcsType, orgName, projectName))
			isRequestBodyValid(t, r, expect)

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(args.statusCodePost)
			if args.statusCodePost == http.StatusOK {
				_, err := w.Write([]byte(apiResponseBody))
				assert.NilError(t, err)
			}
		}
	}

	server := httptest.NewServer(handler)
	defer server.Close()

	cmd, stdout, _ := scaffoldCMD(
		server.URL,
		func(cmd *cobra.Command, args []string) error {
			return nil
		},
		project.CustomReader(testInputReader{
			secret: args.variableVal,
			yesNo:  args.isOverwrite,
		}),
	)
	cmd.SetArgs(append(getCreateSecretArgBase(), args.extraArgs...))

	err := cmd.Execute()
	if err != nil {
		return "", err
	}

	return stdout.String(), nil
}

func getCreateSecretArgBase() []string {
	return []string{
		"secret",
		"create",
		vcsType,
		orgName,
		projectName,
	}
}

func isRequestBodyValid(t *testing.T, r *http.Request, expect string) {
	b, err := io.ReadAll(r.Body)
	assert.NilError(t, err)
	eq, err := equalJSON(string(b), expect)
	assert.NilError(t, err)
	assert.Equal(t, eq, true)
}

func scaffoldCMD(
	baseURL string,
	validator validator.Validator,
	opts ...project.ProjectOption,
) (*cobra.Command, *bytes.Buffer, *bytes.Buffer) {
	config := &settings.Config{
		Token:      "testtoken",
		HTTPClient: http.DefaultClient,
		Host:       baseURL,
		DlHost:     baseURL,
	}
	cmd := project.NewProjectCommand(config, validator, opts...)

	stdout := new(bytes.Buffer)
	stderr := new(bytes.Buffer)
	cmd.SetOut(stdout)
	cmd.SetErr(stderr)

	return cmd, stdout, stderr
}

func TestDLCPurge(t *testing.T) {
	noValidator := func(_ *cobra.Command, _ []string) error {
		return nil
	}

	t.Run("Happy path", func(t *testing.T) {
		handlers := []http.HandlerFunc{
			func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, r.Method, "GET")
				assert.Equal(t, r.URL.String(), fmt.Sprintf("/project/%s/%s/%s", "gh", "whom", "what"))
				assert.DeepEqual(t, r.Header["Circle-Token"], []string{"testtoken"})
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				_, err := w.Write([]byte(`{ "id": "this-is-the-project-id" }`))
				assert.NilError(t, err)
			},
			func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, r.Method, "DELETE")
				assert.Equal(t, r.URL.String(), "/private/output/project/this-is-the-project-id/dlc")
				assert.DeepEqual(t, r.Header["Circle-Token"], []string{"testtoken"})
				w.WriteHeader(http.StatusOK)
			},
		}
		var h http.HandlerFunc = func(w http.ResponseWriter, r *http.Request) {
			var handler http.HandlerFunc
			handler, handlers = handlers[0], handlers[1:]
			handler(w, r)
		}
		server := httptest.NewServer(h)
		defer server.Close()
		cmd, outbuf, errbuf := scaffoldCMD(server.URL, noValidator)
		cmd.SetArgs([]string{"dlc", "purge", "gh", "whom", "what"})
		err := cmd.Execute()
		assert.NilError(t, err)
		assert.Equal(t, outbuf.String(), "Purged DLC for project\n")
		assert.Equal(t, errbuf.String(), "")
	})
	t.Run("Gone", func(t *testing.T) {
		handlers := []http.HandlerFunc{
			func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				_, err := w.Write([]byte(`{ "id": "this-is-the-project-id" }`))
				assert.NilError(t, err)
			},
			func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusGone)
			},
		}
		var h http.HandlerFunc = func(w http.ResponseWriter, r *http.Request) {
			var handler http.HandlerFunc
			handler, handlers = handlers[0], handlers[1:]
			handler(w, r)
		}
		server := httptest.NewServer(h)
		defer server.Close()
		cmd, outbuf, errbuf := scaffoldCMD(server.URL, noValidator)
		cmd.SetArgs([]string{"dlc", "purge", "gh", "whom", "what"})
		err := cmd.Execute()
		assert.Assert(t, err != nil)

		assert.Equal(t, outbuf.String(), "")
		assert.Equal(t, errbuf.String(),
			"Error: No longer supported.\n"+
				"This functionality is no longer supported by this version of the circleci CLI.\n"+
				"Please upgrade to the latest version of the circleci CLI.\n",
		)
	})
	t.Run("Not cloud", func(t *testing.T) {
		// (this test doesn't use httptest because it's testing a
		// misconfiguration and doesn't get as far as making a http request)
		cmd := project.NewProjectCommand(&settings.Config{
			Host:       "some custom value but dlhost is not set",
			HTTPClient: http.DefaultClient,
		}, noValidator)
		outbuf := new(bytes.Buffer)
		errbuf := new(bytes.Buffer)
		cmd.SetOut(outbuf)
		cmd.SetErr(errbuf)
		cmd.SetArgs([]string{"dlc", "purge", "gh", "whom", "what"})
		err := cmd.Execute()
		assert.Assert(t, err != nil)
		assert.Equal(t, outbuf.String(), "")
		assert.Equal(t, errbuf.String(),
			"Error: Misconfiguration.\n"+
				"You have configured a custom API endpoint host for the circleci CLI.\n"+
				"However, this functionality is only supported on circleci.com API endpoints.\n",
		)
	})
}
