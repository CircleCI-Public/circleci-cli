package rest

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/CircleCI-Public/circleci-cli/settings"
	"github.com/CircleCI-Public/circleci-cli/version"
)

func TestClient_DoRequest(t *testing.T) {
	t.Run("PUT with req and resp", func(t *testing.T) {
		fix := &fixture{}
		c, cleanup := fix.Run(http.StatusCreated, `{"key": "value"}`)
		defer cleanup()

		t.Run("Check result", func(t *testing.T) {
			r, err := c.NewRequest("PUT", &url.URL{Path: "my/endpoint"}, struct {
				A string
				B int
			}{
				A: "aaa",
				B: 123,
			})
			assert.Nil(t, err)

			resp := make(map[string]interface{})
			statusCode, err := c.DoRequest(r, &resp)
			assert.Nil(t, err)
			assert.Equal(t, statusCode, http.StatusCreated)
			assert.Equal(t, resp, map[string]interface{}{
				"key": "value",
			})
		})

		t.Run("Check request", func(t *testing.T) {
			assert.Equal(t, fix.URL(), url.URL{Path: "/api/v2/my/endpoint"})
			assert.Equal(t, fix.Method(), "PUT")
			assert.Equal(t, fix.Header(), http.Header{
				"Accept-Encoding": {"gzip"},
				"Accept":          {"application/json"},
				"Circle-Token":    {"fake-token"},
				"Content-Length":  {"20"},
				"Content-Type":    {"application/json"},
				"User-Agent":      {version.UserAgent()},
			})
			assert.Equal(t, fix.Body(), `{"A":"aaa","B":123}`+"\n")
		})
	})

	t.Run("GET with error status", func(t *testing.T) {
		fix := &fixture{}
		c, cleanup := fix.Run(http.StatusBadRequest, `{"message": "the error message"}`)
		defer cleanup()

		t.Run("Check result", func(t *testing.T) {
			r, err := c.NewRequest(http.MethodGet, &url.URL{Path: "my/error/endpoint"}, nil)
			assert.Nil(t, err)

			resp := make(map[string]interface{})
			statusCode, err := c.DoRequest(r, &resp)
			assert.Error(t, err, "the error message")
			assert.Equal(t, statusCode, http.StatusBadRequest)
			assert.Equal(t, resp, map[string]interface{}{})
		})

		t.Run("Check request", func(t *testing.T) {
			assert.Equal(t, fix.URL(), url.URL{Path: "/api/v2/my/error/endpoint"})
			assert.Equal(t, fix.Method(), http.MethodGet)
			assert.Equal(t, fix.Header(), http.Header{
				"Accept-Encoding": {"gzip"},
				"Accept":          {"application/json"},
				"Circle-Token":    {"fake-token"},
				"User-Agent":      {version.UserAgent()},
			})
			assert.Equal(t, fix.Body(), "")
		})
	})

	t.Run("GET with resp only", func(t *testing.T) {
		fix := &fixture{}
		c, cleanup := fix.Run(http.StatusCreated, `{"a": "abc", "b": true}`)
		defer cleanup()

		t.Run("Check result", func(t *testing.T) {
			r, err := c.NewRequest(http.MethodGet, &url.URL{Path: "path"}, nil)
			assert.Nil(t, err)

			resp := make(map[string]interface{})
			statusCode, err := c.DoRequest(r, &resp)
			assert.Nil(t, err)
			assert.Equal(t, statusCode, http.StatusCreated)
			assert.Equal(t, resp, map[string]interface{}{
				"a": "abc",
				"b": true,
			})
		})

		t.Run("Check request", func(t *testing.T) {
			assert.Equal(t, fix.URL(), url.URL{Path: "/api/v2/path"})
			assert.Equal(t, fix.Method(), http.MethodGet)
			assert.Equal(t, fix.Header(), http.Header{
				"Accept-Encoding": {"gzip"},
				"Accept":          {"application/json"},
				"Circle-Token":    {"fake-token"},
				"User-Agent":      {version.UserAgent()},
			})
			assert.Equal(t, fix.Body(), "")
		})
	})
}

func TestAPIRequest(t *testing.T) {
	fix := &fixture{}
	c, cleanup := fix.Run(http.StatusCreated, `{"key": "value"}`)
	defer cleanup()

	t.Run("test new api request sets the default headers", func(t *testing.T) {
		req, err := c.NewRequest("GET", &url.URL{}, struct{}{})
		assert.Nil(t, err)
		assert.Equal(t, req.Header.Get("User-Agent"), "circleci-cli/0.0.0-dev+dirty-local-tree (source)")
		assert.Equal(t, req.Header.Get("Circle-Token"), c.circleToken)
		assert.Equal(t, req.Header.Get("Accept"), "application/json")
	})

	type testPayload struct {
		Message string
	}

	t.Run("test new api request sets the default headers", func(t *testing.T) {
		req, err := c.NewRequest("GET", &url.URL{}, testPayload{Message: "hello"})
		assert.Nil(t, err)
		assert.Equal(t, req.Header.Get("Circleci-Cli-Command"), "")
		assert.Equal(t, req.Header.Get("Content-Type"), "application/json")
	})

	t.Run("test new api request doesn't set content-type with empty payload", func(t *testing.T) {
		req, err := c.NewRequest("GET", &url.URL{}, nil)
		assert.Nil(t, err)
		assert.Equal(t, req.Header.Get("Circleci-Cli-Command"), "")
		assert.Equal(t, req.Header.Get("Content-Type"), "")
	})

	type Options struct {
		OwnerID            string                 `json:"owner_id,omitempty"`
		PipelineParameters map[string]interface{} `json:"pipeline_parameters,omitempty"`
		PipelineValues     map[string]string      `json:"pipeline_values,omitempty"`
	}

	type CompileConfigRequest struct {
		ConfigYaml string  `json:"config_yaml"`
		Options    Options `json:"options"`
	}

	t.Run("config compile and validate payloads have expected shape", func(t *testing.T) {
		req, err := c.NewRequest("GET", &url.URL{}, CompileConfigRequest{
			ConfigYaml: "test-config",
			Options: Options{
				OwnerID: "1234",
				PipelineValues: map[string]string{
					"key": "val",
				},
			},
		})
		assert.Nil(t, err)
		assert.Equal(t, req.Header.Get("Circleci-Cli-Command"), "")
		assert.Equal(t, req.Header.Get("Content-Type"), "application/json")

		reqBody, _ := io.ReadAll(req.Body)
		assert.Contains(t, string(reqBody), `"config_yaml":"test-config"`)
		assert.Contains(t, string(reqBody), `"owner_id":"1234"`)
	})
}

type fixture struct {
	mu     sync.Mutex
	url    url.URL
	method string
	header http.Header
	body   bytes.Buffer
}

func (f *fixture) URL() url.URL {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.url
}

func (f *fixture) Method() string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.method
}

func (f *fixture) Header() http.Header {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.header
}

func (f *fixture) Body() string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.body.String()
}

func (f *fixture) Run(statusCode int, respBody string) (c *Client, cleanup func()) {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		defer f.mu.Unlock()

		defer r.Body.Close()
		_, _ = io.Copy(&f.body, r.Body)
		f.url = *r.URL
		f.header = r.Header
		f.method = r.Method

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(statusCode)
		_, _ = io.WriteString(w, respBody)
	})
	server := httptest.NewServer(mux)

	cfg := &settings.Config{
		Debug:        false,
		Token:        "fake-token",
		RestEndpoint: "api/v2",
		Endpoint:     "api/v2",
		HTTPClient:   http.DefaultClient,
	}

	return NewFromConfig(server.URL, cfg), server.Close
}
