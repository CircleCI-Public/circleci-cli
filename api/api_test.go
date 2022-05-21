package api

import (
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/CircleCI-Public/circleci-cli/mock"
	"github.com/CircleCI-Public/circleci-cli/settings"
	"github.com/pkg/errors"
	"gotest.tools/v3/assert"
	"gotest.tools/v3/assert/cmp"
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

func TestFollowProject(t *testing.T) {
	table := []struct {
		label              string
		transportFn        func(r *http.Request) (*http.Response, error)
		expFollowedProject FollowedProject
		expErr             string
	}{
		{
			label: "fails when http client returns an error",
			transportFn: func(r *http.Request) (*http.Response, error) {
				return nil, errors.New("test error")
			},
			expErr: "test error",
		},
		{
			label: "fails when json decoding fails",
			transportFn: func(r *http.Request) (*http.Response, error) {
				return mock.NewHTTPResponse(200, "{/"), nil
			},
			expErr: "invalid character '/'",
		},
		{
			label: "returns a followed project successfully",
			transportFn: func(r *http.Request) (*http.Response, error) {
				if r.URL.String() != "https://circleci.com/api/v1.1/project/github/test-user/test-project/follow" {
					panic(fmt.Sprintf("unexpected url: %s", r.URL.String()))
				}
				return mock.NewHTTPResponse(200, `{"message": "test-message", "followed": true}`), nil
			},
			expFollowedProject: FollowedProject{
				Message:  "test-message",
				Followed: true,
			},
		},
	}

	for _, ts := range table {
		t.Run(ts.label, func(t *testing.T) {
			httpClient := mock.NewHTTPClient(ts.transportFn)
			config := settings.Config{
				Host:       "https://circleci.com",
				HTTPClient: httpClient,
			}

			fp, err := FollowProject(config, "github", "test-user", "test-project")
			if err != nil {
				if ts.expErr == "" || !strings.Contains(err.Error(), ts.expErr) {
					t.Fatalf("unexpected error following project: %s", err.Error())
				}
				return
			}

			if ts.expErr != "" {
				t.Fatalf("unexpected nil error - expected: %s", ts.expErr)
			}

			assert.Check(t, cmp.DeepEqual(fp, ts.expFollowedProject))
		})
	}
}
