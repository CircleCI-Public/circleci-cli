package rest

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"testing"

	"gotest.tools/v3/assert"
	"gotest.tools/v3/assert/cmp"

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
			assert.NilError(t, err)

			resp := make(map[string]interface{})
			statusCode, err := c.DoRequest(r, &resp)
			assert.NilError(t, err)
			assert.Equal(t, statusCode, http.StatusCreated)
			assert.Check(t, cmp.DeepEqual(resp, map[string]interface{}{
				"key": "value",
			}))
		})

		t.Run("Check request", func(t *testing.T) {
			assert.Check(t, cmp.Equal(fix.URL(), url.URL{Path: "/api/v2/my/endpoint"}))
			assert.Check(t, cmp.Equal(fix.Method(), "PUT"))
			assert.Check(t, cmp.DeepEqual(fix.Header(), http.Header{
				"Accept-Encoding": {"gzip"},
				"Accept-Type":     {"application/json"},
				"Circle-Token":    {"fake-token"},
				"Content-Length":  {"20"},
				"Content-Type":    {"application/json"},
				"User-Agent":      {version.UserAgent()},
			}))
			assert.Check(t, cmp.Equal(fix.Body(), `{"A":"aaa","B":123}`+"\n"))
		})
	})

	t.Run("GET with error status", func(t *testing.T) {
		fix := &fixture{}
		c, cleanup := fix.Run(http.StatusBadRequest, `{"message": "the error message"}`)
		defer cleanup()

		t.Run("Check result", func(t *testing.T) {
			r, err := c.NewRequest("GET", &url.URL{Path: "my/error/endpoint"}, nil)
			assert.NilError(t, err)

			resp := make(map[string]interface{})
			statusCode, err := c.DoRequest(r, &resp)
			assert.Error(t, err, "the error message")
			assert.Equal(t, statusCode, http.StatusBadRequest)
			assert.Check(t, cmp.DeepEqual(resp, map[string]interface{}{}))
		})

		t.Run("Check request", func(t *testing.T) {
			assert.Check(t, cmp.Equal(fix.URL(), url.URL{Path: "/api/v2/my/error/endpoint"}))
			assert.Check(t, cmp.Equal(fix.Method(), "GET"))
			assert.Check(t, cmp.DeepEqual(fix.Header(), http.Header{
				"Accept-Encoding": {"gzip"},
				"Accept-Type":     {"application/json"},
				"Circle-Token":    {"fake-token"},
				"User-Agent":      {version.UserAgent()},
			}))
			assert.Check(t, cmp.Equal(fix.Body(), ""))
		})
	})

	t.Run("GET with resp only", func(t *testing.T) {
		fix := &fixture{}
		c, cleanup := fix.Run(http.StatusCreated, `{"a": "abc", "b": true}`)
		defer cleanup()

		t.Run("Check result", func(t *testing.T) {
			r, err := c.NewRequest("GET", &url.URL{Path: "path"}, nil)
			assert.NilError(t, err)

			resp := make(map[string]interface{})
			statusCode, err := c.DoRequest(r, &resp)
			assert.NilError(t, err)
			assert.Equal(t, statusCode, http.StatusCreated)
			assert.Check(t, cmp.DeepEqual(resp, map[string]interface{}{
				"a": "abc",
				"b": true,
			}))
		})

		t.Run("Check request", func(t *testing.T) {
			assert.Check(t, cmp.Equal(fix.URL(), url.URL{Path: "/api/v2/path"}))
			assert.Check(t, cmp.Equal(fix.Method(), "GET"))
			assert.Check(t, cmp.DeepEqual(fix.Header(), http.Header{
				"Accept-Encoding": {"gzip"},
				"Accept-Type":     {"application/json"},
				"Circle-Token":    {"fake-token"},
				"User-Agent":      {version.UserAgent()},
			}))
			assert.Check(t, cmp.Equal(fix.Body(), ""))
		})
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

	return New(server.URL, "api/v2", "fake-token"), server.Close
}
