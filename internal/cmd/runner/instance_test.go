package runner

import (
	"testing"
	"time"

	"gotest.tools/v3/assert"
)

func TestInstanceStatus(t *testing.T) {
	format := func(d time.Duration) string {
		return time.Now().Add(d).UTC().Format(time.RFC3339Nano)
	}

	t.Run("online when connected under 2 minutes ago", func(t *testing.T) {
		assert.Equal(t, instanceStatus(format(-1*time.Minute)), "online")
	})

	t.Run("online at zero age", func(t *testing.T) {
		assert.Equal(t, instanceStatus(format(0)), "online")
	})

	t.Run("idle when connected 2 to 30 minutes ago", func(t *testing.T) {
		assert.Equal(t, instanceStatus(format(-10*time.Minute)), "idle")
	})

	t.Run("offline when connected over 30 minutes ago", func(t *testing.T) {
		assert.Equal(t, instanceStatus(format(-45*time.Minute)), "offline")
	})

	t.Run("unknown on empty string", func(t *testing.T) {
		assert.Equal(t, instanceStatus(""), "unknown")
	})

	t.Run("unknown on unparseable string", func(t *testing.T) {
		assert.Equal(t, instanceStatus("not-a-timestamp"), "unknown")
	})

	t.Run("accepts RFC3339 without nanoseconds", func(t *testing.T) {
		ts := time.Now().Add(-5 * time.Minute).UTC().Format(time.RFC3339)
		assert.Equal(t, instanceStatus(ts), "idle")
	})

	t.Run("accepts legacy Z-suffix format without nanoseconds", func(t *testing.T) {
		ts := time.Now().Add(-5 * time.Minute).UTC().Format("2006-01-02T15:04:05Z")
		assert.Equal(t, instanceStatus(ts), "idle")
	})

	t.Run("accepts legacy Z-suffix format with nanoseconds", func(t *testing.T) {
		ts := time.Now().Add(-5 * time.Minute).UTC().Format("2006-01-02T15:04:05.999999999Z")
		assert.Equal(t, instanceStatus(ts), "idle")
	})

	t.Run("boundary: exactly 2 minutes is idle not online", func(t *testing.T) {
		ts := time.Now().Add(-2*time.Minute - time.Second).UTC().Format(time.RFC3339Nano)
		assert.Equal(t, instanceStatus(ts), "idle")
	})

	t.Run("boundary: exactly 30 minutes is offline not idle", func(t *testing.T) {
		ts := time.Now().Add(-30*time.Minute - time.Second).UTC().Format(time.RFC3339Nano)
		assert.Equal(t, instanceStatus(ts), "offline")
	})

	t.Run("future timestamp is online", func(t *testing.T) {
		ts := time.Now().Add(1 * time.Minute).UTC().Format(time.RFC3339Nano)
		assert.Equal(t, instanceStatus(ts), "online") // age is negative, < 2min
	})
}
