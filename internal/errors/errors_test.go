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

package errors

import (
	"encoding/json"
	"strings"
	"testing"

	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func TestNew(t *testing.T) {
	e := New("auth.missing", "Auth required", "No token found")
	assert.Check(t, is.Equal(e.Code, "auth.missing"))
	assert.Check(t, is.Equal(e.Title, "Auth required"))
	assert.Check(t, is.Equal(e.Message, "No token found"))
	assert.Check(t, is.Equal(e.ExitCode, ExitGeneralError))
	assert.Check(t, is.Nil(e.Suggestions))
	assert.Check(t, is.Equal(e.Ref, ""))
}

func TestBuilderImmutability(t *testing.T) {
	base := New("x", "X", "x message")

	withSuggestions := base.WithSuggestions("do this")
	withRef := base.WithRef("https://example.com")
	withCode := base.WithExitCode(ExitAuthError)

	// base is unchanged
	assert.Check(t, is.Nil(base.Suggestions))
	assert.Check(t, is.Equal(base.Ref, ""))
	assert.Check(t, is.Equal(base.ExitCode, ExitGeneralError))

	// each derived copy has only its own change
	assert.Check(t, is.Len(withSuggestions.Suggestions, 1))
	assert.Check(t, is.Equal(withSuggestions.Ref, ""))
	assert.Check(t, is.Equal(withSuggestions.ExitCode, ExitGeneralError))

	assert.Check(t, is.Equal(withRef.Ref, "https://example.com"))
	assert.Check(t, is.Nil(withRef.Suggestions))

	assert.Check(t, is.Equal(withCode.ExitCode, ExitAuthError))
	assert.Check(t, is.Equal(withCode.Ref, ""))
}

func TestWithSuggestionsDoesNotShareSlice(t *testing.T) {
	base := New("x", "X", "msg").WithSuggestions("first")
	second := base.WithSuggestions("second")

	// base still has only one suggestion
	assert.Check(t, is.Len(base.Suggestions, 1))
	assert.Check(t, is.Len(second.Suggestions, 2))
}

func TestFormat(t *testing.T) {
	t.Run("message only", func(t *testing.T) {
		e := New("c", "Title", "Something went wrong")
		out := e.Format()
		assert.Check(t, strings.Contains(out, "Something went wrong"))
		assert.Check(t, !strings.Contains(out, "Suggestions"))
		assert.Check(t, !strings.Contains(out, "Reference"))
	})

	t.Run("with suggestions", func(t *testing.T) {
		e := New("c", "T", "msg").WithSuggestions("Try this", "Or that")
		out := e.Format()
		assert.Check(t, strings.Contains(out, "Suggestions:"))
		assert.Check(t, strings.Contains(out, "Try this"))
		assert.Check(t, strings.Contains(out, "Or that"))
	})

	t.Run("with ref", func(t *testing.T) {
		e := New("c", "T", "msg").WithRef("https://docs.example.com")
		out := e.Format()
		assert.Check(t, strings.Contains(out, "Reference: https://docs.example.com"))
	})

	t.Run("without ref omits reference line", func(t *testing.T) {
		e := New("c", "T", "msg")
		out := e.Format()
		assert.Check(t, !strings.Contains(out, "Reference"))
	})
}

func TestFormatJSON(t *testing.T) {
	t.Run("always sets error true", func(t *testing.T) {
		e := New("c", "T", "msg")
		var v map[string]any
		err := json.Unmarshal([]byte(e.FormatJSON()), &v)
		assert.NilError(t, err)
		assert.Check(t, is.Equal(v["error"], true))
	})

	t.Run("includes code and message", func(t *testing.T) {
		e := New("auth.missing", "T", "no token")
		var v map[string]any
		err := json.Unmarshal([]byte(e.FormatJSON()), &v)
		assert.NilError(t, err)
		assert.Check(t, is.Equal(v["code"], "auth.missing"))
		assert.Check(t, is.Equal(v["message"], "no token"))
	})

	t.Run("omits suggestions when empty", func(t *testing.T) {
		e := New("c", "T", "msg")
		var v map[string]any
		err := json.Unmarshal([]byte(e.FormatJSON()), &v)
		assert.NilError(t, err)
		_, hasSuggestions := v["suggestions"]
		assert.Check(t, !hasSuggestions)
	})

	t.Run("includes suggestions when present", func(t *testing.T) {
		e := New("c", "T", "msg").WithSuggestions("do this")
		var v map[string]any
		err := json.Unmarshal([]byte(e.FormatJSON()), &v)
		assert.NilError(t, err)
		suggestions, _ := v["suggestions"].([]any)
		assert.Check(t, is.Len(suggestions, 1))
		assert.Check(t, is.Equal(suggestions[0], "do this"))
	})

	t.Run("omits ref when empty", func(t *testing.T) {
		e := New("c", "T", "msg")
		var v map[string]any
		err := json.Unmarshal([]byte(e.FormatJSON()), &v)
		assert.NilError(t, err)
		_, hasRef := v["ref"]
		assert.Check(t, !hasRef)
	})

	t.Run("includes ref when set", func(t *testing.T) {
		e := New("c", "T", "msg").WithRef("https://docs.example.com")
		var v map[string]any
		err := json.Unmarshal([]byte(e.FormatJSON()), &v)
		assert.NilError(t, err)
		assert.Check(t, is.Equal(v["ref"], "https://docs.example.com"))
	})

	t.Run("includes exit code", func(t *testing.T) {
		e := New("c", "T", "msg").WithExitCode(ExitAuthError)
		var v map[string]any
		err := json.Unmarshal([]byte(e.FormatJSON()), &v)
		assert.NilError(t, err)
		assert.Check(t, is.Equal(v["exit_code"], float64(ExitAuthError)))
	})

	t.Run("output ends with newline", func(t *testing.T) {
		e := New("c", "T", "msg")
		formatted := e.FormatJSON()
		assert.Check(t, strings.HasSuffix(formatted, "\n"))
	})
}

func TestError(t *testing.T) {
	e := New("c", "Auth required", "token missing")
	errMsg := e.Error()
	assert.Check(t, is.Equal(errMsg, "Auth required: token missing"))
}
