// Copyright (c) 2026 Circle Internet Services, Inc.
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.
//
// SPDX-License-Identifier: MIT

package cmdutil

import (
	"errors"
	"net/http"
	"testing"

	"gotest.tools/v3/assert"
	"gotest.tools/v3/assert/cmp"

	clierrors "github.com/CircleCI-Public/circleci-cli/internal/errors"
	"github.com/CircleCI-Public/circleci-cli/internal/httpcl"
	"github.com/CircleCI-Public/circleci-cli/internal/keyring"
)

func TestKeyringConnectHint(t *testing.T) {
	const want = "To connect the secret manager, run `sudo snap connect circleci:password-manager-service`"

	t.Run("access denied inside a snap suggests connecting the interface", func(t *testing.T) {
		t.Setenv("SNAP_INSTANCE_NAME", "")
		t.Setenv("SNAP_NAME", "circleci")
		assert.Check(t, cmp.Equal(KeyringConnectHint(keyring.ErrAccessDenied), want))
	})

	t.Run("uses the running instance name for a parallel install", func(t *testing.T) {
		t.Setenv("SNAP_INSTANCE_NAME", "circleci_beta")
		t.Setenv("SNAP_NAME", "circleci")
		assert.Check(t, cmp.Contains(KeyringConnectHint(keyring.ErrAccessDenied), "circleci_beta:password-manager-service"))
	})

	t.Run("access denied outside a snap has no actionable hint", func(t *testing.T) {
		t.Setenv("SNAP_INSTANCE_NAME", "")
		t.Setenv("SNAP_NAME", "")
		assert.Check(t, cmp.Equal(KeyringConnectHint(keyring.ErrAccessDenied), ""))
	})

	t.Run("other failures produce no hint even inside a snap", func(t *testing.T) {
		t.Setenv("SNAP_NAME", "circleci")
		assert.Check(t, cmp.Equal(KeyringConnectHint(keyring.ErrUnavailable), ""))
		assert.Check(t, cmp.Equal(KeyringConnectHint(nil), ""))
	})
}

func httpErr(status int, body string) error {
	return &httpcl.HTTPError{
		Method:     http.MethodGet,
		Route:      "/api/v3/things",
		StatusCode: status,
		Body:       []byte(body),
	}
}

func TestAPIErr(t *testing.T) {
	t.Run("v3 envelope", func(t *testing.T) {
		err := APIErr(httpErr(http.StatusBadRequest, `{"error": {
			"id": "abc-123",
			"title": "Missing Required Filter",
			"detail": "Query parameter 'filter[workflow_id]' is required."
		}}`), "x", "thing.not_found", "No thing found for %q")
		assert.Check(t, cmp.Equal(err.Code, "api.error"))
		assert.Check(t, cmp.Equal(err.Title, "Missing Required Filter"))
		assert.Check(t, cmp.Equal(err.Message,
			"API returned 400: Missing Required Filter: Query parameter 'filter[workflow_id]' is required.\nerror id: abc-123"))
		assert.Check(t, cmp.Equal(err.ExitCode, clierrors.ExitAPIError))
	})

	t.Run("v3 envelope on 404 keeps resource message, drops redundant detail", func(t *testing.T) {
		err := APIErr(httpErr(http.StatusNotFound, `{"error": {
			"id": "abc-123",
			"title": "Not Found",
			"detail": "Pipeline run not found."
		}}`), "wf-1", "workflow.not_found", "No workflow found for %q")
		assert.Check(t, cmp.Equal(err.Code, "workflow.not_found"))
		assert.Check(t, cmp.Equal(err.Message,
			"No workflow found for \"wf-1\"\nerror id: abc-123"))
		assert.Check(t, cmp.Equal(err.ExitCode, clierrors.ExitNotFound))
	})

	t.Run("v3 envelope on 404 without id adds nothing", func(t *testing.T) {
		err := APIErr(httpErr(http.StatusNotFound, `{"error": {
			"title": "Not Found",
			"detail": "Pipeline run not found."
		}}`), "wf-1", "workflow.not_found", "No workflow found for %q")
		assert.Check(t, cmp.Equal(err.Message, `No workflow found for "wf-1"`))
	})

	t.Run("non-envelope body falls back to raw", func(t *testing.T) {
		err := APIErr(httpErr(http.StatusInternalServerError, `{"message": "boom"}`), "x", "thing.not_found", "No thing found for %q")
		assert.Check(t, cmp.Equal(err.Code, "api.error"))
		assert.Check(t, cmp.Equal(err.Message, `API returned 500: {"message": "boom"}`))
	})

	t.Run("unauthorized", func(t *testing.T) {
		err := APIErr(httpErr(http.StatusUnauthorized, ""), "x", "thing.not_found", "No thing found for %q")
		assert.Check(t, cmp.Equal(err.Code, "auth.token_invalid"))
		assert.Check(t, cmp.Equal(err.ExitCode, clierrors.ExitAuthError))
	})

	t.Run("non-HTTP error", func(t *testing.T) {
		err := APIErr(errors.New("dial tcp: connection refused"), "x", "thing.not_found", "No thing found for %q")
		assert.Check(t, cmp.Equal(err.Code, "api.error"))
		assert.Check(t, cmp.Equal(err.Message, "dial tcp: connection refused"))
	})
}
