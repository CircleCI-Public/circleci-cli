package pipeline

import (
	"testing"

	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func TestParseParams(t *testing.T) {
	t.Run("nil on empty input", func(t *testing.T) {
		result, err := parseParams(nil)
		assert.NilError(t, err)
		assert.Check(t, is.Nil(result))
	})

	t.Run("nil on empty slice", func(t *testing.T) {
		result, err := parseParams([]string{})
		assert.NilError(t, err)
		assert.Check(t, is.Nil(result))
	})

	t.Run("string value", func(t *testing.T) {
		result, err := parseParams([]string{"env=staging"})
		assert.NilError(t, err)
		assert.Equal(t, result["env"], "staging")
	})

	t.Run("bool true", func(t *testing.T) {
		result, err := parseParams([]string{"run_e2e=true"})
		assert.NilError(t, err)
		assert.Equal(t, result["run_e2e"], true)
	})

	t.Run("bool false", func(t *testing.T) {
		result, err := parseParams([]string{"skip_tests=false"})
		assert.NilError(t, err)
		assert.Equal(t, result["skip_tests"], false)
	})

	t.Run("bool is case-sensitive", func(t *testing.T) {
		result, err := parseParams([]string{"flag=True"})
		assert.NilError(t, err)
		assert.Equal(t, result["flag"], "True") // not coerced to bool
	})

	t.Run("integer value", func(t *testing.T) {
		result, err := parseParams([]string{"count=42"})
		assert.NilError(t, err)
		assert.Equal(t, result["count"], int64(42))
	})

	t.Run("zero is integer not string", func(t *testing.T) {
		result, err := parseParams([]string{"retries=0"})
		assert.NilError(t, err)
		assert.Equal(t, result["retries"], int64(0))
	})

	t.Run("negative integer", func(t *testing.T) {
		result, err := parseParams([]string{"offset=-5"})
		assert.NilError(t, err)
		assert.Equal(t, result["offset"], int64(-5))
	})

	t.Run("empty value is a string", func(t *testing.T) {
		result, err := parseParams([]string{"key="})
		assert.NilError(t, err)
		assert.Equal(t, result["key"], "")
	})

	t.Run("value containing equals sign", func(t *testing.T) {
		result, err := parseParams([]string{"expr=a=b"})
		assert.NilError(t, err)
		assert.Equal(t, result["expr"], "a=b") // only first = is the delimiter
	})

	t.Run("multiple params", func(t *testing.T) {
		result, err := parseParams([]string{"env=prod", "count=3", "debug=true"})
		assert.NilError(t, err)
		assert.Equal(t, result["env"], "prod")
		assert.Equal(t, result["count"], int64(3))
		assert.Equal(t, result["debug"], true)
	})

	t.Run("error on missing equals", func(t *testing.T) {
		_, err := parseParams([]string{"noequals"})
		assert.ErrorContains(t, err, "noequals")
	})

	t.Run("error on empty key", func(t *testing.T) {
		_, err := parseParams([]string{"=value"})
		assert.ErrorContains(t, err, "=value")
	})
}
