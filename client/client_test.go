package client

import (
	"context"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/CircleCI-Public/circleci-cli/logger"
)

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
	client := NewClient(srv.URL, "token", logger.NewLogger(false))

	ctx, cancel := context.WithTimeout(ctx, 1*time.Second)
	defer cancel()
	var resp struct {
		Data struct {
			Something string
		}
	}
	err := client.Run(ctx, &Request{Query: "query {}"}, &resp)
	if err != nil {
		t.Errorf(err.Error())
	}

	if calls != 1 {
		t.Errorf("expected %s", string(calls))
	}

	if resp.Data.Something != "yes" {
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

	client := NewClient(srv.URL, "token", logger.NewLogger(false))

	req, err := client.NewUnauthorizedRequest("query {}")
	if err != nil {
		t.Errorf(err.Error())
	}
	req.Var("username", "matryer")

	// check variables
	if req == nil {
		t.Errorf("expected %s", req)
	}

	if req.Variables["username"] != "matryer" {
		t.Errorf("expcted %s", req.Variables["username"])
	}

	var resp struct {
		Data struct {
			Value string
		}
	}
	err = client.Run(ctx, req, &resp)
	if err != nil {
		t.Errorf(err.Error())
	}

	if calls != 1 {
		t.Errorf("expected %s", string(calls))
	}

	if resp.Data.Value != "some data" {
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
	client := NewClient(server.URL, "token", logger.NewLogger(false))

	ctx, cancel := context.WithTimeout(ctx, 1*time.Second)
	defer cancel()

	var responseData struct {
		Data   map[string]interface{}
		Errors []struct {
			Message string
		}
	}
	err := client.Run(ctx, &Request{Query: "query {}"}, &responseData)
	if err != nil {
		t.Errorf(err.Error())
	}

	if len(responseData.Errors) < 1 {
		t.Errorf("expected errors in %+v", responseData)
	}

	if responseData.Errors[0].Message != "Something went wrong" {
		t.Errorf("expected %+v", responseData)
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

	client := NewClient(srv.URL, "token", logger.NewLogger(false))

	req, err := client.NewUnauthorizedRequest("query {}")
	if err != nil {
		t.Errorf(err.Error())
	}
	req.Header.Set("X-Custom-Header", "123")

	var resp struct {
		Data struct {
			Value string
		}
	}
	err = client.Run(ctx, req, &resp)
	if err != nil {
		t.Errorf(err.Error())
	}

	if calls != 1 {
		t.Errorf("expected %s", string(calls))
	}

	if resp.Data.Value != "some data" {
		t.Errorf("expected %+v", resp)
	}
}
