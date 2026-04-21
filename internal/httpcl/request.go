package httpcl

import (
	"encoding/json"
	"io"
	"net/http"
	"net/url"
)

// Request is an individual HTTP request to be executed by Client.Call.
// Use NewRequest to create one.
type Request struct {
	method  string
	route   string
	body    any
	decoder func(io.Reader) error
	headers http.Header
	query   url.Values
}

// NewRequest creates a request with functional options.
func NewRequest(method, route string, opts ...func(*Request)) Request {
	r := Request{
		method:  method,
		route:   route,
		headers: http.Header{},
		query:   url.Values{},
	}
	for _, opt := range opts {
		opt(&r)
	}
	return r
}

// Body sets a value that will be JSON-encoded as the request body.
func Body(v any) func(*Request) {
	return func(r *Request) { r.body = v }
}

// JSONDecoder decodes a 2xx response body as JSON into v.
func JSONDecoder(v any) func(*Request) {
	return func(r *Request) {
		r.decoder = func(rd io.Reader) error {
			return json.NewDecoder(rd).Decode(v)
		}
	}
}

// Header sets a single request header.
func Header(key, val string) func(*Request) {
	return func(r *Request) { r.headers.Set(key, val) }
}

// QueryParam sets a single query parameter.
func QueryParam(key, val string) func(*Request) {
	return func(r *Request) { r.query.Set(key, val) }
}
