package client

import (
	"context"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"regexp"
	"testing"
	"time"

	"github.com/CircleCI-Public/circleci-cli/logger"
)

var log = logger.NewLogger(false)

func TestServerAddress(t *testing.T) {
	var (
		addr     string
		expected string
		err      error
	)

	addr, _ = getServerAddress("https://example.com/graphql", "")

	expected = "https://example.com/graphql"
	if addr != expected {
		t.Errorf("Expected %s, got %s", expected, addr)
	}

	addr, _ = getServerAddress("https://example.com", "graphql-unstable")
	expected = "https://example.com/graphql-unstable"
	if addr != expected {
		t.Errorf("Expected %s, got %s", expected, addr)
	}

	addr, _ = getServerAddress("https://example.com/graphql-unstable", "https://circleci.com/graphql")
	expected = "https://circleci.com/graphql"
	if addr != expected {
		t.Errorf("Expected %s, got %s", expected, addr)
	}

	_, err = getServerAddress("", "")
	expected = "Host () must be absolute URL, including scheme"
	if err.Error() != expected {
		t.Errorf("Expected error without absolute URL")
	}

	_, err = getServerAddress("", ":foo")
	matched, _ := regexp.MatchString("Parsing endpoint", err.Error())
	if !matched {
		t.Errorf("Expected parsing endpoint error")
	}
}

func TestDoJSON(t *testing.T) {
	var calls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++

		b, err := ioutil.ReadAll(r.Body)
		if err != nil {
			t.Errorf(err.Error())
		}

		if string(b) != `{"query":"query {}","variables":null}`+"\n" {
			t.Errorf("expected %s", string(b))
		}

		io.WriteString(w, `{
			"data": {
				"something": "yes"
			}
		}`)
	}))
	defer srv.Close()

	ctx := context.Background()
	client := NewClient(srv.URL, "/", "token")

	ctx, cancel := context.WithTimeout(ctx, 1*time.Second)
	defer cancel()
	var resp struct {
		Something string
	}
	err := client.Run(ctx, log, &Request{Query: "query {}"}, &resp)
	if err != nil {
		t.Errorf(err.Error())
	}

	if calls != 1 {
		t.Errorf("expected %s", string(calls))
	}

	if resp.Something != "yes" {
		t.Errorf("expected %+v", resp)
	}
}

func TestQueryJSON(t *testing.T) {
	var calls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		b, err := ioutil.ReadAll(r.Body)
		if err != nil {
			t.Errorf(err.Error())
		}
		if string(b) != `{"query":"query {}","variables":{"username":"matryer"}}`+"\n" {
			t.Errorf("expected %s", b)
		}
		_, err = io.WriteString(w, `{"data":{"value":"some data"}}`)
		if err != nil {
			t.Errorf(err.Error())
		}
	}))
	defer srv.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	client := NewClient(srv.URL, "/", "token")

	req := NewUnauthorizedRequest("query {}")
	req.Var("username", "matryer")

	// check variables
	if req == nil {
		t.Errorf("expected %s", req)
	}

	if req.Variables["username"] != "matryer" {
		t.Errorf("expcted %s", req.Variables["username"])
	}

	var resp struct {
		Value string
	}
	err := client.Run(ctx, log, req, &resp)
	if err != nil {
		t.Errorf(err.Error())
	}

	if calls != 1 {
		t.Errorf("expected %s", string(calls))
	}

	if resp.Value != "some data" {
		t.Errorf("expected %+v", resp)
	}
}

func TestDoJSONErr(t *testing.T) {
	var calls int
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, req *http.Request) {
		calls++

		body, err := ioutil.ReadAll(req.Body)
		if err != nil {
			t.Errorf(err.Error())
		}

		if string(body) != `{"query":"query {}","variables":null}`+"\n" {
			t.Errorf("expected %s", body)
		}

		io.WriteString(writer,
			`{
				"errors": [
					{
						"message": "Something went wrong"
					},
					{
						"message": "Something else went wrong"
					}
				]
			}`)
	}))

	defer server.Close()

	ctx := context.Background()
	client := NewClient(server.URL, "/", "token")

	ctx, cancel := context.WithTimeout(ctx, 1*time.Second)
	defer cancel()

	var responseData map[string]interface{}

	err := client.Run(ctx, log, &Request{Query: "query {}"}, &responseData)
	if err.Error() != "Something went wrong\nSomething else went wrong" {
		t.Errorf(err.Error())
	}
}

func TestHeader(t *testing.T) {
	var calls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.Header.Get("X-Custom-Header") != "123" {
			t.Errorf("expected %s", r.Header.Get("X-Custom-Header"))
		}

		_, err := io.WriteString(w, `{"data":{"value":"some data"}}`)
		if err != nil {
			t.Errorf(err.Error())
		}
	}))
	defer srv.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	client := NewClient(srv.URL, "/", "token")

	req := NewUnauthorizedRequest("query {}")
	req.Header.Set("X-Custom-Header", "123")

	var resp struct {
		Value string
	}
	err := client.Run(ctx, log, req, &resp)
	if err != nil {
		t.Errorf(err.Error())
	}

	if calls != 1 {
		t.Errorf("expected %s", string(calls))
	}

	if resp.Value != "some data" {
		t.Errorf("expected %+v", resp)
	}
}
