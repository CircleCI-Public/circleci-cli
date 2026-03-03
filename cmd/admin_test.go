package cmd_test

import (
	"fmt"
	"net/http"
	"strings"
	"testing"

	"gotest.tools/v3/assert"

	"github.com/CircleCI-Public/circleci-cli/testhelpers"
)

// respondGQLData returns an http.HandlerFunc that writes a JSON response
// wrapped in {"data": ...}, matching what clitest.AppendPostHandler did.
func respondGQLData(status int, jsonBody string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = fmt.Fprintf(w, `{ "data": %s}`, jsonBody)
	}
}

func TestAdminDeleteNamespaceAlias_UnexpectedFailure(t *testing.T) {
	binary := testhelpers.BuildCLI(t)
	ts := testhelpers.WithTempSettings(t)
	token := "testtoken"

	gqlResponse := `{
		"deleteNamespaceAlias": {
			"errors": [],
			"deleted": false
		}
	}`
	expectedRequest := `{
		"query": "\nmutation($name: String!) {\n  deleteNamespaceAlias(name: $name) {\n    deleted\n    errors {\n      type\n      message\n    }\n  }\n}\n",
		"variables": {
			"name": "foo-ns"
		}
	}`

	ts.Server.AppendHandler(testhelpers.ChainHandlers(
		testhelpers.VerifyGQLRequest(t, token, expectedRequest),
		respondGQLData(http.StatusOK, gqlResponse),
	))

	result := testhelpers.RunCLI(t, binary, []string{
		"admin", "delete-namespace-alias",
		"--skip-update-check",
		"--token", token,
		"--host", ts.Server.URL,
		"--integration-testing",
		"foo-ns",
	}, fmt.Sprintf("HOME=%s", ts.Home), fmt.Sprintf("USERPROFILE=%s", ts.Home))

	assert.Assert(t, result.ExitCode != 0, "expected non-zero exit code, got %d", result.ExitCode)
	assert.Assert(t, strings.Contains(result.Stderr, "namespace alias deletion failed for unknown reasons"),
		"expected stderr to contain failure message, got: %s", result.Stderr)
}

func TestAdminDeleteNamespaceAlias_GraphQLErrors(t *testing.T) {
	binary := testhelpers.BuildCLI(t)
	ts := testhelpers.WithTempSettings(t)
	token := "testtoken"

	gqlResponse := `{
		"deleteNamespaceAlias": {
			"errors": [{"message": "error1"}],
			"deleted": false
		}
	}`
	expectedRequest := `{
		"query": "\nmutation($name: String!) {\n  deleteNamespaceAlias(name: $name) {\n    deleted\n    errors {\n      type\n      message\n    }\n  }\n}\n",
		"variables": {
			"name": "foo-ns"
		}
	}`

	ts.Server.AppendHandler(testhelpers.ChainHandlers(
		testhelpers.VerifyGQLRequest(t, token, expectedRequest),
		respondGQLData(http.StatusOK, gqlResponse),
	))

	result := testhelpers.RunCLI(t, binary, []string{
		"admin", "delete-namespace-alias",
		"--skip-update-check",
		"--token", token,
		"--host", ts.Server.URL,
		"--integration-testing",
		"foo-ns",
	}, fmt.Sprintf("HOME=%s", ts.Home), fmt.Sprintf("USERPROFILE=%s", ts.Home))

	assert.Assert(t, result.ExitCode != 0, "expected non-zero exit code, got %d", result.ExitCode)
	assert.Assert(t, strings.Contains(result.Stderr, "Error: error1"),
		"expected stderr to contain error1, got: %s", result.Stderr)
}

func TestAdminDeleteNamespaceAlias_Success(t *testing.T) {
	binary := testhelpers.BuildCLI(t)
	ts := testhelpers.WithTempSettings(t)
	token := "testtoken"

	gqlResponse := `{
		"deleteNamespaceAlias": {
			"errors": [],
			"deleted": true
		}
	}`
	expectedRequest := `{
		"query": "\nmutation($name: String!) {\n  deleteNamespaceAlias(name: $name) {\n    deleted\n    errors {\n      type\n      message\n    }\n  }\n}\n",
		"variables": {
			"name": "foo-ns"
		}
	}`

	ts.Server.AppendHandler(testhelpers.ChainHandlers(
		testhelpers.VerifyGQLRequest(t, token, expectedRequest),
		respondGQLData(http.StatusOK, gqlResponse),
	))

	result := testhelpers.RunCLI(t, binary, []string{
		"admin", "delete-namespace-alias",
		"--skip-update-check",
		"--token", token,
		"--host", ts.Server.URL,
		"--integration-testing",
		"foo-ns",
	}, fmt.Sprintf("HOME=%s", ts.Home), fmt.Sprintf("USERPROFILE=%s", ts.Home))

	assert.Equal(t, result.ExitCode, 0, "expected exit code 0, got %d\nstderr: %s", result.ExitCode, result.Stderr)
}

func TestAdminDeleteNamespace_NotFound(t *testing.T) {
	binary := testhelpers.BuildCLI(t)
	ts := testhelpers.WithTempSettings(t)
	token := "testtoken"

	gqlRegistryNsResponse := `{
		"registryNamespace": {
			"id": ""
		}
	}`
	expectedRegistryNsRequest := `{
		"query": "\n\t\t\t\tquery($name: String!) {\n\t\t\t\t\tregistryNamespace(\n\t\t\t\t\t\tname: $name\n\t\t\t\t\t){\n\t\t\t\t\t\tid\n\t\t\t\t\t}\n\t\t\t }",
		"variables": {
		  "name": "foo-ns"
		}
	}`

	ts.Server.AppendHandler(testhelpers.ChainHandlers(
		testhelpers.VerifyGQLRequest(t, token, expectedRegistryNsRequest),
		respondGQLData(http.StatusOK, gqlRegistryNsResponse),
	))

	result := testhelpers.RunCLI(t, binary, []string{
		"admin", "delete-namespace",
		"--skip-update-check",
		"--token", token,
		"--host", ts.Server.URL,
		"--integration-testing",
		"foo-ns",
	}, fmt.Sprintf("HOME=%s", ts.Home), fmt.Sprintf("USERPROFILE=%s", ts.Home))

	assert.Assert(t, result.ExitCode != 0, "expected non-zero exit code, got %d", result.ExitCode)
	assert.Assert(t, strings.Contains(result.Stderr, "the namespace 'foo-ns' does not exist"),
		"expected stderr to contain namespace not found message, got: %s", result.Stderr)
}

func TestAdminDeleteNamespace_ListOrbsError(t *testing.T) {
	binary := testhelpers.BuildCLI(t)
	ts := testhelpers.WithTempSettings(t)
	token := "testtoken"

	gqlRegistryNsResponse := `{
		"registryNamespace": {
			"id": "f13a9e13-538c-435c-8f61-78596661acd6"
		}
	}`
	expectedRegistryNsRequest := `{
		"query": "\n\t\t\t\tquery($name: String!) {\n\t\t\t\t\tregistryNamespace(\n\t\t\t\t\t\tname: $name\n\t\t\t\t\t){\n\t\t\t\t\t\tid\n\t\t\t\t\t}\n\t\t\t }",
		"variables": {
		  "name": "foo-ns"
		}
	}`

	gqlListNamespaceOrbsResponse := `{
		"orbs": [
			{"name": "test-orb-1"},
			{"name": "test-orb-2"}
		]
	}`
	expectedListOrbsRequest := `{
		"query": "\nquery namespaceOrbs ($namespace: String, $after: String!, $view: OrbListViewType) {\n\tregistryNamespace(name: $namespace) {\n\t\tname\n                id\n\t\torbs(first: 20, after: $after, view: $view) {\n\t\t\tedges {\n\t\t\t\tcursor\n\t\t\t\tnode {\n\t\t\t\t\tversions { version\n\t\t\t\t\t}\n\t\t\t\t\tname\n\t                                statistics {\n\t\t                           last30DaysBuildCount,\n\t\t                           last30DaysProjectCount,\n\t\t                           last30DaysOrganizationCount\n\t                               }\n\t\t\t\t}\n\t\t\t}\n\t\t\ttotalCount\n\t\t\tpageInfo {\n\t\t\t\thasNextPage\n\t\t\t}\n\t\t}\n\t}\n}\n",
		"variables": {
		  "after": "",
		  "namespace": "foo-ns",
		  "view": "PUBLIC_ONLY"
		}
	}`

	ts.Server.AppendHandler(testhelpers.ChainHandlers(
		testhelpers.VerifyGQLRequest(t, token, expectedRegistryNsRequest),
		respondGQLData(http.StatusOK, gqlRegistryNsResponse),
	))
	ts.Server.AppendHandler(testhelpers.ChainHandlers(
		testhelpers.VerifyGQLRequest(t, token, expectedListOrbsRequest),
		respondGQLData(http.StatusOK, gqlListNamespaceOrbsResponse),
	))

	result := testhelpers.RunCLI(t, binary, []string{
		"admin", "delete-namespace",
		"--skip-update-check",
		"--token", token,
		"--host", ts.Server.URL,
		"--integration-testing",
		"foo-ns",
	}, fmt.Sprintf("HOME=%s", ts.Home), fmt.Sprintf("USERPROFILE=%s", ts.Home))

	assert.Assert(t, result.ExitCode != 0, "expected non-zero exit code, got %d", result.ExitCode)
	assert.Assert(t, strings.Contains(result.Stderr, "unable to list orbs: no namespace found"),
		"expected stderr to contain orb list error, got: %s", result.Stderr)
}

func TestAdminDeleteNamespace_DeleteError(t *testing.T) {
	binary := testhelpers.BuildCLI(t)
	ts := testhelpers.WithTempSettings(t)
	token := "testtoken"

	gqlRegistryNsResponse := `{
		"registryNamespace": {
			"id": "f13a9e13-538c-435c-8f61-78596661acd6"
		}
	}`
	expectedRegistryNsRequest := `{
		"query": "\n\t\t\t\tquery($name: String!) {\n\t\t\t\t\tregistryNamespace(\n\t\t\t\t\t\tname: $name\n\t\t\t\t\t){\n\t\t\t\t\t\tid\n\t\t\t\t\t}\n\t\t\t }",
		"variables": {
		  "name": "foo-ns"
		}
	}`

	gqlListNamespaceOrbsResponse := `{
		"registryNamespace": {
			"id": "f13a9e13-538c-435c-8f61-78596661acd6",
			"orbs": {
				"edges": [
					{
						"node": {
							"name": "test-orb-1"
						}
					}
				]
			}
		}
	}`
	expectedListOrbsRequest := `{
		"query": "\nquery namespaceOrbs ($namespace: String, $after: String!, $view: OrbListViewType) {\n\tregistryNamespace(name: $namespace) {\n\t\tname\n                id\n\t\torbs(first: 20, after: $after, view: $view) {\n\t\t\tedges {\n\t\t\t\tcursor\n\t\t\t\tnode {\n\t\t\t\t\tversions { version\n\t\t\t\t\t}\n\t\t\t\t\tname\n\t                                statistics {\n\t\t                           last30DaysBuildCount,\n\t\t                           last30DaysProjectCount,\n\t\t                           last30DaysOrganizationCount\n\t                               }\n\t\t\t\t}\n\t\t\t}\n\t\t\ttotalCount\n\t\t\tpageInfo {\n\t\t\t\thasNextPage\n\t\t\t}\n\t\t}\n\t}\n}\n",
		"variables": {
		  "after": "",
		  "namespace": "foo-ns",
		  "view": "PUBLIC_ONLY"
		}
	}`

	gqlDeleteNamespaceResponse := `{
		"deleteNamespaceAndRelatedOrbs": {
			"deleted": false,
			"errors": [{"message": "test"}]
		}
	}`
	expectedDeleteNamespaceRequest := `{
		"query": "\nmutation($id: UUID!) {\n  deleteNamespaceAndRelatedOrbs(namespaceId: $id) {\n    deleted\n    errors {\n      type\n      message\n    }\n  }\n}\n",
		"variables": {
		  "id": "f13a9e13-538c-435c-8f61-78596661acd6"
		}
	}`

	ts.Server.AppendHandler(testhelpers.ChainHandlers(
		testhelpers.VerifyGQLRequest(t, token, expectedRegistryNsRequest),
		respondGQLData(http.StatusOK, gqlRegistryNsResponse),
	))
	ts.Server.AppendHandler(testhelpers.ChainHandlers(
		testhelpers.VerifyGQLRequest(t, token, expectedListOrbsRequest),
		respondGQLData(http.StatusOK, gqlListNamespaceOrbsResponse),
	))
	ts.Server.AppendHandler(testhelpers.ChainHandlers(
		testhelpers.VerifyGQLRequest(t, token, expectedDeleteNamespaceRequest),
		respondGQLData(http.StatusOK, gqlDeleteNamespaceResponse),
	))

	result := testhelpers.RunCLI(t, binary, []string{
		"admin", "delete-namespace",
		"--skip-update-check",
		"--token", token,
		"--host", ts.Server.URL,
		"--integration-testing",
		"foo-ns",
	}, fmt.Sprintf("HOME=%s", ts.Home), fmt.Sprintf("USERPROFILE=%s", ts.Home))

	assert.Assert(t, result.ExitCode != 0, "expected non-zero exit code, got %d", result.ExitCode)
	assert.Assert(t, strings.Contains(result.Stderr, "Error: test"),
		"expected stderr to contain error message, got: %s", result.Stderr)
}

func TestAdminDeleteNamespace_Success(t *testing.T) {
	binary := testhelpers.BuildCLI(t)
	ts := testhelpers.WithTempSettings(t)
	token := "testtoken"

	gqlRegistryNsResponse := `{
		"registryNamespace": {
			"id": "f13a9e13-538c-435c-8f61-78596661acd6"
		}
	}`
	expectedRegistryNsRequest := `{
		"query": "\n\t\t\t\tquery($name: String!) {\n\t\t\t\t\tregistryNamespace(\n\t\t\t\t\t\tname: $name\n\t\t\t\t\t){\n\t\t\t\t\t\tid\n\t\t\t\t\t}\n\t\t\t }",
		"variables": {
		  "name": "foo-ns"
		}
	}`

	gqlListNamespaceOrbsResponse := `{
		"registryNamespace": {
			"id": "f13a9e13-538c-435c-8f61-78596661acd6",
			"orbs": {
				"edges": [
					{
						"node": {
							"name": "test-orb-1"
						}
					}
				]
			}
		}
	}`
	expectedListOrbsRequest := `{
		"query": "\nquery namespaceOrbs ($namespace: String, $after: String!, $view: OrbListViewType) {\n\tregistryNamespace(name: $namespace) {\n\t\tname\n                id\n\t\torbs(first: 20, after: $after, view: $view) {\n\t\t\tedges {\n\t\t\t\tcursor\n\t\t\t\tnode {\n\t\t\t\t\tversions { version\n\t\t\t\t\t}\n\t\t\t\t\tname\n\t                                statistics {\n\t\t                           last30DaysBuildCount,\n\t\t                           last30DaysProjectCount,\n\t\t                           last30DaysOrganizationCount\n\t                               }\n\t\t\t\t}\n\t\t\t}\n\t\t\ttotalCount\n\t\t\tpageInfo {\n\t\t\t\thasNextPage\n\t\t\t}\n\t\t}\n\t}\n}\n",
		"variables": {
		  "after": "",
		  "namespace": "foo-ns",
		  "view": "PUBLIC_ONLY"
		}
	}`

	gqlDeleteNamespaceResponse := `{
		"deleteNamespaceAndRelatedOrbs": {
			"deleted": true
		}
	}`
	expectedDeleteNamespaceRequest := `{
		"query": "\nmutation($id: UUID!) {\n  deleteNamespaceAndRelatedOrbs(namespaceId: $id) {\n    deleted\n    errors {\n      type\n      message\n    }\n  }\n}\n",
		"variables": {
		  "id": "f13a9e13-538c-435c-8f61-78596661acd6"
		}
	}`

	ts.Server.AppendHandler(testhelpers.ChainHandlers(
		testhelpers.VerifyGQLRequest(t, token, expectedRegistryNsRequest),
		respondGQLData(http.StatusOK, gqlRegistryNsResponse),
	))
	ts.Server.AppendHandler(testhelpers.ChainHandlers(
		testhelpers.VerifyGQLRequest(t, token, expectedListOrbsRequest),
		respondGQLData(http.StatusOK, gqlListNamespaceOrbsResponse),
	))
	ts.Server.AppendHandler(testhelpers.ChainHandlers(
		testhelpers.VerifyGQLRequest(t, token, expectedDeleteNamespaceRequest),
		respondGQLData(http.StatusOK, gqlDeleteNamespaceResponse),
	))

	result := testhelpers.RunCLI(t, binary, []string{
		"admin", "delete-namespace",
		"--skip-update-check",
		"--token", token,
		"--host", ts.Server.URL,
		"--integration-testing",
		"foo-ns",
	}, fmt.Sprintf("HOME=%s", ts.Home), fmt.Sprintf("USERPROFILE=%s", ts.Home))

	assert.Equal(t, result.ExitCode, 0, "expected exit code 0, got %d\nstderr: %s", result.ExitCode, result.Stderr)
}

func TestAdminRenameNamespace_Success(t *testing.T) {
	binary := testhelpers.BuildCLI(t)
	ts := testhelpers.WithTempSettings(t)
	token := "testtoken"

	gqlGetNsResponse := `{
		"errors": [],
		"registryNamespace": {
			"id": "bb604b45-b6b0-4b81-ad80-796f15eddf87"
		}
	}`
	expectedGetNsRequest := `{
		"query": "\n\t\t\t\tquery($name: String!) {\n\t\t\t\t\tregistryNamespace(\n\t\t\t\t\t\tname: $name\n\t\t\t\t\t){\n\t\t\t\t\t\tid\n\t\t\t\t\t}\n\t\t\t }",
		"variables": {
			"name": "ns-0"
		}
	}`
	expectedRenameRequest := `{
		"query": "\n\t\tmutation($namespaceId: UUID!, $newName: String!){\n\t\t\trenameNamespace(\n\t\t\t\tnamespaceId: $namespaceId,\n\t\t\t\tnewName: $newName\n\t\t\t){\n\t\t\t\tnamespace {\n\t\t\t\t\tid\n\t\t\t\t}\n\t\t\t\terrors {\n\t\t\t\t\tmessage\n\t\t\t\t\ttype\n\t\t\t\t}\n\t\t\t}\n\t\t}",
		"variables": {"newName": "ns-1", "namespaceId": "bb604b45-b6b0-4b81-ad80-796f15eddf87"}
	}`

	gqlRenameResponse := `{"data":{"renameNamespace":{"namespace":{"id":"4e377fe3-330d-4e4c-af62-821850fe9595"},"errors":[]}}}`

	ts.Server.AppendHandler(testhelpers.ChainHandlers(
		testhelpers.VerifyGQLRequest(t, token, expectedGetNsRequest),
		respondGQLData(http.StatusOK, gqlGetNsResponse),
	))
	ts.Server.AppendHandler(testhelpers.ChainHandlers(
		testhelpers.VerifyGQLRequest(t, token, expectedRenameRequest),
		respondGQLData(http.StatusOK, gqlRenameResponse),
	))

	result := testhelpers.RunCLI(t, binary, []string{
		"admin", "rename-namespace",
		"ns-0", "ns-1",
		"--skip-update-check",
		"--token", token,
		"--host", ts.Server.URL,
		"--no-prompt",
	}, fmt.Sprintf("HOME=%s", ts.Home), fmt.Sprintf("USERPROFILE=%s", ts.Home))

	assert.Equal(t, result.ExitCode, 0, "expected exit code 0, got %d\nstderr: %s", result.ExitCode, result.Stderr)
	assert.Assert(t, strings.Contains(result.Stdout, "Namespace `ns-0` renamed to `ns-1`"),
		"expected stdout to contain rename message, got: %s", result.Stdout)
}

func TestAdminRenameNamespace_Error(t *testing.T) {
	binary := testhelpers.BuildCLI(t)
	ts := testhelpers.WithTempSettings(t)
	token := "testtoken"

	gqlGetNsResponse := `{
		"errors": [],
		"registryNamespace": {
			"id": "bb604b45-b6b0-4b81-ad80-796f15eddf87"
		}
	}`
	expectedGetNsRequest := `{
		"query": "\n\t\t\t\tquery($name: String!) {\n\t\t\t\t\tregistryNamespace(\n\t\t\t\t\t\tname: $name\n\t\t\t\t\t){\n\t\t\t\t\t\tid\n\t\t\t\t\t}\n\t\t\t }",
		"variables": {
			"name": "ns-0"
		}
	}`
	expectedRenameRequest := `{
		"query": "\n\t\tmutation($namespaceId: UUID!, $newName: String!){\n\t\t\trenameNamespace(\n\t\t\t\tnamespaceId: $namespaceId,\n\t\t\t\tnewName: $newName\n\t\t\t){\n\t\t\t\tnamespace {\n\t\t\t\t\tid\n\t\t\t\t}\n\t\t\t\terrors {\n\t\t\t\t\tmessage\n\t\t\t\t\ttype\n\t\t\t\t}\n\t\t\t}\n\t\t}",
		"variables": {"newName": "ns-1", "namespaceId": "bb604b45-b6b0-4b81-ad80-796f15eddf87"}
	}`

	gqlRenameResponse := `{
		"renameNamespace": {
			"errors": [
				{"message": "error1"},
				{"message": "error2"}
			],
			"namespace": null
		}
	}`

	ts.Server.AppendHandler(testhelpers.ChainHandlers(
		testhelpers.VerifyGQLRequest(t, token, expectedGetNsRequest),
		respondGQLData(http.StatusOK, gqlGetNsResponse),
	))
	ts.Server.AppendHandler(testhelpers.ChainHandlers(
		testhelpers.VerifyGQLRequest(t, token, expectedRenameRequest),
		respondGQLData(http.StatusOK, gqlRenameResponse),
	))

	result := testhelpers.RunCLI(t, binary, []string{
		"admin", "rename-namespace",
		"ns-0", "ns-1",
		"--skip-update-check",
		"--token", token,
		"--host", ts.Server.URL,
		"--no-prompt",
	}, fmt.Sprintf("HOME=%s", ts.Home), fmt.Sprintf("USERPROFILE=%s", ts.Home))

	assert.Assert(t, result.ExitCode != 0, "expected non-zero exit code, got %d", result.ExitCode)
	assert.Assert(t, strings.Contains(result.Stderr, "Error: error1"),
		"expected stderr to contain error1, got: %s", result.Stderr)
}
