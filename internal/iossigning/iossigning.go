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

// Package iossigning provides helpers for the iOS code signing commands —
// reading certificate / provisioning-profile files from disk and base64
// encoding them for transmission to the CircleCI API.
package iossigning

import (
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
)

// EncodeFile reads the file at path and returns its base name and the
// base64-encoded contents. An empty file is rejected as an explicit error
// so callers don't accidentally upload zero-byte assets.
func EncodeFile(path string) (name, blob string, err error) {
	data, err := os.ReadFile(path) //#nosec:G304 // path is an explicit user-supplied flag (--cert-file or --profile), not attacker-controlled input
	if err != nil {
		return "", "", fmt.Errorf("read %s: %w", path, err)
	}
	if len(data) == 0 {
		return "", "", fmt.Errorf("file is empty: %s", path)
	}
	return filepath.Base(path), base64.StdEncoding.EncodeToString(data), nil
}
