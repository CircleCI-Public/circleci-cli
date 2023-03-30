package graphql

import (
	"io"
	"net/http"
	"net/http/httptest"
	"regexp"
	"testing"
)

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

		b, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf(err.Error())
		}

		if string(b) != `{"query":"query {}","variables":null}`+"\n" {
			t.Errorf("expected %s", string(b))
		}

		_, err = io.WriteString(w, `{
			"data": {
				"something": "yes"
			}
		}`)
		if err != nil {
			t.Errorf(err.Error())
		}
	}))
	defer srv.Close()

	client := NewClient(http.DefaultClient, srv.URL, "/", "token", false)

	var resp struct {
		Something string
	}
	err := client.Run(&Request{Query: "query {}"}, &resp)
	if err != nil {
		t.Errorf(err.Error())
	}

	if calls != 1 {
		t.Errorf("expected %d", calls)
	}

	if resp.Something != "yes" {
		t.Errorf("expected %+v", resp)
	}
}

func TestQueryJSON(t *testing.T) {
	var calls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		b, err := io.ReadAll(r.Body)
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

	client := NewClient(http.DefaultClient, srv.URL, "/", "token", false)

	req := NewRequest("query {}")
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
	err := client.Run(req, &resp)
	if err != nil {
		t.Errorf(err.Error())
	}

	if calls != 1 {
		t.Errorf("expected %d", calls)
	}

	if resp.Value != "some data" {
		t.Errorf("expected %+v", resp)
	}
}

func TestDoJSONErr(t *testing.T) {
	var calls int
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, req *http.Request) {
		calls++

		body, err := io.ReadAll(req.Body)
		if err != nil {
			t.Errorf(err.Error())
		}

		if string(body) != `{"query":"query {}","variables":null}`+"\n" {
			t.Errorf("expected %s", body)
		}

		_, err = io.WriteString(writer,
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
		if err != nil {
			t.Errorf(err.Error())
		}
	}))

	defer server.Close()

	client := NewClient(http.DefaultClient, server.URL, "/", "token", false)

	var responseData map[string]interface{}

	err := client.Run(&Request{Query: "query {}"}, &responseData)
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

	client := NewClient(http.DefaultClient, srv.URL, "/", "token", false)

	req := NewRequest("query {}")
	req.Header.Set("X-Custom-Header", "123")

	var resp struct {
		Value string
	}
	err := client.Run(req, &resp)
	if err != nil {
		t.Errorf(err.Error())
	}

	if calls != 1 {
		t.Errorf("expected %d", calls)
	}

	if resp.Value != "some data" {
		t.Errorf("expected %+v", resp)
	}
}

func TestStatusCode200(t *testing.T) {
	var calls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++

		w.WriteHeader(http.StatusOK)

		_, err := io.WriteString(w, `{"data":{"value":"some data"}}`)
		if err != nil {
			t.Errorf(err.Error())
		}
	}))
	defer srv.Close()

	client := NewClient(http.DefaultClient, srv.URL, "/", "token", false)

	req := NewRequest("query {}")

	var resp interface{}

	err := client.Run(req, &resp)
	if err != nil {
		t.Errorf(err.Error())
	}

	if calls != 1 {
		t.Errorf("expected %d", calls)
	}
}

func TestStatusCode500(t *testing.T) {
	var calls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++

		w.WriteHeader(500)

		_, err := io.WriteString(w, "some: data")
		if err != nil {
			t.Errorf(err.Error())
		}
	}))
	defer srv.Close()

	client := NewClient(http.DefaultClient, srv.URL, "/", "token", false)

	req := NewRequest("query {}")

	var resp interface{}

	err := client.Run(req, &resp)
	if err == nil {
		t.Error("expected error")
	}

	if err.Error() != "failure calling GraphQL API: 500 Internal Server Error" {
		t.Errorf("expected %s", err.Error())
	}

	if calls != 1 {
		t.Errorf("expected %d", calls)
	}
}

func TestStatusCode413(t *testing.T) {
	var calls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++

		w.WriteHeader(413)

		_, err := io.WriteString(w, "some: data")
		if err != nil {
			t.Errorf(err.Error())
		}
	}))
	defer srv.Close()

	client := NewClient(http.DefaultClient, srv.URL, "/", "token", false)

	req := NewRequest("query {}")

	var resp interface{}

	err := client.Run(req, &resp)
	if err == nil {
		t.Error("expected error")
	}

	if err.Error() != "failure calling GraphQL API: 413 Request Entity Too Large" {
		t.Errorf("expected %s", err.Error())
	}

	if calls != 1 {
		t.Errorf("expected %d", calls)
	}
}
