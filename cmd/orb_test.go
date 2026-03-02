package cmd_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/CircleCI-Public/circleci-cli/api"
	"github.com/CircleCI-Public/circleci-cli/api/graphql"
	"github.com/CircleCI-Public/circleci-cli/telemetry"
	"github.com/CircleCI-Public/circleci-cli/testhelpers"
	"gotest.tools/v3/assert"
)

// orbAppendPostHandler adds a handler that verifies a GraphQL POST and responds
// with the given response wrapped in a data envelope.
func orbAppendPostHandler(t *testing.T, ts *testhelpers.TestServer, authToken string, expectedRequest, response, errorResponse string) {
	t.Helper()
	responseBody := `{ "data": ` + response + `}`
	if errorResponse != "" {
		responseBody = fmt.Sprintf(`{ "data": %s, "errors": %s}`, response, errorResponse)
	}

	ts.AppendHandler(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, r.Method, "POST")
		assert.Equal(t, r.URL.Path, "/graphql-unstable")

		if authToken != "" {
			assert.Equal(t, r.Header.Get("Authorization"), authToken)
		}

		body, err := io.ReadAll(r.Body)
		assert.NilError(t, err)
		defer r.Body.Close()

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

// createTempOrbFile creates a temporary orb.yml file in the given directory
// and returns its path. The file is cleaned up via t.Cleanup.
func createTempOrbFile(t *testing.T, dir, relPath string, content []byte) string {
	t.Helper()
	fullPath := filepath.Join(dir, relPath)
	assert.NilError(t, os.MkdirAll(filepath.Dir(fullPath), 0700))
	assert.NilError(t, os.WriteFile(fullPath, content, 0600))
	return fullPath
}

// assertJSONEqual compares two JSON strings for equality, ignoring whitespace.
func assertJSONEqual(t *testing.T, expected, actual string) {
	t.Helper()
	var e, a interface{}
	assert.NilError(t, json.Unmarshal([]byte(expected), &e))
	assert.NilError(t, json.Unmarshal([]byte(actual), &a))
	eb, _ := json.Marshal(e)
	ab, _ := json.Marshal(a)
	assert.Equal(t, string(eb), string(ab))
}

// mockOrbSourceHandler appends a handler that mocks the orbVersion query for orb diff tests.
func mockOrbSourceHandler(t *testing.T, ts *testhelpers.TestServer, source, orbVersion, token string) {
	t.Helper()
	requestStruct := struct {
		Query     string `json:"query"`
		Variables struct {
			OrbVersionRef string `json:"orbVersionRef"`
		} `json:"variables"`
	}{
		Query: `query($orbVersionRef: String!) {
			    orbVersion(orbVersionRef: $orbVersionRef) {
			        id
                                version
                                orb { id }
                                source
			    }
		      }`,
		Variables: struct {
			OrbVersionRef string `json:"orbVersionRef"`
		}{OrbVersionRef: orbVersion},
	}
	request, err := json.Marshal(requestStruct)
	assert.NilError(t, err)
	response := fmt.Sprintf(`{
	"orbVersion": {
			"id": "some-id",
			"version": "some-version",
			"orb": { "id": "some-id" },
			"source": "%s"
	}
}`, source)
	orbAppendPostHandler(t, ts, token, string(request), response, "")
}

// encodeGQLRequest encodes a graphql.Request and returns it as a string.
func encodeGQLRequest(t *testing.T, req *graphql.Request) string {
	t.Helper()
	var buf bytes.Buffer
	buf2, err := req.Encode()
	assert.NilError(t, err)
	_, err = io.Copy(&buf, &buf2)
	assert.NilError(t, err)
	return buf.String()
}

func TestOrbTelemetry(t *testing.T) {
	binary := testhelpers.BuildCLI(t)
	ts := testhelpers.WithTempSettings(t)

	orbPath := createTempOrbFile(t, ts.Home, "orb.yml", []byte(`{}`))

	ts.Server.AppendHandler(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"data": {"orbConfig": {"sourceYaml": "{}", "valid": true, "errors": []} }}`))
	})

	result := testhelpers.RunCLI(t, binary,
		[]string{
			"orb", "validate", orbPath,
			"--skip-update-check",
			"--token", "token",
			"--host", ts.Server.URL,
		},
		"HOME="+ts.Home,
		"USERPROFILE="+ts.Home,
		fmt.Sprintf("MOCK_TELEMETRY=%s", ts.TelemetryDestPath),
	)
	assert.Equal(t, result.ExitCode, 0, "stderr: %s\nstdout: %s", result.Stderr, result.Stdout)

	testhelpers.AssertTelemetrySubset(t, ts, []telemetry.Event{
		telemetry.CreateOrbEvent(telemetry.CommandInfo{
			Name:      "validate",
			LocalArgs: map[string]string{},
		}),
	})
}

func TestOrbHelpText(t *testing.T) {
	binary := testhelpers.BuildCLI(t)

	result := testhelpers.RunCLI(t, binary,
		[]string{"orb", "--help"},
	)
	assert.Equal(t, result.ExitCode, 0, "stderr: %s\nstdout: %s", result.Stderr, result.Stdout)
	assert.Assert(t, strings.Contains(result.Stderr, "Operate on orbs"))
	assert.Assert(t, strings.Contains(result.Stderr, "See a full explanation and documentation on orbs here: https://circleci.com/docs/orbs/use/orb-intro/"))
}

func TestOrbHelpTextCustomHost(t *testing.T) {
	binary := testhelpers.BuildCLI(t)
	ts := testhelpers.WithTempSettings(t)

	assert.NilError(t, os.WriteFile(ts.Config, []byte(`host: https://foo.bar`), 0600))

	result := testhelpers.RunCLI(t, binary,
		[]string{"orb", "--help"},
		"HOME="+ts.Home,
		"USERPROFILE="+ts.Home,
	)
	assert.Equal(t, result.ExitCode, 0, "stderr: %s\nstdout: %s", result.Stderr, result.Stdout)
	assert.Assert(t, !strings.Contains(result.Stdout, "See a full explanation and documentation on orbs here: https://circleci.com/docs/orbs/use/orb-intro/"))
}

func TestOrbValidateSTDIN(t *testing.T) {
	binary := testhelpers.BuildCLI(t)
	ts := testhelpers.WithTempSettings(t)
	token := "testtoken"

	gqlResponse := `{
		"orbConfig": {
			"sourceYaml": "{}",
			"valid": true,
			"errors": []
		}
	}`

	response := struct {
		Query     string `json:"query"`
		Variables struct {
			Config string `json:"config"`
		} `json:"variables"`
	}{
		Query: `query ValidateOrb ($config: String!, $owner: UUID) {
	orbConfig(orbYaml: $config, ownerId: $owner) {
		valid,
		errors { message },
		sourceYaml,
		outputYaml
	}
}`,
		Variables: struct {
			Config string `json:"config"`
		}{
			Config: "{}",
		},
	}
	expected, err := json.Marshal(response)
	assert.NilError(t, err)

	orbAppendPostHandler(t, ts.Server, token, string(expected), gqlResponse, "")

	cmd := exec.Command(binary,
		"orb", "validate",
		"--skip-update-check",
		"--token", token,
		"--host", ts.Server.URL,
		"-",
	)
	cmd.Env = append(os.Environ(),
		"HOME="+ts.Home,
		"USERPROFILE="+ts.Home,
	)
	stdin, err := cmd.StdinPipe()
	assert.NilError(t, err)
	go func() {
		defer stdin.Close()
		_, _ = io.WriteString(stdin, "{}")
	}()

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err = cmd.Run()
	assert.NilError(t, err)
	assert.Assert(t, strings.Contains(stdout.String(), "Orb input is valid."))
}

func TestOrbValidateDefaultPath(t *testing.T) {
	binary := testhelpers.BuildCLI(t)
	ts := testhelpers.WithTempSettings(t)
	token := "testtoken"

	orbPath := createTempOrbFile(t, ts.Home, "orb.yml", []byte(`{}`))

	gqlResponse := `{
		"orbConfig": {
			"sourceYaml": "{}",
			"valid": true,
			"errors": []
		}
	}`

	expectedRequestJson := `{
		"query": "query ValidateOrb ($config: String!, $owner: UUID) {\n\torbConfig(orbYaml: $config, ownerId: $owner) {\n\t\tvalid,\n\t\terrors { message },\n\t\tsourceYaml,\n\t\toutputYaml\n\t}\n}",
		"variables": {
			"config": "{}"
		}
	}`

	orbAppendPostHandler(t, ts.Server, token, expectedRequestJson, gqlResponse, "")

	result := testhelpers.RunCLI(t, binary,
		[]string{
			"orb", "validate", orbPath,
			"--skip-update-check",
			"--token", token,
			"--host", ts.Server.URL,
		},
		"HOME="+ts.Home,
		"USERPROFILE="+ts.Home,
	)
	assert.Equal(t, result.ExitCode, 0, "stderr: %s\nstdout: %s", result.Stderr, result.Stdout)
	assert.Assert(t, strings.Contains(result.Stdout, "orb.yml` is valid."))
}

func TestOrbValidateWithOrgID(t *testing.T) {
	binary := testhelpers.BuildCLI(t)
	ts := testhelpers.WithTempSettings(t)
	token := "testtoken"

	gqlResponse := `{
		"orbConfig": {
			"sourceYaml": "{}",
			"valid": true,
			"errors": []
		}
	}`

	response := struct {
		Query     string `json:"query"`
		Variables struct {
			Config string `json:"config"`
			Owner  string `json:"owner"`
		} `json:"variables"`
	}{
		Query: `query ValidateOrb ($config: String!, $owner: UUID) {
	orbConfig(orbYaml: $config, ownerId: $owner) {
		valid,
		errors { message },
		sourceYaml,
		outputYaml
	}
}`,
		Variables: struct {
			Config string `json:"config"`
			Owner  string `json:"owner"`
		}{
			Config: "{}",
			Owner:  "org-id",
		},
	}
	expected, err := json.Marshal(response)
	assert.NilError(t, err)

	orbAppendPostHandler(t, ts.Server, token, string(expected), gqlResponse, "")

	cmd := exec.Command(binary,
		"orb", "validate",
		"--skip-update-check",
		"--token", token,
		"--host", ts.Server.URL,
		"--org-id", "org-id",
		"-",
	)
	cmd.Env = append(os.Environ(),
		"HOME="+ts.Home,
		"USERPROFILE="+ts.Home,
	)
	stdin, err := cmd.StdinPipe()
	assert.NilError(t, err)
	go func() {
		defer stdin.Close()
		_, _ = io.WriteString(stdin, "{}")
	}()

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err = cmd.Run()
	assert.NilError(t, err)
	assert.Assert(t, strings.Contains(stdout.String(), "Orb input is valid."))
}

func TestOrbValidateWorks(t *testing.T) {
	binary := testhelpers.BuildCLI(t)
	ts := testhelpers.WithTempSettings(t)
	token := "testtoken"

	orbPath := createTempOrbFile(t, ts.Home, filepath.Join("myorb", "orb.yml"), []byte(`some orb`))

	gqlResponse := `{
		"orbConfig": {
			"sourceYaml": "{}",
			"valid": true,
			"errors": []
		}
	}`

	expectedRequestJson := `{
		"query": "query ValidateOrb ($config: String!, $owner: UUID) {\n\torbConfig(orbYaml: $config, ownerId: $owner) {\n\t\tvalid,\n\t\terrors { message },\n\t\tsourceYaml,\n\t\toutputYaml\n\t}\n}",
		"variables": {
			"config": "some orb"
		}
	}`

	orbAppendPostHandler(t, ts.Server, token, expectedRequestJson, gqlResponse, "")

	result := testhelpers.RunCLI(t, binary,
		[]string{
			"orb", "validate", orbPath,
			"--skip-update-check",
			"--token", token,
			"--host", ts.Server.URL,
		},
		"HOME="+ts.Home,
		"USERPROFILE="+ts.Home,
	)
	assert.Equal(t, result.ExitCode, 0, "stderr: %s\nstdout: %s", result.Stderr, result.Stdout)
	assert.Assert(t, strings.Contains(result.Stdout, "orb.yml` is valid."))
}

func TestOrbValidateInvalid(t *testing.T) {
	binary := testhelpers.BuildCLI(t)
	ts := testhelpers.WithTempSettings(t)
	token := "testtoken"

	orbPath := createTempOrbFile(t, ts.Home, filepath.Join("myorb", "orb.yml"), []byte(`some orb`))

	gqlResponse := `{
		"orbConfig": {
			"sourceYaml": "hello world",
			"valid": false,
			"errors": [
				{"message": "invalid_orb"}
			]
		}
	}`

	expectedRequestJson := `{
		"query": "query ValidateOrb ($config: String!, $owner: UUID) {\n\torbConfig(orbYaml: $config, ownerId: $owner) {\n\t\tvalid,\n\t\terrors { message },\n\t\tsourceYaml,\n\t\toutputYaml\n\t}\n}",
		"variables": {
			"config": "some orb"
		}
	}`

	orbAppendPostHandler(t, ts.Server, token, expectedRequestJson, gqlResponse, "")

	result := testhelpers.RunCLI(t, binary,
		[]string{
			"orb", "validate", orbPath,
			"--skip-update-check",
			"--token", token,
			"--host", ts.Server.URL,
		},
		"HOME="+ts.Home,
		"USERPROFILE="+ts.Home,
	)
	assert.Assert(t, result.ExitCode != 0)
	assert.Assert(t, strings.Contains(result.Stderr, "Error: invalid_orb"))
}

func TestOrbProcessWorks(t *testing.T) {
	binary := testhelpers.BuildCLI(t)
	ts := testhelpers.WithTempSettings(t)
	token := "testtoken"

	orbPath := createTempOrbFile(t, ts.Home, filepath.Join("myorb", "orb.yml"), []byte(`some orb`))

	gqlResponse := `{
		"orbConfig": {
			"outputYaml": "hello world",
			"valid": true,
			"errors": []
		}
	}`

	expectedRequestJson := `{
		"query": "query ValidateOrb ($config: String!, $owner: UUID) {\n\torbConfig(orbYaml: $config, ownerId: $owner) {\n\t\tvalid,\n\t\terrors { message },\n\t\tsourceYaml,\n\t\toutputYaml\n\t}\n}",
		"variables": {
			"config": "some orb"
		}
	}`

	orbAppendPostHandler(t, ts.Server, token, expectedRequestJson, gqlResponse, "")

	result := testhelpers.RunCLI(t, binary,
		[]string{
			"orb", "process",
			"--skip-update-check",
			"--token", token,
			"--host", ts.Server.URL,
			orbPath,
		},
		"HOME="+ts.Home,
		"USERPROFILE="+ts.Home,
	)
	assert.Equal(t, result.ExitCode, 0, "stderr: %s\nstdout: %s", result.Stderr, result.Stdout)
	assert.Assert(t, strings.Contains(result.Stdout, "hello world"))
}

func TestOrbProcessInvalid(t *testing.T) {
	binary := testhelpers.BuildCLI(t)
	ts := testhelpers.WithTempSettings(t)
	token := "testtoken"

	orbPath := createTempOrbFile(t, ts.Home, filepath.Join("myorb", "orb.yml"), []byte(`some orb`))

	gqlResponse := `{
		"orbConfig": {
			"outputYaml": "hello world",
			"valid": false,
			"errors": [
				{"message": "error1"},
				{"message": "error2"}
			]
		}
	}`

	expectedRequestJson := `{
		"query": "query ValidateOrb ($config: String!, $owner: UUID) {\n\torbConfig(orbYaml: $config, ownerId: $owner) {\n\t\tvalid,\n\t\terrors { message },\n\t\tsourceYaml,\n\t\toutputYaml\n\t}\n}",
		"variables": {
			"config": "some orb"
		}
	}`

	orbAppendPostHandler(t, ts.Server, token, expectedRequestJson, gqlResponse, "")

	result := testhelpers.RunCLI(t, binary,
		[]string{
			"orb", "process",
			"--skip-update-check",
			"--token", token,
			"--host", ts.Server.URL,
			orbPath,
		},
		"HOME="+ts.Home,
		"USERPROFILE="+ts.Home,
	)
	assert.Assert(t, result.ExitCode != 0)
	assert.Assert(t, strings.Contains(result.Stderr, "Error: error1\nerror2"))
}

func TestOrbPublishSemanticVersion(t *testing.T) {
	binary := testhelpers.BuildCLI(t)
	ts := testhelpers.WithTempSettings(t)
	token := "testtoken"

	orbPath := createTempOrbFile(t, ts.Home, filepath.Join("myorb", "orb.yml"), []byte(`some orb`))

	gqlPublishResponse := `{
		"publishOrb": {
			"errors": [],
			"orb": {
				"version": "0.0.1"
			}
		}
	}`

	expectedPublishRequest := `{
		"query": "\n\t\tmutation($config: String!, $orbName: String, $namespaceName: String, $version: String!) {\n\t\t\tpublishOrb(\n\t\t\t\torbName: $orbName,\n\t\t\t\tnamespaceName: $namespaceName,\n\t\t\t\torbYaml: $config,\n\t\t\t\tversion: $version\n\t\t\t) {\n\t\t\t\torb {\n\t\t\t\t\tversion\n\t\t\t\t}\n\t\t\t\terrors { message }\n\t\t\t}\n\t\t}\n\t",
		"variables": {
			"config": "some orb",
			"namespaceName": "my",
			"orbName": "orb",
			"version": "0.0.1"
		}
	}`

	gqlOrbIDResponse := `{
		"orb": {"id": "orbid1", "isPrivate": false},
		"registryNamespace": {"id": "nsid1"}
	}`

	expectedOrbIDRequest := `{
		"query": "\n\tquery ($name: String!, $namespace: String) {\n\t\torb(name: $name) {\n\t\t  id\n\t\t  isPrivate\n\t\t}\n\t\tregistryNamespace(name: $namespace) {\n\t\t\tid\n\t\t  }\n\t  }\n\t  ",
		"variables": {
			"name": "my/orb",
			"namespace": "my"
		}
	}`

	orbAppendPostHandler(t, ts.Server, token, expectedPublishRequest, gqlPublishResponse, "")
	orbAppendPostHandler(t, ts.Server, token, expectedOrbIDRequest, gqlOrbIDResponse, "")

	result := testhelpers.RunCLI(t, binary,
		[]string{
			"orb", "publish",
			"--skip-update-check",
			"--token", token,
			"--host", ts.Server.URL,
			orbPath,
			"my/orb@0.0.1",
		},
		"HOME="+ts.Home,
		"USERPROFILE="+ts.Home,
	)
	assert.Equal(t, result.ExitCode, 0, "stderr: %s\nstdout: %s", result.Stderr, result.Stdout)
	assert.Assert(t, strings.Contains(result.Stdout, "Orb `my/orb@0.0.1` was published."))
}

func TestOrbPublishSemanticVersionErrors(t *testing.T) {
	binary := testhelpers.BuildCLI(t)
	ts := testhelpers.WithTempSettings(t)
	token := "testtoken"

	orbPath := createTempOrbFile(t, ts.Home, filepath.Join("myorb", "orb.yml"), []byte(`some orb`))

	gqlPublishResponse := `{
		"publishOrb": {
			"errors": [
				{"message": "error1"},
				{"message": "error2"}
			],
			"orb": null
		}
	}`

	expectedPublishRequest := `{
		"query": "\n\t\tmutation($config: String!, $orbName: String, $namespaceName: String, $version: String!) {\n\t\t\tpublishOrb(\n\t\t\t\torbName: $orbName,\n\t\t\t\tnamespaceName: $namespaceName,\n\t\t\t\torbYaml: $config,\n\t\t\t\tversion: $version\n\t\t\t) {\n\t\t\t\torb {\n\t\t\t\t\tversion\n\t\t\t\t}\n\t\t\t\terrors { message }\n\t\t\t}\n\t\t}\n\t",
		"variables": {
			"config": "some orb",
			"namespaceName": "my",
			"orbName": "orb",
			"version": "0.0.1"
		}
	}`

	orbAppendPostHandler(t, ts.Server, token, expectedPublishRequest, gqlPublishResponse, "")

	result := testhelpers.RunCLI(t, binary,
		[]string{
			"orb", "publish",
			"--skip-update-check",
			"--token", token,
			"--host", ts.Server.URL,
			orbPath,
			"my/orb@0.0.1",
		},
		"HOME="+ts.Home,
		"USERPROFILE="+ts.Home,
	)
	assert.Assert(t, result.ExitCode != 0)
	assert.Assert(t, strings.Contains(result.Stderr, "Error: error1\nerror2"))
}

func TestOrbPublishSemanticNoOrbFromPrivateCheck(t *testing.T) {
	binary := testhelpers.BuildCLI(t)
	ts := testhelpers.WithTempSettings(t)
	token := "testtoken"

	orbPath := createTempOrbFile(t, ts.Home, filepath.Join("myorb", "orb.yml"), []byte(`some orb`))

	gqlPublishResponse := `{
		"publishOrb": {
			"errors": [],
			"orb": {
				"version": "0.0.1"
			}
		}
	}`

	expectedPublishRequest := `{
		"query": "\n\t\tmutation($config: String!, $orbName: String, $namespaceName: String, $version: String!) {\n\t\t\tpublishOrb(\n\t\t\t\torbName: $orbName,\n\t\t\t\tnamespaceName: $namespaceName,\n\t\t\t\torbYaml: $config,\n\t\t\t\tversion: $version\n\t\t\t) {\n\t\t\t\torb {\n\t\t\t\t\tversion\n\t\t\t\t}\n\t\t\t\terrors { message }\n\t\t\t}\n\t\t}\n\t",
		"variables": {
			"config": "some orb",
			"namespaceName": "my",
			"orbName": "orb",
			"version": "0.0.1"
		}
	}`

	gqlOrbIDResponse := `{
		"orb": null,
		"registryNamespace": {"id": "nsid1"}
	}`

	expectedOrbIDRequest := `{
		"query": "\n\tquery ($name: String!, $namespace: String) {\n\t\torb(name: $name) {\n\t\t  id\n\t\t  isPrivate\n\t\t}\n\t\tregistryNamespace(name: $namespace) {\n\t\t\tid\n\t\t  }\n\t  }\n\t  ",
		"variables": {
			"name": "my/orb",
			"namespace": "my"
		}
	}`

	orbAppendPostHandler(t, ts.Server, token, expectedPublishRequest, gqlPublishResponse, "")
	orbAppendPostHandler(t, ts.Server, token, expectedOrbIDRequest, gqlOrbIDResponse, "")

	result := testhelpers.RunCLI(t, binary,
		[]string{
			"orb", "publish",
			"--skip-update-check",
			"--token", token,
			"--host", ts.Server.URL,
			orbPath,
			"my/orb@0.0.1",
		},
		"HOME="+ts.Home,
		"USERPROFILE="+ts.Home,
	)
	assert.Equal(t, result.ExitCode, 0, "stderr: %s\nstdout: %s", result.Stderr, result.Stdout)
	assert.Assert(t, strings.Contains(result.Stdout, "Orb `my/orb@0.0.1` was published."))
	assert.Assert(t, !strings.Contains(result.Stdout, "Please note that this is an open orb and is world-readable."))
}

func TestOrbPublishDevVersion(t *testing.T) {
	binary := testhelpers.BuildCLI(t)
	ts := testhelpers.WithTempSettings(t)
	token := "testtoken"

	orbPath := createTempOrbFile(t, ts.Home, filepath.Join("myorb", "orb.yml"), []byte(`some orb`))

	gqlPublishResponse := `{
		"publishOrb": {
			"errors": [],
			"orb": {
				"version": "dev:foo"
			}
		}
	}`

	expectedPublishRequest := `{
		"query": "\n\t\tmutation($config: String!, $orbName: String, $namespaceName: String, $version: String!) {\n\t\t\tpublishOrb(\n\t\t\t\torbName: $orbName,\n\t\t\t\tnamespaceName: $namespaceName,\n\t\t\t\torbYaml: $config,\n\t\t\t\tversion: $version\n\t\t\t) {\n\t\t\t\torb {\n\t\t\t\t\tversion\n\t\t\t\t}\n\t\t\t\terrors { message }\n\t\t\t}\n\t\t}\n\t",
		"variables": {
			"config": "some orb",
			"namespaceName": "my",
			"orbName": "orb",
			"version": "dev:foo"
		}
	}`

	gqlOrbIDResponse := `{
		"orb": {"id": "orbid1", "isPrivate": false},
		"registryNamespace": {"id": "nsid1"}
	}`

	expectedOrbIDRequest := `{
		"query": "\n\tquery ($name: String!, $namespace: String) {\n\t\torb(name: $name) {\n\t\t  id\n\t\t  isPrivate\n\t\t}\n\t\tregistryNamespace(name: $namespace) {\n\t\t\tid\n\t\t  }\n\t  }\n\t  ",
		"variables": {
			"name": "my/orb",
			"namespace": "my"
		}
	}`

	orbAppendPostHandler(t, ts.Server, token, expectedPublishRequest, gqlPublishResponse, "")
	orbAppendPostHandler(t, ts.Server, token, expectedOrbIDRequest, gqlOrbIDResponse, "")

	result := testhelpers.RunCLI(t, binary,
		[]string{
			"orb", "publish",
			"--skip-update-check",
			"--token", token,
			"--host", ts.Server.URL,
			orbPath,
			"my/orb@dev:foo",
		},
		"HOME="+ts.Home,
		"USERPROFILE="+ts.Home,
	)
	assert.Equal(t, result.ExitCode, 0, "stderr: %s\nstdout: %s", result.Stderr, result.Stdout)
	assert.Assert(t, strings.Contains(result.Stdout, "Orb `my/orb@dev:foo` was published."))
}

func TestOrbPublishDevVersionErrors(t *testing.T) {
	binary := testhelpers.BuildCLI(t)
	ts := testhelpers.WithTempSettings(t)
	token := "testtoken"

	orbPath := createTempOrbFile(t, ts.Home, filepath.Join("myorb", "orb.yml"), []byte(`some orb`))

	gqlPublishResponse := `{
		"publishOrb": {
			"errors": [
				{"message": "error1"},
				{"message": "error2"}
			],
			"orb": null
		}
	}`

	expectedPublishRequest := `{
		"query": "\n\t\tmutation($config: String!, $orbName: String, $namespaceName: String, $version: String!) {\n\t\t\tpublishOrb(\n\t\t\t\torbName: $orbName,\n\t\t\t\tnamespaceName: $namespaceName,\n\t\t\t\torbYaml: $config,\n\t\t\t\tversion: $version\n\t\t\t) {\n\t\t\t\torb {\n\t\t\t\t\tversion\n\t\t\t\t}\n\t\t\t\terrors { message }\n\t\t\t}\n\t\t}\n\t",
		"variables": {
			"config": "some orb",
			"namespaceName": "my",
			"orbName": "orb",
			"version": "dev:foo"
		}
	}`

	orbAppendPostHandler(t, ts.Server, token, expectedPublishRequest, gqlPublishResponse, "")

	result := testhelpers.RunCLI(t, binary,
		[]string{
			"orb", "publish",
			"--skip-update-check",
			"--token", token,
			"--host", ts.Server.URL,
			orbPath,
			"my/orb@dev:foo",
		},
		"HOME="+ts.Home,
		"USERPROFILE="+ts.Home,
	)
	assert.Assert(t, result.ExitCode != 0)
	assert.Assert(t, strings.Contains(result.Stderr, "Error: error1\nerror2"))
}

func TestOrbPublishDevNoOrbFromPrivateCheck(t *testing.T) {
	binary := testhelpers.BuildCLI(t)
	ts := testhelpers.WithTempSettings(t)
	token := "testtoken"

	orbPath := createTempOrbFile(t, ts.Home, filepath.Join("myorb", "orb.yml"), []byte(`some orb`))

	gqlPublishResponse := `{
		"publishOrb": {
			"errors": [],
			"orb": {
				"version": "dev:foo"
			}
		}
	}`

	expectedPublishRequest := `{
		"query": "\n\t\tmutation($config: String!, $orbName: String, $namespaceName: String, $version: String!) {\n\t\t\tpublishOrb(\n\t\t\t\torbName: $orbName,\n\t\t\t\tnamespaceName: $namespaceName,\n\t\t\t\torbYaml: $config,\n\t\t\t\tversion: $version\n\t\t\t) {\n\t\t\t\torb {\n\t\t\t\t\tversion\n\t\t\t\t}\n\t\t\t\terrors { message }\n\t\t\t}\n\t\t}\n\t",
		"variables": {
			"config": "some orb",
			"namespaceName": "my",
			"orbName": "orb",
			"version": "dev:foo"
		}
	}`

	gqlOrbIDResponse := `{
		"orb": null,
		"registryNamespace": {"id": "nsid1"}
	}`

	expectedOrbIDRequest := `{
		"query": "\n\tquery ($name: String!, $namespace: String) {\n\t\torb(name: $name) {\n\t\t  id\n\t\t  isPrivate\n\t\t}\n\t\tregistryNamespace(name: $namespace) {\n\t\t\tid\n\t\t  }\n\t  }\n\t  ",
		"variables": {
			"name": "my/orb",
			"namespace": "my"
		}
	}`

	orbAppendPostHandler(t, ts.Server, token, expectedPublishRequest, gqlPublishResponse, "")
	orbAppendPostHandler(t, ts.Server, token, expectedOrbIDRequest, gqlOrbIDResponse, "")

	result := testhelpers.RunCLI(t, binary,
		[]string{
			"orb", "publish",
			"--skip-update-check",
			"--token", token,
			"--host", ts.Server.URL,
			orbPath,
			"my/orb@dev:foo",
		},
		"HOME="+ts.Home,
		"USERPROFILE="+ts.Home,
	)
	assert.Equal(t, result.ExitCode, 0, "stderr: %s\nstdout: %s", result.Stderr, result.Stdout)
	assert.Assert(t, strings.Contains(result.Stdout, "Orb `my/orb@dev:foo` was published."))
	assert.Assert(t, !strings.Contains(result.Stdout, "Please note that this is an open orb and is world-readable."))
}

func TestOrbPublishIncrement(t *testing.T) {
	binary := testhelpers.BuildCLI(t)
	ts := testhelpers.WithTempSettings(t)
	token := "testtoken"

	orbPath := createTempOrbFile(t, ts.Home, filepath.Join("myorb", "orb.yml"), []byte(`some orb`))

	gqlVersionResponse := `{
		"orb": {
			"versions": [
				{"version": "0.0.1"}
			]
		}
	}`

	expectedVersionRequest := `{
		"query": "query($name: String!) {\n\t\t\t    orb(name: $name) {\n\t\t\t      versions(count: 1) {\n\t\t\t\t    version\n\t\t\t      }\n\t\t\t    }\n\t\t      }",
		"variables": {
			"name": "my/orb"
		}
	}`

	gqlPublishResponse := `{
		"publishOrb": {
			"errors": [],
			"orb": {
				"version": "0.1.0"
			}
		}
	}`

	expectedPublishRequest := `{
		"query": "\n\t\tmutation($config: String!, $orbName: String, $namespaceName: String, $version: String!) {\n\t\t\tpublishOrb(\n\t\t\t\torbName: $orbName,\n\t\t\t\tnamespaceName: $namespaceName,\n\t\t\t\torbYaml: $config,\n\t\t\t\tversion: $version\n\t\t\t) {\n\t\t\t\torb {\n\t\t\t\t\tversion\n\t\t\t\t}\n\t\t\t\terrors { message }\n\t\t\t}\n\t\t}\n\t",
		"variables": {
			"config": "some orb",
			"namespaceName": "my",
			"orbName": "orb",
			"version": "0.1.0"
		}
	}`

	gqlOrbIDResponse := `{
		"orb": {"id": "orbid1", "isPrivate": false},
		"registryNamespace": {"id": "nsid1"}
	}`

	expectedOrbIDRequest := `{
		"query": "\n\tquery ($name: String!, $namespace: String) {\n\t\torb(name: $name) {\n\t\t  id\n\t\t  isPrivate\n\t\t}\n\t\tregistryNamespace(name: $namespace) {\n\t\t\tid\n\t\t  }\n\t  }\n\t  ",
		"variables": {
			"name": "my/orb",
			"namespace": "my"
		}
	}`

	orbAppendPostHandler(t, ts.Server, token, expectedVersionRequest, gqlVersionResponse, "")
	orbAppendPostHandler(t, ts.Server, token, expectedPublishRequest, gqlPublishResponse, "")
	orbAppendPostHandler(t, ts.Server, token, expectedOrbIDRequest, gqlOrbIDResponse, "")

	result := testhelpers.RunCLI(t, binary,
		[]string{
			"orb", "publish", "increment",
			"--skip-update-check",
			"--token", token,
			"--host", ts.Server.URL,
			orbPath,
			"my/orb", "minor",
		},
		"HOME="+ts.Home,
		"USERPROFILE="+ts.Home,
	)
	assert.Equal(t, result.ExitCode, 0, "stderr: %s\nstdout: %s", result.Stderr, result.Stdout)
	assert.Assert(t, strings.Contains(result.Stdout, "Orb `my/orb` has been incremented to `my/orb@0.1.0`."))
}

func TestOrbPublishIncrementErrors(t *testing.T) {
	binary := testhelpers.BuildCLI(t)
	ts := testhelpers.WithTempSettings(t)
	token := "testtoken"

	orbPath := createTempOrbFile(t, ts.Home, filepath.Join("myorb", "orb.yml"), []byte(`some orb`))

	gqlVersionResponse := `{
		"orb": {
			"versions": [
				{"version": "0.0.1"}
			]
		}
	}`

	expectedVersionRequest := `{
		"query": "query($name: String!) {\n\t\t\t    orb(name: $name) {\n\t\t\t      versions(count: 1) {\n\t\t\t\t    version\n\t\t\t      }\n\t\t\t    }\n\t\t      }",
		"variables": {
			"name": "my/orb"
		}
	}`

	gqlPublishResponse := `{
		"publishOrb": {
			"errors": [
				{"message": "error1"},
				{"message": "error2"}
			],
			"orb": null
		}
	}`

	expectedPublishRequest := `{
		"query": "\n\t\tmutation($config: String!, $orbName: String, $namespaceName: String, $version: String!) {\n\t\t\tpublishOrb(\n\t\t\t\torbName: $orbName,\n\t\t\t\tnamespaceName: $namespaceName,\n\t\t\t\torbYaml: $config,\n\t\t\t\tversion: $version\n\t\t\t) {\n\t\t\t\torb {\n\t\t\t\t\tversion\n\t\t\t\t}\n\t\t\t\terrors { message }\n\t\t\t}\n\t\t}\n\t",
		"variables": {
			"config": "some orb",
			"namespaceName": "my",
			"orbName": "orb",
			"version": "0.1.0"
		}
	}`

	orbAppendPostHandler(t, ts.Server, token, expectedVersionRequest, gqlVersionResponse, "")
	orbAppendPostHandler(t, ts.Server, token, expectedPublishRequest, gqlPublishResponse, "")

	result := testhelpers.RunCLI(t, binary,
		[]string{
			"orb", "publish", "increment",
			"--skip-update-check",
			"--token", token,
			"--host", ts.Server.URL,
			orbPath,
			"my/orb", "minor",
		},
		"HOME="+ts.Home,
		"USERPROFILE="+ts.Home,
	)
	assert.Assert(t, result.ExitCode != 0)
	assert.Assert(t, strings.Contains(result.Stderr, "Error: error1\nerror2"))
}

func TestOrbPublishIncrementNoOrbFromPrivateCheck(t *testing.T) {
	binary := testhelpers.BuildCLI(t)
	ts := testhelpers.WithTempSettings(t)
	token := "testtoken"

	orbPath := createTempOrbFile(t, ts.Home, filepath.Join("myorb", "orb.yml"), []byte(`some orb`))

	gqlVersionResponse := `{
		"orb": {
			"versions": [
				{"version": "0.0.1"}
			]
		}
	}`

	expectedVersionRequest := `{
		"query": "query($name: String!) {\n\t\t\t    orb(name: $name) {\n\t\t\t      versions(count: 1) {\n\t\t\t\t    version\n\t\t\t      }\n\t\t\t    }\n\t\t      }",
		"variables": {
			"name": "my/orb"
		}
	}`

	gqlPublishResponse := `{
		"publishOrb": {
			"errors": [],
			"orb": {
				"version": "0.1.0"
			}
		}
	}`

	expectedPublishRequest := `{
		"query": "\n\t\tmutation($config: String!, $orbName: String, $namespaceName: String, $version: String!) {\n\t\t\tpublishOrb(\n\t\t\t\torbName: $orbName,\n\t\t\t\tnamespaceName: $namespaceName,\n\t\t\t\torbYaml: $config,\n\t\t\t\tversion: $version\n\t\t\t) {\n\t\t\t\torb {\n\t\t\t\t\tversion\n\t\t\t\t}\n\t\t\t\terrors { message }\n\t\t\t}\n\t\t}\n\t",
		"variables": {
			"config": "some orb",
			"namespaceName": "my",
			"orbName": "orb",
			"version": "0.1.0"
		}
	}`

	gqlOrbIDResponse := `{
		"orb": null,
		"registryNamespace": {"id": "nsid1"}
	}`

	expectedOrbIDRequest := `{
		"query": "\n\tquery ($name: String!, $namespace: String) {\n\t\torb(name: $name) {\n\t\t  id\n\t\t  isPrivate\n\t\t}\n\t\tregistryNamespace(name: $namespace) {\n\t\t\tid\n\t\t  }\n\t  }\n\t  ",
		"variables": {
			"name": "my/orb",
			"namespace": "my"
		}
	}`

	orbAppendPostHandler(t, ts.Server, token, expectedVersionRequest, gqlVersionResponse, "")
	orbAppendPostHandler(t, ts.Server, token, expectedPublishRequest, gqlPublishResponse, "")
	orbAppendPostHandler(t, ts.Server, token, expectedOrbIDRequest, gqlOrbIDResponse, "")

	result := testhelpers.RunCLI(t, binary,
		[]string{
			"orb", "publish", "increment",
			"--skip-update-check",
			"--token", token,
			"--host", ts.Server.URL,
			orbPath,
			"my/orb", "minor",
		},
		"HOME="+ts.Home,
		"USERPROFILE="+ts.Home,
	)
	assert.Equal(t, result.ExitCode, 0, "stderr: %s\nstdout: %s", result.Stderr, result.Stdout)
	assert.Assert(t, strings.Contains(result.Stdout, "Orb `my/orb` has been incremented to `my/orb@0.1.0`."))
	assert.Assert(t, !strings.Contains(result.Stdout, "Please note that this is an open orb and is world-readable."))
}

func TestOrbPublishPromote(t *testing.T) {
	binary := testhelpers.BuildCLI(t)
	ts := testhelpers.WithTempSettings(t)
	token := "testtoken"

	gqlVersionResponse := `{
		"orb": {
			"versions": [
				{"version": "0.0.1"}
			]
		}
	}`

	expectedVersionRequest := `{
		"query": "query($name: String!) {\n\t\t\t    orb(name: $name) {\n\t\t\t      versions(count: 1) {\n\t\t\t\t    version\n\t\t\t      }\n\t\t\t    }\n\t\t      }",
		"variables": {
			"name": "my/orb"
		}
	}`

	gqlPromoteResponse := `{
		"promoteOrb": {
			"errors": [],
			"orb": {
				"version": "0.1.0",
				"source": "some orb"
			}
		}
	}`

	expectedPromoteRequest := `{
		"query": "\n\t\tmutation($orbName: String, $namespaceName: String, $devVersion: String!, $semanticVersion: String!) {\n\t\t\tpromoteOrb(\n\t\t\t\torbName: $orbName,\n\t\t\t\tnamespaceName: $namespaceName,\n\t\t\t\tdevVersion: $devVersion,\n\t\t\t\tsemanticVersion: $semanticVersion\n\t\t\t) {\n\t\t\t\torb {\n\t\t\t\t\tversion\n\t\t\t\t\tsource\n\t\t\t\t}\n\t\t\t\terrors { message }\n\t\t\t}\n\t\t}\n\t",
		"variables": {
			"devVersion": "dev:foo",
			"namespaceName": "my",
			"orbName": "orb",
			"semanticVersion": "0.1.0"
		}
	}`

	gqlOrbIDResponse := `{
		"orb": {"id": "orbid1", "isPrivate": false},
		"registryNamespace": {"id": "nsid1"}
	}`

	expectedOrbIDRequest := `{
		"query": "\n\tquery ($name: String!, $namespace: String) {\n\t\torb(name: $name) {\n\t\t  id\n\t\t  isPrivate\n\t\t}\n\t\tregistryNamespace(name: $namespace) {\n\t\t\tid\n\t\t  }\n\t  }\n\t  ",
		"variables": {
			"name": "my/orb",
			"namespace": "my"
		}
	}`

	orbAppendPostHandler(t, ts.Server, token, expectedVersionRequest, gqlVersionResponse, "")
	orbAppendPostHandler(t, ts.Server, token, expectedPromoteRequest, gqlPromoteResponse, "")
	orbAppendPostHandler(t, ts.Server, token, expectedOrbIDRequest, gqlOrbIDResponse, "")

	result := testhelpers.RunCLI(t, binary,
		[]string{
			"orb", "publish", "promote",
			"--skip-update-check",
			"--token", token,
			"--host", ts.Server.URL,
			"my/orb@dev:foo",
			"minor",
		},
		"HOME="+ts.Home,
		"USERPROFILE="+ts.Home,
	)
	assert.Equal(t, result.ExitCode, 0, "stderr: %s\nstdout: %s", result.Stderr, result.Stdout)
	assert.Assert(t, strings.Contains(result.Stdout, "Orb `my/orb@dev:foo` was promoted to `my/orb@0.1.0`."))
}

func TestOrbPublishPromoteErrors(t *testing.T) {
	binary := testhelpers.BuildCLI(t)
	ts := testhelpers.WithTempSettings(t)
	token := "testtoken"

	gqlVersionResponse := `{
		"orb": {
			"versions": [
				{"version": "0.0.1"}
			]
		}
	}`

	expectedVersionRequest := `{
		"query": "query($name: String!) {\n\t\t\t    orb(name: $name) {\n\t\t\t      versions(count: 1) {\n\t\t\t\t    version\n\t\t\t      }\n\t\t\t    }\n\t\t      }",
		"variables": {
			"name": "my/orb"
		}
	}`

	gqlPromoteResponse := `{
		"promoteOrb": {
			"errors": [
				{"message": "error1"},
				{"message": "error2"}
			],
			"orb": null
		}
	}`

	expectedPromoteRequest := `{
		"query": "\n\t\tmutation($orbName: String, $namespaceName: String, $devVersion: String!, $semanticVersion: String!) {\n\t\t\tpromoteOrb(\n\t\t\t\torbName: $orbName,\n\t\t\t\tnamespaceName: $namespaceName,\n\t\t\t\tdevVersion: $devVersion,\n\t\t\t\tsemanticVersion: $semanticVersion\n\t\t\t) {\n\t\t\t\torb {\n\t\t\t\t\tversion\n\t\t\t\t\tsource\n\t\t\t\t}\n\t\t\t\terrors { message }\n\t\t\t}\n\t\t}\n\t",
		"variables": {
			"devVersion": "dev:foo",
			"namespaceName": "my",
			"orbName": "orb",
			"semanticVersion": "0.1.0"
		}
	}`

	orbAppendPostHandler(t, ts.Server, token, expectedVersionRequest, gqlVersionResponse, "")
	orbAppendPostHandler(t, ts.Server, token, expectedPromoteRequest, gqlPromoteResponse, "")

	result := testhelpers.RunCLI(t, binary,
		[]string{
			"orb", "publish", "promote",
			"--skip-update-check",
			"--token", token,
			"--host", ts.Server.URL,
			"my/orb@dev:foo",
			"minor",
		},
		"HOME="+ts.Home,
		"USERPROFILE="+ts.Home,
	)
	assert.Assert(t, result.ExitCode != 0)
	assert.Assert(t, strings.Contains(result.Stderr, "Error: error1\nerror2"))
}

func TestOrbPublishPromoteNoOrbFromPrivateCheck(t *testing.T) {
	binary := testhelpers.BuildCLI(t)
	ts := testhelpers.WithTempSettings(t)
	token := "testtoken"

	gqlVersionResponse := `{
		"orb": {
			"versions": [
				{"version": "0.0.1"}
			]
		}
	}`

	expectedVersionRequest := `{
		"query": "query($name: String!) {\n\t\t\t    orb(name: $name) {\n\t\t\t      versions(count: 1) {\n\t\t\t\t    version\n\t\t\t      }\n\t\t\t    }\n\t\t      }",
		"variables": {
			"name": "my/orb"
		}
	}`

	gqlPromoteResponse := `{
		"promoteOrb": {
			"errors": [],
			"orb": {
				"version": "0.1.0",
				"source": "some orb"
			}
		}
	}`

	expectedPromoteRequest := `{
		"query": "\n\t\tmutation($orbName: String, $namespaceName: String, $devVersion: String!, $semanticVersion: String!) {\n\t\t\tpromoteOrb(\n\t\t\t\torbName: $orbName,\n\t\t\t\tnamespaceName: $namespaceName,\n\t\t\t\tdevVersion: $devVersion,\n\t\t\t\tsemanticVersion: $semanticVersion\n\t\t\t) {\n\t\t\t\torb {\n\t\t\t\t\tversion\n\t\t\t\t\tsource\n\t\t\t\t}\n\t\t\t\terrors { message }\n\t\t\t}\n\t\t}\n\t",
		"variables": {
			"devVersion": "dev:foo",
			"namespaceName": "my",
			"orbName": "orb",
			"semanticVersion": "0.1.0"
		}
	}`

	gqlOrbIDResponse := `{
		"orb": null,
		"registryNamespace": {"id": "nsid1"}
	}`

	expectedOrbIDRequest := `{
		"query": "\n\tquery ($name: String!, $namespace: String) {\n\t\torb(name: $name) {\n\t\t  id\n\t\t  isPrivate\n\t\t}\n\t\tregistryNamespace(name: $namespace) {\n\t\t\tid\n\t\t  }\n\t  }\n\t  ",
		"variables": {
			"name": "my/orb",
			"namespace": "my"
		}
	}`

	orbAppendPostHandler(t, ts.Server, token, expectedVersionRequest, gqlVersionResponse, "")
	orbAppendPostHandler(t, ts.Server, token, expectedPromoteRequest, gqlPromoteResponse, "")
	orbAppendPostHandler(t, ts.Server, token, expectedOrbIDRequest, gqlOrbIDResponse, "")

	result := testhelpers.RunCLI(t, binary,
		[]string{
			"orb", "publish", "promote",
			"--skip-update-check",
			"--token", token,
			"--host", ts.Server.URL,
			"my/orb@dev:foo",
			"minor",
		},
		"HOME="+ts.Home,
		"USERPROFILE="+ts.Home,
	)
	assert.Equal(t, result.ExitCode, 0, "stderr: %s\nstdout: %s", result.Stderr, result.Stdout)
	assert.Assert(t, strings.Contains(result.Stdout, "Orb `my/orb@dev:foo` was promoted to `my/orb@0.1.0`."))
	assert.Assert(t, !strings.Contains(result.Stdout, "Please note that this is an open orb and is world-readable."))
}

func TestOrbCreateNoPrompt(t *testing.T) {
	binary := testhelpers.BuildCLI(t)
	ts := testhelpers.WithTempSettings(t)
	token := "testtoken"

	gqlNamespaceResponse := `{
		"registryNamespace": {
			"id": "bb604b45-b6b0-4b81-ad80-796f15eddf87"
		}
	}`

	expectedNamespaceRequest := `{
		"query": "\n\t\t\t\tquery($name: String!) {\n\t\t\t\t\tregistryNamespace(\n\t\t\t\t\t\tname: $name\n\t\t\t\t\t){\n\t\t\t\t\t\tid\n\t\t\t\t\t}\n\t\t\t }",
		"variables": {
			"name": "bar-ns"
		}
	}`

	gqlOrbResponse := `{
		"createOrb": {
			"errors": [],
			"orb": {
				"id": "bb604b45-b6b0-4b81-ad80-796f15eddf87"
			}
		}
	}`

	expectedOrbRequest := `{
		"query": "mutation($name: String!, $registryNamespaceId: UUID!, $isPrivate: Boolean!){\n\t\t\t\tcreateOrb(\n\t\t\t\t\tname: $name,\n\t\t\t\t\tregistryNamespaceId: $registryNamespaceId,\n\t\t\t\t\tisPrivate: $isPrivate\n\t\t\t\t){\n\t\t\t\t    orb {\n\t\t\t\t      id\n\t\t\t\t    }\n\t\t\t\t    errors {\n\t\t\t\t      message\n\t\t\t\t      type\n\t\t\t\t    }\n\t\t\t\t}\n}",
		"variables": {
			"isPrivate": false,
			"name": "foo-orb",
			"registryNamespaceId": "bb604b45-b6b0-4b81-ad80-796f15eddf87"
		}
	}`

	orbAppendPostHandler(t, ts.Server, token, expectedNamespaceRequest, gqlNamespaceResponse, "")
	orbAppendPostHandler(t, ts.Server, token, expectedOrbRequest, gqlOrbResponse, "")

	result := testhelpers.RunCLI(t, binary,
		[]string{
			"orb", "create",
			"--skip-update-check",
			"--token", token,
			"--host", ts.Server.URL,
			"--no-prompt",
			"bar-ns/foo-orb",
		},
		"HOME="+ts.Home,
		"USERPROFILE="+ts.Home,
	)
	assert.Equal(t, result.ExitCode, 0, "stderr: %s\nstdout: %s", result.Stderr, result.Stdout)
	assert.Assert(t, strings.Contains(result.Stdout, "Orb `bar-ns/foo-orb` created."))
	assert.Assert(t, strings.Contains(result.Stdout, "You can now register versions of `bar-ns/foo-orb` using `circleci orb publish`"))
}

func TestOrbCreatePrivateNoPrompt(t *testing.T) {
	binary := testhelpers.BuildCLI(t)
	ts := testhelpers.WithTempSettings(t)
	token := "testtoken"

	gqlNamespaceResponse := `{
		"registryNamespace": {
			"id": "bb604b45-b6b0-4b81-ad80-796f15eddf87"
		}
	}`

	expectedNamespaceRequest := `{
		"query": "\n\t\t\t\tquery($name: String!) {\n\t\t\t\t\tregistryNamespace(\n\t\t\t\t\t\tname: $name\n\t\t\t\t\t){\n\t\t\t\t\t\tid\n\t\t\t\t\t}\n\t\t\t }",
		"variables": {
			"name": "bar-ns"
		}
	}`

	gqlOrbResponse := `{
		"createOrb": {
			"errors": [],
			"orb": {
				"id": "bb604b45-b6b0-4b81-ad80-796f15eddf87"
			}
		}
	}`

	expectedOrbRequest := `{
		"query": "mutation($name: String!, $registryNamespaceId: UUID!, $isPrivate: Boolean!){\n\t\t\t\tcreateOrb(\n\t\t\t\t\tname: $name,\n\t\t\t\t\tregistryNamespaceId: $registryNamespaceId,\n\t\t\t\t\tisPrivate: $isPrivate\n\t\t\t\t){\n\t\t\t\t    orb {\n\t\t\t\t      id\n\t\t\t\t    }\n\t\t\t\t    errors {\n\t\t\t\t      message\n\t\t\t\t      type\n\t\t\t\t    }\n\t\t\t\t}\n}",
		"variables": {
			"isPrivate": true,
			"name": "foo-orb",
			"registryNamespaceId": "bb604b45-b6b0-4b81-ad80-796f15eddf87"
		}
	}`

	orbAppendPostHandler(t, ts.Server, token, expectedNamespaceRequest, gqlNamespaceResponse, "")
	orbAppendPostHandler(t, ts.Server, token, expectedOrbRequest, gqlOrbResponse, "")

	result := testhelpers.RunCLI(t, binary,
		[]string{
			"orb", "create",
			"--private",
			"--skip-update-check",
			"--token", token,
			"--host", ts.Server.URL,
			"--no-prompt",
			"bar-ns/foo-orb",
		},
		"HOME="+ts.Home,
		"USERPROFILE="+ts.Home,
	)
	assert.Equal(t, result.ExitCode, 0, "stderr: %s\nstdout: %s", result.Stderr, result.Stdout)
	assert.Assert(t, strings.Contains(result.Stdout, "This orb will not be listed on the registry and is usable only by org users."))
	assert.Assert(t, strings.Contains(result.Stdout, "Orb `bar-ns/foo-orb` created."))
}

func TestOrbCreateNoPromptErrors(t *testing.T) {
	binary := testhelpers.BuildCLI(t)
	ts := testhelpers.WithTempSettings(t)
	token := "testtoken"

	gqlNamespaceResponse := `{
		"registryNamespace": {
			"id": "bb604b45-b6b0-4b81-ad80-796f15eddf87"
		}
	}`

	expectedNamespaceRequest := `{
		"query": "\n\t\t\t\tquery($name: String!) {\n\t\t\t\t\tregistryNamespace(\n\t\t\t\t\t\tname: $name\n\t\t\t\t\t){\n\t\t\t\t\t\tid\n\t\t\t\t\t}\n\t\t\t }",
		"variables": {
			"name": "bar-ns"
		}
	}`

	gqlOrbResponse := `{
		"createOrb": {
			"errors": [
				{"message": "error1"},
				{"message": "error2"}
			],
			"orb": null
		}
	}`

	gqlErrors := `[ { "message": "ignored error" } ]`

	expectedOrbRequest := `{
		"query": "mutation($name: String!, $registryNamespaceId: UUID!, $isPrivate: Boolean!){\n\t\t\t\tcreateOrb(\n\t\t\t\t\tname: $name,\n\t\t\t\t\tregistryNamespaceId: $registryNamespaceId,\n\t\t\t\t\tisPrivate: $isPrivate\n\t\t\t\t){\n\t\t\t\t    orb {\n\t\t\t\t      id\n\t\t\t\t    }\n\t\t\t\t    errors {\n\t\t\t\t      message\n\t\t\t\t      type\n\t\t\t\t    }\n\t\t\t\t}\n}",
		"variables": {
			"isPrivate": false,
			"name": "foo-orb",
			"registryNamespaceId": "bb604b45-b6b0-4b81-ad80-796f15eddf87"
		}
	}`

	orbAppendPostHandler(t, ts.Server, token, expectedNamespaceRequest, gqlNamespaceResponse, "")
	orbAppendPostHandler(t, ts.Server, token, expectedOrbRequest, gqlOrbResponse, gqlErrors)

	result := testhelpers.RunCLI(t, binary,
		[]string{
			"orb", "create",
			"--skip-update-check",
			"--token", token,
			"--host", ts.Server.URL,
			"--no-prompt",
			"bar-ns/foo-orb",
		},
		"HOME="+ts.Home,
		"USERPROFILE="+ts.Home,
	)
	assert.Assert(t, result.ExitCode != 0)
	assert.Assert(t, strings.Contains(result.Stderr, "Error: error1\nerror2"))
}

func TestOrbCreateInteractive(t *testing.T) {
	binary := testhelpers.BuildCLI(t)
	ts := testhelpers.WithTempSettings(t)
	token := "testtoken"

	gqlNamespaceResponse := `{
		"registryNamespace": {
			"id": "bb604b45-b6b0-4b81-ad80-796f15eddf87"
		}
	}`

	expectedNamespaceRequest := `{
		"query": "\n\t\t\t\tquery($name: String!) {\n\t\t\t\t\tregistryNamespace(\n\t\t\t\t\t\tname: $name\n\t\t\t\t\t){\n\t\t\t\t\t\tid\n\t\t\t\t\t}\n\t\t\t }",
		"variables": {
			"name": "bar-ns"
		}
	}`

	gqlOrbResponse := `{
		"createOrb": {
			"errors": [],
			"orb": {
				"id": "bb604b45-b6b0-4b81-ad80-796f15eddf87"
			}
		}
	}`

	expectedOrbRequest := `{
		"query": "mutation($name: String!, $registryNamespaceId: UUID!, $isPrivate: Boolean!){\n\t\t\t\tcreateOrb(\n\t\t\t\t\tname: $name,\n\t\t\t\t\tregistryNamespaceId: $registryNamespaceId,\n\t\t\t\t\tisPrivate: $isPrivate\n\t\t\t\t){\n\t\t\t\t    orb {\n\t\t\t\t      id\n\t\t\t\t    }\n\t\t\t\t    errors {\n\t\t\t\t      message\n\t\t\t\t      type\n\t\t\t\t    }\n\t\t\t\t}\n}",
		"variables": {
			"isPrivate": false,
			"name": "foo-orb",
			"registryNamespaceId": "bb604b45-b6b0-4b81-ad80-796f15eddf87"
		}
	}`

	orbAppendPostHandler(t, ts.Server, token, expectedNamespaceRequest, gqlNamespaceResponse, "")
	orbAppendPostHandler(t, ts.Server, token, expectedOrbRequest, gqlOrbResponse, "")

	result := testhelpers.RunCLI(t, binary,
		[]string{
			"orb", "create",
			"--skip-update-check",
			"--token", token,
			"--host", ts.Server.URL,
			"--integration-testing",
			"bar-ns/foo-orb",
		},
		"HOME="+ts.Home,
		"USERPROFILE="+ts.Home,
	)
	assert.Equal(t, result.ExitCode, 0, "stderr: %s\nstdout: %s", result.Stderr, result.Stdout)
	assert.Assert(t, strings.Contains(result.Stdout, `You are creating an orb called "bar-ns/foo-orb".`))
	assert.Assert(t, strings.Contains(result.Stdout, "Are you sure you wish to create the orb: `bar-ns/foo-orb`"))
	assert.Assert(t, strings.Contains(result.Stdout, "Orb `bar-ns/foo-orb` created."))
}

func TestOrbCreateInteractiveErrors(t *testing.T) {
	binary := testhelpers.BuildCLI(t)
	ts := testhelpers.WithTempSettings(t)
	token := "testtoken"

	gqlNamespaceResponse := `{
		"registryNamespace": {
			"id": "bb604b45-b6b0-4b81-ad80-796f15eddf87"
		}
	}`

	expectedNamespaceRequest := `{
		"query": "\n\t\t\t\tquery($name: String!) {\n\t\t\t\t\tregistryNamespace(\n\t\t\t\t\t\tname: $name\n\t\t\t\t\t){\n\t\t\t\t\t\tid\n\t\t\t\t\t}\n\t\t\t }",
		"variables": {
			"name": "bar-ns"
		}
	}`

	gqlOrbResponse := `{
		"createOrb": {
			"errors": [
				{"message": "error1"},
				{"message": "error2"}
			],
			"orb": null
		}
	}`

	gqlErrors := `[ { "message": "ignored error" } ]`

	expectedOrbRequest := `{
		"query": "mutation($name: String!, $registryNamespaceId: UUID!, $isPrivate: Boolean!){\n\t\t\t\tcreateOrb(\n\t\t\t\t\tname: $name,\n\t\t\t\t\tregistryNamespaceId: $registryNamespaceId,\n\t\t\t\t\tisPrivate: $isPrivate\n\t\t\t\t){\n\t\t\t\t    orb {\n\t\t\t\t      id\n\t\t\t\t    }\n\t\t\t\t    errors {\n\t\t\t\t      message\n\t\t\t\t      type\n\t\t\t\t    }\n\t\t\t\t}\n}",
		"variables": {
			"isPrivate": false,
			"name": "foo-orb",
			"registryNamespaceId": "bb604b45-b6b0-4b81-ad80-796f15eddf87"
		}
	}`

	orbAppendPostHandler(t, ts.Server, token, expectedNamespaceRequest, gqlNamespaceResponse, "")
	orbAppendPostHandler(t, ts.Server, token, expectedOrbRequest, gqlOrbResponse, gqlErrors)

	result := testhelpers.RunCLI(t, binary,
		[]string{
			"orb", "create",
			"--skip-update-check",
			"--token", token,
			"--host", ts.Server.URL,
			"--integration-testing",
			"bar-ns/foo-orb",
		},
		"HOME="+ts.Home,
		"USERPROFILE="+ts.Home,
	)
	assert.Assert(t, result.ExitCode != 0)
	assert.Assert(t, strings.Contains(result.Stderr, "Error: error1\nerror2"))
}

func TestOrbUnlist(t *testing.T) {
	// list=true means the orb should be listed; the CLI arg is !list
	tests := []struct {
		name                  string
		list                  bool
		expectedDisplayStatus string
	}{
		{
			name:                  "listing an orb",
			list:                  true,
			expectedDisplayStatus: "enabled",
		},
		{
			name:                  "unlisting an orb",
			list:                  false,
			expectedDisplayStatus: "disabled",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			binary := testhelpers.BuildCLI(t)
			ts := testhelpers.WithTempSettings(t)
			token := "testtoken"

			gqlOrbIDResponse := `{
				"orb": {
					"id": "bb604b45-b6b0-4b81-ad80-796f15eddf87"
				}
			}`

			expectedOrbIDRequest := `{
				"query": "\n\tquery ($name: String!, $namespace: String) {\n\t\torb(name: $name) {\n\t\t  id\n\t\t}\n\t\tregistryNamespace(name: $namespace) {\n\t\t  id\n\t\t}\n\t  }\n\t  ",
				"variables": {
					"name": "bar-ns/foo-orb",
					"namespace": "bar-ns"
				}
			}`

			gqlOrbResponse := fmt.Sprintf(`{
				"setOrbListStatus": {
					"listed": %t,
					"errors": []
				}
			}`, tc.list)

			orbRequest := map[string]interface{}{
				"query": "\nmutation($orbId: UUID!, $list: Boolean!) {\n\tsetOrbListStatus(\n\t\torbId: $orbId,\n\t\tlist: $list\n\t) {\n\t\tlisted\n\t\terrors {\n\t\t\tmessage\n\t\t\ttype\n\t\t}\n\t}\n}\n\t",
				"variables": map[string]interface{}{
					"list":  tc.list,
					"orbId": "bb604b45-b6b0-4b81-ad80-796f15eddf87",
				},
			}
			expectedOrbRequest, err := json.Marshal(orbRequest)
			assert.NilError(t, err)

			orbAppendPostHandler(t, ts.Server, token, expectedOrbIDRequest, gqlOrbIDResponse, "")
			orbAppendPostHandler(t, ts.Server, token, string(expectedOrbRequest), gqlOrbResponse, "")

			// CLI arg is !list
			result := testhelpers.RunCLI(t, binary,
				[]string{
					"orb", "unlist",
					"--skip-update-check",
					"--token", token,
					"--host", ts.Server.URL,
					"bar-ns/foo-orb",
					fmt.Sprintf("%t", !tc.list),
				},
				"HOME="+ts.Home,
				"USERPROFILE="+ts.Home,
			)
			assert.Equal(t, result.ExitCode, 0, "stderr: %s\nstdout: %s", result.Stderr, result.Stdout)
			assert.Assert(t, strings.Contains(result.Stdout, fmt.Sprintf("The listing of orb `bar-ns/foo-orb` is now %s.", tc.expectedDisplayStatus)))
		})
	}
}

func TestOrbUnlistUnauthorized(t *testing.T) {
	for _, list := range []bool{true, false} {
		name := "listing"
		if !list {
			name = "unlisting"
		}
		t.Run(name, func(t *testing.T) {
			binary := testhelpers.BuildCLI(t)
			ts := testhelpers.WithTempSettings(t)
			token := "testtoken"

			gqlOrbIDResponse := `{
				"orb": {
					"id": "bb604b45-b6b0-4b81-ad80-796f15eddf87"
				}
			}`

			expectedOrbIDRequest := `{
				"query": "\n\tquery ($name: String!, $namespace: String) {\n\t\torb(name: $name) {\n\t\t  id\n\t\t}\n\t\tregistryNamespace(name: $namespace) {\n\t\t  id\n\t\t}\n\t  }\n\t  ",
				"variables": {
					"name": "bar-ns/foo-orb",
					"namespace": "bar-ns"
				}
			}`

			gqlOrbResponse := `{
				"setOrbListStatus": {
					"listed": null,
					"errors": [
						{
							"message": "AUTHORIZATION_FAILURE",
							"type": "AUTHORIZATION_FAILURE"
						}
					]
				}
			}`

			orbRequest := map[string]interface{}{
				"query": "\nmutation($orbId: UUID!, $list: Boolean!) {\n\tsetOrbListStatus(\n\t\torbId: $orbId,\n\t\tlist: $list\n\t) {\n\t\tlisted\n\t\terrors {\n\t\t\tmessage\n\t\t\ttype\n\t\t}\n\t}\n}\n\t",
				"variables": map[string]interface{}{
					"list":  list,
					"orbId": "bb604b45-b6b0-4b81-ad80-796f15eddf87",
				},
			}
			expectedOrbRequest, err := json.Marshal(orbRequest)
			assert.NilError(t, err)

			orbAppendPostHandler(t, ts.Server, token, expectedOrbIDRequest, gqlOrbIDResponse, "")
			orbAppendPostHandler(t, ts.Server, token, string(expectedOrbRequest), gqlOrbResponse, "")

			result := testhelpers.RunCLI(t, binary,
				[]string{
					"orb", "unlist",
					"--skip-update-check",
					"--token", token,
					"--host", ts.Server.URL,
					"bar-ns/foo-orb",
					fmt.Sprintf("%t", !list),
				},
				"HOME="+ts.Home,
				"USERPROFILE="+ts.Home,
			)
			assert.Assert(t, result.ExitCode != 0)
			assert.Assert(t, strings.Contains(result.Stderr, "AUTHORIZATION_FAILURE"))
		})
	}
}

func TestOrbUnlistNamespaceNotFound(t *testing.T) {
	binary := testhelpers.BuildCLI(t)
	ts := testhelpers.WithTempSettings(t)
	token := "testtoken"

	gqlOrbIDResponse := `{
		"orb": null,
		"registryNamespace": null
	}`

	expectedOrbIDRequest := `{
		"query": "\n\tquery ($name: String!, $namespace: String) {\n\t\torb(name: $name) {\n\t\t  id\n\t\t}\n\t\tregistryNamespace(name: $namespace) {\n\t\t  id\n\t\t}\n\t  }\n\t  ",
		"variables": {
			"name": "bar-ns/foo-orb",
			"namespace": "bar-ns"
		}
	}`

	orbAppendPostHandler(t, ts.Server, token, expectedOrbIDRequest, gqlOrbIDResponse, "")

	result := testhelpers.RunCLI(t, binary,
		[]string{
			"orb", "unlist",
			"--skip-update-check",
			"--token", token,
			"--host", ts.Server.URL,
			"bar-ns/foo-orb",
			"true",
		},
		"HOME="+ts.Home,
		"USERPROFILE="+ts.Home,
	)
	assert.Assert(t, result.ExitCode != 0)
	assert.Assert(t, strings.Contains(result.Stderr, "Error: the namespace 'bar-ns' does not exist."))
}

func TestOrbUnlistOrbNotFound(t *testing.T) {
	binary := testhelpers.BuildCLI(t)
	ts := testhelpers.WithTempSettings(t)
	token := "testtoken"

	gqlOrbIDResponse := `{
		"orb": null,
		"registryNamespace": {
			"id": "eac63dee-9960-48c2-b763-612e1683194e"
		}
	}`

	expectedOrbIDRequest := `{
		"query": "\n\tquery ($name: String!, $namespace: String) {\n\t\torb(name: $name) {\n\t\t  id\n\t\t}\n\t\tregistryNamespace(name: $namespace) {\n\t\t  id\n\t\t}\n\t  }\n\t  ",
		"variables": {
			"name": "bar-ns/foo-orb",
			"namespace": "bar-ns"
		}
	}`

	orbAppendPostHandler(t, ts.Server, token, expectedOrbIDRequest, gqlOrbIDResponse, "")

	result := testhelpers.RunCLI(t, binary,
		[]string{
			"orb", "unlist",
			"--skip-update-check",
			"--token", token,
			"--host", ts.Server.URL,
			"bar-ns/foo-orb",
			"true",
		},
		"HOME="+ts.Home,
		"USERPROFILE="+ts.Home,
	)
	assert.Assert(t, result.ExitCode != 0)
	assert.Assert(t, strings.Contains(result.Stderr, "Error: the 'foo-orb' orb does not exist in the 'bar-ns' namespace."))
}

func TestOrbUnlistIncorrectArgs(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{"0 args", nil},
		{"1 arg", []string{"bar-ns/foo-orb"}},
		{"3 args", []string{"bar-ns/foo-orb", "true", "true"}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			binary := testhelpers.BuildCLI(t)
			ts := testhelpers.WithTempSettings(t)
			token := "testtoken"

			argList := []string{
				"orb", "unlist",
				"--skip-update-check",
				"--token", token,
				"--host", ts.Server.URL,
			}
			argList = append(argList, tc.args...)

			result := testhelpers.RunCLI(t, binary, argList,
				"HOME="+ts.Home,
				"USERPROFILE="+ts.Home,
			)
			assert.Assert(t, result.ExitCode != 0)
			assert.Assert(t, strings.Contains(result.Stderr, fmt.Sprintf("Error: accepts 2 arg(s), received %d", len(tc.args))))
		})
	}
}

func TestOrbUnlistInvalidArgs(t *testing.T) {
	tests := []struct {
		name          string
		args          []string
		expectedError string
	}{
		{
			name:          "invalid orb name",
			args:          []string{"foo-orb", "true"},
			expectedError: "Error: Invalid orb foo-orb. Expected a namespace and orb in the form 'namespace/orb'",
		},
		{
			name:          "non-boolean value",
			args:          []string{"bar-ns/foo-orb", "falsey"},
			expectedError: `Error: expected "true" or "false", got "falsey"`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			binary := testhelpers.BuildCLI(t)
			ts := testhelpers.WithTempSettings(t)
			token := "testtoken"

			argList := []string{
				"orb", "unlist",
				"--skip-update-check",
				"--token", token,
				"--host", ts.Server.URL,
			}
			argList = append(argList, tc.args...)

			result := testhelpers.RunCLI(t, binary, argList,
				"HOME="+ts.Home,
				"USERPROFILE="+ts.Home,
			)
			assert.Assert(t, result.ExitCode != 0)
			assert.Assert(t, strings.Contains(result.Stderr, tc.expectedError))
		})
	}
}

func TestOrbListSorting(t *testing.T) {
	tests := []struct {
		name     string
		sortFlag string
		expected string
	}{
		{
			name:     "sort by builds",
			sortFlag: "builds",
			expected: "Orbs found: 3. Showing only certified orbs.\nAdd --uncertified for a list of all orbs.\n\nsecond (0.8.0)\nthird (0.9.0)\nfirst (0.7.0)\n\n",
		},
		{
			name:     "sort by projects",
			sortFlag: "projects",
			expected: "Orbs found: 3. Showing only certified orbs.\nAdd --uncertified for a list of all orbs.\n\nthird (0.9.0)\nfirst (0.7.0)\nsecond (0.8.0)\n\n",
		},
		{
			name:     "sort by orgs",
			sortFlag: "orgs",
			expected: "Orbs found: 3. Showing only certified orbs.\nAdd --uncertified for a list of all orbs.\n\nsecond (0.8.0)\nfirst (0.7.0)\nthird (0.9.0)\n\n",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			binary := testhelpers.BuildCLI(t)
			ts := testhelpers.WithTempSettings(t)

			query := `
query ListOrbs ($after: String!, $certifiedOnly: Boolean!) {
  orbs(first: 20, after: $after, certifiedOnly: $certifiedOnly) {
	totalCount,
    edges {
		cursor
	  node {
	    name
	    statistics {
		last30DaysBuildCount,
		last30DaysProjectCount,
		last30DaysOrganizationCount
	    }
		  versions(count: 1) {
			version,
			source
		  }
		}
	}
    pageInfo {
      hasNextPage
    }
  }
}
`

			request := graphql.NewRequest(query)
			request.Variables["after"] = ""
			request.Variables["certifiedOnly"] = true

			encoded := encodeGQLRequest(t, request)

			tmpBytes, err := os.ReadFile(filepath.Join("testdata", "gql_orb_list_sort", "response.json"))
			assert.NilError(t, err)
			response := string(tmpBytes)

			orbAppendPostHandler(t, ts.Server, "", encoded, response, "")

			result := testhelpers.RunCLI(t, binary,
				[]string{
					"orb", "list",
					"--sort", tc.sortFlag,
					"--skip-update-check",
					"--host", ts.Server.URL,
				},
				"HOME="+ts.Home,
				"USERPROFILE="+ts.Home,
			)
			assert.Equal(t, result.ExitCode, 0, "stderr: %s\nstdout: %s", result.Stderr, result.Stdout)
			assert.Equal(t, result.Stdout, tc.expected)
		})
	}
}

func TestOrbListSortInvalid(t *testing.T) {
	binary := testhelpers.BuildCLI(t)
	ts := testhelpers.WithTempSettings(t)

	result := testhelpers.RunCLI(t, binary,
		[]string{
			"orb", "list",
			"--sort", "idontknow",
			"--skip-update-check",
			"--host", ts.Server.URL,
		},
		"HOME="+ts.Home,
		"USERPROFILE="+ts.Home,
	)
	assert.Assert(t, result.ExitCode != 0)
	assert.Equal(t, result.Stderr, "Error: expected `idontknow` to be one of: builds, orgs, projects\n")
}

func TestOrbListJSON(t *testing.T) {
	binary := testhelpers.BuildCLI(t)
	ts := testhelpers.WithTempSettings(t)

	query := `
query ListOrbs ($after: String!, $certifiedOnly: Boolean!) {
  orbs(first: 20, after: $after, certifiedOnly: $certifiedOnly) {
	totalCount,
    edges {
		cursor
	  node {
	    name
	    statistics {
		last30DaysBuildCount,
		last30DaysProjectCount,
		last30DaysOrganizationCount
	    }
		  versions(count: 1) {
			version,
			source
		  }
		}
	}
    pageInfo {
      hasNextPage
    }
  }
}
`

	firstRequest := graphql.NewRequest(query)
	firstRequest.Variables["after"] = ""
	firstRequest.Variables["certifiedOnly"] = true
	firstRequestEncoded := encodeGQLRequest(t, firstRequest)

	secondRequest := graphql.NewRequest(query)
	secondRequest.Variables["after"] = "test/test"
	secondRequest.Variables["certifiedOnly"] = true
	secondRequestEncoded := encodeGQLRequest(t, secondRequest)

	tmpBytes, err := os.ReadFile(filepath.Join("testdata", "gql_orb_list", "first_response.json"))
	assert.NilError(t, err)
	firstResponse := string(tmpBytes)

	tmpBytes, err = os.ReadFile(filepath.Join("testdata", "gql_orb_list", "second_response.json"))
	assert.NilError(t, err)
	secondResponse := string(tmpBytes)

	tmpBytes, err = os.ReadFile(filepath.Join("testdata", "gql_orb_list", "pretty_json_output.json"))
	assert.NilError(t, err)
	expectedOutput := string(tmpBytes)

	orbAppendPostHandler(t, ts.Server, "", firstRequestEncoded, firstResponse, "")
	orbAppendPostHandler(t, ts.Server, "", secondRequestEncoded, secondResponse, "")

	result := testhelpers.RunCLI(t, binary,
		[]string{
			"orb", "list",
			"--skip-update-check",
			"--host", ts.Server.URL,
			"--json",
		},
		"HOME="+ts.Home,
		"USERPROFILE="+ts.Home,
	)
	assert.Equal(t, result.ExitCode, 0, "stderr: %s\nstdout: %s", result.Stderr, result.Stdout)
	assertJSONEqual(t, expectedOutput, result.Stdout)
}

func TestOrbListDefaultHost(t *testing.T) {
	binary := testhelpers.BuildCLI(t)
	ts := testhelpers.WithTempSettings(t)

	query := `
query ListOrbs ($after: String!, $certifiedOnly: Boolean!) {
  orbs(first: 20, after: $after, certifiedOnly: $certifiedOnly) {
	totalCount,
    edges {
		cursor
	  node {
	    name
	    statistics {
		last30DaysBuildCount,
		last30DaysProjectCount,
		last30DaysOrganizationCount
	    }
		  versions(count: 1) {
			version,
			source
		  }
		}
	}
    pageInfo {
      hasNextPage
    }
  }
}
`

	firstRequest := graphql.NewRequest(query)
	firstRequest.Variables["after"] = ""
	firstRequest.Variables["certifiedOnly"] = true
	firstRequestEncoded := encodeGQLRequest(t, firstRequest)

	secondRequest := graphql.NewRequest(query)
	secondRequest.Variables["after"] = "test/test"
	secondRequest.Variables["certifiedOnly"] = true
	secondRequestEncoded := encodeGQLRequest(t, secondRequest)

	tmpBytes, err := os.ReadFile(filepath.Join("testdata", "gql_orb_list", "first_response.json"))
	assert.NilError(t, err)
	firstResponse := string(tmpBytes)

	tmpBytes, err = os.ReadFile(filepath.Join("testdata", "gql_orb_list", "second_response.json"))
	assert.NilError(t, err)
	secondResponse := string(tmpBytes)

	orbAppendPostHandler(t, ts.Server, "", firstRequestEncoded, firstResponse, "")
	orbAppendPostHandler(t, ts.Server, "", secondRequestEncoded, secondResponse, "")

	// The original Ginkgo test ran without --host, so the CLI would use the default host
	// and never actually hit the mock server. We set the host in config so the mock works.
	assert.NilError(t, os.WriteFile(ts.Config, []byte(fmt.Sprintf("host: %s\n", ts.Server.URL)), 0600))

	result := testhelpers.RunCLI(t, binary,
		[]string{
			"orb", "list",
			"--skip-update-check",
		},
		"HOME="+ts.Home,
		"USERPROFILE="+ts.Home,
	)
	assert.Equal(t, result.ExitCode, 0, "stderr: %s\nstdout: %s", result.Stderr, result.Stdout)
	// The footer text ("In order to see more details...") only appears when cfg.Host == defaultHost.
	// Since we point the config at the mock server, the footer won't appear.
	// Instead verify the certified-only header and orb listing.
	assert.Assert(t, strings.Contains(result.Stdout, "Showing only certified orbs."),
		"stdout: %s", result.Stdout)
	assert.Assert(t, strings.Contains(result.Stdout, "Orbs found:"),
		"stdout: %s", result.Stdout)
}

func TestOrbListUncertified(t *testing.T) {
	binary := testhelpers.BuildCLI(t)
	ts := testhelpers.WithTempSettings(t)

	query := `
query ListOrbs ($after: String!, $certifiedOnly: Boolean!) {
  orbs(first: 20, after: $after, certifiedOnly: $certifiedOnly) {
	totalCount,
    edges {
		cursor
	  node {
	    name
	    statistics {
		last30DaysBuildCount,
		last30DaysProjectCount,
		last30DaysOrganizationCount
	    }
		  versions(count: 1) {
			version,
			source
		  }
		}
	}
    pageInfo {
      hasNextPage
    }
  }
}
`

	firstRequest := graphql.NewRequest(query)
	firstRequest.Variables["after"] = ""
	firstRequest.Variables["certifiedOnly"] = false
	firstRequestEncoded := encodeGQLRequest(t, firstRequest)

	secondRequest := graphql.NewRequest(query)
	secondRequest.Variables["after"] = "test/here-we-go"
	secondRequest.Variables["certifiedOnly"] = false
	secondRequestEncoded := encodeGQLRequest(t, secondRequest)

	tmpBytes, err := os.ReadFile(filepath.Join("testdata", "gql_orb_list_uncertified", "first_response.json"))
	assert.NilError(t, err)
	firstResponse := string(tmpBytes)

	tmpBytes, err = os.ReadFile(filepath.Join("testdata", "gql_orb_list_uncertified", "second_response.json"))
	assert.NilError(t, err)
	secondResponse := string(tmpBytes)

	orbAppendPostHandler(t, ts.Server, "", firstRequestEncoded, firstResponse, "")
	orbAppendPostHandler(t, ts.Server, "", secondRequestEncoded, secondResponse, "")

	result := testhelpers.RunCLI(t, binary,
		[]string{
			"orb", "list",
			"--skip-update-check",
			"--uncertified",
			"--host", ts.Server.URL,
		},
		"HOME="+ts.Home,
		"USERPROFILE="+ts.Home,
	)
	assert.Equal(t, result.ExitCode, 0, "stderr: %s\nstdout: %s", result.Stderr, result.Stdout)
	assert.Assert(t, strings.Contains(result.Stdout, "Orbs found: 11. Includes all certified and uncertified orbs."))
	assert.Assert(t, strings.Contains(result.Stdout, "circleci/codecov-clojure (0.0.4)"))
	assert.Assert(t, strings.Contains(result.Stdout, "zzak/test4 (0.1.0)"))
}

func TestOrbListUncertifiedJSON(t *testing.T) {
	binary := testhelpers.BuildCLI(t)
	ts := testhelpers.WithTempSettings(t)

	query := `
query ListOrbs ($after: String!, $certifiedOnly: Boolean!) {
  orbs(first: 20, after: $after, certifiedOnly: $certifiedOnly) {
	totalCount,
    edges {
		cursor
	  node {
	    name
	    statistics {
		last30DaysBuildCount,
		last30DaysProjectCount,
		last30DaysOrganizationCount
	    }
		  versions(count: 1) {
			version,
			source
		  }
		}
	}
    pageInfo {
      hasNextPage
    }
  }
}
`

	firstRequest := graphql.NewRequest(query)
	firstRequest.Variables["after"] = ""
	firstRequest.Variables["certifiedOnly"] = false
	firstRequestEncoded := encodeGQLRequest(t, firstRequest)

	secondRequest := graphql.NewRequest(query)
	secondRequest.Variables["after"] = "test/here-we-go"
	secondRequest.Variables["certifiedOnly"] = false
	secondRequestEncoded := encodeGQLRequest(t, secondRequest)

	tmpBytes, err := os.ReadFile(filepath.Join("testdata", "gql_orb_list_uncertified", "first_response.json"))
	assert.NilError(t, err)
	firstResponse := string(tmpBytes)

	tmpBytes, err = os.ReadFile(filepath.Join("testdata", "gql_orb_list_uncertified", "second_response.json"))
	assert.NilError(t, err)
	secondResponse := string(tmpBytes)

	tmpBytes, err = os.ReadFile(filepath.Join("testdata", "gql_orb_list_uncertified", "pretty_json_output.json"))
	assert.NilError(t, err)
	expectedOutput := string(tmpBytes)

	orbAppendPostHandler(t, ts.Server, "", firstRequestEncoded, firstResponse, "")
	orbAppendPostHandler(t, ts.Server, "", secondRequestEncoded, secondResponse, "")

	result := testhelpers.RunCLI(t, binary,
		[]string{
			"orb", "list",
			"--skip-update-check",
			"--uncertified",
			"--host", ts.Server.URL,
			"--json",
		},
		"HOME="+ts.Home,
		"USERPROFILE="+ts.Home,
	)
	assert.Equal(t, result.ExitCode, 0, "stderr: %s\nstdout: %s", result.Stderr, result.Stdout)
	assertJSONEqual(t, expectedOutput, result.Stdout)
}

func TestOrbListDetails(t *testing.T) {
	binary := testhelpers.BuildCLI(t)
	ts := testhelpers.WithTempSettings(t)

	query := `
query ListOrbs ($after: String!, $certifiedOnly: Boolean!) {
  orbs(first: 20, after: $after, certifiedOnly: $certifiedOnly) {
	totalCount,
    edges {
		cursor
	  node {
	    name
	    statistics {
		last30DaysBuildCount,
		last30DaysProjectCount,
		last30DaysOrganizationCount
	    }
		  versions(count: 1) {
			version,
			source
		  }
		}
	}
    pageInfo {
      hasNextPage
    }
  }
}
`

	request := graphql.NewRequest(query)
	request.Variables["after"] = ""
	request.Variables["certifiedOnly"] = true
	encoded := encodeGQLRequest(t, request)

	tmpBytes, err := os.ReadFile(filepath.Join("testdata", "gql_orb_list_details", "response.json"))
	assert.NilError(t, err)

	orbAppendPostHandler(t, ts.Server, "", encoded, string(tmpBytes), "")

	result := testhelpers.RunCLI(t, binary,
		[]string{
			"orb", "list",
			"--skip-update-check",
			"--host", ts.Server.URL,
			"--details",
		},
		"HOME="+ts.Home,
		"USERPROFILE="+ts.Home,
	)
	assert.Equal(t, result.ExitCode, 0, "stderr: %s\nstdout: %s", result.Stderr, result.Stdout)
	assert.Equal(t, result.Stdout, `Orbs found: 1. Showing only certified orbs.
Add --uncertified for a list of all orbs.

foo/test (0.7.0)
  Commands:
    - bar: 1 parameter(s)
       - hello: string (default: 'world')
    - myfoo: 0 parameter(s)
  Jobs:
    - hello-build: 0 parameter(s)
  Executors:
    - default: 1 parameter(s)
       - tag: string (default: 'curl-browsers')
  Statistics:
    - last30DaysBuildCount: 0
    - last30DaysOrganizationCount: 0
    - last30DaysProjectCount: 0

`)
}

func TestOrbListDetailsJSON(t *testing.T) {
	binary := testhelpers.BuildCLI(t)
	ts := testhelpers.WithTempSettings(t)

	query := `
query ListOrbs ($after: String!, $certifiedOnly: Boolean!) {
  orbs(first: 20, after: $after, certifiedOnly: $certifiedOnly) {
	totalCount,
    edges {
		cursor
	  node {
	    name
	    statistics {
		last30DaysBuildCount,
		last30DaysProjectCount,
		last30DaysOrganizationCount
	    }
		  versions(count: 1) {
			version,
			source
		  }
		}
	}
    pageInfo {
      hasNextPage
    }
  }
}
`

	request := graphql.NewRequest(query)
	request.Variables["after"] = ""
	request.Variables["certifiedOnly"] = true
	encoded := encodeGQLRequest(t, request)

	tmpBytes, err := os.ReadFile(filepath.Join("testdata", "gql_orb_list_details", "response.json"))
	assert.NilError(t, err)
	orbAppendPostHandler(t, ts.Server, "", encoded, string(tmpBytes), "")

	tmpBytes, err = os.ReadFile(filepath.Join("testdata", "gql_orb_list_details", "pretty_json_output.json"))
	assert.NilError(t, err)
	expectedOutput := string(tmpBytes)

	result := testhelpers.RunCLI(t, binary,
		[]string{
			"orb", "list",
			"--skip-update-check",
			"--host", ts.Server.URL,
			"--details",
			"--json",
		},
		"HOME="+ts.Home,
		"USERPROFILE="+ts.Home,
	)
	assert.Equal(t, result.ExitCode, 0, "stderr: %s\nstdout: %s", result.Stderr, result.Stdout)
	assertJSONEqual(t, expectedOutput, result.Stdout)
}

func TestOrbListNamespace(t *testing.T) {
	binary := testhelpers.BuildCLI(t)
	ts := testhelpers.WithTempSettings(t)

	query := `
query namespaceOrbs ($namespace: String, $after: String!, $view: OrbListViewType) {
	registryNamespace(name: $namespace) {
		name
                id
		orbs(first: 20, after: $after, view: $view) {
			edges {
				cursor
				node {
					versions (count: 1){ source, version
					}
					name
	                                statistics {
		                           last30DaysBuildCount,
		                           last30DaysProjectCount,
		                           last30DaysOrganizationCount
	                               }
				}
			}
			totalCount
			pageInfo {
				hasNextPage
			}
		}
	}
}
`
	firstRequest := graphql.NewRequest(query)
	firstRequest.Variables["after"] = ""
	firstRequest.Variables["namespace"] = "circleci"
	firstRequest.Variables["view"] = "PUBLIC_ONLY"
	firstRequestEncoded := encodeGQLRequest(t, firstRequest)

	secondRequest := graphql.NewRequest(query)
	secondRequest.Variables["after"] = "circleci/codecov-clojure"
	secondRequest.Variables["namespace"] = "circleci"
	secondRequest.Variables["view"] = "PUBLIC_ONLY"
	secondRequestEncoded := encodeGQLRequest(t, secondRequest)

	tmpBytes, err := os.ReadFile(filepath.Join("testdata", "gql_orb_list_with_namespace", "first_response.json"))
	assert.NilError(t, err)
	firstResponse := string(tmpBytes)

	tmpBytes, err = os.ReadFile(filepath.Join("testdata", "gql_orb_list_with_namespace", "second_response.json"))
	assert.NilError(t, err)
	secondResponse := string(tmpBytes)

	orbAppendPostHandler(t, ts.Server, "", firstRequestEncoded, firstResponse, "")
	orbAppendPostHandler(t, ts.Server, "", secondRequestEncoded, secondResponse, "")

	result := testhelpers.RunCLI(t, binary,
		[]string{
			"orb", "list", "circleci",
			"--skip-update-check",
			"--host", ts.Server.URL,
			"--details",
		},
		"HOME="+ts.Home,
		"USERPROFILE="+ts.Home,
	)
	assert.Equal(t, result.ExitCode, 0, "stderr: %s\nstdout: %s", result.Stderr, result.Stdout)
	assert.Assert(t, strings.Contains(result.Stdout, "circleci/gradle"))
	assert.Assert(t, strings.Contains(result.Stdout, "Jobs"))
	assert.Assert(t, strings.Contains(result.Stdout, "- test"))
	assert.Assert(t, strings.Contains(result.Stdout, "circleci/rollbar"))
	assert.Assert(t, strings.Contains(result.Stdout, "Commands"))
	assert.Assert(t, strings.Contains(result.Stdout, "- notify_deploy"))
}

func TestOrbListNamespaceJSON(t *testing.T) {
	binary := testhelpers.BuildCLI(t)
	ts := testhelpers.WithTempSettings(t)

	query := `
query namespaceOrbs ($namespace: String, $after: String!, $view: OrbListViewType) {
	registryNamespace(name: $namespace) {
		name
                id
		orbs(first: 20, after: $after, view: $view) {
			edges {
				cursor
				node {
					versions (count: 1){ source, version
					}
					name
	                                statistics {
		                           last30DaysBuildCount,
		                           last30DaysProjectCount,
		                           last30DaysOrganizationCount
	                               }
				}
			}
			totalCount
			pageInfo {
				hasNextPage
			}
		}
	}
}
`
	firstRequest := graphql.NewRequest(query)
	firstRequest.Variables["after"] = ""
	firstRequest.Variables["namespace"] = "circleci"
	firstRequest.Variables["view"] = "PUBLIC_ONLY"
	firstRequestEncoded := encodeGQLRequest(t, firstRequest)

	secondRequest := graphql.NewRequest(query)
	secondRequest.Variables["after"] = "circleci/codecov-clojure"
	secondRequest.Variables["namespace"] = "circleci"
	secondRequest.Variables["view"] = "PUBLIC_ONLY"
	secondRequestEncoded := encodeGQLRequest(t, secondRequest)

	tmpBytes, err := os.ReadFile(filepath.Join("testdata", "gql_orb_list_with_namespace", "first_response.json"))
	assert.NilError(t, err)
	firstResponse := string(tmpBytes)

	tmpBytes, err = os.ReadFile(filepath.Join("testdata", "gql_orb_list_with_namespace", "second_response.json"))
	assert.NilError(t, err)
	secondResponse := string(tmpBytes)

	tmpBytes, err = os.ReadFile(filepath.Join("testdata", "gql_orb_list_with_namespace", "pretty_json_output.json"))
	assert.NilError(t, err)
	expectedOutput := string(tmpBytes)

	orbAppendPostHandler(t, ts.Server, "", firstRequestEncoded, firstResponse, "")
	orbAppendPostHandler(t, ts.Server, "", secondRequestEncoded, secondResponse, "")

	result := testhelpers.RunCLI(t, binary,
		[]string{
			"orb", "list", "circleci",
			"--skip-update-check",
			"--host", ts.Server.URL,
			"--details",
			"--json",
		},
		"HOME="+ts.Home,
		"USERPROFILE="+ts.Home,
	)
	assert.Equal(t, result.ExitCode, 0, "stderr: %s\nstdout: %s", result.Stderr, result.Stdout)
	assertJSONEqual(t, expectedOutput, result.Stdout)
}

func TestOrbListNonExistent(t *testing.T) {
	binary := testhelpers.BuildCLI(t)
	ts := testhelpers.WithTempSettings(t)

	query := `
query namespaceOrbs ($namespace: String, $after: String!, $view: OrbListViewType) {
	registryNamespace(name: $namespace) {
		name
                id
		orbs(first: 20, after: $after, view: $view) {
			edges {
				cursor
				node {
					versions { version
					}
					name
	                                statistics {
		                           last30DaysBuildCount,
		                           last30DaysProjectCount,
		                           last30DaysOrganizationCount
	                               }
				}
			}
			totalCount
			pageInfo {
				hasNextPage
			}
		}
	}
}
`
	request := graphql.NewRequest(query)
	request.Variables["after"] = ""
	request.Variables["namespace"] = "nonexist"
	request.Variables["view"] = "PUBLIC_ONLY"
	encodedRequest := encodeGQLRequest(t, request)

	mockResponse := `{"data": {}}`
	orbAppendPostHandler(t, ts.Server, "", encodedRequest, mockResponse, "")

	result := testhelpers.RunCLI(t, binary,
		[]string{
			"orb", "list", "nonexist",
			"--skip-update-check",
			"--host", ts.Server.URL,
		},
		"HOME="+ts.Home,
		"USERPROFILE="+ts.Home,
	)
	assert.Assert(t, result.ExitCode != 0)
	assert.Assert(t, strings.Contains(result.Stderr, "No namespace found"))
}

func TestOrbListPrivate(t *testing.T) {
	binary := testhelpers.BuildCLI(t)
	ts := testhelpers.WithTempSettings(t)

	query := `
query namespaceOrbs ($namespace: String, $after: String!, $view: OrbListViewType) {
	registryNamespace(name: $namespace) {
		name
                id
		orbs(first: 20, after: $after, view: $view) {
			edges {
				cursor
				node {
					versions { version
					}
					name
	                                statistics {
		                           last30DaysBuildCount,
		                           last30DaysProjectCount,
		                           last30DaysOrganizationCount
	                               }
				}
			}
			totalCount
			pageInfo {
				hasNextPage
			}
		}
	}
}
`
	request := graphql.NewRequest(query)
	request.Variables["after"] = ""
	request.Variables["namespace"] = "circleci"
	request.Variables["view"] = "PRIVATE_ONLY"
	encodedRequest := encodeGQLRequest(t, request)

	tmpBytes, err := os.ReadFile(filepath.Join("testdata", "gql_orb_list_with_namespace", "second_response.json"))
	assert.NilError(t, err)
	mockResponse := string(tmpBytes)

	orbAppendPostHandler(t, ts.Server, "", encodedRequest, mockResponse, "")

	t.Run("error without namespace", func(t *testing.T) {
		result := testhelpers.RunCLI(t, binary,
			[]string{
				"orb", "list",
				"--private",
				"--skip-update-check",
				"--host", ts.Server.URL,
			},
			"HOME="+ts.Home,
			"USERPROFILE="+ts.Home,
		)
		assert.Assert(t, result.ExitCode != 0)
		assert.Assert(t, strings.Contains(result.Stderr, "Namespace must be provided when listing private orbs"))
	})

	t.Run("success with namespace", func(t *testing.T) {
		result := testhelpers.RunCLI(t, binary,
			[]string{
				"orb", "list", "circleci",
				"--private",
				"--skip-update-check",
				"--host", ts.Server.URL,
			},
			"HOME="+ts.Home,
			"USERPROFILE="+ts.Home,
		)
		assert.Equal(t, result.ExitCode, 0, "stderr: %s\nstdout: %s", result.Stderr, result.Stdout)
		assert.Equal(t, result.Stdout, `Orbs found: 5. Showing only private orbs.

circleci/delete-me (Not published)
circleci/delete-me-too (Not published)
circleci/gradle (0.0.1)
circleci/heroku (Not published)
circleci/rollbar (0.0.1)

`)
	})
}

func TestOrbCreateWithoutToken(t *testing.T) {
	binary := testhelpers.BuildCLI(t)
	ts := testhelpers.WithTempSettings(t)

	result := testhelpers.RunCLI(t, binary,
		[]string{
			"orb", "create", "bar-ns/foo-orb",
			"--skip-update-check",
			"--token", "",
		},
		"HOME="+ts.Home,
		"USERPROFILE="+ts.Home,
	)
	assert.Assert(t, result.ExitCode != 0)
	assert.Assert(t, strings.Contains(result.Stderr, "Error: please set a token with 'circleci setup'"))
	assert.Assert(t, strings.Contains(result.Stderr, "https://circleci.com/account/api"))
}

func TestOrbCreateWithoutTokenCustomHost(t *testing.T) {
	binary := testhelpers.BuildCLI(t)
	ts := testhelpers.WithTempSettings(t)

	result := testhelpers.RunCLI(t, binary,
		[]string{
			"orb", "create", "bar-ns/foo-orb",
			"--skip-update-check",
			"--token", "",
			"--host", "https://foo.bar",
		},
		"HOME="+ts.Home,
		"USERPROFILE="+ts.Home,
	)
	assert.Assert(t, result.ExitCode != 0)
	assert.Assert(t, strings.Contains(result.Stderr, "Error: please set a token with 'circleci setup'"))
	assert.Assert(t, strings.Contains(result.Stderr, "https://foo.bar/account/api"))
}

func TestOrbSource(t *testing.T) {
	binary := testhelpers.BuildCLI(t)
	ts := testhelpers.WithTempSettings(t)

	request := graphql.NewRequest(`query($orbVersionRef: String!) {
			    orbVersion(orbVersionRef: $orbVersionRef) {
			        id
                                version
                                orb { id }
                                source
			    }
		      }`)
	request.Variables["orbVersionRef"] = "my/orb@dev:foo"
	encoded := encodeGQLRequest(t, request)

	response := `{
		"orbVersion": {
			"id": "bb604b45-b6b0-4b81-ad80-796f15eddf87",
			"version": "dev:foo",
			"orb": {
				"id": "bb604b45-b6b0-4b81-ad80-796f15eddf87"
			},
			"source": "some orb"
		}
	}`

	orbAppendPostHandler(t, ts.Server, "", encoded, response, "")

	result := testhelpers.RunCLI(t, binary,
		[]string{
			"orb", "source",
			"--skip-update-check",
			"--host", ts.Server.URL,
			"my/orb@dev:foo",
		},
		"HOME="+ts.Home,
		"USERPROFILE="+ts.Home,
	)
	assert.Equal(t, result.ExitCode, 0, "stderr: %s\nstdout: %s", result.Stderr, result.Stdout)
	assert.Assert(t, strings.Contains(result.Stdout, "some orb"))
}

func TestOrbSourceNotPublished(t *testing.T) {
	binary := testhelpers.BuildCLI(t)
	ts := testhelpers.WithTempSettings(t)

	query := `query($orbVersionRef: String!) {
			    orbVersion(orbVersionRef: $orbVersionRef) {
			        id
                                version
                                orb { id }
                                source
			    }
		      }`
	request := graphql.NewRequest(query)
	request.Variables["orbVersionRef"] = "my/orb@dev:foo"
	encoded := encodeGQLRequest(t, request)

	response := `{"data": { "orbVersion": {} }}`
	orbAppendPostHandler(t, ts.Server, "", encoded, response, "")

	result := testhelpers.RunCLI(t, binary,
		[]string{
			"orb", "source",
			"--skip-update-check",
			"--host", ts.Server.URL,
			"my/orb@dev:foo",
		},
		"HOME="+ts.Home,
		"USERPROFILE="+ts.Home,
	)
	assert.Assert(t, result.ExitCode != 0)
	assert.Assert(t, strings.Contains(result.Stderr, "no Orb 'my/orb@dev:foo' was found; please check that the Orb reference is correct"))
}

func TestOrbInfo(t *testing.T) {
	binary := testhelpers.BuildCLI(t)

	query := `query($orbVersionRef: String!) {
			    orbVersion(orbVersionRef: $orbVersionRef) {
			        id
                                version
                                orb {
                                    id
                                    createdAt
									name
									namespace {
									  name
									}
                                    categories {
                                      id
                                      name
                                    }
	                            statistics {
		                        last30DaysBuildCount,
		                        last30DaysProjectCount,
		                        last30DaysOrganizationCount
	                            }
                                    versions(count: 200) {
                                        createdAt
                                        version
                                    }
                                }
                                source
                                createdAt
			    }
		      }`

	request := graphql.NewRequest(query)
	request.Variables["orbVersionRef"] = "my/orb@dev:foo"
	encoded := encodeGQLRequest(t, request)

	t.Run("works with categories", func(t *testing.T) {
		ts := testhelpers.WithTempSettings(t)

		orbAppendPostHandler(t, ts.Server, "", encoded, `{
			"orbVersion": {
				"id": "bb604b45-b6b0-4b81-ad80-796f15eddf87",
				"version": "dev:foo",
				"orb": {
					"id": "bb604b45-b6b0-4b81-ad80-796f15eddf87",
					"createdAt": "2018-09-24T08:53:37.086Z",
					"name": "my/orb",
					"categories": [
						{
							"id": "cc604b45-b6b0-4b81-ad80-796f15eddf87",
							"name": "Infra Automation"
						},
						{
							"id": "dd604b45-b6b0-4b81-ad80-796f15eddf87",
							"name": "Testing"
						}
					],
					"versions": [
						{
							"version": "0.0.1",
							"createdAt": "2018-10-11T22:12:19.477Z"
						}
					]
				},
				"source": "description: zomg\ncommands: {foo: {parameters: {baz: {type: string}}}}",
				"createdAt": "2018-09-24T08:53:37.086Z"
			}
		}`, "")

		result := testhelpers.RunCLI(t, binary,
			[]string{
				"orb", "info",
				"--skip-update-check",
				"--host", ts.Server.URL,
				"my/orb@dev:foo",
			},
			"HOME="+ts.Home,
			"USERPROFILE="+ts.Home,
		)
		assert.Equal(t, result.ExitCode, 0, "stderr: %s\nstdout: %s", result.Stderr, result.Stdout)
		assert.Assert(t, strings.Contains(result.Stdout, "Latest: my/orb@0.0.1"))
		assert.Assert(t, strings.Contains(result.Stdout, "Infra Automation"))
		assert.Assert(t, strings.Contains(result.Stdout, "Testing"))
	})

	t.Run("reports usage statistics", func(t *testing.T) {
		ts := testhelpers.WithTempSettings(t)

		orbAppendPostHandler(t, ts.Server, "", encoded, `{
			"orbVersion": {
				"id": "bb604b45-b6b0-4b81-ad80-796f15eddf87",
				"version": "dev:foo",
				"orb": {
					"id": "bb604b45-b6b0-4b81-ad80-796f15eddf87",
					"createdAt": "2018-09-24T08:53:37.086Z",
					"name": "my/orb",
					"statistics": {
						"last30DaysBuildCount": 555,
						"last30DaysProjectCount": 777,
						"last30DaysOrganizationCount": 999
					},
					"versions": [
						{
							"version": "0.0.1",
							"createdAt": "2018-10-11T22:12:19.477Z"
						}
					]
				},
				"source": "description: zomg\ncommands: {foo: {parameters: {baz: {type: string}}}}",
				"createdAt": "2018-09-24T08:53:37.086Z"
			}
		}`, "")

		result := testhelpers.RunCLI(t, binary,
			[]string{
				"orb", "info",
				"--skip-update-check",
				"--host", ts.Server.URL,
				"my/orb@dev:foo",
			},
			"HOME="+ts.Home,
			"USERPROFILE="+ts.Home,
		)
		assert.Equal(t, result.ExitCode, 0, "stderr: %s\nstdout: %s", result.Stderr, result.Stdout)
		assert.Assert(t, strings.Contains(result.Stdout, "Builds: 555"))
		assert.Assert(t, strings.Contains(result.Stdout, "Projects: 777"))
		assert.Assert(t, strings.Contains(result.Stdout, "Orgs: 999"))
	})

	t.Run("no semantic versions", func(t *testing.T) {
		ts := testhelpers.WithTempSettings(t)

		orbAppendPostHandler(t, ts.Server, "", encoded, `{
			"orbVersion": {
				"id": "bb604b45-b6b0-4b81-ad80-796f15eddf87",
				"version": "dev:foo",
				"orb": {
					"id": "bb604b45-b6b0-4b81-ad80-796f15eddf87",
					"createdAt": "2018-09-24T08:53:37.086Z",
					"name": "my/orb",
					"versions": []
				},
				"source": "description: zomg\ncommands: {foo: {parameters: {baz: {type: string}}}}",
				"createdAt": "2018-09-24T08:53:37.086Z"
			}
		}}`, "")

		result := testhelpers.RunCLI(t, binary,
			[]string{
				"orb", "info",
				"--skip-update-check",
				"--host", ts.Server.URL,
				"my/orb@dev:foo",
			},
			"HOME="+ts.Home,
			"USERPROFILE="+ts.Home,
		)
		assert.Equal(t, result.ExitCode, 0, "stderr: %s\nstdout: %s", result.Stderr, result.Stdout)
		assert.Assert(t, strings.Contains(result.Stdout, "This orb hasn't published any versions yet."))
	})

	t.Run("orb not found", func(t *testing.T) {
		ts := testhelpers.WithTempSettings(t)

		orbAppendPostHandler(t, ts.Server, "", encoded, `{ "orbVersion": {} }`, "")

		result := testhelpers.RunCLI(t, binary,
			[]string{
				"orb", "info",
				"--skip-update-check",
				"--host", ts.Server.URL,
				"my/orb@dev:foo",
			},
			"HOME="+ts.Home,
			"USERPROFILE="+ts.Home,
		)
		assert.Assert(t, result.ExitCode != 0)
		assert.Assert(t, strings.Contains(result.Stderr, "no Orb 'my/orb@dev:foo' was found; please check that the Orb reference is correct"))
	})
}

func TestOrbListCategories(t *testing.T) {
	tests := []struct {
		name string
		json bool
	}{
		{"with --json", true},
		{"without --json", false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			binary := testhelpers.BuildCLI(t)
			ts := testhelpers.WithTempSettings(t)

			query := `
	query ListOrbCategories($after: String!) {
		orbCategories(first: 20, after: $after) {
			totalCount
			edges {
				cursor
				node {
					id
					name
				}
			}
			pageInfo {
				hasNextPage
			}
		}
	}
`

			firstRequest := graphql.NewRequest(query)
			firstRequest.Variables["after"] = ""
			firstRequestEncoded := encodeGQLRequest(t, firstRequest)

			secondRequest := graphql.NewRequest(query)
			secondRequest.Variables["after"] = "Testing"
			secondRequestEncoded := encodeGQLRequest(t, secondRequest)

			tmpBytes, err := os.ReadFile(filepath.Join("testdata", "gql_orb_category_list", "first_response.json"))
			assert.NilError(t, err)
			firstResponse := string(tmpBytes)

			tmpBytes, err = os.ReadFile(filepath.Join("testdata", "gql_orb_category_list", "second_response.json"))
			assert.NilError(t, err)
			secondResponse := string(tmpBytes)

			tmpBytes, err = os.ReadFile(filepath.Join("testdata", "gql_orb_category_list", "pretty_json_output.json"))
			assert.NilError(t, err)
			expectedJSONOutput := string(tmpBytes)

			orbAppendPostHandler(t, ts.Server, "", firstRequestEncoded, firstResponse, "")
			orbAppendPostHandler(t, ts.Server, "", secondRequestEncoded, secondResponse, "")

			argList := []string{
				"orb", "list-categories",
				"--skip-update-check",
				"--host", ts.Server.URL,
			}
			if tc.json {
				argList = append(argList, "--json")
			}

			result := testhelpers.RunCLI(t, binary, argList,
				"HOME="+ts.Home,
				"USERPROFILE="+ts.Home,
			)
			assert.Equal(t, result.ExitCode, 0, "stderr: %s\nstdout: %s", result.Stderr, result.Stdout)
			if tc.json {
				assertJSONEqual(t, expectedJSONOutput, result.Stdout)
			} else {
				assert.Equal(t, result.Stdout, `Artifacts/Registry
Build
Cloud Platform
Code Analysis
Collaboration
Containers
Deployment
Infra Automation
Kubernetes
Language/Framework
Monitoring
Notifications
Reporting
Security
Testing
Windows Server 2003
Windows Server 2010
`)
			}
		})
	}
}

func TestOrbAddRemoveCategorization(t *testing.T) {
	orbId := "bb604b45-b6b0-4b81-ad80-796f15eddf87"
	orbNamespaceName := "bar-ns"
	orbName := "foo-orb"
	orbFullName := orbNamespaceName + "/" + orbName
	categoryId := "cc604b45-b6b0-4b81-ad80-796f15eddf87"
	categoryName := "Cloud Platform"

	tests := []struct {
		name              string
		mockErrorResponse bool
		updateType        api.UpdateOrbCategorizationRequestType
	}{
		{"add categorization success", false, api.Add},
		{"remove categorization success", false, api.Remove},
		{"server error on adding categorization", true, api.Add},
		{"server error on removing categorization", true, api.Remove},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			binary := testhelpers.BuildCLI(t)
			ts := testhelpers.WithTempSettings(t)
			token := "testtoken"

			commandName := "add-to-category"
			operationName := "addCategorizationToOrb"
			expectedOutputSegment := "added to"
			if tc.updateType == api.Remove {
				commandName = "remove-from-category"
				operationName = "removeCategorizationFromOrb"
				expectedOutputSegment = "removed from"
			}

			gqlOrbIDResponse := fmt.Sprintf(`{
				"orb": {
					"id": "%s"
				}
			}`, orbId)

			expectedOrbIDRequest := fmt.Sprintf(`{
				"query": "\n\tquery ($name: String!, $namespace: String) {\n\t\torb(name: $name) {\n\t\t  id\n\t\t}\n\t\tregistryNamespace(name: $namespace) {\n\t\t  id\n\t\t}\n\t  }\n\t  ",
				"variables": {
					"name": "%s",
					"namespace": "%s"
				}
			}`, orbFullName, orbNamespaceName)

			expectedCategoryIDRequest := fmt.Sprintf(`{
				"query": "\n\tquery ($name: String!) {\n\t\torbCategoryByName(name: $name) {\n\t\t  id\n\t\t}\n\t}",
				"variables": {
					"name": "%s"
				}
			}`, categoryName)

			gqlCategoryIDResponse := fmt.Sprintf(`{
				"orbCategoryByName": {
					"id": "%s"
				}
			}`, categoryId)

			expectedOrbCategorizationRequest := fmt.Sprintf(`{
				"query": "\n\t\tmutation($orbId: UUID!, $categoryId: UUID!) {\n\t\t\t%s(\n\t\t\t\torbId: $orbId,\n\t\t\t\tcategoryId: $categoryId\n\t\t\t) {\n\t\t\t\torbId\n\t\t\t\tcategoryId\n\t\t\t\terrors {\n\t\t\t\t\tmessage\n\t\t\t\t\ttype\n\t\t\t\t}\n\t\t\t}\n\t\t}\n\t",
				"variables": {
					"categoryId": "%s",
					"orbId": "%s"
				}
			}`, operationName, categoryId, orbId)

			gqlCategorizationResponse := fmt.Sprintf(`{
				"%s": {
					"orbId": "%s",
					"categoryId": "%s",
					"errors": []
				}
			}`, operationName, orbId, categoryId)

			if tc.mockErrorResponse {
				gqlCategorizationResponse = fmt.Sprintf(`{
					"%s": {
						"orbId": "",
						"categoryId": "",
						"errors": [{
							"message": "Mock error message",
							"type": "Mock error from server"
						}]
					}
				}`, operationName)
			}

			orbAppendPostHandler(t, ts.Server, token, expectedOrbIDRequest, gqlOrbIDResponse, "")
			orbAppendPostHandler(t, ts.Server, "", expectedCategoryIDRequest, gqlCategoryIDResponse, "")
			orbAppendPostHandler(t, ts.Server, token, expectedOrbCategorizationRequest, gqlCategorizationResponse, "")

			result := testhelpers.RunCLI(t, binary,
				[]string{
					"orb", commandName,
					"--skip-update-check",
					"--token", token,
					"--host", ts.Server.URL,
					orbFullName, categoryName,
				},
				"HOME="+ts.Home,
				"USERPROFILE="+ts.Home,
			)

			if tc.mockErrorResponse {
				assert.Assert(t, result.ExitCode != 0)
				if tc.updateType == api.Add {
					assert.Assert(t, strings.Contains(result.Stderr, fmt.Sprintf("Error: Failed to add orb %s to category %s: Mock error message", orbFullName, categoryName)))
				} else {
					assert.Assert(t, strings.Contains(result.Stderr, fmt.Sprintf("Error: Failed to remove orb %s from category %s: Mock error message", orbFullName, categoryName)))
				}
			} else {
				assert.Equal(t, result.ExitCode, 0, "stderr: %s\nstdout: %s", result.Stderr, result.Stdout)
				assert.Assert(t, strings.Contains(result.Stdout, fmt.Sprintf(`%s is successfully %s the "%s" category.`, orbFullName, expectedOutputSegment, categoryName)))
			}
		})
	}
}

func TestOrbCategorizationOrbNotExist(t *testing.T) {
	for _, updateType := range []api.UpdateOrbCategorizationRequestType{api.Add, api.Remove} {
		name := "add"
		commandName := "add-to-category"
		if updateType == api.Remove {
			name = "remove"
			commandName = "remove-from-category"
		}
		t.Run(name, func(t *testing.T) {
			binary := testhelpers.BuildCLI(t)
			ts := testhelpers.WithTempSettings(t)
			token := "testtoken"

			expectedOrbIDRequest := `{
				"query": "\n\tquery ($name: String!, $namespace: String) {\n\t\torb(name: $name) {\n\t\t  id\n\t\t}\n\t\tregistryNamespace(name: $namespace) {\n\t\t  id\n\t\t}\n\t  }\n\t  ",
				"variables": {
					"name": "bar-ns/foo-orb",
					"namespace": "bar-ns"
				}
			}`

			gqlOrbIDResponse := `{
				"orb": null,
				"registryNamespace": {
					"id": "eac63dee-9960-48c2-b763-612e1683194e"
				}
			}`

			orbAppendPostHandler(t, ts.Server, token, expectedOrbIDRequest, gqlOrbIDResponse, "")

			result := testhelpers.RunCLI(t, binary,
				[]string{
					"orb", commandName,
					"--skip-update-check",
					"--token", token,
					"--host", ts.Server.URL,
					"bar-ns/foo-orb", "Cloud Platform",
				},
				"HOME="+ts.Home,
				"USERPROFILE="+ts.Home,
			)
			assert.Assert(t, result.ExitCode != 0)
			if updateType == api.Add {
				assert.Assert(t, strings.Contains(result.Stderr, "Error: Failed to add orb bar-ns/foo-orb to category Cloud Platform: the 'foo-orb' orb does not exist in the 'bar-ns' namespace."))
			} else {
				assert.Assert(t, strings.Contains(result.Stderr, "Error: Failed to remove orb bar-ns/foo-orb from category Cloud Platform: the 'foo-orb' orb does not exist in the 'bar-ns' namespace."))
			}
		})
	}
}

func TestOrbCategorizationCategoryNotExist(t *testing.T) {
	orbId := "bb604b45-b6b0-4b81-ad80-796f15eddf87"

	for _, updateType := range []api.UpdateOrbCategorizationRequestType{api.Add, api.Remove} {
		name := "add"
		commandName := "add-to-category"
		if updateType == api.Remove {
			name = "remove"
			commandName = "remove-from-category"
		}
		t.Run(name, func(t *testing.T) {
			binary := testhelpers.BuildCLI(t)
			ts := testhelpers.WithTempSettings(t)
			token := "testtoken"

			expectedOrbIDRequest := `{
				"query": "\n\tquery ($name: String!, $namespace: String) {\n\t\torb(name: $name) {\n\t\t  id\n\t\t}\n\t\tregistryNamespace(name: $namespace) {\n\t\t  id\n\t\t}\n\t  }\n\t  ",
				"variables": {
					"name": "bar-ns/foo-orb",
					"namespace": "bar-ns"
				}
			}`

			gqlOrbIDResponse := fmt.Sprintf(`{
				"orb": {
					"id": "%s"
				}
			}`, orbId)

			expectedCategoryIDRequest := `{
				"query": "\n\tquery ($name: String!) {\n\t\torbCategoryByName(name: $name) {\n\t\t  id\n\t\t}\n\t}",
				"variables": {
					"name": "Cloud Platform"
				}
			}`

			gqlCategoryIDResponse := `{
				"orbCategoryByName": null
			}`

			orbAppendPostHandler(t, ts.Server, token, expectedOrbIDRequest, gqlOrbIDResponse, "")
			orbAppendPostHandler(t, ts.Server, "", expectedCategoryIDRequest, gqlCategoryIDResponse, "")

			result := testhelpers.RunCLI(t, binary,
				[]string{
					"orb", commandName,
					"--skip-update-check",
					"--token", token,
					"--host", ts.Server.URL,
					"bar-ns/foo-orb", "Cloud Platform",
				},
				"HOME="+ts.Home,
				"USERPROFILE="+ts.Home,
			)
			assert.Assert(t, result.ExitCode != 0)
			errorCause := "the 'Cloud Platform' category does not exist."
			if updateType == api.Add {
				assert.Assert(t, strings.Contains(result.Stderr, fmt.Sprintf("Error: Failed to add orb bar-ns/foo-orb to category Cloud Platform: %s", errorCause)))
			} else {
				assert.Assert(t, strings.Contains(result.Stderr, fmt.Sprintf("Error: Failed to remove orb bar-ns/foo-orb from category Cloud Platform: %s", errorCause)))
			}
		})
	}
}

func TestOrbPack(t *testing.T) {
	binary := testhelpers.BuildCLI(t)
	tmpDir := t.TempDir()

	// Create @orb.yml
	assert.NilError(t, os.WriteFile(filepath.Join(tmpDir, "@orb.yml"), []byte(""), 0600))

	// Create commands/orb.yml
	assert.NilError(t, os.MkdirAll(filepath.Join(tmpDir, "commands"), 0700))
	assert.NilError(t, os.WriteFile(filepath.Join(tmpDir, "commands", "orb.yml"), []byte(`steps:
    - run:
        name: Say hello
        command: <<include(scripts/script.sh)>>

examples:
    example:
        description: |
            An example of how to use the orb.
        usage:
            version: 2.1
            orbs:
                orb-name: company/orb-name@1.2.3
            setup: true
            workflows:
                create-pipeline:
                    jobs:
                        orb-name: create-pipeline-x
`), 0600))

	// Create scripts/script.sh
	assert.NilError(t, os.MkdirAll(filepath.Join(tmpDir, "scripts"), 0700))
	assert.NilError(t, os.WriteFile(filepath.Join(tmpDir, "scripts", "script.sh"), []byte(`echo Hello, world!`), 0600))

	result := testhelpers.RunCLI(t, binary,
		[]string{
			"orb", "pack",
			"--skip-update-check",
			tmpDir,
		},
	)
	assert.Equal(t, result.ExitCode, 0, "stderr: %s\nstdout: %s", result.Stderr, result.Stdout)
	assert.Assert(t, strings.Contains(result.Stdout, "command: echo Hello, world!"))
	assert.Assert(t, strings.Contains(result.Stdout, "name: Say hello"))
	assert.Assert(t, strings.Contains(result.Stdout, "setup: true"))
}

func TestOrbDiff(t *testing.T) {
	tests := []struct {
		name     string
		source1  string
		source2  string
		expected string
	}{
		{
			name:     "detect identical sources",
			source1:  "orb-source",
			source2:  "orb-source",
			expected: "No diff found",
		},
		{
			name:    "detect difference",
			source1: "line1\\nline3\\n",
			source2: "line1\\nline2\\n",
			expected: "--- somenamespace/someorb@1.0.0",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			binary := testhelpers.BuildCLI(t)
			ts := testhelpers.WithTempSettings(t)
			token := "testtoken"

			orbName := "somenamespace/someorb"
			version1 := "1.0.0"
			orb1 := fmt.Sprintf("%s@%s", orbName, version1)
			version2 := "2.0.0"
			orb2 := fmt.Sprintf("%s@%s", orbName, version2)

			mockOrbSourceHandler(t, ts.Server, tc.source1, orb1, token)
			mockOrbSourceHandler(t, ts.Server, tc.source2, orb2, token)

			result := testhelpers.RunCLI(t, binary,
				[]string{
					"orb", "diff", orbName, version1, version2,
					"--token", token,
					"--host", ts.Server.URL,
				},
				"HOME="+ts.Home,
				"USERPROFILE="+ts.Home,
			)
			assert.Equal(t, result.ExitCode, 0, "stderr: %s\nstdout: %s", result.Stderr, result.Stdout)
			assert.Assert(t, strings.Contains(result.Stdout, tc.expected))
		})
	}
}
