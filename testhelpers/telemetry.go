// telemetry.go provides a mock telemetry client for tests, replacing the
// gomega-based clitest telemetry helpers.
//
// Migration path: replace clitest.CompareTelemetryEvent with direct
// assertions on MockTelemetry.Events() using gotest.tools/v3/assert.
package testhelpers

import (
	"encoding/json"
	"os"
	"sync"
	"testing"

	"github.com/CircleCI-Public/circleci-cli/telemetry"
)

// MockTelemetry implements telemetry.Client for use in tests.
// It records all tracked events for later assertion.
type MockTelemetry struct {
	mu     sync.Mutex
	events []telemetry.Event
}

// NewMockTelemetry returns a ready-to-use MockTelemetry.
func NewMockTelemetry() *MockTelemetry {
	return &MockTelemetry{
		events: make([]telemetry.Event, 0),
	}
}

// Track records a telemetry event.
func (m *MockTelemetry) Track(event telemetry.Event) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.events = append(m.events, event)
	return nil
}

// Enabled returns true so telemetry events are recorded in tests.
func (m *MockTelemetry) Enabled() bool {
	return true
}

// Close is a no-op for the mock.
func (m *MockTelemetry) Close() error {
	return nil
}

// Events returns a copy of all recorded telemetry events.
func (m *MockTelemetry) Events() []telemetry.Event {
	m.mu.Lock()
	defer m.mu.Unlock()
	result := make([]telemetry.Event, len(m.events))
	copy(result, m.events)
	return result
}

// Reset clears all recorded events.
func (m *MockTelemetry) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.events = m.events[:0]
}

// ReadTelemetryEventsFromFile reads and parses telemetry events from the
// MOCK_TELEMETRY destination file. This replaces clitest.ReadTelemetryEvents.
func ReadTelemetryEventsFromFile(t testing.TB, path string) []telemetry.Event {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadTelemetryEventsFromFile: %v", err)
	}
	var result []telemetry.Event
	if err := json.Unmarshal(content, &result); err != nil {
		t.Fatalf("ReadTelemetryEventsFromFile: unmarshalling: %v", err)
	}
	return result
}

// AssertTelemetrySubset checks that every event in expected appears somewhere
// in the events read from the TelemetryDestPath file. This replaces
// clitest.CompareTelemetryEventSubset.
func AssertTelemetrySubset(t testing.TB, ts *TempSettings, expected []telemetry.Event) {
	t.Helper()
	actual := ReadTelemetryEventsFromFile(t, ts.TelemetryDestPath)
	for _, exp := range expected {
		found := false
		for _, act := range actual {
			if telemetryEventsEqual(exp, act) {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("AssertTelemetrySubset: expected event not found:\n  expected: %+v\n  in actual: %+v", exp, actual)
		}
	}
}

func telemetryEventsEqual(a, b telemetry.Event) bool {
	aj, _ := json.Marshal(a)
	bj, _ := json.Marshal(b)
	return string(aj) == string(bj)
}
