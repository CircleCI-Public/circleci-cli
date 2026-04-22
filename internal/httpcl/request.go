// Copyright (c) 2026 Circle Internet Services, Inc.
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.
//
// SPDX-License-Identifier: MIT

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
