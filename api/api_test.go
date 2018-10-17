package api

import (
	"testing"
)

func TestOrbVersionRef(t *testing.T) {
	var (
		orbRef   string
		expected string
	)

	orbRef = orbVersionRef("foo/bar@baz")

	expected = "foo/bar@baz"
	if orbRef != expected {
		t.Errorf("Expected %s, got %s", expected, orbRef)
	}

	orbRef = orbVersionRef("omg/bbq")
	expected = "omg/bbq@volatile"
	if orbRef != expected {
		t.Errorf("Expected %s, got %s", expected, orbRef)
	}

	orbRef = orbVersionRef("omg/bbq@too@many@ats")
	expected = "omg/bbq@too@many@ats"
	if orbRef != expected {
		t.Errorf("Expected %s, got %s", expected, orbRef)
	}
}
