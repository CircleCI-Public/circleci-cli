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

package apiclient

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/CircleCI-Public/circleci-cli/internal/httpcl"
)

// Error is the common error envelope returned by v3 API endpoints:
//
//	{"error": {"id": "...", "title": "...", "detail": "...", "source": {...}}}
type Error struct {
	ID     string      `json:"id"`
	Title  string      `json:"title"`
	Detail string      `json:"detail"`
	Source ErrorSource `json:"source"`
}

// ErrorSource locates the cause of an Error within the request,
// e.g. a JSON pointer to an invalid field.
type ErrorSource struct {
	Error   string `json:"error"`
	Offset  int    `json:"offset"`
	Pointer string `json:"pointer"`
}

func (e *Error) Error() string {
	if e.Detail == "" {
		return e.Title
	}
	if e.Title == "" {
		return e.Detail
	}
	return e.Title + ": " + e.Detail
}

// Message renders the error for human display: "title: detail" on the first
// line, followed by the source location and error id when present.
func (e *Error) Message() string {
	var b strings.Builder
	b.WriteString(e.Error())
	if e.Source.Pointer != "" {
		_, _ = fmt.Fprintf(&b, "\n  at %s", e.Source.Pointer)
		if e.Source.Offset > 0 {
			_, _ = fmt.Fprintf(&b, " (offset %d)", e.Source.Offset)
		}
	}
	if e.Source.Error != "" {
		_, _ = fmt.Fprintf(&b, "\n  %s", e.Source.Error)
	}
	if e.ID != "" {
		_, _ = fmt.Fprintf(&b, "\nerror id: %s", e.ID)
	}
	return b.String()
}

// ParseError extracts the v3 error envelope from the response body of an
// *httpcl.HTTPError. It returns false when err is not an HTTP error or the
// body does not carry the envelope (e.g. v1/v2 endpoints, HTML error pages).
func ParseError(err error) (*Error, bool) {
	he, ok := errors.AsType[*httpcl.HTTPError](err)
	if !ok || len(he.Body) == 0 {
		return nil, false
	}
	var env struct {
		Error *Error `json:"error"`
	}
	if json.Unmarshal(he.Body, &env) != nil || env.Error == nil {
		return nil, false
	}
	if env.Error.Title == "" && env.Error.Detail == "" {
		return nil, false
	}
	return env.Error, true
}
