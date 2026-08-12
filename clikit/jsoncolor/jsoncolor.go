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

// Package jsoncolor writes ANSI-colorized, indented JSON to an io.Writer.
package jsoncolor

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

const (
	colorDelim  = "1;37" // bold white
	colorKey    = "1;34" // bright blue
	colorNull   = "36"   // cyan
	colorString = "32"   // green
	colorBool   = "33"   // yellow
)

// Write reads JSON from r and writes colorized, indented JSON to w.
// indent is the string used for each level of nesting (e.g. "\t" or "  ").
// ANSI escape codes are always emitted; callers should suppress output to w
// when color is not desired (e.g. when w is not a TTY).
func Write(w io.Writer, r io.Reader, indent string) error {
	dec := json.NewDecoder(r)
	dec.UseNumber()

	var (
		idx   int
		stack []json.Delim
	)

	for {
		t, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		switch tt := t.(type) {
		case json.Delim:
			switch tt {
			case '{', '[':
				stack = append(stack, tt)
				idx = 0
				ansi(w, colorDelim, tt)
				if dec.More() {
					_, _ = fmt.Fprint(w, "\n", strings.Repeat(indent, len(stack)))
				}
				continue
			case '}', ']':
				stack = stack[:len(stack)-1]
				idx = 0
				ansi(w, colorDelim, tt)
			}
		default:
			b, err := marshalJSON(tt)
			if err != nil {
				return err
			}

			isKey := len(stack) > 0 && stack[len(stack)-1] == '{' && idx%2 == 0
			idx++

			var color string
			switch {
			case isKey:
				color = colorKey
			case tt == nil:
				color = colorNull
			default:
				switch tt.(type) {
				case string:
					color = colorString
				case bool:
					color = colorBool
				}
			}

			if color == "" {
				_, _ = w.Write(b)
			} else {
				ansi(w, color, b)
			}

			if isKey {
				ansi(w, colorDelim, ":")
				_, _ = io.WriteString(w, " ")
				continue
			}
		}

		switch {
		case dec.More():
			ansi(w, colorDelim, ",")
			_, _ = io.WriteString(w, "\n")
			_, _ = io.WriteString(w, strings.Repeat(indent, len(stack)))
		case len(stack) > 0:
			_, _ = fmt.Fprint(w, "\n", strings.Repeat(indent, len(stack)-1))
		default:
			_, _ = fmt.Fprint(w, "\n")
		}
	}

	return nil
}

// ansi wraps s in an ANSI SGR escape sequence with the given color code.
func ansi(w io.Writer, color, end any) {
	_, _ = fmt.Fprintf(w, "\x1b[%sm%s\x1b[m", color, end)
}

// marshalJSON encodes v as JSON with HTML escaping disabled.
func marshalJSON(v any) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)

	if err := enc.Encode(v); err != nil {
		return nil, err
	}

	b := buf.Bytes()
	// json.Encoder always appends a trailing newline; strip it.
	if len(b) > 0 && b[len(b)-1] == '\n' {
		return b[:len(b)-1], nil
	}

	return b, nil
}
