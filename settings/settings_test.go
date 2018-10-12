package settings

import (
	"regexp"
	"testing"
)

func TestGraphQLServerAddress(t *testing.T) {
	var (
		addr     string
		expected string
		err      error
	)

	addr, _ = GraphQLServerAddress("", "https://example.com/graphql")

	expected = "https://example.com/graphql"
	if addr != expected {
		t.Errorf("Expected %s, got %s", expected, addr)
	}

	addr, _ = GraphQLServerAddress("graphql-unstable", "https://example.com")
	expected = "https://example.com/graphql-unstable"
	if addr != expected {
		t.Errorf("Expected %s, got %s", expected, addr)
	}

	addr, _ = GraphQLServerAddress("https://circleci.com/graphql", "https://example.com/graphql-unstable")
	expected = "https://circleci.com/graphql"
	if addr != expected {
		t.Errorf("Expected %s, got %s", expected, addr)
	}

	_, err = GraphQLServerAddress("", "")
	expected = "Host () must be absolute URL, including scheme"
	if err.Error() != expected {
		t.Errorf("Expected error without absolute URL")
	}

	_, err = GraphQLServerAddress(":foo", "")
	matched, _ := regexp.MatchString("Parsing endpoint", err.Error())
	if !matched {
		t.Errorf("Expected parsing endpoint error")
	}
}
