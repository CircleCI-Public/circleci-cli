package telemetry

import (
	"testing"

	"github.com/spf13/cobra"
	"gotest.tools/v3/assert"
)

func TestGetCommandInformation(t *testing.T) {
	t.Run("sensitive flags are redacted", func(t *testing.T) {
		cmd := &cobra.Command{Use: "test"}
		cmd.Flags().String("token", "", "a token")
		cmd.Flags().String("verbose", "", "verbosity")

		_ = cmd.Flags().Set("token", "secret123")
		_ = cmd.Flags().Set("verbose", "true")

		info := GetCommandInformation(cmd, false)
		assert.Equal(t, info.Name, "test")
		assert.Equal(t, info.LocalArgs["token"], "[REDACTED]")
		assert.Equal(t, info.LocalArgs["verbose"], "true")
	})

	t.Run("unset flags are not included", func(t *testing.T) {
		cmd := &cobra.Command{Use: "test"}
		cmd.Flags().String("token", "default", "a token")
		cmd.Flags().String("verbose", "false", "verbosity")

		// Don't set any flags
		info := GetCommandInformation(cmd, false)
		assert.Equal(t, len(info.LocalArgs), 0)
	})

	t.Run("all sensitive flags are redacted", func(t *testing.T) {
		cmd := &cobra.Command{Use: "test"}
		cmd.Flags().String("token", "", "")
		cmd.Flags().String("env-value", "", "")
		cmd.Flags().String("mock-telemetry", "", "")

		_ = cmd.Flags().Set("token", "abc")
		_ = cmd.Flags().Set("env-value", "xyz")
		_ = cmd.Flags().Set("mock-telemetry", "path/to/file")

		info := GetCommandInformation(cmd, false)
		assert.Equal(t, info.LocalArgs["token"], "[REDACTED]")
		assert.Equal(t, info.LocalArgs["env-value"], "[REDACTED]")
		assert.Equal(t, info.LocalArgs["mock-telemetry"], "[REDACTED]")
	})

	t.Run("getParent true collects parent flags", func(t *testing.T) {
		parent := &cobra.Command{Use: "parent"}
		parent.Flags().String("token", "", "a token")
		parent.Flags().String("host", "", "api host")

		child := &cobra.Command{Use: "child"}
		child.Flags().String("format", "", "output format")
		parent.AddCommand(child)

		_ = parent.Flags().Set("token", "secret")
		_ = parent.Flags().Set("host", "https://example.com")
		_ = child.Flags().Set("format", "json")

		info := GetCommandInformation(child, true)
		assert.Equal(t, info.Name, "child")
		assert.Equal(t, info.LocalArgs["token"], "[REDACTED]")
		assert.Equal(t, info.LocalArgs["host"], "https://example.com")
		assert.Equal(t, info.LocalArgs["format"], "json")
	})

	t.Run("getParent false does not collect parent flags", func(t *testing.T) {
		parent := &cobra.Command{Use: "parent"}
		parent.Flags().String("token", "", "a token")
		parent.Flags().String("host", "", "api host")

		child := &cobra.Command{Use: "child"}
		child.Flags().String("format", "", "output format")
		parent.AddCommand(child)

		_ = parent.Flags().Set("token", "secret")
		_ = parent.Flags().Set("host", "https://example.com")
		_ = child.Flags().Set("format", "json")

		info := GetCommandInformation(child, false)
		assert.Equal(t, info.Name, "child")
		assert.Equal(t, info.LocalArgs["format"], "json")
		_, hasToken := info.LocalArgs["token"]
		assert.Assert(t, !hasToken)
		_, hasHost := info.LocalArgs["host"]
		assert.Assert(t, !hasHost)
	})
}

func TestUsedFlagValues(t *testing.T) {
	t.Run("sensitive flags are redacted", func(t *testing.T) {
		cmd := &cobra.Command{Use: "test"}
		cmd.Flags().String("token", "", "a token")
		cmd.Flags().String("verbose", "", "verbosity")

		_ = cmd.Flags().Set("token", "secret123")
		_ = cmd.Flags().Set("verbose", "true")

		flags := UsedFlagValues(cmd)
		assert.Equal(t, flags["token"], "[REDACTED]")
		assert.Equal(t, flags["verbose"], "true")
	})

	t.Run("unset flags are not included", func(t *testing.T) {
		cmd := &cobra.Command{Use: "test"}
		cmd.Flags().String("token", "default", "a token")
		cmd.Flags().String("verbose", "false", "verbosity")

		flags := UsedFlagValues(cmd)
		assert.Equal(t, len(flags), 0)
	})

	t.Run("all sensitive flags are redacted", func(t *testing.T) {
		cmd := &cobra.Command{Use: "test"}
		cmd.Flags().String("token", "", "")
		cmd.Flags().String("env-value", "", "")
		cmd.Flags().String("mock-telemetry", "", "")
		cmd.Flags().String("safe-flag", "", "")

		_ = cmd.Flags().Set("token", "abc")
		_ = cmd.Flags().Set("env-value", "xyz")
		_ = cmd.Flags().Set("mock-telemetry", "path/to/file")
		_ = cmd.Flags().Set("safe-flag", "hello")

		flags := UsedFlagValues(cmd)
		assert.Equal(t, flags["token"], "[REDACTED]")
		assert.Equal(t, flags["env-value"], "[REDACTED]")
		assert.Equal(t, flags["mock-telemetry"], "[REDACTED]")
		assert.Equal(t, flags["safe-flag"], "hello")
	})
}
