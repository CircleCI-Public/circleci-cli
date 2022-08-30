package config

import (
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"regexp"
	"testing"
)

var source_config = `version: "2.1"
workflows:
  bar:
	jobs:
	  - foo

jobs:
  foo:
	machine: true
	steps:
	  - run: echo Hello World`

var compiled_config = `version: 2
	  workflows:
		version: 2
		bar:
		  jobs:
			- foo
	  
	  jobs:
		foo:
		  machine: true
		  steps:
		  - run:
			  command: echo Hello World`

var options = &Options{owner_id: "123"}

func TestServerAddress(t *testing.T) {
	var (
		addr     string
		expected string
		err      error
	)

	addr, _ = getServerAddress("https://example.api.circleci.com", "")

	expected = "https://example.api.circleci.com"
	if addr != expected {
		t.Errorf("Expected %s, got %s", expected, addr)
	}

	addr, _ = getServerAddress("https://example.api.circleci.com", "compile-config-with-defaults")
	expected = "https://example.api.circleci.com/compile-config-with-defaults"
	if addr != expected {
		t.Errorf("Expected %s, got %s", expected, addr)
	}

	addr, _ = getServerAddress("https://example.api.circleci.com/compile-config-with-defaults", "https://api.circleci.com/compile-config-with-defaults")
	expected = "https://api.circleci.com/compile-config-with-defaults"
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

		if string(b) != source_config+"\n" {
			t.Errorf("expected %s", string(b))
		}

		_, err = io.WriteString(w, compiled_config)
		if err != nil {
			t.Errorf(err.Error())
		}
	}))
	defer srv.Close()

	client := NewClient(http.DefaultClient, srv.URL, "/", "token", false)

	compiled, err := client.CompileConfigWithDefaults(source_config, *options)
	if err != nil {
		t.Errorf(err.Error())
	}

	if calls != 1 {
		t.Errorf("expected %d", calls)
	}

	if compiled == "" {
		t.Errorf("expected %+v", compiled_config)
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

		if string(body) != source_config+"\n" {
			t.Errorf("expected %s", body)
		}

		_, err = io.WriteString(writer, compiled_config)
		if err != nil {
			t.Errorf(err.Error())
		}
	}))

	defer server.Close()

	client := NewClient(http.DefaultClient, server.URL, "/", "token", false)

	_, err := client.CompileConfigWithDefaults(source_config, *options)
	if err != nil {
		t.Errorf("expected empty compiled config string")
	}
}

func TestHeader(t *testing.T) {
	var calls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.Header.Get("X-Custom-Header") != "123" {
			t.Errorf("expected %s", r.Header.Get("X-Custom-Header"))
		}

		_, err := io.WriteString(w, compiled_config)
		if err != nil {
			t.Errorf(err.Error())
		}
	}))
	defer srv.Close()

	client := NewClient(http.DefaultClient, srv.URL, "/", "token", false)

	compiled, err := client.CompileConfigWithDefaults(source_config, *options)
	if err != nil {
		t.Errorf(err.Error())
	}

	if calls != 1 {
		t.Errorf("expected %d", calls)
	}

	if compiled == "" {
		t.Errorf("expected %+v", compiled_config)
	}
}

func TestStatusCode200(t *testing.T) {
	var calls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++

		w.WriteHeader(http.StatusOK)

		_, err := io.WriteString(w, compiled_config)
		if err != nil {
			t.Errorf(err.Error())
		}
	}))
	defer srv.Close()

	client := NewClient(http.DefaultClient, srv.URL, "/", "token", false)

	_, err := client.CompileConfigWithDefaults(source_config, *options)
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

		_, err := io.WriteString(w, "")
		if err != nil {
			t.Errorf(err.Error())
		}
	}))
	defer srv.Close()

	client := NewClient(http.DefaultClient, srv.URL, "/", "token", false)

	_, err := client.CompileConfigWithDefaults(source_config, *options)
	if err == nil {
		t.Error("expected error")
	}

	if err.Error() != "failure calling config API: 500 Internal Server Error" {
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

		_, err := io.WriteString(w, "")
		if err != nil {
			t.Errorf(err.Error())
		}
	}))
	defer srv.Close()

	client := NewClient(http.DefaultClient, srv.URL, "/", "token", false)

	_, err := client.CompileConfigWithDefaults(source_config, *options)
	if err == nil {
		t.Error("expected error")
	}

	if err.Error() != "failure calling config API: 413 Request Entity Too Large" {
		t.Errorf("expected %s", err.Error())
	}

	if calls != 1 {
		t.Errorf("expected %d", calls)
	}
}
