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

// Package jq facilitates processing of JSON strings using jq expressions.
package jq

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"strconv"
	"strings"

	"github.com/itchyny/gojq"

	"github.com/CircleCI-Public/circleci-cli/internal/jsoncolor"
)

// Options controls how Evaluate formats its output.
type Options struct {
	Expr     string
	Indent   string
	Colorize bool
}

// Evaluate runs a jq expression against the JSON in r and writes the results
// to w. Scalar results are written as raw strings (no quotes); objects and
// arrays are marshalled as JSON, optionally with indentation and color.
func Evaluate(r io.Reader, w io.Writer, opts Options) error {
	query, err := gojq.Parse(opts.Expr)
	if err != nil {
		if e, ok := errors.AsType[*gojq.ParseError](err); ok {
			str, line, column := lineColumn(opts.Expr, e.Offset-len(e.Token))
			return fmt.Errorf(
				"failed to parse jq expression at line %d, column %d:\n    %s\n    %*c  %w",
				line, column, str, column, '^', err,
			)
		}
		return err
	}

	code, err := gojq.Compile(query, gojq.WithEnvironLoader(os.Environ))
	if err != nil {
		return err
	}

	b, err := io.ReadAll(r)
	if err != nil {
		return err
	}

	var v any
	err = json.Unmarshal(b, &v)
	if err != nil {
		return err
	}

	iter := code.Run(v)
	for {
		v, ok := iter.Next()
		if !ok {
			break
		}

		if err, ok := v.(error); ok {
			if e, ok := errors.AsType[*gojq.HaltError](err); ok && e.Value() == nil {
				break
			}
			return err
		}

		if text, ok := scalarString(v); ok {
			_, err = fmt.Fprintln(w, text)
			if err != nil {
				return err
			}
			continue
		}

		err = writeJSON(w, v, opts.Indent, opts.Colorize)
		if err != nil {
			return err
		}
	}

	return nil
}

// scalarString converts a jq scalar value to its raw string representation.
// Returns ("", false) for non-scalar types (objects, arrays).
func scalarString(v any) (string, bool) {
	switch tt := v.(type) {
	case string:
		return tt, true
	case float64:
		if math.Trunc(tt) == tt {
			return strconv.FormatFloat(tt, 'f', 0, 64), true
		}
		return strconv.FormatFloat(tt, 'f', 2, 64), true
	case bool:
		return strconv.FormatBool(tt), true
	case nil:
		return "null", true
	default:
		return "", false
	}
}

// writeJSON marshals v to JSON and writes it to w, followed by a newline.
func writeJSON(w io.Writer, v any, indent string, colorize bool) error {
	var b []byte
	var err error
	if indent == "" {
		b, err = json.Marshal(v)
	} else {
		b, err = json.MarshalIndent(v, "", indent)
	}
	if err != nil {
		return err
	}

	if colorize {
		return jsoncolor.Write(w, bytes.NewReader(b), indent)
	}

	if _, err = w.Write(b); err != nil {
		return err
	}
	_, err = w.Write([]byte{'\n'})
	return err
}

// lineColumn returns the source line containing offset, and its 1-based line
// and column numbers.
func lineColumn(expr string, offset int) (string, int, int) {
	lineNum := 1
	for {
		nl := strings.Index(expr, "\n")
		if nl < 0 || nl >= offset {
			end := nl
			if end < 0 {
				end = len(expr)
			}
			return expr[:end], lineNum, offset + 1
		}
		expr = expr[nl+1:]
		offset -= nl + 1
		lineNum++
	}
}
