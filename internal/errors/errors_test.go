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
	assert.Equal(t, e.Code, "auth.missing")
	assert.Equal(t, e.Title, "Auth required")
	assert.Equal(t, e.Message, "No token found")
	assert.Equal(t, e.ExitCode, ExitGeneralError)
	assert.Check(t, is.Nil(e.Suggestions))
	assert.Equal(t, e.Ref, "")
}

func TestBuilderImmutability(t *testing.T) {
	base := New("x", "X", "x message")

	withSuggestions := base.WithSuggestions("do this")
	withRef := base.WithRef("https://example.com")
	withCode := base.WithExitCode(ExitAuthError)

	// base is unchanged
	assert.Check(t, is.Nil(base.Suggestions))
	assert.Equal(t, base.Ref, "")
	assert.Equal(t, base.ExitCode, ExitGeneralError)

	// each derived copy has only its own change
	assert.Equal(t, len(withSuggestions.Suggestions), 1)
	assert.Equal(t, withSuggestions.Ref, "")
	assert.Equal(t, withSuggestions.ExitCode, ExitGeneralError)

	assert.Equal(t, withRef.Ref, "https://example.com")
	assert.Check(t, is.Nil(withRef.Suggestions))

	assert.Equal(t, withCode.ExitCode, ExitAuthError)
	assert.Equal(t, withCode.Ref, "")
}

func TestWithSuggestionsDoesNotShareSlice(t *testing.T) {
	base := New("x", "X", "msg").WithSuggestions("first")
	second := base.WithSuggestions("second")

	// base still has only one suggestion
	assert.Equal(t, len(base.Suggestions), 1)
	assert.Equal(t, len(second.Suggestions), 2)
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
		assert.NilError(t, json.Unmarshal([]byte(e.FormatJSON()), &v))
		assert.Equal(t, v["error"], true)
	})

	t.Run("includes code and message", func(t *testing.T) {
		e := New("auth.missing", "T", "no token")
		var v map[string]any
		assert.NilError(t, json.Unmarshal([]byte(e.FormatJSON()), &v))
		assert.Equal(t, v["code"], "auth.missing")
		assert.Equal(t, v["message"], "no token")
	})

	t.Run("omits suggestions when empty", func(t *testing.T) {
		e := New("c", "T", "msg")
		var v map[string]any
		assert.NilError(t, json.Unmarshal([]byte(e.FormatJSON()), &v))
		_, hasSuggestions := v["suggestions"]
		assert.Check(t, !hasSuggestions)
	})

	t.Run("includes suggestions when present", func(t *testing.T) {
		e := New("c", "T", "msg").WithSuggestions("do this")
		var v map[string]any
		assert.NilError(t, json.Unmarshal([]byte(e.FormatJSON()), &v))
		suggestions, _ := v["suggestions"].([]any)
		assert.Equal(t, len(suggestions), 1)
		assert.Equal(t, suggestions[0], "do this")
	})

	t.Run("omits ref when empty", func(t *testing.T) {
		e := New("c", "T", "msg")
		var v map[string]any
		assert.NilError(t, json.Unmarshal([]byte(e.FormatJSON()), &v))
		_, hasRef := v["ref"]
		assert.Check(t, !hasRef)
	})

	t.Run("includes ref when set", func(t *testing.T) {
		e := New("c", "T", "msg").WithRef("https://docs.example.com")
		var v map[string]any
		assert.NilError(t, json.Unmarshal([]byte(e.FormatJSON()), &v))
		assert.Equal(t, v["ref"], "https://docs.example.com")
	})

	t.Run("includes exit code", func(t *testing.T) {
		e := New("c", "T", "msg").WithExitCode(ExitAuthError)
		var v map[string]any
		assert.NilError(t, json.Unmarshal([]byte(e.FormatJSON()), &v))
		assert.Equal(t, v["exit_code"], float64(ExitAuthError))
	})

	t.Run("output ends with newline", func(t *testing.T) {
		e := New("c", "T", "msg")
		assert.Check(t, strings.HasSuffix(e.FormatJSON(), "\n"))
	})
}

func TestError(t *testing.T) {
	e := New("c", "Auth required", "token missing")
	assert.Equal(t, e.Error(), "Auth required: token missing")
}
