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

package cmdutil

import (
	"fmt"
	"strconv"
	"strings"
)

// ParseParams converts ["key=value", ...] into a map, coercing values to bool
// or int64 where unambiguous. "true"/"false" become bool; numeric strings
// become int64; everything else stays a string.
func ParseParams(raw []string) (map[string]any, error) {
	if len(raw) == 0 {
		return nil, nil
	}
	out := make(map[string]any, len(raw))
	for _, p := range raw {
		k, v, found := strings.Cut(p, "=")
		if !found || k == "" {
			return nil, fmt.Errorf("%q is not valid: expected key=value", p)
		}
		switch v {
		case "true":
			out[k] = true
		case "false":
			out[k] = false
		default:
			if n, err := strconv.ParseInt(v, 10, 64); err == nil {
				out[k] = n
			} else {
				out[k] = v
			}
		}
	}
	return out, nil
}
