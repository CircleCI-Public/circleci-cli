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

package httprecorder

import (
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"gotest.tools/v3/assert"
	"gotest.tools/v3/assert/cmp"
)

func TestRequest_Decode(t *testing.T) {
	// language=json
	const body = `{"a": "value-a", "b": "value-b"}`
	req := Request{Body: new(body)}
	m := make(map[string]string)
	err := req.Decode(&m)
	assert.Assert(t, err)
	assert.Check(t, cmp.DeepEqual(m, map[string]string{
		"a": "value-a",
		"b": "value-b",
	}))
}

func TestRequestRecorder_AllRequests(t *testing.T) {
	r := New()

	t.Run("Make request", func(t *testing.T) {
		err := r.Record(newRequest(t, http.MethodGet, "https://hostname/path", "the-body",
			http.Header{
				"a": []string{"value-a"},
			},
		))
		assert.Assert(t, err)
	})

	t.Run("Check all requests", func(t *testing.T) {
		assert.Check(t, cmp.DeepEqual(r.AllRequests(), []Request{
			{
				Method: http.MethodGet,
				URL:    newURL(t, "https://hostname/path"),
				Header: http.Header{
					"a": []string{"value-a"},
				},
				Body: new("the-body"),
			},
		}))
	})
}

func TestRequestRecorder_LastRequest(t *testing.T) {
	r := New()

	t.Run("Make first request", func(t *testing.T) {
		err := r.Record(newRequest(t, http.MethodGet, "https://hostname-a/path-a", "the-body-a",
			http.Header{
				"a": []string{"value-a"},
			},
		))
		assert.Assert(t, err)
	})

	t.Run("Check last request", func(t *testing.T) {
		assert.Check(t, cmp.DeepEqual(r.LastRequest(), &Request{
			Method: http.MethodGet,
			URL:    newURL(t, "https://hostname-a/path-a"),
			Header: http.Header{
				"a": []string{"value-a"},
			},
			Body: new("the-body-a"),
		}))
	})

	t.Run("Make second request", func(t *testing.T) {
		err := r.Record(newRequest(t, http.MethodPost, "https://hostname-b/path-b", "the-body-b",
			http.Header{
				"b": []string{"value-b"},
			},
		))
		assert.Assert(t, err)
	})

	t.Run("Check last request changed", func(t *testing.T) {
		assert.Check(t, cmp.DeepEqual(r.LastRequest(), &Request{
			Method: http.MethodPost,
			URL:    newURL(t, "https://hostname-b/path-b"),
			Header: http.Header{
				"b": []string{"value-b"},
			},
			Body: new("the-body-b"),
		}))
	})
}

func TestRequestRecorder_Reset(t *testing.T) {
	r := New()

	t.Run("Make first request", func(t *testing.T) {
		err := r.Record(newRequest(t, http.MethodGet, "https://hostname/path", "the-body", http.Header{}))
		assert.Assert(t, err)
	})

	t.Run("Check there are requests", func(t *testing.T) {
		assert.Check(t, cmp.Len(r.AllRequests(), 1))
	})

	t.Run("Reset recorder", func(t *testing.T) {
		r.Reset()
	})

	t.Run("Check no requests left", func(t *testing.T) {
		assert.Check(t, cmp.Len(r.AllRequests(), 0))
	})
}

func TestRequestRecorder_FindRequests(t *testing.T) {
	r := New()

	t.Run("Make requests", func(t *testing.T) {
		err := r.Record(newRequest(t, http.MethodGet, "https://hostname-a/path-a", "the-body-a-1",
			http.Header{
				"a-1": []string{"value-a-1"},
			},
		))
		assert.Assert(t, err)

		err = r.Record(newRequest(t, http.MethodGet, "https://hostname-a/path-a", "the-body-a-2",
			http.Header{
				"a-2": []string{"value-a-2"},
			},
		))
		assert.Assert(t, err)

		err = r.Record(newRequest(t, http.MethodPost, "https://hostname-b/path-b", "the-body-b-1",
			http.Header{
				"b-1": []string{"value-b-1"},
			},
		))
		assert.Assert(t, err)

		err = r.Record(newRequest(t, http.MethodPost, "https://hostname-b/path-b", "the-body-b-2",
			http.Header{
				"b-2": []string{"value-b-2"},
			},
		))
		assert.Assert(t, err)

		err = r.Record(newRequest(t, http.MethodPut, "https://hostname-c/path-c", "the-body-c-1",
			http.Header{
				"c-1": []string{"value-c-1"},
			},
		))
		assert.Assert(t, err)

		err = r.Record(newRequest(t, http.MethodPut, "https://hostname-c/path-c", "the-body-c-2",
			http.Header{
				"c-2": []string{"value-c-2"},
			},
		))
		assert.Assert(t, err)
	})

	t.Run("Find first request", func(t *testing.T) {
		assert.Check(t, cmp.DeepEqual(r.FindRequests(http.MethodGet, newURL(t, "https://hostname-a/path-a")), []Request{
			{
				Method: http.MethodGet,
				URL:    newURL(t, "https://hostname-a/path-a"),
				Header: http.Header{
					"a-1": []string{"value-a-1"},
				},
				Body: new("the-body-a-1"),
			},
			{
				Method: http.MethodGet,
				URL:    newURL(t, "https://hostname-a/path-a"),
				Header: http.Header{
					"a-2": []string{"value-a-2"},
				},
				Body: new("the-body-a-2"),
			},
		}))
	})

	t.Run("Find second request", func(t *testing.T) {
		assert.Check(t, cmp.DeepEqual(r.FindRequests(http.MethodPost, newURL(t, "https://hostname-b/path-b")), []Request{
			{
				Method: http.MethodPost,
				URL:    newURL(t, "https://hostname-b/path-b"),
				Header: http.Header{
					"b-1": []string{"value-b-1"},
				},
				Body: new("the-body-b-1"),
			},
			{
				Method: http.MethodPost,
				URL:    newURL(t, "https://hostname-b/path-b"),
				Header: http.Header{
					"b-2": []string{"value-b-2"},
				},
				Body: new("the-body-b-2"),
			},
		}))
	})

	t.Run("No request found with wrong method", func(t *testing.T) {
		assert.Check(t, cmp.Nil(r.FindRequests(http.MethodPost, newURL(t, "https://hostname-a/path-a"))))
		assert.Check(t, cmp.Nil(r.FindRequests(http.MethodPut, newURL(t, "https://hostname-b/path-b"))))
		assert.Check(t, cmp.Nil(r.FindRequests(http.MethodGet, newURL(t, "https://hostname-c/path-c"))))
	})

	t.Run("No request found with wrong URL", func(t *testing.T) {
		assert.Check(t, cmp.Nil(r.FindRequests(http.MethodGet, newURL(t, "https://hostname-a-not/path-a"))))
		assert.Check(t, cmp.Nil(r.FindRequests(http.MethodPost, newURL(t, "https://hostname-b-not/path-b"))))
		assert.Check(t, cmp.Nil(r.FindRequests(http.MethodPut, newURL(t, "https://hostname-c-not/path-c"))))
	})
}

func newRequest(t *testing.T, method, rawurl, body string, h http.Header) *http.Request {
	t.Helper()
	u := newURL(t, rawurl)
	return &http.Request{
		Method: method,
		URL:    &u,
		Header: h,
		Body:   io.NopCloser(strings.NewReader(body)),
	}
}

func newURL(t *testing.T, rawurl string) url.URL {
	t.Helper()
	u, err := url.Parse(rawurl)
	assert.Assert(t, err)
	return *u
}
