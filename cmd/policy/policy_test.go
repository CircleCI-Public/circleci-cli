package policy

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"gotest.tools/v3/assert"
	"gotest.tools/v3/assert/cmp"

	"github.com/CircleCI-Public/circleci-cli/settings"
)

func Test_ListPolicies(t *testing.T) {
	t.Run("without owner-id", func(t *testing.T) {
		config := &settings.Config{Token: "testtoken", HTTPClient: &http.Client{}}
		cmd := NewCommand(config, nil)
		stdout := new(bytes.Buffer)
		stderr := new(bytes.Buffer)
		cmd.SetOut(stdout)
		cmd.SetErr(stderr)
		defer stdout.Reset()
		defer stderr.Reset()

		cmd.SetArgs([]string{
			"list",
		})

		err := cmd.Execute()
		assert.Error(t, err, "required flag(s) \"owner-id\" not set")
		assert.Assert(t, cmp.Contains(stdout.String(), "required flag(s) \"owner-id\" not set"))
	})

	t.Run("invalid active filter value", func(t *testing.T) {
		config := &settings.Config{Token: "testtoken", HTTPClient: &http.Client{}}
		cmd := NewCommand(config, nil)
		stdout := new(bytes.Buffer)
		stderr := new(bytes.Buffer)
		cmd.SetOut(stdout)
		cmd.SetErr(stderr)
		defer stdout.Reset()
		defer stderr.Reset()

		cmd.SetArgs([]string{
			"list",
			"--owner-id", "ownerID",
			"--active=badValue",
		})

		err := cmd.Execute()
		assert.Error(t, err, "invalid argument \"badValue\" for \"--active\" flag: strconv.ParseBool: parsing \"badValue\": invalid syntax")
		assert.Assert(t, cmp.Contains(stdout.String(), "invalid argument \"badValue\" for \"--active\" flag: strconv.ParseBool: parsing \"badValue\": invalid syntax"))
	})

	t.Run("gets forbidden error", func(t *testing.T) {
		expectedResponse := `{"error": "Forbidden"}`
		svr := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusForbidden)
			w.Write([]byte(expectedResponse))
		}))
		defer svr.Close()

		config := &settings.Config{Token: "testtoken", HTTPClient: &http.Client{}}
		cmd := NewCommand(config, nil)
		stdout := new(bytes.Buffer)
		stderr := new(bytes.Buffer)
		cmd.SetOut(stdout)
		cmd.SetErr(stderr)
		defer stdout.Reset()
		defer stderr.Reset()

		cmd.SetArgs([]string{
			"list",
			"--owner-id", "ownerID",
			"--policy-base-url", svr.URL,
		})

		err := cmd.Execute()
		assert.Error(t, err, "failed to list policies: unexpected status-code: 403 - Forbidden")
		assert.Assert(t, cmp.Contains(stdout.String(), "failed to list policies: unexpected status-code: 403 - Forbidden"))
	})

	t.Run("successfully gets list of policies", func(t *testing.T) {
		expectedResponse := `[
			{
				"id": "60b7e1a5-c1d7-4422-b813-7a12d353d7c6",
				"name": "policy_1",
				"owner_id": "462d67f8-b232-4da4-a7de-0c86dd667d3f",
				"context": "config",
				"active": false,
				"created_at": "2022-05-31T14:15:10.86097Z",
				"modified_at": null
			},
			{
				"id": "a917a0ab-ceb6-482d-9a4e-f2f6b8bdfdcd",
				"name": "policy_2",
				"owner_id": "462d67f8-b232-4da4-a7de-0c86dd667d3f",
				"context": "config",
				"active": true,
				"created_at": "2022-05-31T14:15:23.582383Z",
				"modified_at": "2022-05-31T14:15:46.72321Z"
			}
		]`
		svr := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte(expectedResponse))
		}))
		defer svr.Close()

		config := &settings.Config{Token: "testtoken", HTTPClient: &http.Client{}}
		cmd := NewCommand(config, nil)
		stdout := new(bytes.Buffer)
		stderr := new(bytes.Buffer)
		cmd.SetOut(stdout)
		cmd.SetErr(stderr)
		defer stdout.Reset()
		defer stderr.Reset()

		cmd.SetArgs([]string{
			"list",
			"--owner-id", "ownerID",
			"--policy-base-url", svr.URL,
		})

		err := cmd.Execute()
		assert.NilError(t, err)
		assert.Equal(t, stdout.String(), expectedResponse)
	})
}
