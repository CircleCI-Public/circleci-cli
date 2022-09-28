package config

import (
	"encoding/json"
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

var options = &Options{OwnerId: "123"}

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

		var decodedBody ConfigCompileRequest
		_ = json.Unmarshal(b, &decodedBody)

		if decodedBody.ConfigYml != source_config {
			t.Errorf("expected %s", source_config)
		}

		result, err := json.Marshal(&Response{Valid: true, Source_Yaml: source_config, Output_yaml: compiled_config, Errors: nil})

		if err != nil {
			t.Errorf(err.Error())
		}

		_, err = io.WriteString(w, string(result))
		if err != nil {
			t.Errorf(err.Error())
		}
	}))
	defer srv.Close()

	client := NewClient(http.DefaultClient, srv.URL, "/", "token", false)

	var resp Response
	err := client.Run(&Request{ConfigYml: source_config, Options: *options}, &resp)

	if err != nil {
		t.Errorf(err.Error())
	}

	if calls != 1 {
		t.Errorf("expected %d", calls)
	}

	if resp.Output_yaml == "" {
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

		var decodedBody ConfigCompileRequest
		_ = json.Unmarshal(body, &decodedBody)

		if decodedBody.ConfigYml != source_config {
			t.Errorf("expected %s", body)
		}

		result, err := json.Marshal(&Response{Valid: false, Source_Yaml: source_config, Output_yaml: "", Errors: []ConfigError{
			{Message: "Something went wrong"},
		}})

		if err != nil {
			t.Errorf(err.Error())
		}

		_, err = io.WriteString(writer, string(result))
		if err != nil {
			t.Errorf(err.Error())
		}
	}))

	defer server.Close()

	client := NewClient(http.DefaultClient, server.URL, "/", "token", false)

	var resp Response

	err := client.Run(&Request{ConfigYml: source_config, Options: *options}, &resp)
	if err != nil {
		t.Errorf("expected empty compiled config string")
	}
}

func TestStatusCode200(t *testing.T) {
	var calls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++

		result, err := json.Marshal(&Response{Valid: false, Source_Yaml: source_config, Output_yaml: "", Errors: nil})

		if err != nil {
			t.Errorf(err.Error())
		}

		w.WriteHeader(http.StatusOK)

		_, err = io.WriteString(w, string(result))
		if err != nil {
			t.Errorf(err.Error())
		}
	}))
	defer srv.Close()

	client := NewClient(http.DefaultClient, srv.URL, "/", "token", false)

	var resp Response

	err := client.Run(&Request{ConfigYml: source_config, Options: *options}, &resp)
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

	var resp Response

	err := client.Run(&Request{ConfigYml: source_config, Options: *options}, &resp)
	if err == nil {
		t.Error("expected error")
	}

	if err.Error() != "failure calling compile config API: 500 Internal Server Error" {
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

	var resp Response

	err := client.Run(&Request{ConfigYml: source_config, Options: *options}, &resp)
	if err == nil {
		t.Error("expected error")
	}

	if err.Error() != "failure calling compile config API: 413 Request Entity Too Large" {
		t.Errorf("expected %s", err.Error())
	}

	if calls != 1 {
		t.Errorf("expected %d", calls)
	}
}
