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

// Package httprecorder provides a simple way of recording and later retrieving all
// requests that are sent to an HTTP handler.
//
// If you are writing an HTTP client for an external dependency, this is likely what
// you need to test it is sending the right payloads, etc.
package httprecorder

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"slices"
	"sync"
)

type Request struct {
	Method string
	URL    url.URL
	Header http.Header
	Body   *string
}

// Decode decodes the JSON from the request into the supplied pointer
func (r *Request) Decode(x any) error {
	if r.Body == nil {
		return errors.New("nil body")
	}

	return json.Unmarshal([]byte(*r.Body), x)
}

type RequestRecorder struct {
	mu       sync.RWMutex
	requests []Request
}

func New() *RequestRecorder {
	return &RequestRecorder{}
}

// Record stores a copy of the incoming request ensuring the body can still
// be consumed by the caller.
func (r *RequestRecorder) Record(request *http.Request) (err error) {
	req := Request{
		Method: request.Method,
		URL:    *request.URL,
	}

	req.Header = make(http.Header)
	for k, v := range request.Header {
		req.Header[k] = v
	}

	body, err := io.ReadAll(request.Body)
	if err != nil {
		return err
	}
	req.Body = new(string(body))
	request.Body = io.NopCloser(bytes.NewReader(body))

	r.mu.Lock()
	defer r.mu.Unlock()
	r.requests = append(r.requests, req)

	return nil
}

func (r *RequestRecorder) Reset() {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.requests = nil
}

func (r *RequestRecorder) AllRequests() []Request {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return slices.Clone(r.requests)
}

func (r *RequestRecorder) LastRequest() *Request {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if len(r.requests) == 0 {
		return nil
	}
	req := r.requests[len(r.requests)-1]
	return &req
}

func (r *RequestRecorder) FindRequests(method string, u url.URL) []Request {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var requests []Request
	for _, req := range r.requests {
		if req.Method == method && req.URL == u {
			requests = append(requests, req)
		}
	}
	return requests
}
