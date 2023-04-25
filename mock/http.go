package mock

import (
	"io"
	"net/http"
	"strings"
)

type transport struct {
	f func(*http.Request) (*http.Response, error)
}

func (t *transport) RoundTrip(r *http.Request) (*http.Response, error) {
	return t.f(r)
}

func NewHTTPClient(f func(*http.Request) (*http.Response, error)) *http.Client {
	return &http.Client{
		Transport: &transport{f: f},
	}
}

func NewHTTPResponse(code int, body string) *http.Response {
	return &http.Response{
		StatusCode: code,
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}
