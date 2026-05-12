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

package iossigning_test

import (
	"encoding/base64"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gotest.tools/v3/assert"
	"gotest.tools/v3/assert/cmp"

	"github.com/CircleCI-Public/circleci-cli-v2/internal/iossigning"
)

func TestEncodeFile_ReturnsBasenameAndBase64(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "Distribution.p12")
	content := []byte{0x30, 0x82, 0x04, 0x21, 0x02, 0x01, 0x03} // fake p12 header bytes
	assert.NilError(t, os.WriteFile(path, content, 0o600))

	name, blob, err := iossigning.EncodeFile(path)
	assert.NilError(t, err)
	assert.Equal(t, name, "Distribution.p12")
	assert.Equal(t, blob, base64.StdEncoding.EncodeToString(content))
}

func TestEncodeFile_NestedPathReturnsBasenameOnly(t *testing.T) {
	dir := t.TempDir()
	nested := filepath.Join(dir, "subdir")
	assert.NilError(t, os.MkdirAll(nested, 0o700))
	path := filepath.Join(nested, "MyApp.mobileprovision")
	assert.NilError(t, os.WriteFile(path, []byte("profile"), 0o600))

	name, _, err := iossigning.EncodeFile(path)
	assert.NilError(t, err)
	assert.Equal(t, name, "MyApp.mobileprovision")
}

func TestEncodeFile_EmptyFileIsRejected(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "Empty.p12")
	assert.NilError(t, os.WriteFile(path, []byte{}, 0o600))

	_, _, err := iossigning.EncodeFile(path)
	assert.ErrorContains(t, err, "file is empty")
}

func TestEncodeFile_MissingFileErrors(t *testing.T) {
	_, _, err := iossigning.EncodeFile(filepath.Join(t.TempDir(), "does-not-exist.p12"))
	assert.Assert(t, err != nil)
	assert.Assert(t, strings.Contains(err.Error(), "no such file") || strings.Contains(err.Error(), "cannot find"))
}

func TestEncodeFile_RoundTrip(t *testing.T) {
	original := []byte("provisioning profile contents\nspread across lines\n")
	dir := t.TempDir()
	path := filepath.Join(dir, "profile.mobileprovision")
	assert.NilError(t, os.WriteFile(path, original, 0o600))

	_, blob, err := iossigning.EncodeFile(path)
	assert.NilError(t, err)

	decoded, err := base64.StdEncoding.DecodeString(blob)
	assert.NilError(t, err)
	assert.Check(t, cmp.DeepEqual(decoded, original))
}
