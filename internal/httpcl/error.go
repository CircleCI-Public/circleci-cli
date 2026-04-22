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
	"errors"
	"fmt"
	"net/http"
)

// HTTPError represents a non-2xx HTTP response.
type HTTPError struct {
	Method     string
	Route      string
	StatusCode int
	Body       []byte // raw response body
}

func (e *HTTPError) Error() string {
	return fmt.Sprintf("%s %s: %d %s", e.Method, e.Route, e.StatusCode, http.StatusText(e.StatusCode))
}

// HasStatusCode checks if err is an *HTTPError with any of the given status codes.
func HasStatusCode(err error, codes ...int) bool {
	var he *HTTPError
	if !errors.As(err, &he) {
		return false
	}
	for _, c := range codes {
		if he.StatusCode == c {
			return true
		}
	}
	return false
}
