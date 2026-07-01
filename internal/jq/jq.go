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

// errHalt is a sentinel signalling that a jq program invoked halt (a HaltError
// with a nil value). It stops output without being treated as a failure.
var errHalt = errors.New("jq: halt")

// Error wraps a failure that originates in the jq expression itself — a parse
// error, a compile error, or a runtime evaluation error (e.g. building an
// object with a non-string key). Callers use errors.As to distinguish a bad
// --jq expression from input/data errors (a malformed value in the stream) so
// they can report it as an invalid argument rather than an API or I/O failure.
type Error struct {
	Expr string // the offending jq expression
	Err  error  // the underlying parse/compile/eval error
}

func (e *Error) Error() string { return e.Err.Error() }
func (e *Error) Unwrap() error { return e.Err }

// Evaluate runs a jq expression against the single JSON value in r and writes
// the results to w. Scalar results are written as raw strings (no quotes);
// objects and arrays are marshalled as JSON, optionally with indentation and
// color.
//
// Use EvaluateStream when r holds a stream of values that the expression should
// aggregate across (via jq's input/inputs).
func Evaluate(r io.Reader, w io.Writer, opts Options) error {
	code, err := compile(opts.Expr)
	if err != nil {
		return err
	}

	b, err := io.ReadAll(r)
	if err != nil {
		return err
	}

	var v any
	if err := json.Unmarshal(b, &v); err != nil {
		return err
	}

	if err := writeResults(code.Run(v), w, opts); err != nil && !errors.Is(err, errHalt) {
		return &Error{Expr: opts.Expr, Err: err}
	}
	return nil
}

// EvaluateStream runs a jq expression against a stream of JSON values read from
// r (values may be whitespace- or newline-separated, as produced by a JSONL
// writer). Output formatting matches Evaluate.
//
// The expression is evaluated with jq's standard multi-input semantics: it runs
// once per input value (so a simple filter like `.name` prints one result per
// record), and the input/inputs builtins pull the remaining values from the
// same stream. This lets expressions aggregate across records, e.g.
// `[.,inputs] | length` or `reduce inputs as $x (0; . + $x.run_time)`.
func EvaluateStream(r io.Reader, w io.Writer, opts Options) error {
	inputs := &streamIter{dec: json.NewDecoder(r)}
	code, err := compile(opts.Expr, gojq.WithInputIter(inputs))
	if err != nil {
		return err
	}

	for {
		v, ok := inputs.Next()
		if !ok {
			return nil
		}
		if err, isErr := v.(error); isErr {
			return err // a malformed value in the stream (a data error, not the expr)
		}
		if err := writeResults(code.Run(v), w, opts); err != nil {
			if errors.Is(err, errHalt) {
				return nil // halt stops the whole stream, not just this input
			}
			return &Error{Expr: opts.Expr, Err: err}
		}
	}
}

// compile parses and compiles a jq expression, wrapping parse errors with the
// offending source line and column. extra compiler options are appended to the
// standard environ loader.
func compile(expr string, extra ...gojq.CompilerOption) (*gojq.Code, error) {
	query, err := gojq.Parse(expr)
	if err != nil {
		if e, ok := errors.AsType[*gojq.ParseError](err); ok {
			str, line, column := lineColumn(expr, e.Offset-len(e.Token))
			return nil, &Error{Expr: expr, Err: fmt.Errorf(
				"failed to parse jq expression at line %d, column %d:\n    %s\n    %*c  %w",
				line, column, str, column, '^', err,
			)}
		}
		return nil, &Error{Expr: expr, Err: err}
	}

	opts := append([]gojq.CompilerOption{gojq.WithEnvironLoader(os.Environ)}, extra...)
	code, err := gojq.Compile(query, opts...)
	if err != nil {
		return nil, &Error{Expr: expr, Err: err}
	}
	return code, nil
}

// writeResults drains a single jq run, formatting each produced value to w. It
// returns errHalt when the program called halt so callers can stop cleanly.
func writeResults(iter gojq.Iter, w io.Writer, opts Options) error {
	for {
		v, ok := iter.Next()
		if !ok {
			return nil
		}

		if err, ok := v.(error); ok {
			if e, ok := errors.AsType[*gojq.HaltError](err); ok && e.Value() == nil {
				return errHalt
			}
			return err
		}

		if text, ok := scalarString(v); ok {
			if _, err := fmt.Fprintln(w, text); err != nil {
				return err
			}
			continue
		}

		if err := writeJSON(w, v, opts.Indent, opts.Colorize); err != nil {
			return err
		}
	}
}

// streamIter adapts a json.Decoder to gojq.Iter, decoding one JSON value per
// call. A decode error is surfaced as the iterator value (gojq's convention for
// in-band errors) after which the iterator is exhausted; io.EOF ends it cleanly.
type streamIter struct {
	dec  *json.Decoder
	done bool
}

func (it *streamIter) Next() (any, bool) {
	if it.done {
		return nil, false
	}
	var v any
	if err := it.dec.Decode(&v); err != nil {
		it.done = true
		if err == io.EOF {
			return nil, false
		}
		return err, true
	}
	return v, true
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
