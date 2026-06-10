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

package apiclient_test

import (
	"errors"
	"net/http"
	"testing"

	"gotest.tools/v3/assert"
	"gotest.tools/v3/assert/cmp"

	"github.com/CircleCI-Public/circleci-cli/internal/apiclient"
	"github.com/CircleCI-Public/circleci-cli/internal/httpcl"
)

func httpErr(body string) error {
	return &httpcl.HTTPError{
		Method:     http.MethodGet,
		Route:      "/api/v3/things",
		StatusCode: http.StatusBadRequest,
		Body:       []byte(body),
	}
}

func TestParseError(t *testing.T) {
	t.Run("full envelope", func(t *testing.T) {
		e, ok := apiclient.ParseError(httpErr(`{"error": {
			"id": "0196d5c2-1111-2222-3333-444455556666",
			"title": "Missing Required Filter",
			"detail": "Query parameter 'filter[workflow_id]' is required.",
			"source": {"pointer": "/filter/workflow_id", "offset": 12, "error": "missing"}
		}}`))
		assert.Assert(t, ok)
		assert.Check(t, cmp.Equal(e.ID, "0196d5c2-1111-2222-3333-444455556666"))
		assert.Check(t, cmp.Equal(e.Title, "Missing Required Filter"))
		assert.Check(t, cmp.Equal(e.Detail, "Query parameter 'filter[workflow_id]' is required."))
		assert.Check(t, cmp.Equal(e.Source.Pointer, "/filter/workflow_id"))
		assert.Check(t, cmp.Equal(e.Source.Offset, 12))
		assert.Check(t, cmp.Equal(e.Source.Error, "missing"))
	})

	t.Run("not an HTTPError", func(t *testing.T) {
		_, ok := apiclient.ParseError(errors.New("dial tcp: connection refused"))
		assert.Check(t, !ok)
	})

	t.Run("non-JSON body", func(t *testing.T) {
		_, ok := apiclient.ParseError(httpErr("<html>502 Bad Gateway</html>"))
		assert.Check(t, !ok)
	})

	t.Run("JSON without envelope", func(t *testing.T) {
		_, ok := apiclient.ParseError(httpErr(`{"message": "not found"}`))
		assert.Check(t, !ok)
	})

	t.Run("empty envelope", func(t *testing.T) {
		_, ok := apiclient.ParseError(httpErr(`{"error": {}}`))
		assert.Check(t, !ok)
	})

	t.Run("string error field", func(t *testing.T) {
		_, ok := apiclient.ParseError(httpErr(`{"error": "boom"}`))
		assert.Check(t, !ok)
	})
}

func TestError_Message(t *testing.T) {
	t.Run("title and detail", func(t *testing.T) {
		e := &apiclient.Error{Title: "Bad Request", Detail: "name is required"}
		assert.Check(t, cmp.Equal(e.Message(), "Bad Request: name is required"))
	})

	t.Run("detail only", func(t *testing.T) {
		e := &apiclient.Error{Detail: "name is required"}
		assert.Check(t, cmp.Equal(e.Message(), "name is required"))
	})

	t.Run("everything", func(t *testing.T) {
		e := &apiclient.Error{
			ID:     "abc-123",
			Title:  "Invalid Config",
			Detail: "field is not a string",
			Source: apiclient.ErrorSource{Pointer: "/spec/name", Offset: 42, Error: "expected string, got int"},
		}
		assert.Check(t, cmp.Equal(e.Message(),
			"Invalid Config: field is not a string\n"+
				"  at /spec/name (offset 42)\n"+
				"  expected string, got int\n"+
				"error id: abc-123"))
	})
}
