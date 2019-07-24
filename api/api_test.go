package api_test

import (
	"testing"

  . "github.com/CircleCI-Public/circleci-cli/api"
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

func TestErrorString(t *testing.T) {
  var gqlCollection = []GQLResponseError {
    gql {
      Message: "This is a test message",
    },
    gql {
      Message: "This is another test message",
    },
  }

  output := Error(gqlCollection)

  equals(t, "This is a test message\nThis is another test message", output)

}

// assert fails the test if the condition is false.
func assert(tb testing.TB, condition bool, msg string, v ...interface{}) {
	if !condition {
		_, file, line, _ := runtime.Caller(1)
		fmt.Printf("\033[31m%s:%d: "+msg+"\033[39m\n\n", append([]interface{}{filepath.Base(file), line}, v...)...)
		tb.FailNow()
	}
}

// ok fails the test if an err is not nil.
func ok(tb testing.TB, err error) {
	if err != nil {
		_, file, line, _ := runtime.Caller(1)
		fmt.Printf("\033[31m%s:%d: unexpected error: %s\033[39m\n\n", filepath.Base(file), line, err.Error())
		tb.FailNow()
	}
}

// equals fails the test if exp is not equal to act.
func equals(tb testing.TB, exp, act interface{}) {
	if !reflect.DeepEqual(exp, act) {
		_, file, line, _ := runtime.Caller(1)
		fmt.Printf("\033[31m%s:%d:\n\texpected: %#v\n\tactual: %#v\033[39m\n\n", filepath.Base(file), line, exp, act)
		tb.FailNow()
	}
}
