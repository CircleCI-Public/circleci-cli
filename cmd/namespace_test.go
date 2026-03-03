package cmd_test

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

	"gotest.tools/v3/assert"

	"github.com/CircleCI-Public/circleci-cli/telemetry"
	"github.com/CircleCI-Public/circleci-cli/testhelpers"
)

// appendGQLPostHandler adds a handler to the test server that verifies a POST
// to /graphql-unstable with the given auth token and expected request JSON,
// then responds with the given response JSON wrapped in a data envelope.
func appendGQLPostHandler(t *testing.T, ts *testhelpers.TestServer, authToken string, expectedRequest, response string, errorResponse string) {
	t.Helper()
	responseBody := `{ "data": ` + response + `}`
	if errorResponse != "" {
		responseBody = fmt.Sprintf(`{ "data": %s, "errors": %s}`, response, errorResponse)
	}

	ts.AppendHandler(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, r.Method, "POST")
		assert.Equal(t, r.URL.Path, "/graphql-unstable")

		if authToken != "" {
			assert.DeepEqual(t, r.Header["Authorization"], []string{authToken})
		}

		body, err := io.ReadAll(r.Body)
		assert.NilError(t, err)
		defer r.Body.Close() //nolint:errcheck

		// Compare as JSON to ignore whitespace differences
		var expectedJSON, actualJSON interface{}
		assert.NilError(t, json.Unmarshal([]byte(expectedRequest), &expectedJSON))
		assert.NilError(t, json.Unmarshal(body, &actualJSON))

		expectedBytes, _ := json.Marshal(expectedJSON)
		actualBytes, _ := json.Marshal(actualJSON)
		assert.Equal(t, string(expectedBytes), string(actualBytes), "JSON request body mismatch")

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(responseBody))
	})
}

func TestNamespaceTelemetry(t *testing.T) {
	binary := testhelpers.BuildCLI(t)
	ts := testhelpers.WithTempSettings(t)
	token := "testtoken"

	ts.Server.AppendHandler(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"data":{"organization":{"name":"test-org","id":"bb604b45-b6b0-4b81-ad80-796f15eddf87"}}}`))
	})

	result := testhelpers.RunCLI(t, binary,
		[]string{
			"namespace", "create",
			"--skip-update-check",
			"--token", token,
			"--host", ts.Server.URL,
			"--integration-testing",
			"foo-ns",
			"--org-id", `"bb604b45-b6b0-4b81-ad80-796f15eddf87"`,
		},
		"HOME="+ts.Home,
		"USERPROFILE="+ts.Home,
		fmt.Sprintf("MOCK_TELEMETRY=%s", ts.TelemetryDestPath),
	)
	assert.Equal(t, result.ExitCode, 0, "stderr: %s\nstdout: %s", result.Stderr, result.Stdout)

	testhelpers.AssertTelemetrySubset(t, ts, []telemetry.Event{
		telemetry.CreateNamespaceEvent(telemetry.CommandInfo{
			Name: "create",
			LocalArgs: map[string]string{
				"integration-testing": "true",
				"org-id":              `"bb604b45-b6b0-4b81-ad80-796f15eddf87"`,
			},
		}),
	})
}

func TestNamespaceCreateWithOrgID(t *testing.T) {
	binary := testhelpers.BuildCLI(t)
	ts := testhelpers.WithTempSettings(t)
	token := "testtoken"

	gqlOrganizationResponse := `{
		"organization": {
			"name": "test-org",
			"id": "bb604b45-b6b0-4b81-ad80-796f15eddf87"
		}
	}`

	expectedOrganizationRequest := `{
		"query": "\n\t\t\tmutation($name: String!, $organizationId: UUID!) {\n\t\t\t\tcreateNamespace(\n\t\t\t\t\tname: $name,\n\t\t\t\t\torganizationId: $organizationId\n\t\t\t\t) {\n\t\t\t\t\tnamespace {\n\t\t\t\t\t\tid\n\t\t\t\t\t}\n\t\t\t\t\terrors {\n\t\t\t\t\t\tmessage\n\t\t\t\t\t\ttype\n\t\t\t\t\t}\n\t\t\t\t}\n\t\t\t}",
		"variables": {
			"name": "foo-ns",
			"organizationId": "\"bb604b45-b6b0-4b81-ad80-796f15eddf87\""
		}
	}`

	appendGQLPostHandler(t, ts.Server, token, expectedOrganizationRequest, gqlOrganizationResponse, "")

	result := testhelpers.RunCLI(t, binary,
		[]string{
			"namespace", "create",
			"--skip-update-check",
			"--token", token,
			"--host", ts.Server.URL,
			"--integration-testing",
			"foo-ns",
			"--org-id", `"bb604b45-b6b0-4b81-ad80-796f15eddf87"`,
		},
		"HOME="+ts.Home,
		"USERPROFILE="+ts.Home,
	)
	assert.Equal(t, result.ExitCode, 0, "stderr: %s\nstdout: %s", result.Stderr, result.Stdout)

	assert.Assert(t, strings.Contains(result.Stdout, `You are creating a namespace called "foo-ns".`))
	assert.Assert(t, strings.Contains(result.Stdout, `This is the only namespace permitted for your organization with id "bb604b45-b6b0-4b81-ad80-796f15eddf87".`))
	assert.Assert(t, strings.Contains(result.Stdout, "Are you sure you wish to create the namespace: `foo-ns`"))
	assert.Assert(t, strings.Contains(result.Stdout, "Namespace `foo-ns` created."))
	assert.Assert(t, strings.Contains(result.Stdout, "Please note that any orbs you publish in this namespace are open orbs and are world-readable."))
}

func TestNamespaceCreateWithOrgNameAndVcs(t *testing.T) {
	binary := testhelpers.BuildCLI(t)
	ts := testhelpers.WithTempSettings(t)
	token := "testtoken"

	gqlOrganizationResponse := `{
		"organization": {
			"name": "test-org",
			"id": "bb604b45-b6b0-4b81-ad80-796f15eddf87"
		}
	}`

	expectedOrganizationRequest := `{
		"query": "query($orgName: String!, $vcsType: VCSType!) {\n\torganization(name: $orgName, vcsType: $vcsType) {\n\t\tid\n\t\tname\n\t\tvcsType\n\t}\n}",
		"variables": {
			"orgName": "test-org",
			"vcsType": "BITBUCKET"
		}
	}`

	gqlNsResponse := `{
		"createNamespace": {
			"errors": [],
			"namespace": {
				"id": "bb604b45-b6b0-4b81-ad80-796f15eddf87"
			}
		}
	}`

	expectedNsRequest := `{
		"query": "\n\t\t\tmutation($name: String!, $organizationId: UUID!) {\n\t\t\t\tcreateNamespace(\n\t\t\t\t\tname: $name,\n\t\t\t\t\torganizationId: $organizationId\n\t\t\t\t) {\n\t\t\t\t\tnamespace {\n\t\t\t\t\t\tid\n\t\t\t\t\t}\n\t\t\t\t\terrors {\n\t\t\t\t\t\tmessage\n\t\t\t\t\t\ttype\n\t\t\t\t\t}\n\t\t\t\t}\n\t\t\t}",
		"variables": {
			"name": "foo-ns",
			"organizationId": "bb604b45-b6b0-4b81-ad80-796f15eddf87"
		}
	}`

	appendGQLPostHandler(t, ts.Server, token, expectedOrganizationRequest, gqlOrganizationResponse, "")
	appendGQLPostHandler(t, ts.Server, token, expectedNsRequest, gqlNsResponse, "")

	result := testhelpers.RunCLI(t, binary,
		[]string{
			"namespace", "create",
			"--skip-update-check",
			"--token", token,
			"--host", ts.Server.URL,
			"--integration-testing",
			"foo-ns",
			"BITBUCKET",
			"test-org",
		},
		"HOME="+ts.Home,
		"USERPROFILE="+ts.Home,
	)
	assert.Equal(t, result.ExitCode, 0, "stderr: %s\nstdout: %s", result.Stderr, result.Stdout)

	assert.Assert(t, strings.Contains(result.Stdout, `You are creating a namespace called "foo-ns".`))
	assert.Assert(t, strings.Contains(result.Stdout, "This is the only namespace permitted for your bitbucket organization, test-org."))
	assert.Assert(t, strings.Contains(result.Stdout, "Are you sure you wish to create the namespace: `foo-ns`"))
	assert.Assert(t, strings.Contains(result.Stdout, "Namespace `foo-ns` created."))
	assert.Assert(t, strings.Contains(result.Stdout, "Please note that any orbs you publish in this namespace are open orbs and are world-readable."))
}

func TestNamespaceCreateDuplicate(t *testing.T) {
	binary := testhelpers.BuildCLI(t)
	ts := testhelpers.WithTempSettings(t)
	token := "testtoken"

	gqlOrganizationResponse := `{
		"organization": {
			"name": "test-org",
			"id": "bb604b45-b6b0-4b81-ad80-796f15eddf87"
		}
	}`

	expectedOrganizationRequest := `{
		"query": "query($orgName: String!, $vcsType: VCSType!) {\n\torganization(name: $orgName, vcsType: $vcsType) {\n\t\tid\n\t\tname\n\t\tvcsType\n\t}\n}",
		"variables": {
			"orgName": "test-org",
			"vcsType": "BITBUCKET"
		}
	}`

	gqlNsResponse := `{
		"createNamespace": {
			"errors": [],
			"namespace": {
				"id": "bb604b45-b6b0-4b81-ad80-796f15eddf87"
			}
		}
	}`

	expectedNsRequest := `{
		"query": "\n\t\t\tmutation($name: String!, $organizationId: UUID!) {\n\t\t\t\tcreateNamespace(\n\t\t\t\t\tname: $name,\n\t\t\t\t\torganizationId: $organizationId\n\t\t\t\t) {\n\t\t\t\t\tnamespace {\n\t\t\t\t\t\tid\n\t\t\t\t\t}\n\t\t\t\t\terrors {\n\t\t\t\t\t\tmessage\n\t\t\t\t\t\ttype\n\t\t\t\t\t}\n\t\t\t\t}\n\t\t\t}",
		"variables": {
			"name": "foo-ns",
			"organizationId": "bb604b45-b6b0-4b81-ad80-796f15eddf87"
		}
	}`

	appendGQLPostHandler(t, ts.Server, token, expectedOrganizationRequest, gqlOrganizationResponse, "")
	appendGQLPostHandler(t, ts.Server, token, expectedNsRequest, gqlNsResponse, "")

	result := testhelpers.RunCLI(t, binary,
		[]string{
			"namespace", "create",
			"--skip-update-check",
			"--token", token,
			"--host", ts.Server.URL,
			"--integration-testing",
			"foo-ns",
			"BITBUCKET",
			"test-org",
		},
		"HOME="+ts.Home,
		"USERPROFILE="+ts.Home,
	)
	assert.Equal(t, result.ExitCode, 0, "stderr: %s\nstdout: %s", result.Stderr, result.Stdout)

	assert.Assert(t, strings.Contains(result.Stdout, `You are creating a namespace called "foo-ns".`))
	assert.Assert(t, strings.Contains(result.Stdout, "This is the only namespace permitted for your bitbucket organization, test-org."))
	assert.Assert(t, strings.Contains(result.Stdout, "Are you sure you wish to create the namespace: `foo-ns`"))
	assert.Assert(t, strings.Contains(result.Stdout, "Namespace `foo-ns` created."))
	assert.Assert(t, strings.Contains(result.Stdout, "Please note that any orbs you publish in this namespace are open orbs and are world-readable."))
}

func TestNamespaceCreateGraphQLErrors(t *testing.T) {
	binary := testhelpers.BuildCLI(t)
	ts := testhelpers.WithTempSettings(t)
	token := "testtoken"

	gqlOrganizationResponse := `{
		"organization": {
			"name": "test-org",
			"id": "bb604b45-b6b0-4b81-ad80-796f15eddf87"
		}
	}`

	expectedOrganizationRequest := `{
		"query": "query($orgName: String!, $vcsType: VCSType!) {\n\torganization(name: $orgName, vcsType: $vcsType) {\n\t\tid\n\t\tname\n\t\tvcsType\n\t}\n}",
		"variables": {
			"orgName": "test-org",
			"vcsType": "BITBUCKET"
		}
	}`

	gqlResponse := `{
		"createNamespace": {
			"errors": [
				{"message": "error1"},
				{"message": "error2"}
			],
			"namespace": null
		}
	}`

	gqlNativeErrors := `[ { "message": "ignored error" } ]`

	expectedRequestJSON := `{
		"query": "\n\t\t\tmutation($name: String!, $organizationId: UUID!) {\n\t\t\t\tcreateNamespace(\n\t\t\t\t\tname: $name,\n\t\t\t\t\torganizationId: $organizationId\n\t\t\t\t) {\n\t\t\t\t\tnamespace {\n\t\t\t\t\t\tid\n\t\t\t\t\t}\n\t\t\t\t\terrors {\n\t\t\t\t\t\tmessage\n\t\t\t\t\t\ttype\n\t\t\t\t\t}\n\t\t\t\t}\n\t\t\t}",
		"variables": {
			"name": "foo-ns",
			"organizationId": "bb604b45-b6b0-4b81-ad80-796f15eddf87"
		}
	}`

	appendGQLPostHandler(t, ts.Server, token, expectedOrganizationRequest, gqlOrganizationResponse, "")
	appendGQLPostHandler(t, ts.Server, token, expectedRequestJSON, gqlResponse, gqlNativeErrors)

	result := testhelpers.RunCLI(t, binary,
		[]string{
			"namespace", "create",
			"--skip-update-check",
			"--token", token,
			"--host", ts.Server.URL,
			"--integration-testing",
			"foo-ns",
			"BITBUCKET",
			"test-org",
		},
		"HOME="+ts.Home,
		"USERPROFILE="+ts.Home,
	)
	assert.Assert(t, result.ExitCode != 0)
	assert.Assert(t, strings.Contains(result.Stderr, "Error: error1"))
	assert.Assert(t, strings.Contains(result.Stderr, "error2"))
}
